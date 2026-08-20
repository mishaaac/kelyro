package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

func (repository learningDiagnosticRepository) Create(ctx context.Context, attempt learning.DiagnosticAttempt) error {
	const operation = "create SQLite diagnostic"
	if err := attempt.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	if attempt.Status != learning.DiagnosticInProgress || len(attempt.Observations) != 0 {
		return invalidLearning(operation, fmt.Errorf("new diagnostic must be unanswered and in progress"))
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO diagnostic_attempts
(id, student_id, curriculum_instance_id, diagnostic_id, diagnostic_version, definition_fingerprint, status,
 started_at, updated_at, completed_at, skipped_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attempt.ID.String(), attempt.StudentID.String(), attempt.CurriculumInstanceID.String(), attempt.Diagnostic.ID.String(),
		attempt.Diagnostic.Version, attempt.DefinitionFingerprint, attempt.Status, encodeTimestamp(attempt.StartedAt),
		encodeTimestamp(attempt.UpdatedAt), encodeOptionalTimestamp(attempt.CompletedAt), encodeOptionalTimestamp(attempt.SkippedAt))
	return classifyLearningError(operation, err)
}

func (repository learningDiagnosticRepository) Get(ctx context.Context, id learning.ID) (learning.DiagnosticAttempt, error) {
	const operation = "get SQLite diagnostic"
	if err := id.Validate(); err != nil {
		return learning.DiagnosticAttempt{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	attempt, err := getSQLiteDiagnostic(operationContext, repository.executor, id)
	if err != nil {
		return learning.DiagnosticAttempt{}, classifyLearningError(operation, err)
	}
	return attempt, nil
}

func (repository learningDiagnosticRepository) Find(ctx context.Context, studentID, instanceID learning.ID, reference learning.DiagnosticRef) (learning.DiagnosticAttempt, error) {
	const operation = "find SQLite diagnostic"
	if err := studentID.Validate(); err != nil {
		return learning.DiagnosticAttempt{}, invalidLearning(operation, err)
	}
	if err := instanceID.Validate(); err != nil {
		return learning.DiagnosticAttempt{}, invalidLearning(operation, err)
	}
	if err := reference.Validate(); err != nil {
		return learning.DiagnosticAttempt{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var idValue string
	err := repository.executor.QueryRowContext(operationContext, `SELECT id FROM diagnostic_attempts
WHERE student_id = ? AND curriculum_instance_id = ? AND diagnostic_id = ? AND diagnostic_version = ?`,
		studentID.String(), instanceID.String(), reference.ID.String(), reference.Version).Scan(&idValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, classifyLearningError(operation, err)
	}
	id, err := decodeID(idValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, corruptLearning(operation, err)
	}
	attempt, err := getSQLiteDiagnostic(operationContext, repository.executor, id)
	if err != nil {
		return learning.DiagnosticAttempt{}, classifyLearningError(operation, err)
	}
	return attempt, nil
}

func (repository learningDiagnosticRepository) Save(ctx context.Context, attempt learning.DiagnosticAttempt) error {
	const operation = "save SQLite diagnostic"
	if err := attempt.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	return repository.atomic(ctx, operation, func(ctx context.Context, target executor) error {
		current, err := getSQLiteDiagnostic(ctx, target, attempt.ID)
		if err != nil {
			return err
		}
		if current.StudentID != attempt.StudentID || current.CurriculumInstanceID != attempt.CurriculumInstanceID || current.Diagnostic != attempt.Diagnostic || current.DefinitionFingerprint != attempt.DefinitionFingerprint || current.StartedAt != attempt.StartedAt {
			return application.Classify(application.ErrorInvalidState, operation, fmt.Errorf("diagnostic identity is immutable"))
		}
		if current.Status != learning.DiagnosticInProgress {
			return application.Classify(application.ErrorInvalidState, operation, fmt.Errorf("terminal diagnostic is immutable"))
		}
		if len(attempt.Observations) < len(current.Observations) || len(attempt.Observations) > len(current.Observations)+1 {
			return application.Classify(application.ErrorInvalidState, operation, fmt.Errorf("diagnostic observations must append one at a time"))
		}
		for index := range current.Observations {
			if current.Observations[index] != attempt.Observations[index] {
				return application.Classify(application.ErrorInvalidState, operation, fmt.Errorf("diagnostic observations are immutable"))
			}
		}
		if attempt.Status == learning.DiagnosticSkipped && len(attempt.Observations) != 0 {
			return application.Classify(application.ErrorInvalidState, operation, fmt.Errorf("partial diagnostic cannot be skipped"))
		}
		result, err := target.ExecContext(ctx, `UPDATE diagnostic_attempts SET status=?, updated_at=?, completed_at=?, skipped_at=? WHERE id=?`,
			attempt.Status, encodeTimestamp(attempt.UpdatedAt), encodeOptionalTimestamp(attempt.CompletedAt), encodeOptionalTimestamp(attempt.SkippedAt), attempt.ID.String())
		if err != nil {
			return err
		}
		if err := requireAffected(result); err != nil {
			return err
		}
		for position := len(current.Observations); position < len(attempt.Observations); position++ {
			observation := attempt.Observations[position]
			if _, err := target.ExecContext(ctx, `INSERT INTO diagnostic_observations
(attempt_id, item_id, concept_id, evidence_id, score, answered_at, position) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				attempt.ID.String(), observation.ItemID.String(), observation.ConceptID.String(), observation.EvidenceID.String(), observation.Score.Value(), encodeTimestamp(observation.AnsweredAt), position); err != nil {
				return err
			}
		}
		return nil
	})
}

func getSQLiteDiagnostic(ctx context.Context, target executor, id learning.ID) (learning.DiagnosticAttempt, error) {
	var idValue, studentValue, instanceValue, diagnosticValue, diagnosticVersion, fingerprint, status, startedValue, updatedValue string
	var completedValue, skippedValue sql.NullString
	if err := target.QueryRowContext(ctx, `SELECT id, student_id, curriculum_instance_id, diagnostic_id, diagnostic_version,
definition_fingerprint, status, started_at, updated_at, completed_at, skipped_at FROM diagnostic_attempts WHERE id = ?`, id.String()).Scan(
		&idValue, &studentValue, &instanceValue, &diagnosticValue, &diagnosticVersion, &fingerprint, &status, &startedValue, &updatedValue, &completedValue, &skippedValue); err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	decodedID, err := decodeID(idValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	studentID, err := decodeID(studentValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	instanceID, err := decodeID(instanceValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	diagnosticID, err := decodeID(diagnosticValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	startedAt, err := decodeTimestamp(startedValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	updatedAt, err := decodeTimestamp(updatedValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	completedAt, err := decodeOptionalTimestamp(completedValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	skippedAt, err := decodeOptionalTimestamp(skippedValue)
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	rows, err := target.QueryContext(ctx, `SELECT item_id, concept_id, evidence_id, score, answered_at FROM diagnostic_observations WHERE attempt_id = ? ORDER BY position`, id.String())
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	defer rows.Close()
	observations := make([]learning.DiagnosticObservation, 0)
	for rows.Next() {
		var itemValue, conceptValue, evidenceValue, answeredValue string
		var scoreValue float64
		if err := rows.Scan(&itemValue, &conceptValue, &evidenceValue, &scoreValue, &answeredValue); err != nil {
			return learning.DiagnosticAttempt{}, err
		}
		itemID, err := decodeID(itemValue)
		if err != nil {
			return learning.DiagnosticAttempt{}, err
		}
		conceptID, err := decodeID(conceptValue)
		if err != nil {
			return learning.DiagnosticAttempt{}, err
		}
		evidenceID, err := decodeID(evidenceValue)
		if err != nil {
			return learning.DiagnosticAttempt{}, err
		}
		score, err := learning.NewMasteryScore(scoreValue)
		if err != nil {
			return learning.DiagnosticAttempt{}, err
		}
		answeredAt, err := decodeTimestamp(answeredValue)
		if err != nil {
			return learning.DiagnosticAttempt{}, err
		}
		observations = append(observations, learning.DiagnosticObservation{ItemID: itemID, ConceptID: conceptID, EvidenceID: evidenceID, Score: score, AnsweredAt: answeredAt})
	}
	if err := rows.Err(); err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	attempt := learning.DiagnosticAttempt{ID: decodedID, StudentID: studentID, CurriculumInstanceID: instanceID,
		Diagnostic: learning.DiagnosticRef{ID: diagnosticID, Version: diagnosticVersion}, DefinitionFingerprint: fingerprint,
		Status: learning.DiagnosticAttemptStatus(status), Observations: observations, StartedAt: startedAt, UpdatedAt: updatedAt,
		CompletedAt: completedAt, SkippedAt: skippedAt}
	return attempt, attempt.Validate()
}
