package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

const studySessionColumns = `id,student_id,goal_id,curriculum_instance_id,started_at,ended_at,last_activity_at,status,active_duration_ns,activity_count,policy_version,idle_timeout_ns`

func (repository learningStudySessionRepository) Create(ctx context.Context, session learning.StudySession) error {
	const operation = "create SQLite study session"
	if err := session.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO study_session_lifecycle
(id,student_id,goal_id,curriculum_instance_id,started_at,ended_at,last_activity_at,status,active_duration_ns,activity_count,policy_version,idle_timeout_ns)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, studySessionValues(session)...)
	return classifyLearningError(operation, err)
}

func (repository learningStudySessionRepository) Get(ctx context.Context, id learning.ID) (learning.StudySession, error) {
	const operation = "get SQLite study session"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	session, err := scanStudySession(repository.executor.QueryRowContext(operationContext, "SELECT "+studySessionColumns+" FROM study_session_lifecycle WHERE id=?", id.String()))
	if err != nil {
		return learning.StudySession{}, classifyLearningError(operation, err)
	}
	return validateScannedStudySession(operation, session)
}

func (repository learningStudySessionRepository) ActiveByStudent(ctx context.Context, studentID learning.ID) (learning.StudySession, error) {
	const operation = "get active SQLite study session"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	session, err := scanStudySession(repository.executor.QueryRowContext(operationContext, "SELECT "+studySessionColumns+" FROM study_session_lifecycle WHERE student_id=? AND status='active'", studentID.String()))
	if err != nil {
		return learning.StudySession{}, classifyLearningError(operation, err)
	}
	return validateScannedStudySession(operation, session)
}

func (repository learningStudySessionRepository) ListByGoal(ctx context.Context, studentID, goalID learning.ID) ([]learning.StudySession, error) {
	const operation = "list SQLite study sessions"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, "SELECT "+studySessionColumns+" FROM study_session_lifecycle WHERE student_id=? AND goal_id=? ORDER BY started_at,id", studentID.String(), goalID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	sessions := make([]learning.StudySession, 0)
	for rows.Next() {
		session, scanErr := scanStudySession(rows)
		if scanErr != nil {
			return nil, corruptLearning(operation, scanErr)
		}
		session, scanErr = validateScannedStudySession(operation, session)
		if scanErr != nil {
			return nil, scanErr
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return sessions, nil
}

func (repository learningStudySessionRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.StudySession, error) {
	const operation = "list SQLite study sessions by student"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, "SELECT "+studySessionColumns+" FROM study_session_lifecycle WHERE student_id=? ORDER BY started_at,id", studentID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	sessions := make([]learning.StudySession, 0)
	for rows.Next() {
		session, scanErr := scanStudySession(rows)
		if scanErr != nil {
			return nil, corruptLearning(operation, scanErr)
		}
		session, scanErr = validateScannedStudySession(operation, session)
		if scanErr != nil {
			return nil, scanErr
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return sessions, nil
}

func (repository learningStudySessionRepository) Update(ctx context.Context, session learning.StudySession) error {
	const operation = "update SQLite study session"
	if err := session.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var ended any
	if session.EndedAt != nil {
		ended = encodeTimestamp(*session.EndedAt)
	}
	result, err := repository.executor.ExecContext(operationContext, `UPDATE study_session_lifecycle SET
ended_at=?,last_activity_at=?,status=?,active_duration_ns=?,activity_count=?
WHERE id=? AND student_id=? AND goal_id=? AND curriculum_instance_id=? AND started_at=? AND policy_version=? AND idle_timeout_ns=?`,
		ended, encodeTimestamp(session.LastActivityAt), string(session.Status), int64(session.ActiveDuration), session.ActivityCount,
		session.ID.String(), session.StudentID.String(), session.GoalID.String(), session.CurriculumInstanceID.String(),
		encodeTimestamp(session.StartedAt), session.PolicyVersion, int64(session.IdleTimeout))
	if err != nil {
		return classifyLearningError(operation, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return classifyLearningError(operation, err)
	}
	if changed == 0 {
		return classifyLearningError(operation, sql.ErrNoRows)
	}
	return nil
}

func studySessionValues(session learning.StudySession) []any {
	var ended any
	if session.EndedAt != nil {
		ended = encodeTimestamp(*session.EndedAt)
	}
	return []any{
		session.ID.String(), session.StudentID.String(), session.GoalID.String(), session.CurriculumInstanceID.String(),
		encodeTimestamp(session.StartedAt), ended, encodeTimestamp(session.LastActivityAt), string(session.Status),
		int64(session.ActiveDuration), session.ActivityCount, session.PolicyVersion, int64(session.IdleTimeout),
	}
}

func scanStudySession(scanner rowScanner) (learning.StudySession, error) {
	var idValue, studentValue, goalValue, instanceValue, startedValue, lastActivityValue, statusValue, policyVersion string
	var endedValue sql.NullString
	var activeDuration, idleTimeout int64
	var activityCount int
	if err := scanner.Scan(&idValue, &studentValue, &goalValue, &instanceValue, &startedValue, &endedValue, &lastActivityValue, &statusValue, &activeDuration, &activityCount, &policyVersion, &idleTimeout); err != nil {
		return learning.StudySession{}, err
	}
	ids := make([]learning.ID, 4)
	for index, value := range []string{idValue, studentValue, goalValue, instanceValue} {
		id, err := decodeID(value)
		if err != nil {
			return learning.StudySession{}, err
		}
		ids[index] = id
	}
	startedAt, err := decodeTimestamp(startedValue)
	if err != nil {
		return learning.StudySession{}, err
	}
	lastActivityAt, err := decodeTimestamp(lastActivityValue)
	if err != nil {
		return learning.StudySession{}, err
	}
	var endedAt *learning.Timestamp
	if endedValue.Valid {
		decoded, err := decodeTimestamp(endedValue.String)
		if err != nil {
			return learning.StudySession{}, err
		}
		endedAt = &decoded
	}
	return learning.StudySession{
		ID: ids[0], StudentID: ids[1], GoalID: ids[2], CurriculumInstanceID: ids[3],
		StartedAt: startedAt, EndedAt: endedAt, LastActivityAt: lastActivityAt,
		Status: learning.StudySessionStatus(statusValue), ActiveDuration: time.Duration(activeDuration),
		ActivityCount: activityCount, PolicyVersion: policyVersion, IdleTimeout: time.Duration(idleTimeout),
	}, nil
}

func validateScannedStudySession(operation string, session learning.StudySession) (learning.StudySession, error) {
	if err := session.Validate(); err != nil {
		return learning.StudySession{}, corruptLearning(operation, fmt.Errorf("invalid stored study session: %w", err))
	}
	return session, nil
}
