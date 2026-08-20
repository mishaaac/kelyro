package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	sqliteDriver "modernc.org/sqlite"
)

type learningRepository struct {
	executor executor
	timeout  time.Duration
}

type learningStudentRepository struct{ learningRepository }
type learningGoalRepository struct{ learningRepository }
type learningOnboardingRepository struct{ learningRepository }
type learningCurriculumRepository struct{ learningRepository }
type learningConceptRepository struct{ learningRepository }
type learningEvidenceRepository struct{ learningRepository }
type learningMistakeRepository struct{ learningRepository }
type learningRetentionRepository struct{ learningRepository }
type learningSessionRepository struct{ learningRepository }
type learningReviewRepository struct{ learningRepository }
type learningStreakRepository struct{ learningRepository }
type learningAchievementRepository struct{ learningRepository }
type learningAnalyticsRepository struct{ learningRepository }
type learningDailyPlanRepository struct{ learningRepository }

// LearningRepositories returns Student Core adapters backed by this database.
func (database *Database) LearningRepositories() application.Repositories {
	return newLearningRepositories(database.sql, database.timeout)
}

func newLearningRepositories(target executor, timeout time.Duration) application.Repositories {
	repository := learningRepository{executor: target, timeout: timeout}
	return application.Repositories{
		Students: learningStudentRepository{repository}, Goals: learningGoalRepository{repository},
		Onboarding: learningOnboardingRepository{repository},
		Curricula:  learningCurriculumRepository{repository}, Concepts: learningConceptRepository{repository},
		Evidence: learningEvidenceRepository{repository}, Mistakes: learningMistakeRepository{repository},
		Retention: learningRetentionRepository{repository}, Sessions: learningSessionRepository{repository},
		Reviews: learningReviewRepository{repository}, Streaks: learningStreakRepository{repository},
		Achievements: learningAchievementRepository{repository}, Analytics: learningAnalyticsRepository{repository},
		DailyPlans: learningDailyPlanRepository{repository},
	}
}

// WithinTransaction implements application.UnitOfWork with a real SQLite
// transaction and exposes no database handle to the application layer.
func (database *Database) WithinTransaction(ctx context.Context, work func(application.Repositories) error) error {
	const operation = "run SQLite learning transaction"
	if work == nil {
		return application.Classify(application.ErrorInvalidState, operation, errors.New("transaction callback is nil"))
	}
	operationContext, cancel := database.operationContext(ctx)
	defer cancel()
	transaction, err := database.sql.BeginTx(operationContext, nil)
	if err != nil {
		return classifyLearningError(operation, err)
	}
	if err := work(newLearningRepositories(transaction, database.timeout)); err != nil {
		if rollbackErr := transaction.Rollback(); rollbackErr != nil {
			return errors.Join(err, classifyLearningError(operation, rollbackErr))
		}
		return err
	}
	if err := transaction.Commit(); err != nil {
		return classifyLearningError(operation, err)
	}
	return nil
}

