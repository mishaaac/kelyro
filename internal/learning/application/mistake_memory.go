package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type MistakeMemoryOption func(*mistakeMemoryService)

func WithMistakeMemoryClock(now func() time.Time) MistakeMemoryOption {
	return func(service *mistakeMemoryService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithMistakeMemoryIDGenerator(generate func(string) (learning.ID, error)) MistakeMemoryOption {
	return func(service *mistakeMemoryService) {
		if generate != nil {
			service.generateID = generate
		}
	}
}

type mistakeMemoryService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
	generateID func(string) (learning.ID, error)
}

func NewMistakeMemoryService(profiles ProfileService, unitOfWork UnitOfWork, options ...MistakeMemoryOption) MistakeMemoryService {
	service := &mistakeMemoryService{profiles: profiles, unitOfWork: unitOfWork, now: time.Now, generateID: randomMistakeMemoryID}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *mistakeMemoryService) Record(ctx context.Context, input RecordMistakeInput) (MistakeRecordResult, error) {
	const operation = "record mistake pattern"
	input.Summary = strings.TrimSpace(input.Summary)
	input.SourceRef = strings.TrimSpace(input.SourceRef)
	student, err := service.student(ctx, operation)
	if err != nil {
		return MistakeRecordResult{}, err
	}
	if err := validateRecordMistakeInput(input); err != nil {
		return MistakeRecordResult{}, invalid(operation, err)
	}

	var result MistakeRecordResult
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		existing, findErr := repositories.Mistakes.FindByKey(ctx, student.ID, input.ConceptID, input.Key)
		switch {
		case findErr == nil:
			if existing.Category != input.Category || existing.Summary != input.Summary {
				return Classify(ErrorConflict, operation, errors.New("mistake key already identifies a different category or summary"))
			}
			updated, observeErr := existing.Observe(input.ObservedAt, input.SourceRef)
			if observeErr != nil {
				return invalid(operation, observeErr)
			}
			event, eventErr := service.event(updated.ID, learning.MistakeObservedEvent, input.ObservedAt, input.SourceRef)
			if eventErr != nil {
				return eventErr
			}
			if updateErr := repositories.Mistakes.Update(ctx, updated); updateErr != nil {
				return updateErr
			}
			if appendErr := repositories.Mistakes.AppendEvent(ctx, event); appendErr != nil {
				return appendErr
			}
			result = MistakeRecordResult{Mistake: updated}
			return nil
		case !errors.Is(findErr, ErrNotFound):
			return findErr
		}

		id, idErr := service.id(operation, "mistake")
		if idErr != nil {
			return idErr
		}
		created, createErr := learning.NewMistake(id, student.ID, input.ConceptID, input.Key, input.Category, input.Summary, input.ObservedAt, input.SourceRef)
		if createErr != nil {
			return invalid(operation, createErr)
		}
		event, eventErr := service.event(created.ID, learning.MistakeObservedEvent, input.ObservedAt, input.SourceRef)
		if eventErr != nil {
			return eventErr
		}
		if createErr := repositories.Mistakes.Create(ctx, created); createErr != nil {
			return createErr
		}
		if appendErr := repositories.Mistakes.AppendEvent(ctx, event); appendErr != nil {
			return appendErr
		}
		result = MistakeRecordResult{Mistake: created, Created: true}
		return nil
	})
	if err != nil {
		return MistakeRecordResult{}, repositoryError(operation, err)
	}
	return result, nil
}

func (service *mistakeMemoryService) List(ctx context.Context) ([]learning.Mistake, error) {
	const operation = "list mistake memory"
	student, err := service.student(ctx, operation)
	if err != nil {
		return nil, err
	}
	var mistakes []learning.Mistake
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		var listErr error
		mistakes, listErr = repositories.Mistakes.ListByStudent(ctx, student.ID)
		return listErr
	})
	return mistakes, repositoryError(operation, err)
}

func (service *mistakeMemoryService) Get(ctx context.Context, id learning.ID) (MistakeView, error) {
	return service.get(ctx, "show mistake memory", id)
}

func (service *mistakeMemoryService) Reinforce(ctx context.Context, id learning.ID, sourceRef string) (MistakeView, error) {
	return service.transition(ctx, "reinforce mistake pattern", id, sourceRef, learning.MistakeReinforcedEvent)
}

func (service *mistakeMemoryService) Resolve(ctx context.Context, id learning.ID, sourceRef string) (MistakeView, error) {
	return service.transition(ctx, "resolve mistake pattern", id, sourceRef, learning.MistakeResolvedEvent)
}

