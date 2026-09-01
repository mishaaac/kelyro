package memory

import (
	"context"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type researchFinalizationRepository struct{ store *Store }

func (repository researchFinalizationRepository) Finalize(ctx context.Context, finalization application.ResearchFinalization) (application.ResearchFinalizationResult, error) {
	const operation = "finalize memory research run and queue"
	if err := contextError(operation, ctx); err != nil {
		return application.ResearchFinalizationResult{}, err
	}
	if err := finalization.Validate(); err != nil {
		return application.ResearchFinalizationResult{}, invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	run, exists := repository.store.runs[finalization.RunID]
	if !exists {
		return application.ResearchFinalizationResult{}, notFound(operation)
	}
	item, exists := repository.store.triggerQueue[finalization.QueueItemID]
	if !exists {
		return application.ResearchFinalizationResult{}, notFound(operation)
	}
	execution, exists := repository.store.queueExecutions[finalization.QueueItemID]
	if !exists {
		return application.ResearchFinalizationResult{}, notFound(operation)
	}
	if execution.Status != application.ResearchQueueExecutionClaimed || execution.RunID != run.ID ||
		execution.Attempts != finalization.Attempts || item.Status != research.ResearchQueueQueued || run.RequestID != item.Request.ID {
		return application.ResearchFinalizationResult{}, invalid(operation, errRelationship("research finalization identity is stale"))
	}
	if finalization.BundleID != nil {
		bundle, found := repository.store.bundles[*finalization.BundleID]
		if !found {
			return application.ResearchFinalizationResult{}, notFound(operation)
		}
		if bundle.RunID != run.ID {
			return application.ResearchFinalizationResult{}, invalid(operation, errRelationship("research finalization bundle belongs to another run"))
		}
	}
	terminal, err := research.TransitionResearchRunV1(run, finalization.RunStatus, finalization.FinalizedAt)
	if err != nil {
		return application.ResearchFinalizationResult{}, invalid(operation, err)
	}
	execution.Status = finalization.ExecutionStatus
	execution.ChangedAt = finalization.FinalizedAt
	execution.FailureKind = finalization.FailureKind
	execution.BundleID = cloneID(finalization.BundleID)
	switch finalization.ExecutionStatus {
	case application.ResearchQueueExecutionRetry:
		item.StatusChangedAt = nil
	case application.ResearchQueueExecutionCompleted, application.ResearchQueueExecutionFailed:
		item.Status = research.ResearchQueueDispatched
		item.StatusChangedAt = &finalization.FinalizedAt
	case application.ResearchQueueExecutionCancelled:
		item.Status = research.ResearchQueueCancelled
		item.StatusChangedAt = &finalization.FinalizedAt
	}
	if err := execution.Validate(); err != nil {
		return application.ResearchFinalizationResult{}, invalid(operation, err)
	}
	if err := item.Validate(); err != nil {
		return application.ResearchFinalizationResult{}, invalid(operation, err)
	}
	repository.store.runs[run.ID] = cloneRun(terminal)
	repository.store.queueExecutions[item.ID] = cloneQueueExecution(execution)
	repository.store.triggerQueue[item.ID] = cloneResearchQueueItem(item)
	return application.ResearchFinalizationResult{Run: cloneRun(terminal), Execution: cloneQueueExecution(execution)}, nil
}

func cloneQueueExecution(execution application.ResearchQueueExecution) application.ResearchQueueExecution {
	clone := execution
	clone.BundleID = cloneID(execution.BundleID)
	return clone
}

func cloneID(id *research.ID) *research.ID {
	if id == nil {
		return nil
	}
	copy := *id
	return &copy
}
