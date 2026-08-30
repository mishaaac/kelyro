package application

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
	triggerpolicy "github.com/mishaaac/kelyro/internal/research/trigger"
)

type researchTriggerService struct {
	repository ResearchTriggerQueueRepository
}

func NewResearchTriggerService(repository ResearchTriggerQueueRepository) ResearchTriggerService {
	return &researchTriggerService{repository: repository}
}

func (service *researchTriggerService) Evaluate(ctx context.Context, input triggerpolicy.Input) (triggerpolicy.Decision, error) {
	const operation = "evaluate research trigger"
	decision, err := triggerpolicy.EvaluateV1(input)
	if err != nil {
		return triggerpolicy.Decision{}, invalid(operation, err)
	}
	if !decision.ShouldResearch {
		return decision, nil
	}
	if err := requireDependency(operation, "research trigger queue repository", service.repository); err != nil {
		return triggerpolicy.Decision{}, err
	}
	queued, err := service.repository.Enqueue(ctx, *decision.QueueItem)
	if err != nil {
		return triggerpolicy.Decision{}, repositoryError(operation, err)
	}
	decision.QueueItem = &queued
	decision.Triggers = append([]research.ResearchTrigger(nil), queued.Triggers...)
	decision.Priority = queued.Priority
	return decision, nil
}

func (service *researchTriggerService) Get(ctx context.Context, id research.ID) (research.ResearchQueueItem, error) {
	const operation = "get research trigger queue item"
	if err := id.Validate(); err != nil {
		return research.ResearchQueueItem{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "research trigger queue repository", service.repository); err != nil {
		return research.ResearchQueueItem{}, err
	}
	item, err := service.repository.Get(ctx, id)
	return item, repositoryError(operation, err)
}

func (service *researchTriggerService) Queued(ctx context.Context) ([]research.ResearchQueueItem, error) {
	const operation = "list queued research triggers"
	if err := requireDependency(operation, "research trigger queue repository", service.repository); err != nil {
		return nil, err
	}
	items, err := service.repository.ListQueued(ctx)
	return items, repositoryError(operation, err)
}

func (service *researchTriggerService) MarkDispatched(ctx context.Context, id research.ID, at research.Timestamp) (research.ResearchQueueItem, error) {
	return service.transition(ctx, id, at, research.ResearchQueueDispatched)
}

func (service *researchTriggerService) Cancel(ctx context.Context, id research.ID, at research.Timestamp) (research.ResearchQueueItem, error) {
	return service.transition(ctx, id, at, research.ResearchQueueCancelled)
}

func (service *researchTriggerService) transition(ctx context.Context, id research.ID, at research.Timestamp, status research.ResearchQueueStatus) (research.ResearchQueueItem, error) {
	const operation = "transition research trigger queue item"
	if err := id.Validate(); err != nil {
		return research.ResearchQueueItem{}, invalid(operation, err)
	}
	if err := at.Validate(); err != nil {
		return research.ResearchQueueItem{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "research trigger queue repository", service.repository); err != nil {
		return research.ResearchQueueItem{}, err
	}
	item, err := service.repository.Get(ctx, id)
	if err != nil {
		return research.ResearchQueueItem{}, repositoryError(operation, err)
	}
	if item.Status != research.ResearchQueueQueued {
		return research.ResearchQueueItem{}, invalid(operation, fmt.Errorf("research queue item is already %s", item.Status))
	}
	item.Status, item.StatusChangedAt = status, &at
	if err := item.Validate(); err != nil {
		return research.ResearchQueueItem{}, invalid(operation, err)
	}
	if err := service.repository.Update(ctx, item); err != nil {
		return research.ResearchQueueItem{}, repositoryError(operation, err)
	}
	return item, nil
}
