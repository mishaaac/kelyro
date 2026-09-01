package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

const ResearchQueueWorkerV1 = "research-queue-worker-v1"

type ResearchQueueExecutionStatus string

const (
	ResearchQueueExecutionClaimed   ResearchQueueExecutionStatus = "claimed"
	ResearchQueueExecutionRetry     ResearchQueueExecutionStatus = "retry"
	ResearchQueueExecutionCompleted ResearchQueueExecutionStatus = "completed"
	ResearchQueueExecutionFailed    ResearchQueueExecutionStatus = "failed"
	ResearchQueueExecutionCancelled ResearchQueueExecutionStatus = "cancelled"
)

func (status ResearchQueueExecutionStatus) Validate() error {
	switch status {
	case ResearchQueueExecutionClaimed, ResearchQueueExecutionRetry, ResearchQueueExecutionCompleted,
		ResearchQueueExecutionFailed, ResearchQueueExecutionCancelled:
		return nil
	default:
		return fmt.Errorf("invalid research queue execution status %q", status)
	}
}

type ResearchQueueExecution struct {
	QueueItemID      research.ID
	RunID            research.ID
	Status           ResearchQueueExecutionStatus
	Attempts         int
	ChangedAt        research.Timestamp
	FailureKind      ErrorKind
	BundleID         *research.ID
	AlgorithmVersion string
}

func (execution ResearchQueueExecution) Validate() error {
	if err := execution.QueueItemID.Validate(); err != nil {
		return fmt.Errorf("research queue execution item: %w", err)
	}
	if err := execution.RunID.Validate(); err != nil {
		return fmt.Errorf("research queue execution run: %w", err)
	}
	if err := execution.Status.Validate(); err != nil {
		return err
	}
	if execution.Attempts < 1 {
		return errors.New("research queue execution attempts must be positive")
	}
	if err := execution.ChangedAt.Validate(); err != nil {
		return err
	}
	if execution.Status == ResearchQueueExecutionRetry || execution.Status == ResearchQueueExecutionFailed {
		if !validResearchQueueFailureKind(execution.FailureKind) {
			return fmt.Errorf("research queue execution failure kind %q is invalid", execution.FailureKind)
		}
	} else if execution.FailureKind != "" {
		return errors.New("non-failed research queue execution has a failure kind")
	}
	if execution.BundleID != nil {
		if err := execution.BundleID.Validate(); err != nil {
			return fmt.Errorf("research queue execution bundle: %w", err)
		}
		if execution.Status != ResearchQueueExecutionCompleted {
			return errors.New("only completed research queue execution may reference a bundle")
		}
	}
	if execution.AlgorithmVersion != ResearchQueueWorkerV1 {
		return fmt.Errorf("research queue execution algorithm must be %q", ResearchQueueWorkerV1)
	}
	return nil
}

type ResearchQueueExecutionClaim struct {
	QueueItemID research.ID
	RunID       research.ID
	At          research.Timestamp
}

func (claim ResearchQueueExecutionClaim) Validate() error {
	if err := claim.QueueItemID.Validate(); err != nil {
		return fmt.Errorf("research queue claim item: %w", err)
	}
	if err := claim.RunID.Validate(); err != nil {
		return fmt.Errorf("research queue claim run: %w", err)
	}
	return claim.At.Validate()
}

type ResearchQueueExecutionClaimResult struct {
	Execution ResearchQueueExecution
	Acquired  bool
}

type ResearchQueueConsumeRequest struct {
	QueueItemID research.ID
	RunID       research.ID
	Mode        ResearchMode
}

func (request ResearchQueueConsumeRequest) Validate() error {
	if err := request.QueueItemID.Validate(); err != nil {
		return fmt.Errorf("consume research queue item: %w", err)
	}
	if err := request.RunID.Validate(); err != nil {
		return fmt.Errorf("consume research run: %w", err)
	}
	return request.Mode.Validate()
}

type ResearchQueueConsumeDisposition string

const (
	ResearchQueueConsumeCompleted ResearchQueueConsumeDisposition = "completed"
	ResearchQueueConsumeRetry     ResearchQueueConsumeDisposition = "retry"
	ResearchQueueConsumeFailed    ResearchQueueConsumeDisposition = "failed"
	ResearchQueueConsumeCancelled ResearchQueueConsumeDisposition = "cancelled"
	ResearchQueueConsumeInFlight  ResearchQueueConsumeDisposition = "in_flight"
)

type ResearchQueueConsumeResult struct {
	QueueItem        research.ResearchQueueItem
	Execution        ResearchQueueExecution
	Orchestration    LiveResearchOrchestrationResult
	Disposition      ResearchQueueConsumeDisposition
	Idempotent       bool
	Audit            *research.ResearchRunAudit
	AlgorithmVersion string
}

