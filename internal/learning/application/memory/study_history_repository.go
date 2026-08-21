package memory

import (
	"context"
	"reflect"
	"sort"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

type studyHistoryRepository struct{ store *Store }

func (repository studyHistoryRepository) Record(ctx context.Context, event learning.StudyEvent) error {
	const operation = "record memory study event"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if existing, exists := repository.store.history[event.ID]; exists {
		if reflect.DeepEqual(existing, event) {
			return nil
		}
		return conflict(operation)
	}
	for _, existing := range repository.store.history {
		if existing.StudentID == event.StudentID && existing.Type == event.Type && existing.SourceID == event.SourceID {
			if reflect.DeepEqual(existing, event) {
				return nil
			}
			return conflict(operation)
		}
	}
	repository.store.history[event.ID] = cloneStudyEvent(event)
	return nil
}

func (repository studyHistoryRepository) Get(ctx context.Context, id learning.ID) (learning.StudyEvent, error) {
	if err := contextError("get memory study event", ctx); err != nil {
		return learning.StudyEvent{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	event, exists := repository.store.history[id]
	if !exists {
		return learning.StudyEvent{}, notFound("get memory study event")
	}
	return cloneStudyEvent(event), nil
}

func (repository studyHistoryRepository) ListByStudent(ctx context.Context, studentID learning.ID, from, to *learning.Timestamp) ([]learning.StudyEvent, error) {
	if err := contextError("list memory study events", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	events := make([]learning.StudyEvent, 0)
	for _, event := range repository.store.history {
		if event.StudentID != studentID || (from != nil && event.OccurredAt.Before(*from)) || (to != nil && !event.OccurredAt.Before(*to)) {
			continue
		}
		events = append(events, cloneStudyEvent(event))
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].OccurredAt == events[j].OccurredAt {
			return events[i].ID.String() > events[j].ID.String()
		}
		return events[i].OccurredAt.After(events[j].OccurredAt)
	})
	return events, nil
}
