package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mishaaac/kelyro/internal/learning"
)

func (repository learningOnboardingRepository) Get(ctx context.Context, studentID learning.ID) (learning.OnboardingInterview, error) {
	const operation = "get SQLite onboarding"
	if err := studentID.Validate(); err != nil {
		return learning.OnboardingInterview{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var flowID, flowVersion, status, currentQuestion, answersJSON, createdValue, updatedValue string
	var completedValue, cancelledValue sql.NullString
	err := repository.executor.QueryRowContext(operationContext, `SELECT flow_id, flow_version, status, current_question_id,
answers_json, created_at, updated_at, completed_at, cancelled_at FROM onboarding_interviews WHERE student_id = ?`, studentID.String()).Scan(
		&flowID, &flowVersion, &status, &currentQuestion, &answersJSON, &createdValue, &updatedValue, &completedValue, &cancelledValue)
	if err != nil {
		return learning.OnboardingInterview{}, classifyLearningError(operation, err)
	}
	createdAt, err := decodeTimestamp(createdValue)
	if err != nil {
		return learning.OnboardingInterview{}, corruptLearning(operation, err)
	}
	updatedAt, err := decodeTimestamp(updatedValue)
	if err != nil {
		return learning.OnboardingInterview{}, corruptLearning(operation, err)
	}
	completedAt, err := decodeOptionalTimestamp(completedValue)
	if err != nil {
		return learning.OnboardingInterview{}, corruptLearning(operation, err)
	}
	cancelledAt, err := decodeOptionalTimestamp(cancelledValue)
	if err != nil {
		return learning.OnboardingInterview{}, corruptLearning(operation, err)
	}
	answers := make(map[string]string)
	if err := json.Unmarshal([]byte(answersJSON), &answers); err != nil {
		return learning.OnboardingInterview{}, corruptLearning(operation, fmt.Errorf("decode onboarding answers: %w", err))
	}
	return learning.OnboardingInterview{
		StudentID: studentID, FlowID: flowID, FlowVersion: flowVersion, Status: learning.OnboardingStatus(status),
		CurrentQuestionID: currentQuestion, Answers: answers, CreatedAt: createdAt, UpdatedAt: updatedAt,
		CompletedAt: completedAt, CancelledAt: cancelledAt,
	}, nil
}

func (repository learningOnboardingRepository) Save(ctx context.Context, interview learning.OnboardingInterview) error {
	const operation = "save SQLite onboarding"
	if err := interview.StudentID.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	if interview.FlowID == "" || interview.FlowVersion == "" || !interview.Status.Valid() {
		return invalidLearning(operation, fmt.Errorf("onboarding identity or status is invalid"))
	}
	answers, err := json.Marshal(interview.Answers)
	if err != nil {
		return invalidLearning(operation, fmt.Errorf("encode onboarding answers: %w", err))
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err = repository.executor.ExecContext(operationContext, `INSERT INTO onboarding_interviews
(student_id, flow_id, flow_version, status, current_question_id, answers_json, created_at, updated_at, completed_at, cancelled_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(student_id) DO UPDATE SET flow_id = excluded.flow_id, flow_version = excluded.flow_version,
status = excluded.status, current_question_id = excluded.current_question_id, answers_json = excluded.answers_json,
created_at = excluded.created_at, updated_at = excluded.updated_at, completed_at = excluded.completed_at,
cancelled_at = excluded.cancelled_at`, interview.StudentID.String(), interview.FlowID, interview.FlowVersion, interview.Status,
		interview.CurrentQuestionID, string(answers), encodeTimestamp(interview.CreatedAt), encodeTimestamp(interview.UpdatedAt),
		encodeOptionalTimestamp(interview.CompletedAt), encodeOptionalTimestamp(interview.CancelledAt))
	return classifyLearningError(operation, err)
}
