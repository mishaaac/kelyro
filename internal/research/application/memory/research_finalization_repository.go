package memory

import (
	"context"
	"sort"

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
	if finalization.Cost != nil {
		cost := *finalization.Cost
		terminal.Cost = &cost
	}
	if finalization.Audit != nil {
		if err := validateFinalizationAuditMemory(repository.store, terminal, *finalization.Audit); err != nil {
			return application.ResearchFinalizationResult{}, err
		}
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
	result := application.ResearchFinalizationResult{Run: cloneRun(terminal), Execution: cloneQueueExecution(execution)}
	if finalization.Audit != nil {
		audit := cloneResearchAudit(*finalization.Audit)
		repository.store.runAudit[terminal.ID] = append(repository.store.runAudit[terminal.ID], audit)
		sort.Slice(repository.store.runAudit[terminal.ID], func(i, j int) bool {
			left, right := repository.store.runAudit[terminal.ID][i], repository.store.runAudit[terminal.ID][j]
			if !left.RecordedAt.Time().Equal(right.RecordedAt.Time()) {
				return left.RecordedAt.Before(right.RecordedAt)
			}
			return left.ID.String() < right.ID.String()
		})
		result.Audit = &audit
	}
	return result, nil
}

func validateFinalizationAuditMemory(store *Store, terminal research.ResearchRun, audit research.ResearchRunAudit) error {
	const operation = "finalize memory research run and queue"
	if audit.RunID != terminal.ID || audit.Outcome != terminal.Status || !audit.StartedAt.Time().Equal(terminal.StartedAt.Time()) ||
		!equalMemoryAuditTimestamp(audit.CompletedAt, terminal.CompletedAt) {
		return invalid(operation, errRelationship("terminal audit lifecycle does not match Research Run"))
	}
	request := store.requests[terminal.RequestID]
	if request.Topic.Technology != audit.TargetTechnology || !equalMemoryAuditVersion(request.TargetVersion, audit.TargetVersion) {
		return invalid(operation, errRelationship("terminal audit target does not match Research Request"))
	}
	for _, item := range audit.Sources {
		source, sourceExists := store.sources[item.SourceID]
		snapshot, snapshotExists := store.snapshots[item.SnapshotID]
		if !sourceExists || !snapshotExists {
			return notFound(operation)
		}
		if snapshot.SourceID != item.SourceID || snapshot.Locator != item.Locator || source.ID != item.SourceID || snapshot.Fetch.ContentHash != item.SnapshotHash {
			return invalid(operation, errRelationship("terminal audit snapshot does not match durable data"))
		}
	}
	for _, stored := range store.runAudit[terminal.ID] {
		if stored.ID == audit.ID || stored.RecordedAt.Time().Equal(audit.RecordedAt.Time()) {
			return conflict(operation)
		}
	}
	return nil
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
