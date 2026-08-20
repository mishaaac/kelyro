package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mishaaac/kelyro/internal/learning"
)

func (repository learningSetupRepository) Get(ctx context.Context, studentID learning.ID) (learning.LearnerSetup, error) {
	const operation = "get SQLite learner setup"
	if err := studentID.Validate(); err != nil {
		return learning.LearnerSetup{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var status, createdValue, updatedValue string
	var instanceValue, attemptValue, completedValue sql.NullString
	var diagnosticOptIn bool
	err := repository.executor.QueryRowContext(operationContext, `SELECT status, curriculum_instance_id, diagnostic_attempt_id,
diagnostic_opt_in, created_at, updated_at, setup_completed_at FROM learner_setups WHERE student_id = ?`, studentID.String()).Scan(
		&status, &instanceValue, &attemptValue, &diagnosticOptIn, &createdValue, &updatedValue, &completedValue)
	if err != nil {
		return learning.LearnerSetup{}, classifyLearningError(operation, err)
	}
	instanceID, err := decodeOptionalID(instanceValue)
	if err != nil {
		return learning.LearnerSetup{}, corruptLearning(operation, err)
	}
	attemptID, err := decodeOptionalID(attemptValue)
	if err != nil {
		return learning.LearnerSetup{}, corruptLearning(operation, err)
	}
	createdAt, err := decodeTimestamp(createdValue)
	if err != nil {
		return learning.LearnerSetup{}, corruptLearning(operation, err)
	}
	updatedAt, err := decodeTimestamp(updatedValue)
	if err != nil {
		return learning.LearnerSetup{}, corruptLearning(operation, err)
	}
	completedAt, err := decodeOptionalTimestamp(completedValue)
	if err != nil {
		return learning.LearnerSetup{}, corruptLearning(operation, err)
	}
	setup := learning.LearnerSetup{StudentID: studentID, Status: learning.LearnerSetupStatus(status), CurriculumInstanceID: instanceID,
		DiagnosticAttemptID: attemptID, DiagnosticOptIn: diagnosticOptIn, CreatedAt: createdAt, UpdatedAt: updatedAt, SetupCompletedAt: completedAt}
	if err := setup.Validate(); err != nil {
		return learning.LearnerSetup{}, corruptLearning(operation, err)
	}
	return setup, nil
}

func (repository learningSetupRepository) Save(ctx context.Context, setup learning.LearnerSetup) error {
	const operation = "save SQLite learner setup"
	if err := setup.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO learner_setups
(student_id, status, curriculum_instance_id, diagnostic_attempt_id, diagnostic_opt_in, created_at, updated_at, setup_completed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(student_id) DO UPDATE SET status=excluded.status, curriculum_instance_id=excluded.curriculum_instance_id,
diagnostic_attempt_id=excluded.diagnostic_attempt_id, diagnostic_opt_in=excluded.diagnostic_opt_in,
updated_at=excluded.updated_at, setup_completed_at=excluded.setup_completed_at`, setup.StudentID.String(), setup.Status,
		encodeOptionalID(setup.CurriculumInstanceID), encodeOptionalID(setup.DiagnosticAttemptID), setup.DiagnosticOptIn,
		encodeTimestamp(setup.CreatedAt), encodeTimestamp(setup.UpdatedAt), encodeOptionalTimestamp(setup.SetupCompletedAt))
	return classifyLearningError(operation, err)
}

func (repository learningSetupRepository) ResetDevelopment(ctx context.Context, studentID learning.ID) error {
	const operation = "reset SQLite learner setup"
	if err := studentID.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	return repository.atomic(ctx, operation, func(ctx context.Context, target executor) error {
		var instanceValue sql.NullString
		err := target.QueryRowContext(ctx, "SELECT curriculum_instance_id FROM learner_setups WHERE student_id = ?", studentID.String()).Scan(&instanceValue)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		var instanceID *learning.ID
		if err == nil {
			instanceID, err = decodeOptionalID(instanceValue)
			if err != nil {
				return err
			}
		}
		evidenceIDs := make([]string, 0)
		if instanceID != nil {
			rows, queryErr := target.QueryContext(ctx, `SELECT observation.evidence_id
FROM diagnostic_observations AS observation
JOIN diagnostic_attempts AS attempt ON attempt.id = observation.attempt_id
WHERE attempt.student_id = ? AND attempt.curriculum_instance_id = ? ORDER BY observation.evidence_id`, studentID.String(), instanceID.String())
			if queryErr != nil {
				return queryErr
			}
			for rows.Next() {
				var evidenceID string
				if scanErr := rows.Scan(&evidenceID); scanErr != nil {
					_ = rows.Close()
					return scanErr
				}
				evidenceIDs = append(evidenceIDs, evidenceID)
			}
			if rowsErr := rows.Err(); rowsErr != nil {
				_ = rows.Close()
				return rowsErr
			}
			if closeErr := rows.Close(); closeErr != nil {
				return closeErr
			}
		}
		if _, err := target.ExecContext(ctx, "DELETE FROM learner_setups WHERE student_id = ?", studentID.String()); err != nil {
			return err
		}
		if instanceID != nil {
			if _, err := target.ExecContext(ctx, "DELETE FROM diagnostic_attempts WHERE student_id = ? AND curriculum_instance_id = ?", studentID.String(), instanceID.String()); err != nil {
				return err
			}
			for _, evidenceID := range evidenceIDs {
				if _, err := target.ExecContext(ctx, "DELETE FROM learning_evidence WHERE id = ?", evidenceID); err != nil {
					return err
				}
			}
			if _, err := target.ExecContext(ctx, "DELETE FROM learner_curriculum_instances WHERE id = ? AND student_id = ?", instanceID.String(), studentID.String()); err != nil {
				return err
			}
		}
		if _, err := target.ExecContext(ctx, "DELETE FROM onboarding_interviews WHERE student_id = ?", studentID.String()); err != nil {
			return err
		}
		return nil
	})
}

func decodeOptionalID(value sql.NullString) (*learning.ID, error) {
	if !value.Valid {
		return nil, nil
	}
	id, err := decodeID(value.String)
	if err != nil {
		return nil, fmt.Errorf("decode optional id: %w", err)
	}
	return &id, nil
}

func encodeOptionalID(value *learning.ID) any {
	if value == nil {
		return nil
	}
	return value.String()
}
