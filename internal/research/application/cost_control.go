package application

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchCostService struct{ repository ResearchCostRepository }

func NewResearchCostService(repository ResearchCostRepository) ResearchCostService {
	return &researchCostService{repository: repository}
}

func (service *researchCostService) Evaluate(ctx context.Context, request CostControlRequest) (CostControlDecision, error) {
	const operation = "evaluate research cost"
	if err := request.Validate(); err != nil {
		return CostControlDecision{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "research cost repository", service.repository); err != nil {
		return CostControlDecision{}, err
	}
	reservation := CostReservation{RunID: request.RunID, Usage: request.ProposedUsage, At: request.At}
	if request.ValidCacheAvailable {
		if err := service.repository.RecordCacheSavings(ctx, reservation); err != nil {
			return CostControlDecision{}, repositoryError(operation, err)
		}
		return service.avoid(ctx, reservation.RunID, CostControlUseValidCache, "A valid cached result is available; network research was skipped.")
	}
	if request.VerificationSatisfied {
		return service.avoid(ctx, reservation.RunID, CostControlVerificationSatisfied, "Verification requirements are already satisfied; additional network research was skipped.")
	}
	if request.RequiredPrimarySources > 0 && request.PrimarySources >= request.RequiredPrimarySources {
		return service.avoid(ctx, reservation.RunID, CostControlPrimarySufficient, fmt.Sprintf("%d primary sources satisfy the requirement; additional network research was skipped.", request.PrimarySources))
	}
	reserved, err := service.repository.Reserve(ctx, reservation)
	if err != nil {
		return CostControlDecision{}, repositoryError(operation, err)
	}
	if reserved.Allowed {
		return CostControlDecision{NetworkAllowed: true, Reason: CostControlAllowed, Metadata: reserved.Metadata}, nil
	}
	return CostControlDecision{
		BudgetStopped: true, Reason: CostControlBudgetExceeded, Scope: reserved.Scope,
		UserExplanation: reserved.Reason, Metadata: reserved.Metadata,
	}, nil
}

func (service *researchCostService) avoid(ctx context.Context, runID research.ID, reason CostControlReason, explanation string) (CostControlDecision, error) {
	const operation = "read avoided research cost"
	metadata, err := service.repository.Metadata(ctx, runID)
	if err != nil {
		return CostControlDecision{}, repositoryError(operation, err)
	}
	return CostControlDecision{Reason: reason, UserExplanation: explanation, Metadata: metadata}, nil
}

func (service *researchCostService) Metadata(ctx context.Context, runID research.ID) (research.ResearchCostMetadata, error) {
	const operation = "get research cost metadata"
	if err := runID.Validate(); err != nil {
		return research.ResearchCostMetadata{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "research cost repository", service.repository); err != nil {
		return research.ResearchCostMetadata{}, err
	}
	metadata, err := service.repository.Metadata(ctx, runID)
	return metadata, repositoryError(operation, err)
}

func (service *researchCostService) Stats(ctx context.Context, asOf research.Timestamp) (ResearchCostStats, error) {
	const operation = "get research cost stats"
	if err := asOf.Validate(); err != nil {
		return ResearchCostStats{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "research cost repository", service.repository); err != nil {
		return ResearchCostStats{}, err
	}
	stats, err := service.repository.Stats(ctx, asOf)
	if err == nil {
		err = stats.Validate()
	}
	return stats, repositoryError(operation, err)
}
