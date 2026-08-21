package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

const studyHistoryColumns = `id,student_id,event_type,source_id,occurred_at,goal_id,curriculum_instance_id,concept_id,history_version`

func (repository learningStudyHistoryRepository) Record(ctx context.Context, event learning.StudyEvent) error {
	const operation = "record SQLite study event"
	if err := event.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	result, err := repository.executor.ExecContext(operationContext, `INSERT INTO study_history_events
(id,student_id,event_type,source_id,occurred_at,goal_id,curriculum_instance_id,concept_id,history_version)
VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, studyHistoryValues(event)...)
	if err != nil {
		return classifyLearningError(operation, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return classifyLearningError(operation, err)
	}
	if changed == 1 {
		return nil
	}
	existing, err := scanStudyEvent(repository.executor.QueryRowContext(operationContext, "SELECT "+studyHistoryColumns+" FROM study_history_events WHERE id=? OR (student_id=? AND event_type=? AND source_id=?) LIMIT 1",
		event.ID.String(), event.StudentID.String(), string(event.Type), event.SourceID.String()))
	if err != nil {
		return classifyLearningError(operation, err)
	}
	if reflect.DeepEqual(existing, event) {
		return nil
	}
	return application.Classify(application.ErrorConflict, operation, fmt.Errorf("study event source is already recorded with different content"))
}

func (repository learningStudyHistoryRepository) Get(ctx context.Context, id learning.ID) (learning.StudyEvent, error) {
	const operation = "get SQLite study event"
	if err := id.Validate(); err != nil {
		return learning.StudyEvent{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	event, err := scanStudyEvent(repository.executor.QueryRowContext(operationContext, "SELECT "+studyHistoryColumns+" FROM study_history_events WHERE id=?", id.String()))
	if err != nil {
		return learning.StudyEvent{}, classifyLearningError(operation, err)
	}
	return validateScannedStudyEvent(operation, event)
}

func (repository learningStudyHistoryRepository) ListByStudent(ctx context.Context, studentID learning.ID, from, to *learning.Timestamp) ([]learning.StudyEvent, error) {
	const operation = "list SQLite study events"
	if err := studentID.Validate(); err != nil {
		return nil, invalidLearning(operation, err)
	}
	query := "SELECT " + studyHistoryColumns + " FROM study_history_events WHERE student_id=?"
	arguments := []any{studentID.String()}
	if from != nil {
		if err := from.Validate(); err != nil {
			return nil, invalidLearning(operation, err)
		}
		query += " AND occurred_at>=?"
		arguments = append(arguments, encodeTimestamp(*from))
	}
	if to != nil {
		if err := to.Validate(); err != nil {
			return nil, invalidLearning(operation, err)
		}
		query += " AND occurred_at<?"
		arguments = append(arguments, encodeTimestamp(*to))
	}
	query += " ORDER BY occurred_at DESC,id DESC"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, query, arguments...)
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	events := make([]learning.StudyEvent, 0)
	for rows.Next() {
		event, scanErr := scanStudyEvent(rows)
		if scanErr != nil {
			return nil, corruptLearning(operation, scanErr)
		}
		event, scanErr = validateScannedStudyEvent(operation, event)
		if scanErr != nil {
			return nil, scanErr
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return events, nil
}

func studyHistoryValues(event learning.StudyEvent) []any {
	return []any{event.ID.String(), event.StudentID.String(), string(event.Type), event.SourceID.String(), encodeTimestamp(event.OccurredAt),
		encodeOptionalID(event.GoalID), encodeOptionalID(event.CurriculumInstanceID), encodeOptionalID(event.ConceptID), event.Version}
}

func scanStudyEvent(scanner rowScanner) (learning.StudyEvent, error) {
	var idValue, studentValue, eventType, sourceValue, occurredValue, version string
	var goalValue, instanceValue, conceptValue sql.NullString
	if err := scanner.Scan(&idValue, &studentValue, &eventType, &sourceValue, &occurredValue, &goalValue, &instanceValue, &conceptValue, &version); err != nil {
		return learning.StudyEvent{}, err
	}
	ids := make([]learning.ID, 3)
	for index, value := range []string{idValue, studentValue, sourceValue} {
		id, err := decodeID(value)
		if err != nil {
			return learning.StudyEvent{}, err
		}
		ids[index] = id
	}
	occurredAt, err := decodeTimestamp(occurredValue)
	if err != nil {
		return learning.StudyEvent{}, err
	}
	optional := make([]*learning.ID, 3)
	for index, value := range []sql.NullString{goalValue, instanceValue, conceptValue} {
		if !value.Valid {
			continue
		}
		id, err := decodeID(value.String)
		if err != nil {
			return learning.StudyEvent{}, err
		}
		optional[index] = &id
	}
	return learning.StudyEvent{ID: ids[0], StudentID: ids[1], Type: learning.StudyEventType(eventType), SourceID: ids[2],
		OccurredAt: occurredAt, GoalID: optional[0], CurriculumInstanceID: optional[1], ConceptID: optional[2], Version: version}, nil
}

func validateScannedStudyEvent(operation string, event learning.StudyEvent) (learning.StudyEvent, error) {
	if err := event.Validate(); err != nil {
		return learning.StudyEvent{}, corruptLearning(operation, fmt.Errorf("invalid stored study event: %w", err))
	}
	return event, nil
}
