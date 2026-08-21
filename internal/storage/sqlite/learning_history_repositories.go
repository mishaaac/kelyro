package sqlite

import (
	"context"
	"database/sql"

	"github.com/mishaaac/kelyro/internal/learning"
)

func (repository learningSessionRepository) Append(ctx context.Context, session learning.LearningSession) error {
	const operation = "append SQLite learning session"
	if err := session.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	return repository.atomic(ctx, operation, func(ctx context.Context, target executor) error {
		if _, err := target.ExecContext(ctx, `INSERT INTO study_sessions (id,student_id,goal_id,started_at,ended_at) VALUES (?,?,?,?,?)`, session.ID.String(), session.StudentID.String(), session.GoalID.String(), encodeTimestamp(session.StartedAt), encodeTimestamp(session.EndedAt)); err != nil {
			return err
		}
		for position, activity := range session.Activities {
			if _, err := target.ExecContext(ctx, `INSERT INTO study_activities (id,session_id,activity_type,started_at,ended_at,position) VALUES (?,?,?,?,?,?)`, activity.ID.String(), session.ID.String(), activity.Type, encodeTimestamp(activity.StartedAt), encodeTimestamp(activity.EndedAt), position); err != nil {
				return err
			}
			for conceptPosition, conceptID := range activity.ConceptIDs {
				if _, err := target.ExecContext(ctx, `INSERT INTO study_activity_concepts (activity_id,concept_id,position) VALUES (?,?,?)`, activity.ID.String(), conceptID.String(), conceptPosition); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (repository learningSessionRepository) Get(ctx context.Context, id learning.ID) (learning.LearningSession, error) {
	const operation = "get SQLite learning session"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	session, err := scanSessionHeader(repository.executor.QueryRowContext(operationContext, `SELECT id,student_id,goal_id,started_at,ended_at FROM study_sessions WHERE id=?`, id.String()))
	if err != nil {
		return learning.LearningSession{}, classifyLearningError(operation, err)
	}
	session.Activities, err = loadActivities(operationContext, repository.executor, session.ID)
	if err != nil {
		return learning.LearningSession{}, classifyLearningError(operation, err)
	}
	if err := session.Validate(); err != nil {
		return learning.LearningSession{}, corruptLearning(operation, err)
	}
	return session, nil
}

func (repository learningSessionRepository) ListByGoal(ctx context.Context, studentID, goalID learning.ID) ([]learning.LearningSession, error) {
	const operation = "list SQLite learning sessions"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT id,student_id,goal_id,started_at,ended_at FROM study_sessions WHERE student_id=? AND goal_id=? ORDER BY started_at,id`, studentID.String(), goalID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	items := make([]learning.LearningSession, 0)
	for rows.Next() {
		item, err := scanSessionHeader(rows)
		if err != nil {
			_ = rows.Close()
			return nil, corruptLearning(operation, err)
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	for index := range items {
		items[index].Activities, err = loadActivities(operationContext, repository.executor, items[index].ID)
		if err != nil {
			return nil, classifyLearningError(operation, err)
		}
		if err := items[index].Validate(); err != nil {
			return nil, corruptLearning(operation, err)
		}
	}
	return items, nil
}

func scanSessionHeader(scanner rowScanner) (learning.LearningSession, error) {
	var idValue, studentValue, goalValue, startedValue, endedValue string
	if err := scanner.Scan(&idValue, &studentValue, &goalValue, &startedValue, &endedValue); err != nil {
		return learning.LearningSession{}, err
	}
	id, err := decodeID(idValue)
	if err != nil {
		return learning.LearningSession{}, err
	}
	studentID, err := decodeID(studentValue)
	if err != nil {
		return learning.LearningSession{}, err
	}
	goalID, err := decodeID(goalValue)
	if err != nil {
		return learning.LearningSession{}, err
	}
	startedAt, err := decodeTimestamp(startedValue)
	if err != nil {
		return learning.LearningSession{}, err
	}
	endedAt, err := decodeTimestamp(endedValue)
	if err != nil {
		return learning.LearningSession{}, err
	}
	return learning.LearningSession{ID: id, StudentID: studentID, GoalID: goalID, StartedAt: startedAt, EndedAt: endedAt}, nil
}

func loadActivities(ctx context.Context, target executor, sessionID learning.ID) ([]learning.StudyActivity, error) {
	rows, err := target.QueryContext(ctx, `SELECT id,activity_type,started_at,ended_at FROM study_activities WHERE session_id=? ORDER BY position`, sessionID.String())
	if err != nil {
		return nil, err
	}
	items := make([]learning.StudyActivity, 0)
	for rows.Next() {
		var idValue, kind, startedValue, endedValue string
		if err := rows.Scan(&idValue, &kind, &startedValue, &endedValue); err != nil {
			_ = rows.Close()
			return nil, err
		}
		id, err := decodeID(idValue)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		startedAt, err := decodeTimestamp(startedValue)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		endedAt, err := decodeTimestamp(endedValue)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		items = append(items, learning.StudyActivity{ID: id, Type: learning.ActivityType(kind), StartedAt: startedAt, EndedAt: endedAt})
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range items {
		values, err := queryStrings(ctx, target, "SELECT concept_id FROM study_activity_concepts WHERE activity_id=? ORDER BY position", items[index].ID.String())
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			id, err := decodeID(value)
			if err != nil {
				return nil, err
			}
			items[index].ConceptIDs = append(items[index].ConceptIDs, id)
		}
		if err := items[index].Validate(); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (repository learningReviewRepository) GetSchedule(ctx context.Context, studentID, conceptID learning.ID) (learning.ReviewSchedule, error) {
	const operation = "get SQLite review schedule"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var studentValue, conceptValue, dueValue, reviewType, updatedValue, algorithmVersion string
	var introducedValue sql.NullString
	var imported, estimatedMinutes, critical int
	err := repository.executor.QueryRowContext(operationContext, `SELECT student_id,concept_id,introduced_at,due_at,imported,
review_type,estimated_minutes,critical_prerequisite,updated_at,algorithm_version
FROM review_schedule WHERE student_id=? AND concept_id=?`, studentID.String(), conceptID.String()).Scan(
		&studentValue, &conceptValue, &introducedValue, &dueValue, &imported, &reviewType, &estimatedMinutes, &critical, &updatedValue, &algorithmVersion)
	if err != nil {
		return learning.ReviewSchedule{}, classifyLearningError(operation, err)
	}
	student, err := decodeID(studentValue)
	if err != nil {
		return learning.ReviewSchedule{}, corruptLearning(operation, err)
	}
	concept, err := decodeID(conceptValue)
	if err != nil {
		return learning.ReviewSchedule{}, corruptLearning(operation, err)
	}
	introduced, err := decodeOptionalTimestamp(introducedValue)
	if err != nil {
		return learning.ReviewSchedule{}, corruptLearning(operation, err)
	}
	due, err := decodeTimestamp(dueValue)
	if err != nil {
		return learning.ReviewSchedule{}, corruptLearning(operation, err)
	}
	updatedAt, err := decodeTimestamp(updatedValue)
	if err != nil {
		return learning.ReviewSchedule{}, corruptLearning(operation, err)
	}
	schedule := learning.ReviewSchedule{
		StudentID: student, ConceptID: concept, IntroducedAt: introduced, DueAt: due, Imported: imported == 1,
		Type: learning.ReviewType(reviewType), EstimatedMinutes: estimatedMinutes, CriticalPrerequisite: critical == 1,
		UpdatedAt: updatedAt, AlgorithmVersion: algorithmVersion,
	}
	if err := schedule.Validate(); err != nil {
		return learning.ReviewSchedule{}, corruptLearning(operation, err)
	}
	return schedule, nil
}

func (repository learningReviewRepository) SaveSchedule(ctx context.Context, schedule learning.ReviewSchedule) error {
	const operation = "save SQLite review schedule"
	if err := schedule.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO review_schedule
(student_id,concept_id,introduced_at,due_at,imported,review_type,estimated_minutes,critical_prerequisite,updated_at,algorithm_version)
VALUES (?,?,?,?,?,?,?,?,?,?) ON CONFLICT(student_id,concept_id) DO UPDATE SET introduced_at=excluded.introduced_at,
due_at=excluded.due_at,imported=excluded.imported,review_type=excluded.review_type,estimated_minutes=excluded.estimated_minutes,
critical_prerequisite=excluded.critical_prerequisite,updated_at=excluded.updated_at,algorithm_version=excluded.algorithm_version`,
		schedule.StudentID.String(), schedule.ConceptID.String(), encodeOptionalTimestamp(schedule.IntroducedAt), encodeTimestamp(schedule.DueAt),
		boolInt(schedule.Imported), schedule.Type, schedule.EstimatedMinutes, boolInt(schedule.CriticalPrerequisite),
		encodeTimestamp(schedule.UpdatedAt), schedule.AlgorithmVersion)
	return classifyLearningError(operation, err)
}

func (repository learningReviewRepository) CreateItem(ctx context.Context, item learning.ReviewItem) error {
	const operation = "create SQLite review item"
	if err := item.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO review_items
(id,student_id,concept_id,due_at,status,completed_at,review_type,estimated_minutes,critical_prerequisite,
 outcome,score,skipped_at,postponed_at,postpone_count,created_at,scheduler_version)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID.String(), item.StudentID.String(), item.ConceptID.String(),
		encodeTimestamp(item.DueAt), item.Status, encodeOptionalTimestamp(item.CompletedAt), item.Type, item.EstimatedMinutes,
		boolInt(item.CriticalPrerequisite), item.Outcome, encodeOptionalScore(item.Score), encodeOptionalTimestamp(item.SkippedAt),
		encodeOptionalTimestamp(item.PostponedAt), item.PostponeCount, encodeTimestamp(item.CreatedAt), item.AlgorithmVersion)
	return classifyLearningError(operation, err)
}

func (repository learningReviewRepository) GetItem(ctx context.Context, id learning.ID) (learning.ReviewItem, error) {
	const operation = "get SQLite review item"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	item, err := scanReviewItem(repository.executor.QueryRowContext(operationContext, reviewItemSelect+" WHERE id=?", id.String()))
	if err != nil {
		return learning.ReviewItem{}, classifyLearningError(operation, err)
	}
	return item, nil
}

func (repository learningReviewRepository) UpdateItem(ctx context.Context, item learning.ReviewItem) error {
	const operation = "update SQLite review item"
	if err := item.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	result, err := repository.executor.ExecContext(operationContext, `UPDATE review_items SET student_id=?,concept_id=?,due_at=?,status=?,completed_at=?,
review_type=?,estimated_minutes=?,critical_prerequisite=?,outcome=?,score=?,skipped_at=?,postponed_at=?,postpone_count=?,created_at=?,scheduler_version=?
WHERE id=?`, item.StudentID.String(), item.ConceptID.String(), encodeTimestamp(item.DueAt), item.Status,
		encodeOptionalTimestamp(item.CompletedAt), item.Type, item.EstimatedMinutes, boolInt(item.CriticalPrerequisite), item.Outcome,
		encodeOptionalScore(item.Score), encodeOptionalTimestamp(item.SkippedAt), encodeOptionalTimestamp(item.PostponedAt),
		item.PostponeCount, encodeTimestamp(item.CreatedAt), item.AlgorithmVersion, item.ID.String())
	if err == nil {
		err = requireAffected(result)
	}
	return classifyLearningError(operation, err)
}

func (repository learningReviewRepository) PendingByConcept(ctx context.Context, studentID, conceptID learning.ID) (learning.ReviewItem, error) {
	const operation = "get SQLite pending review"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	item, err := scanReviewItem(repository.executor.QueryRowContext(operationContext,
		reviewItemSelect+" WHERE student_id=? AND concept_id=? AND status='pending'", studentID.String(), conceptID.String()))
	if err != nil {
		return learning.ReviewItem{}, classifyLearningError(operation, err)
	}
	return item, nil
}

func (repository learningReviewRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.ReviewItem, error) {
	const operation = "list SQLite review items"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext,
		reviewItemSelect+" WHERE student_id=? ORDER BY due_at,id", studentID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return scanReviewItems(operation, rows)
}

func (repository learningReviewRepository) ListDue(ctx context.Context, studentID learning.ID, asOf learning.Timestamp) ([]learning.ReviewItem, error) {
	const operation = "list SQLite due reviews"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext,
		reviewItemSelect+" WHERE student_id=? AND status='pending' AND due_at<=? ORDER BY due_at,id", studentID.String(), encodeTimestamp(asOf))
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return scanReviewItems(operation, rows)
}

const reviewItemSelect = `SELECT id,student_id,concept_id,due_at,status,completed_at,review_type,estimated_minutes,
critical_prerequisite,outcome,score,skipped_at,postponed_at,postpone_count,created_at,scheduler_version FROM review_items`

func scanReviewItems(operation string, rows *sql.Rows) ([]learning.ReviewItem, error) {
	defer rows.Close()
	items := make([]learning.ReviewItem, 0)
	for rows.Next() {
		item, err := scanReviewItem(rows)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return items, nil
}

func scanReviewItem(scanner rowScanner) (learning.ReviewItem, error) {
	var idValue, studentValue, conceptValue, dueValue, status, reviewType, outcome, createdValue, algorithmVersion string
	var completedValue, skippedValue, postponedValue sql.NullString
	var scoreValue sql.NullFloat64
	var estimatedMinutes, critical, postponeCount int
	if err := scanner.Scan(&idValue, &studentValue, &conceptValue, &dueValue, &status, &completedValue,
		&reviewType, &estimatedMinutes, &critical, &outcome, &scoreValue, &skippedValue, &postponedValue,
		&postponeCount, &createdValue, &algorithmVersion); err != nil {
		return learning.ReviewItem{}, err
	}
	id, err := decodeID(idValue)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	studentID, err := decodeID(studentValue)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	conceptID, err := decodeID(conceptValue)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	dueAt, err := decodeTimestamp(dueValue)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	completedAt, err := decodeOptionalTimestamp(completedValue)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	skippedAt, err := decodeOptionalTimestamp(skippedValue)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	postponedAt, err := decodeOptionalTimestamp(postponedValue)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	createdAt, err := decodeTimestamp(createdValue)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	score, err := decodeOptionalScore(scoreValue)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	item := learning.ReviewItem{
		ID: id, StudentID: studentID, ConceptID: conceptID, DueAt: dueAt, Status: learning.ReviewStatus(status),
		CompletedAt: completedAt, Type: learning.ReviewType(reviewType), EstimatedMinutes: estimatedMinutes,
		CriticalPrerequisite: critical == 1, Outcome: learning.ReviewOutcome(outcome), Score: score,
		SkippedAt: skippedAt, PostponedAt: postponedAt, PostponeCount: postponeCount,
		CreatedAt: createdAt, AlgorithmVersion: algorithmVersion,
	}
	return item, item.Validate()
}

func encodeOptionalScore(value *learning.MasteryScore) any {
	if value == nil {
		return nil
	}
	return value.Value()
}

func decodeOptionalScore(value sql.NullFloat64) (*learning.MasteryScore, error) {
	if !value.Valid {
		return nil, nil
	}
	score, err := learning.NewMasteryScore(value.Float64)
	if err != nil {
		return nil, err
	}
	return &score, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