func (service *mistakeMemoryService) transition(ctx context.Context, operation string, id learning.ID, sourceRef string, eventType learning.MistakeEventType) (MistakeView, error) {
	sourceRef = strings.TrimSpace(sourceRef)
	if err := id.Validate(); err != nil {
		return MistakeView{}, invalid(operation, err)
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return MistakeView{}, err
	}
	at, err := service.timestamp(operation)
	if err != nil {
		return MistakeView{}, err
	}
	var view MistakeView
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		mistake, getErr := repositories.Mistakes.Get(ctx, student.ID, id)
		if getErr != nil {
			return getErr
		}
		var updated learning.Mistake
		var transitionErr error
		switch eventType {
		case learning.MistakeReinforcedEvent:
			updated, transitionErr = mistake.Reinforce(at)
		case learning.MistakeResolvedEvent:
			updated, transitionErr = mistake.Resolve(at)
		default:
			transitionErr = errors.New("unsupported mistake transition")
		}
		if transitionErr != nil {
			return invalid(operation, transitionErr)
		}
		event, eventErr := service.event(id, eventType, at, sourceRef)
		if eventErr != nil {
			return eventErr
		}
		if updateErr := repositories.Mistakes.Update(ctx, updated); updateErr != nil {
			return updateErr
		}
		if appendErr := repositories.Mistakes.AppendEvent(ctx, event); appendErr != nil {
			return appendErr
		}
		history, historyErr := repositories.Mistakes.ListEvents(ctx, id)
		if historyErr != nil {
			return historyErr
		}
		view = MistakeView{Mistake: updated, History: history}
		return nil
	})
	if err != nil {
		return MistakeView{}, repositoryError(operation, err)
	}
	return view, nil
}

func (service *mistakeMemoryService) get(ctx context.Context, operation string, id learning.ID) (MistakeView, error) {
	if err := id.Validate(); err != nil {
		return MistakeView{}, invalid(operation, err)
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return MistakeView{}, err
	}
	var view MistakeView
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		mistake, getErr := repositories.Mistakes.Get(ctx, student.ID, id)
		if getErr != nil {
			return getErr
		}
		history, historyErr := repositories.Mistakes.ListEvents(ctx, id)
		if historyErr != nil {
			return historyErr
		}
		view = MistakeView{Mistake: mistake, History: history}
		return nil
	})
	if err != nil {
		return MistakeView{}, repositoryError(operation, err)
	}
	return view, nil
}

func (service *mistakeMemoryService) student(ctx context.Context, operation string) (learning.Student, error) {
	if service == nil || service.profiles == nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, errors.New("profile service is not configured"))
	}
	return service.profiles.Show(ctx)
}

func (service *mistakeMemoryService) withRepositories(ctx context.Context, operation string, work func(Repositories) error) error {
	if service.unitOfWork == nil {
		return Classify(ErrorUnavailable, operation, errors.New("unit of work is not configured"))
	}
	return service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		if repositories.Mistakes == nil {
			return Classify(ErrorUnavailable, operation, errors.New("mistake repository is not configured"))
		}
		return work(repositories)
	})
}

func (service *mistakeMemoryService) timestamp(operation string) (learning.Timestamp, error) {
	if service.now == nil {
		return learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("clock is not configured"))
	}
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Timestamp{}, invalid(operation, err)
	}
	return timestamp, nil
}

func (service *mistakeMemoryService) id(operation, kind string) (learning.ID, error) {
	if service.generateID == nil {
		return learning.ID{}, Classify(ErrorUnavailable, operation, errors.New("mistake id generator is not configured"))
	}
	id, err := service.generateID(kind)
	if err != nil {
		return learning.ID{}, Classify(ErrorUnavailable, operation, fmt.Errorf("generate %s id: %w", kind, err))
	}
	if err := id.Validate(); err != nil {
		return learning.ID{}, invalid(operation, err)
	}
	return id, nil
}

func (service *mistakeMemoryService) event(mistakeID learning.ID, eventType learning.MistakeEventType, occurredAt learning.Timestamp, sourceRef string) (learning.MistakeEvent, error) {
	id, err := service.id("record mistake history", "mistake-event")
	if err != nil {
		return learning.MistakeEvent{}, err
	}
	event, err := learning.NewMistakeEvent(id, mistakeID, eventType, occurredAt, sourceRef)
	if err != nil {
		return learning.MistakeEvent{}, invalid("record mistake history", err)
	}
	return event, nil
}

func validateRecordMistakeInput(input RecordMistakeInput) error {
	probeID, _ := learning.NewID("mistake.validation")
	studentID, _ := learning.NewID("student.validation")
	_, err := learning.NewMistake(probeID, studentID, input.ConceptID, input.Key, input.Category, input.Summary, input.ObservedAt, input.SourceRef)
	return err
}

func randomMistakeMemoryID(kind string) (learning.ID, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return learning.ID{}, err
	}
	return learning.NewID(kind + "." + hex.EncodeToString(value))
}