type ResearchQueueConsumerDependencies struct {
	Queue         ResearchTriggerService
	Research      ResearchService
	Orchestrator  LiveResearchOrchestrator
	Finalization  ResearchFinalizationService
	TerminalAudit LiveResearchTerminalAuditService
	Clock         Clock
}

type researchQueueConsumer struct {
	dependencies ResearchQueueConsumerDependencies
}

func NewResearchQueueConsumer(dependencies ResearchQueueConsumerDependencies) (ResearchQueueConsumer, error) {
	const operation = "configure research queue consumer"
	for _, dependency := range []struct {
		name  string
		value any
	}{
		{"research trigger service", dependencies.Queue},
		{"research service", dependencies.Research},
		{"live research orchestrator", dependencies.Orchestrator},
		{"research finalization service", dependencies.Finalization},
		{"research terminal audit service", dependencies.TerminalAudit},
		{"clock", dependencies.Clock},
	} {
		if err := requireDependency(operation, dependency.name, dependency.value); err != nil {
			return nil, err
		}
	}
	return &researchQueueConsumer{dependencies: dependencies}, nil
}

func (consumer *researchQueueConsumer) Consume(ctx context.Context, request ResearchQueueConsumeRequest) (ResearchQueueConsumeResult, error) {
	const operation = "consume research queue item"
	if err := request.Validate(); err != nil {
		return ResearchQueueConsumeResult{}, invalid(operation, err)
	}
	if ctx == nil {
		return ResearchQueueConsumeResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return ResearchQueueConsumeResult{}, Classify(ErrorUnavailable, operation, err)
	}
	item, run, err := consumer.loadIdentity(ctx, request)
	if err != nil {
		return ResearchQueueConsumeResult{}, err
	}
	claim, err := consumer.dependencies.Queue.ClaimExecution(ctx, ResearchQueueExecutionClaim{
		QueueItemID: item.ID, RunID: run.ID, At: consumer.dependencies.Clock.Now(),
	})
	if err != nil {
		return ResearchQueueConsumeResult{}, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	result := ResearchQueueConsumeResult{
		QueueItem: item, Execution: claim.Execution, AlgorithmVersion: ResearchQueueWorkerV1,
	}
	if !claim.Acquired {
		return consumer.reconcileExisting(ctx, result, run)
	}
	orchestrated, executeErr := consumer.dependencies.Orchestrator.Execute(ctx, LiveResearchOrchestrationRequest{
		QueueItemID: item.ID, RunID: run.ID, Mode: request.Mode,
	})
	result.Orchestration = orchestrated
	if executeErr == nil {
		if orchestrated.Run.ID != run.ID || orchestrated.Run.Status != research.ResearchRunRunning ||
			orchestrated.Artifacts.Bundle == nil || orchestrated.Artifacts.Bundle.RunID != run.ID {
			executeErr = invalid(operation, errors.New("orchestrator returned success without a matching running run and bundle"))
		}
	}
	if executeErr == nil {
		result.Disposition = ResearchQueueConsumeCompleted
		return consumer.finalize(ctx, result, request.Mode, research.ResearchRunCompleted, ResearchQueueExecutionCompleted, orchestrated.Artifacts.Bundle.ID, "", nil)
	}
	if errors.Is(executeErr, context.Canceled) || errors.Is(executeErr, context.DeadlineExceeded) || ctx.Err() != nil {
		result.Disposition = ResearchQueueConsumeCancelled
		return consumer.finalize(context.WithoutCancel(ctx), result, request.Mode, research.ResearchRunCancelled, ResearchQueueExecutionCancelled, research.ID{}, "", executeErr)
	}
	kind, classified := KindOf(executeErr)
	if classified && researchQueueFailureIsTransient(kind) {
		result.Disposition = ResearchQueueConsumeRetry
		return consumer.finalize(ctx, result, request.Mode, research.ResearchRunFailed, ResearchQueueExecutionRetry, research.ID{}, kind, executeErr)
	}
	if !classified {
		kind = ErrorInvalidState
	}
	result.Disposition = ResearchQueueConsumeFailed
	return consumer.finalize(ctx, result, request.Mode, research.ResearchRunFailed, ResearchQueueExecutionFailed, research.ID{}, kind, executeErr)
}

func (consumer *researchQueueConsumer) loadIdentity(ctx context.Context, request ResearchQueueConsumeRequest) (research.ResearchQueueItem, research.ResearchRun, error) {
	const operation = "load research queue identity"
	item, err := consumer.dependencies.Queue.Get(ctx, request.QueueItemID)
	if err != nil {
		return research.ResearchQueueItem{}, research.ResearchRun{}, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	run, err := consumer.dependencies.Research.Run(ctx, request.RunID)
	if err != nil {
		return research.ResearchQueueItem{}, research.ResearchRun{}, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	if run.RequestID != item.Request.ID {
		return research.ResearchQueueItem{}, research.ResearchRun{}, invalid(operation, errors.New("queue item and run identify different research requests"))
	}
	return item, run, nil
}

func (consumer *researchQueueConsumer) reconcileExisting(ctx context.Context, result ResearchQueueConsumeResult, run research.ResearchRun) (ResearchQueueConsumeResult, error) {
	const operation = "reconcile research queue execution"
	if result.Execution.RunID != run.ID {
		return result, invalid(operation, errors.New("research queue item is owned by a different run"))
	}
	result.Idempotent = true
	switch result.Execution.Status {
	case ResearchQueueExecutionCompleted:
		if run.Status != research.ResearchRunCompleted {
			return result, invalid(operation, errors.New("completed queue execution has a non-completed run"))
		}
		result.Disposition = ResearchQueueConsumeCompleted
	case ResearchQueueExecutionFailed:
		result.Disposition = ResearchQueueConsumeFailed
	case ResearchQueueExecutionCancelled:
		result.Disposition = ResearchQueueConsumeCancelled
	case ResearchQueueExecutionClaimed:
		result.Disposition = ResearchQueueConsumeInFlight
	default:
		return result, invalid(operation, fmt.Errorf("cannot reconcile queue execution status %q", result.Execution.Status))
	}
	item, err := consumer.dependencies.Queue.Get(ctx, result.QueueItem.ID)
	if err != nil {
		return result, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	result.QueueItem = item
	trail, auditErr := consumer.dependencies.Research.AuditTrail(ctx, run.ID)
	if auditErr != nil {
		return result, boundaryError(ErrorPersistenceFailure, operation, auditErr)
	}
	for index := len(trail) - 1; index >= 0; index-- {
		if trail[index].Outcome.IsTerminal() {
			audit := trail[index]
			result.Audit = &audit
			break
		}
	}
	return result, nil
}

func (consumer *researchQueueConsumer) finalize(ctx context.Context, result ResearchQueueConsumeResult, mode ResearchMode, runStatus research.ResearchRunStatus, status ResearchQueueExecutionStatus, bundleID research.ID, kind ErrorKind, cause error) (ResearchQueueConsumeResult, error) {
	finalizedAt := consumer.dependencies.Clock.Now()
	running, loadErr := consumer.dependencies.Research.Run(ctx, result.Execution.RunID)
	if loadErr != nil {
		return result, errors.Join(cause, boundaryError(ErrorPersistenceFailure, "load Research Run for terminal audit", loadErr))
	}
	auditResult, auditErr := consumer.dependencies.TerminalAudit.Prepare(ctx, LiveResearchTerminalAuditRequest{
		Run: running, Outcome: runStatus, FinalizedAt: finalizedAt, Mode: mode, FailureKind: kind,
		Artifacts: result.Orchestration.Artifacts,
	})
	if auditErr != nil {
		return result, errors.Join(cause, boundaryError(ErrorPersistenceFailure, "prepare terminal research audit", auditErr))
	}
	finalization := ResearchFinalization{
		QueueItemID: result.Execution.QueueItemID, RunID: result.Execution.RunID,
		RunStatus: runStatus, ExecutionStatus: status, Attempts: result.Execution.Attempts,
		FailureKind: kind, Audit: &auditResult.Audit, Cost: &auditResult.Cost,
		FinalizedAt: finalizedAt, AlgorithmVersion: ResearchFinalizationV1,
	}
	if status == ResearchQueueExecutionCompleted {
		finalization.BundleID = &bundleID
	}
	finalized, err := consumer.dependencies.Finalization.Finalize(ctx, finalization)
	if err != nil {
		return result, errors.Join(cause, boundaryError(ErrorPersistenceFailure, "finalize research run and queue", err))
	}
	result.Execution = finalized.Execution
	result.Orchestration.Run = finalized.Run
	result.Audit = finalized.Audit
	item, err := consumer.dependencies.Queue.Get(ctx, finalized.Execution.QueueItemID)
	if err != nil {
		return result, errors.Join(cause, boundaryError(ErrorPersistenceFailure, "reload settled research queue item", err))
	}
	result.QueueItem = item
	return result, cause
}

func researchQueueFailureIsTransient(kind ErrorKind) bool {
	switch kind {
	case ErrorConflict, ErrorUnavailable, ErrorPersistenceFailure, ErrorExternalFailure:
		return true
	default:
		return false
	}
}

func validResearchQueueFailureKind(kind ErrorKind) bool {
	switch kind {
	case ErrorNotFound, ErrorConflict, ErrorInvalidState, ErrorUnavailable, ErrorPersistenceFailure,
		ErrorExternalFailure, ErrorNetworkResearchBlocked, ErrorBudgetExceeded:
		return true
	default:
		return false
	}
}

var _ ResearchQueueConsumer = (*researchQueueConsumer)(nil)
