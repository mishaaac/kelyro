package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

const ResearchFinalizationV1 = "research-finalization-v1"

type ResearchFinalization struct {
	QueueItemID      research.ID
	RunID            research.ID
	RunStatus        research.ResearchRunStatus
	ExecutionStatus  ResearchQueueExecutionStatus
	Attempts         int
	BundleID         *research.ID
	FailureKind      ErrorKind
	Audit            *research.ResearchRunAudit
	Cost             *research.ResearchCostMetadata
	FinalizedAt      research.Timestamp
	AlgorithmVersion string
}

func (finalization ResearchFinalization) Validate() error {
	if err := finalization.QueueItemID.Validate(); err != nil {
		return fmt.Errorf("research finalization queue item: %w", err)
	}
	if err := finalization.RunID.Validate(); err != nil {
		return fmt.Errorf("research finalization run: %w", err)
	}
	if finalization.Attempts < 1 {
		return errors.New("research finalization attempts must be positive")
	}
	if err := finalization.FinalizedAt.Validate(); err != nil {
		return err
	}
	if finalization.AlgorithmVersion != ResearchFinalizationV1 {
		return fmt.Errorf("research finalization algorithm must be %q", ResearchFinalizationV1)
	}
	switch finalization.ExecutionStatus {
	case ResearchQueueExecutionCompleted:
		if finalization.RunStatus != research.ResearchRunCompleted || finalization.BundleID == nil || finalization.FailureKind != "" {
			return errors.New("completed research finalization requires completed run and bundle without failure")
		}
	case ResearchQueueExecutionRetry, ResearchQueueExecutionFailed:
		if finalization.RunStatus != research.ResearchRunFailed || finalization.BundleID != nil || !validResearchQueueFailureKind(finalization.FailureKind) {
			return errors.New("failed research finalization requires failed run and safe failure kind")
		}
	case ResearchQueueExecutionCancelled:
		if finalization.RunStatus != research.ResearchRunCancelled || finalization.BundleID != nil || finalization.FailureKind != "" {
			return errors.New("cancelled research finalization is invalid")
		}
	default:
		return fmt.Errorf("research finalization cannot settle execution as %q", finalization.ExecutionStatus)
	}
	if finalization.BundleID != nil {
		if err := finalization.BundleID.Validate(); err != nil {
			return fmt.Errorf("research finalization bundle: %w", err)
		}
	}
	if (finalization.Audit == nil) != (finalization.Cost == nil) {
		return errors.New("research finalization audit and cost must be supplied together")
	}
	if finalization.Audit != nil {
		if err := finalization.Audit.Validate(); err != nil {
			return fmt.Errorf("research finalization audit: %w", err)
		}
		if err := finalization.Cost.Validate(); err != nil {
			return fmt.Errorf("research finalization cost: %w", err)
		}
		if finalization.Audit.RunID != finalization.RunID || finalization.Audit.Outcome != finalization.RunStatus ||
			finalization.Audit.CompletedAt == nil || !finalization.Audit.CompletedAt.Time().Equal(finalization.FinalizedAt.Time()) ||
			finalization.Audit.Execution == nil || finalization.Audit.Execution.CostUsed != finalization.Cost.Used ||
			finalization.Audit.Execution.CacheSavings != finalization.Cost.CacheSavings ||
			finalization.Audit.Execution.StoppedByBudget != finalization.Cost.StoppedByBudget ||
			finalization.Audit.Execution.FailureKind != string(finalization.FailureKind) ||
			!sameFinalizationBundleID(finalization.Audit.Execution.BundleID, finalization.BundleID) {
			return errors.New("research finalization audit does not match terminal lifecycle and cost")
		}
	}
	return nil
}

func sameFinalizationBundleID(left, right *research.ID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

type ResearchFinalizationResult struct {
	Run       research.ResearchRun
	Execution ResearchQueueExecution
	Audit     *research.ResearchRunAudit
}

func (result ResearchFinalizationResult) Validate() error {
	if err := result.Run.Validate(); err != nil {
		return err
	}
	if err := result.Execution.Validate(); err != nil {
		return err
	}
	if result.Run.ID != result.Execution.RunID {
		return errors.New("research finalization result run and execution do not agree")
	}
	if result.Audit != nil {
		if err := result.Audit.Validate(); err != nil {
			return err
		}
		if result.Audit.RunID != result.Run.ID || result.Audit.Outcome != result.Run.Status {
			return errors.New("research finalization result audit does not agree with run")
		}
	}
	switch result.Execution.Status {
	case ResearchQueueExecutionCompleted:
		if result.Run.Status != research.ResearchRunCompleted || result.Execution.BundleID == nil {
			return errors.New("completed research finalization result is invalid")
		}
	case ResearchQueueExecutionRetry, ResearchQueueExecutionFailed:
		if result.Run.Status != research.ResearchRunFailed {
			return errors.New("failed research finalization result is invalid")
		}
	case ResearchQueueExecutionCancelled:
		if result.Run.Status != research.ResearchRunCancelled {
			return errors.New("cancelled research finalization result is invalid")
		}
	default:
		return errors.New("research finalization result is not terminal")
	}
	return nil
}

type ResearchFinalizationService interface {
	Finalize(context.Context, ResearchFinalization) (ResearchFinalizationResult, error)
}

type researchFinalizationService struct {
	repository ResearchFinalizationRepository
}

func NewResearchFinalizationService(repository ResearchFinalizationRepository) ResearchFinalizationService {
	return &researchFinalizationService{repository: repository}
}

func (service *researchFinalizationService) Finalize(ctx context.Context, finalization ResearchFinalization) (ResearchFinalizationResult, error) {
	const operation = "finalize research run and queue"
	if ctx == nil {
		return ResearchFinalizationResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := finalization.Validate(); err != nil {
		return ResearchFinalizationResult{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "research finalization repository", service.repository); err != nil {
		return ResearchFinalizationResult{}, err
	}
	result, err := service.repository.Finalize(ctx, finalization)
	if err != nil {
		return ResearchFinalizationResult{}, repositoryError(operation, err)
	}
	if err := result.Validate(); err != nil {
		return ResearchFinalizationResult{}, invalid(operation, err)
	}
	return result, nil
}

var _ ResearchFinalizationService = (*researchFinalizationService)(nil)