// SeedCurriculum installs a deterministic, versioned curriculum fixture. It
// is intentionally an infrastructure helper, not a curriculum compiler.
func (database *Database) SeedCurriculum(ctx context.Context, reference learning.CurriculumRef, concepts []learning.Concept, prerequisites []learning.Prerequisite) error {
	const operation = "seed SQLite curriculum"
	if err := reference.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	known := make(map[learning.ID]struct{}, len(concepts))
	topics := make(map[learning.ID]struct{})
	for _, concept := range concepts {
		if err := concept.Validate(); err != nil {
			return invalidLearning(operation, err)
		}
		if _, exists := known[concept.ID]; exists {
			return application.Classify(application.ErrorConflict, operation, fmt.Errorf("duplicate concept %q", concept.ID))
		}
		known[concept.ID] = struct{}{}
		topics[concept.TopicID] = struct{}{}
	}
	for _, prerequisite := range prerequisites {
		if err := prerequisite.Validate(); err != nil {
			return invalidLearning(operation, err)
		}
		if _, exists := known[prerequisite.ConceptID]; !exists {
			return invalidLearning(operation, fmt.Errorf("unknown concept %q", prerequisite.ConceptID))
		}
		if _, exists := known[prerequisite.RequiredConceptID]; !exists {
			return invalidLearning(operation, fmt.Errorf("unknown required concept %q", prerequisite.RequiredConceptID))
		}
	}

	repository := learningRepository{executor: database.sql, timeout: database.timeout}
	return repository.atomic(ctx, operation, func(ctx context.Context, target executor) error {
		if _, err := target.ExecContext(ctx, "INSERT INTO curriculum_instances (id, version) VALUES (?, ?)", reference.ID.String(), reference.Version); err != nil {
			return err
		}
		topicIDs := make([]string, 0, len(topics))
		for topicID := range topics {
			topicIDs = append(topicIDs, topicID.String())
		}
		sort.Strings(topicIDs)
		for position, topicID := range topicIDs {
			if _, err := target.ExecContext(ctx, `INSERT INTO curriculum_nodes
(curriculum_id, curriculum_version, node_id, node_type, title, position)
VALUES (?, ?, ?, 'topic', ?, ?)`, reference.ID.String(), reference.Version, topicID, topicID, position); err != nil {
				return err
			}
		}
		ordered := append([]learning.Concept(nil), concepts...)
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID.String() < ordered[j].ID.String() })
		for position, concept := range ordered {
			if _, err := target.ExecContext(ctx, "INSERT INTO concept_registry (id) VALUES (?) ON CONFLICT(id) DO NOTHING", concept.ID.String()); err != nil {
				return err
			}
			if _, err := target.ExecContext(ctx, `INSERT INTO curriculum_nodes
(curriculum_id, curriculum_version, node_id, node_type, parent_node_id, concept_id, title, position)
VALUES (?, ?, ?, 'concept', ?, ?, ?, ?)`, reference.ID.String(), reference.Version, concept.ID.String(), concept.TopicID.String(), concept.ID.String(), concept.Title, position); err != nil {
				return err
			}
		}
		for _, edge := range prerequisites {
			if _, err := target.ExecContext(ctx, `INSERT INTO curriculum_edges
(curriculum_id, curriculum_version, concept_id, required_concept_id) VALUES (?, ?, ?, ?)`,
				reference.ID.String(), reference.Version, edge.ConceptID.String(), edge.RequiredConceptID.String()); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repository learningStudentRepository) Create(ctx context.Context, student learning.Student) error {
	const operation = "create SQLite student"
	if err := student.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	return repository.atomic(ctx, operation, func(ctx context.Context, target executor) error {
		if _, err := target.ExecContext(ctx, `INSERT INTO students (id, created_at, updated_at) VALUES (?, ?, ?)`,
			student.ID.String(), encodeTimestamp(student.CreatedAt), encodeTimestamp(student.UpdatedAt)); err != nil {
			return err
		}
		return writeStudentProfile(ctx, target, student)
	})
}

func (repository learningStudentRepository) Get(ctx context.Context, id learning.ID) (learning.Student, error) {
	const operation = "get SQLite student"
	if err := id.Validate(); err != nil {
		return learning.Student{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var displayName, experience, preferredLanguage, timezone, createdAt, updatedAt string
	var dailyMinutes, weeklyDaysTarget int
	err := repository.executor.QueryRowContext(operationContext, `SELECT COALESCE(p.preferred_display_name, ''), p.experience,
p.preferred_language, p.daily_minutes, p.weekly_days_target, p.timezone, s.created_at, s.updated_at
FROM students s JOIN student_profiles p ON p.student_id = s.id WHERE s.id = ?`, id.String()).Scan(
		&displayName, &experience, &preferredLanguage, &dailyMinutes, &weeklyDaysTarget, &timezone, &createdAt, &updatedAt)
	if err != nil {
		return learning.Student{}, classifyLearningError(operation, err)
	}
	preferences, err := queryStrings(operationContext, repository.executor,
		"SELECT preference FROM student_preferences WHERE student_id = ? ORDER BY position", id.String())
	if err != nil {
		return learning.Student{}, classifyLearningError(operation, err)
	}
	days, err := queryInts(operationContext, repository.executor,
		"SELECT weekday FROM student_preferred_days WHERE student_id = ? ORDER BY position", id.String())
	if err != nil {
		return learning.Student{}, classifyLearningError(operation, err)
	}
	created, err := decodeTimestamp(createdAt)
	if err != nil {
		return learning.Student{}, corruptLearning(operation, err)
	}
	updated, err := decodeTimestamp(updatedAt)
	if err != nil {
		return learning.Student{}, corruptLearning(operation, err)
	}
	student := learning.Student{ID: id, CreatedAt: created, UpdatedAt: updated,
		Profile: learning.StudentProfile{DisplayName: displayName, Experience: learning.ExperienceLevel(experience),
			PreferredLanguage: preferredLanguage, Timezone: timezone,
			Availability: learning.Availability{DailyMinutes: dailyMinutes, WeeklyDaysTarget: weeklyDaysTarget, PreferredDays: days}}}
	for _, preference := range preferences {
		student.Profile.Preferences = append(student.Profile.Preferences, learning.StudyPreference(preference))
	}
	if err := student.Validate(); err != nil {
		return learning.Student{}, corruptLearning(operation, err)
	}
	return student, nil
}

func (repository learningStudentRepository) Update(ctx context.Context, student learning.Student) error {
	const operation = "update SQLite student"
	if err := student.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	return repository.atomic(ctx, operation, func(ctx context.Context, target executor) error {
		result, err := target.ExecContext(ctx, "UPDATE students SET updated_at = ? WHERE id = ?", encodeTimestamp(student.UpdatedAt), student.ID.String())
		if err != nil {
			return err
		}
		if err := requireAffected(result); err != nil {
			return err
		}
		if _, err := target.ExecContext(ctx, "DELETE FROM student_preferences WHERE student_id = ?", student.ID.String()); err != nil {
			return err
		}
		if _, err := target.ExecContext(ctx, "DELETE FROM student_preferred_days WHERE student_id = ?", student.ID.String()); err != nil {
			return err
		}
		if _, err := target.ExecContext(ctx, `UPDATE student_profiles SET display_name = ?, preferred_display_name = ?, experience = ?,
preferred_language = ?, weekly_minutes = ?, daily_minutes = ?, weekly_days_target = ?, timezone = ? WHERE student_id = ?`,
			legacyDisplayName(student.Profile.DisplayName), optionalDisplayName(student.Profile.DisplayName), student.Profile.Experience,
			student.Profile.PreferredLanguage, student.Profile.Availability.WeeklyMinutes(), student.Profile.Availability.DailyMinutes,
			student.Profile.Availability.WeeklyDaysTarget, student.Profile.Timezone, student.ID.String()); err != nil {
			return err
		}
		return writeStudentCollections(ctx, target, student)
	})
}

func writeStudentProfile(ctx context.Context, target executor, student learning.Student) error {
	if _, err := target.ExecContext(ctx, `INSERT INTO student_profiles
(student_id, display_name, preferred_display_name, experience, preferred_language, weekly_minutes, daily_minutes, weekly_days_target, timezone)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, student.ID.String(), legacyDisplayName(student.Profile.DisplayName), optionalDisplayName(student.Profile.DisplayName),
		student.Profile.Experience, student.Profile.PreferredLanguage, student.Profile.Availability.WeeklyMinutes(),
		student.Profile.Availability.DailyMinutes, student.Profile.Availability.WeeklyDaysTarget, student.Profile.Timezone); err != nil {
		return err
	}
	return writeStudentCollections(ctx, target, student)
}

func legacyDisplayName(displayName string) string {
	if displayName == "" {
		return "Learner"
	}
	return displayName
}

func optionalDisplayName(displayName string) any {
	if displayName == "" {
		return nil
	}
	return displayName
}

func writeStudentCollections(ctx context.Context, target executor, student learning.Student) error {
	for position, preference := range student.Profile.Preferences {
		if _, err := target.ExecContext(ctx, "INSERT INTO student_preferences (student_id, preference, position) VALUES (?, ?, ?)", student.ID.String(), preference, position); err != nil {
			return err
		}
	}
	for position, day := range student.Profile.Availability.PreferredDays {
		if _, err := target.ExecContext(ctx, "INSERT INTO student_preferred_days (student_id, weekday, position) VALUES (?, ?, ?)", student.ID.String(), day, position); err != nil {
			return err
		}
	}
	return nil
}

func (repository learningRepository) atomic(ctx context.Context, operation string, work func(context.Context, executor) error) error {
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	if _, nested := repository.executor.(*sql.Tx); nested {
		return classifyLearningError(operation, work(operationContext, repository.executor))
	}
	database, ok := repository.executor.(*sql.DB)
	if !ok {
		return corruptLearning(operation, errors.New("unsupported SQLite executor"))
	}
	transaction, err := database.BeginTx(operationContext, nil)
	if err != nil {
		return classifyLearningError(operation, err)
	}
	if err := work(operationContext, transaction); err != nil {
		_ = transaction.Rollback()
		return classifyLearningError(operation, err)
	}
	if err := transaction.Commit(); err != nil {
		return classifyLearningError(operation, err)
	}
	return nil
}

func classifyLearningError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if _, classified := application.KindOf(err); classified {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return application.Classify(application.ErrorNotFound, operation, err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return application.Classify(application.ErrorUnavailable, operation, err)
	}
	var driverErr *sqliteDriver.Error
	if errors.As(err, &driverErr) {
		code := driverErr.Code()
		switch code {
		case 1555, 2067: // SQLITE_CONSTRAINT_PRIMARYKEY and SQLITE_CONSTRAINT_UNIQUE.
			return application.Classify(application.ErrorConflict, operation, err)
		}
		switch code & 0xff {
		case 5, 6, 10, 13, 14: // busy, locked, I/O, full, cannot open.
			return application.Classify(application.ErrorUnavailable, operation, err)
		case 19: // Other constraint failures describe invalid aggregate state.
			return application.Classify(application.ErrorInvalidState, operation, err)
		}
	}
	return application.Classify(application.ErrorPersistenceFailure, operation, err)
}

func invalidLearning(operation string, err error) error {
	return application.Classify(application.ErrorInvalidState, operation, err)
}

func corruptLearning(operation string, err error) error {
	return application.Classify(application.ErrorPersistenceFailure, operation, err)
}

func requireAffected(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func encodeTimestamp(value learning.Timestamp) string {
	return value.Time().UTC().Format(timestampFormat)
}

func encodeOptionalTimestamp(value *learning.Timestamp) any {
	if value == nil {
		return nil
	}
	return encodeTimestamp(*value)
}

func decodeTimestamp(value string) (learning.Timestamp, error) {
	parsed, err := time.Parse(timestampFormat, value)
	if err != nil {
		return learning.Timestamp{}, err
	}
	return learning.NewTimestamp(parsed)
}

func decodeOptionalTimestamp(value sql.NullString) (*learning.Timestamp, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := decodeTimestamp(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func decodeID(value string) (learning.ID, error) { return learning.NewID(value) }

func queryStrings(ctx context.Context, target executor, query string, arguments ...any) ([]string, error) {
	rows, err := target.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func queryInts(ctx context.Context, target executor, query string, arguments ...any) ([]int, error) {
	rows, err := target.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]int, 0)
	for rows.Next() {
		var value int
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

var _ application.UnitOfWork = (*Database)(nil)
