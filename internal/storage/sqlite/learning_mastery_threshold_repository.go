package sqlite

import (
	"context"
	"database/sql"

	"github.com/mishaaac/kelyro/internal/learning"
)

func (repository learningMasteryThresholdRepository) Get(ctx context.Context, studentID learning.ID) (learning.MasteryThresholdSettings, error) {
	const operation = "get SQLite mastery threshold"
	if err := studentID.Validate(); err != nil {
		return learning.MasteryThresholdSettings{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var policyVersion, updatedValue string
	var studentDefault float64
	var workspaceOverride sql.NullFloat64
	err := repository.executor.QueryRowContext(operationContext, `SELECT policy_version, student_default, workspace_override, updated_at
FROM mastery_threshold_settings WHERE student_id = ?`, studentID.String()).Scan(
		&policyVersion, &studentDefault, &workspaceOverride, &updatedValue)
	if err != nil {
		return learning.MasteryThresholdSettings{}, classifyLearningError(operation, err)
	}
	defaultThreshold, err := learning.NewMasteryThreshold(studentDefault)
	if err != nil {
		return learning.MasteryThresholdSettings{}, corruptLearning(operation, err)
	}
	updatedAt, err := decodeTimestamp(updatedValue)
	if err != nil {
		return learning.MasteryThresholdSettings{}, corruptLearning(operation, err)
	}
	settings := learning.MasteryThresholdSettings{
		StudentID: studentID, PolicyVersion: policyVersion, StudentDefault: defaultThreshold, UpdatedAt: updatedAt,
	}
	if workspaceOverride.Valid {
		threshold, decodeErr := learning.NewMasteryThreshold(workspaceOverride.Float64)
		if decodeErr != nil {
			return learning.MasteryThresholdSettings{}, corruptLearning(operation, decodeErr)
		}
		settings.WorkspaceOverride = &threshold
	}
	if err := settings.Validate(); err != nil {
		return learning.MasteryThresholdSettings{}, corruptLearning(operation, err)
	}
	return settings, nil
}

func (repository learningMasteryThresholdRepository) Save(ctx context.Context, settings learning.MasteryThresholdSettings) error {
	const operation = "save SQLite mastery threshold"
	if err := settings.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var workspaceOverride any
	if settings.WorkspaceOverride != nil {
		workspaceOverride = settings.WorkspaceOverride.Value()
	}
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO mastery_threshold_settings
(student_id, policy_version, student_default, workspace_override, updated_at) VALUES (?, ?, ?, ?, ?)
ON CONFLICT(student_id) DO UPDATE SET policy_version = excluded.policy_version, student_default = excluded.student_default,
workspace_override = excluded.workspace_override, updated_at = excluded.updated_at`, settings.StudentID.String(), settings.PolicyVersion,
		settings.StudentDefault.Value(), workspaceOverride, encodeTimestamp(settings.UpdatedAt))
	return classifyLearningError(operation, err)
}
