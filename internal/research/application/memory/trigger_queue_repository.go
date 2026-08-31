package memory

import (
	"context"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type researchTriggerQueueRepository struct{ store *Store }

func (repository researchTriggerQueueRepository) Enqueue(ctx context.Context, item research.ResearchQueueItem) (research.ResearchQueueItem, error) {
	const operation = "enqueue memory research trigger"
	if err := contextError(operation, ctx); err != nil {
		return research.ResearchQueueItem{}, err
	}
	if err := item.Validate(); err != nil {
		return research.ResearchQueueItem{}, invalid(operation, err)
	}
	if item.Status != research.ResearchQueueQueued {
		return research.ResearchQueueItem{}, invalid(operation, errRelationship("new research queue item is not queued"))
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.triggerQueue[item.ID]; exists {
		return research.ResearchQueueItem{}, conflict(operation)
	}
	for _, existing := range repository.store.triggerQueue {
		if existing.Status == research.ResearchQueueQueued && existing.DedupeKey == item.DedupeKey {
			return cloneResearchQueueItem(existing), nil
		}
	}
	repository.store.triggerQueue[item.ID] = cloneResearchQueueItem(item)
	return cloneResearchQueueItem(item), nil
}

func (repository researchTriggerQueueRepository) Get(ctx context.Context, id research.ID) (research.ResearchQueueItem, error) {
	const operation = "get memory research trigger"
	if err := contextError(operation, ctx); err != nil {
		return research.ResearchQueueItem{}, err
	}
	if err := id.Validate(); err != nil {
		return research.ResearchQueueItem{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	item, exists := repository.store.triggerQueue[id]
	if !exists {
		return research.ResearchQueueItem{}, notFound(operation)
	}
	return cloneResearchQueueItem(item), nil
}

func (repository researchTriggerQueueRepository) ListQueued(ctx context.Context) ([]research.ResearchQueueItem, error) {
	const operation = "list memory research triggers"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]research.ResearchQueueItem, 0)
	for _, item := range repository.store.triggerQueue {
		if item.Status == research.ResearchQueueQueued {
			if execution, exists := repository.store.queueExecutions[item.ID]; exists && execution.Status == application.ResearchQueueExecutionClaimed {
				continue
			}
			items = append(items, cloneResearchQueueItem(item))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority.Rank() != items[j].Priority.Rank() {
			return items[i].Priority.Rank() < items[j].Priority.Rank()
		}
		if !items[i].QueuedAt.Time().Equal(items[j].QueuedAt.Time()) {
			return items[i].QueuedAt.Before(items[j].QueuedAt)
		}
		return items[i].ID.String() < items[j].ID.String()
	})
	return items, nil
}

func (repository researchTriggerQueueRepository) Update(ctx context.Context, item research.ResearchQueueItem) error {
	const operation = "update memory research trigger"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := item.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	stored, exists := repository.store.triggerQueue[item.ID]
	if !exists {
		return notFound(operation)
	}
	if stored.Status != research.ResearchQueueQueued || item.Status == research.ResearchQueueQueued || !sameResearchQueueIdentity(stored, item) {
		return invalid(operation, errRelationship("research queue transition is invalid"))
	}
	repository.store.triggerQueue[item.ID] = cloneResearchQueueItem(item)
	return nil
}

func (repository researchTriggerQueueRepository) ClaimExecution(ctx context.Context, claim application.ResearchQueueExecutionClaim) (application.ResearchQueueExecutionClaimResult, error) {
	const operation = "claim memory research queue execution"
	if err := contextError(operation, ctx); err != nil {
		return application.ResearchQueueExecutionClaimResult{}, err
	}
	if err := claim.Validate(); err != nil {
		return application.ResearchQueueExecutionClaimResult{}, invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	item, exists := repository.store.triggerQueue[claim.QueueItemID]
	if !exists {
		return application.ResearchQueueExecutionClaimResult{}, notFound(operation)
	}
	existing, hasExecution := repository.store.queueExecutions[claim.QueueItemID]
	if item.Status != research.ResearchQueueQueued || (hasExecution && existing.Status != application.ResearchQueueExecutionRetry) {
		if !hasExecution {
			return application.ResearchQueueExecutionClaimResult{}, invalid(operation, errRelationship("research queue item is not claimable"))
		}
		return application.ResearchQueueExecutionClaimResult{Execution: existing}, nil
	}
	attempts := 1
	if hasExecution {
		attempts = existing.Attempts + 1
	}
	execution := application.ResearchQueueExecution{
		QueueItemID: claim.QueueItemID, RunID: claim.RunID, Status: application.ResearchQueueExecutionClaimed,
		Attempts: attempts, ChangedAt: claim.At, AlgorithmVersion: application.ResearchQueueWorkerV1,
	}
	repository.store.queueExecutions[claim.QueueItemID] = execution
	return application.ResearchQueueExecutionClaimResult{Execution: execution, Acquired: true}, nil
}

func (repository researchTriggerQueueRepository) GetExecution(ctx context.Context, id research.ID) (application.ResearchQueueExecution, error) {
	const operation = "get memory research queue execution"
	if err := contextError(operation, ctx); err != nil {
		return application.ResearchQueueExecution{}, err
	}
	if err := id.Validate(); err != nil {
		return application.ResearchQueueExecution{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	execution, exists := repository.store.queueExecutions[id]
	if !exists {
		return application.ResearchQueueExecution{}, notFound(operation)
	}
	return execution, nil
}

func (repository researchTriggerQueueRepository) UpdateExecution(ctx context.Context, expected application.ResearchQueueExecutionStatus, execution application.ResearchQueueExecution) error {
	const operation = "update memory research queue execution"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := expected.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := execution.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	stored, exists := repository.store.queueExecutions[execution.QueueItemID]
	if !exists {
		return notFound(operation)
	}
	item, exists := repository.store.triggerQueue[execution.QueueItemID]
	if !exists {
		return notFound(operation)
	}
	if stored.Status != expected || stored.RunID != execution.RunID || stored.Attempts != execution.Attempts ||
		execution.ChangedAt.Before(stored.ChangedAt) || item.Status != research.ResearchQueueQueued {
		return invalid(operation, errRelationship("research queue execution transition is invalid"))
	}
	switch execution.Status {
	case application.ResearchQueueExecutionRetry:
		item.StatusChangedAt = nil
	case application.ResearchQueueExecutionCompleted, application.ResearchQueueExecutionFailed:
		item.Status = research.ResearchQueueDispatched
		item.StatusChangedAt = &execution.ChangedAt
	case application.ResearchQueueExecutionCancelled:
		item.Status = research.ResearchQueueCancelled
		item.StatusChangedAt = &execution.ChangedAt
	default:
		return invalid(operation, errRelationship("research queue execution settlement is invalid"))
	}
	if err := item.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.queueExecutions[execution.QueueItemID] = execution
	repository.store.triggerQueue[item.ID] = cloneResearchQueueItem(item)
	return nil
}

func sameResearchQueueIdentity(left, right research.ResearchQueueItem) bool {
	return left.ID == right.ID && sameRequest(left.Request, right.Request) && left.Priority == right.Priority &&
		left.DedupeKey == right.DedupeKey && left.QueuedAt.Time().Equal(right.QueuedAt.Time()) &&
		left.AlgorithmVersion == right.AlgorithmVersion && equalResearchTriggers(left.Triggers, right.Triggers)
}

func equalResearchTriggers(left, right []research.ResearchTrigger) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
