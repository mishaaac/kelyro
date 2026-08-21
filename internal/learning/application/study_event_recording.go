package application

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/learning"
)

func recordStudyEvent(ctx context.Context, repository StudyHistoryRepository, studentID learning.ID, eventType learning.StudyEventType,
	sourceID learning.ID, occurredAt learning.Timestamp, goalID, instanceID, conceptID *learning.ID) error {
	if repository == nil {
		return Classify(ErrorUnavailable, "record study event", fmt.Errorf("study history repository is not configured"))
	}
	eventID, err := learning.NewID("history." + string(eventType) + "." + sourceID.String())
	if err != nil {
		return Classify(ErrorInvalidState, "record study event", err)
	}
	event, err := learning.NewStudyEvent(eventID, studentID, eventType, sourceID, occurredAt, goalID, instanceID, conceptID)
	if err != nil {
		return Classify(ErrorInvalidState, "record study event", err)
	}
	return repository.Record(ctx, event)
}
