package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	conflictpolicy "github.com/mishaaac/kelyro/internal/research/conflict"
)

type conflictResolutionService struct {
	repository ConflictRepository
	claims     ClaimRepository
	sources    SourceRepository
	trust      TrustRegistryRepository
	clock      Clock
}

func NewConflictResolutionService(
	repository ConflictRepository,
	claims ClaimRepository,
	sources SourceRepository,
	trust TrustRegistryRepository,
	clock Clock,
) ConflictResolutionService {
	return &conflictResolutionService{
		repository: repository, claims: claims, sources: sources, trust: trust, clock: clock,
	}
}

func (service *conflictResolutionService) Assess(
	ctx context.Context,
	request ConflictAssessmentRequest,
) (research.Conflict, error) {
	const operation = "assess source conflict"
	if err := request.Validate(); err != nil {
		return research.Conflict{}, invalid(operation, err)
	}
	for name, dependency := range map[string]any{
		"conflict repository": service.repository,
		"claim repository":    service.claims,
		"source repository":   service.sources,
		"trust repository":    service.trust,
		"clock":               service.clock,
	} {
		if err := requireDependency(operation, name, dependency); err != nil {
			return research.Conflict{}, err
		}
	}
	detectedAt := service.clock.Now()
	if err := detectedAt.Validate(); err != nil {
		return research.Conflict{}, invalid(operation, fmt.Errorf("clock: %w", err))
	}

	observations := make([]conflictpolicy.Observation, 0, len(request.Observations))
	for index, reference := range request.Observations {
		claim, err := service.claims.Get(ctx, reference.ClaimID)
		if err != nil {
			return research.Conflict{}, repositoryError(operation, err)
		}
		source, err := service.sources.Get(ctx, reference.SourceID)
		if err != nil {
			return research.Conflict{}, repositoryError(operation, err)
		}
		decision, err := service.trust.LatestDecision(ctx, reference.SourceID)
		if err != nil {
			return research.Conflict{}, repositoryError(operation, err)
		}
		if decision.State != research.TrustAccepted && decision.State != research.TrustAcceptedSupplement {
			return research.Conflict{}, invalid(operation,
				fmt.Errorf("conflict observation %d source has no accepted trust decision", index))
		}
		if decision.EvaluatedAt.After(detectedAt) {
			return research.Conflict{}, invalid(operation,
				fmt.Errorf("conflict observation %d trust decision is after detection", index))
		}
		observations = append(observations, conflictpolicy.Observation{
			Claim: claim, Source: source, AuthorityTier: decision.Tier,
		})
	}
	sort.Slice(observations, func(i, j int) bool {
		return observations[i].Claim.ID.String() < observations[j].Claim.ID.String()
	})
	identity := []string{
		string(request.Relation),
		observations[0].Claim.ID.String(), observations[0].Source.ID.String(),
		observations[1].Claim.ID.String(), observations[1].Source.ID.String(),
		detectedAt.Time().Format(time.RFC3339Nano),
	}
	result, err := conflictpolicy.Resolve(conflictpolicy.Input{
		ID: stableResearchID("conflict", identity...), Relation: request.Relation,
		Observations: observations, DetectedAt: detectedAt,
	})
	if err != nil {
		return research.Conflict{}, invalid(operation, err)
	}
	if err := service.repository.Append(ctx, result); err != nil {
		return research.Conflict{}, repositoryError(operation, err)
	}
	return result, nil
}

func (service *conflictResolutionService) Get(ctx context.Context, id research.ID) (research.Conflict, error) {
	const operation = "get source conflict"
	if err := id.Validate(); err != nil {
		return research.Conflict{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "conflict repository", service.repository); err != nil {
		return research.Conflict{}, err
	}
	result, err := service.repository.Get(ctx, id)
	return result, repositoryError(operation, err)
}

func (service *conflictResolutionService) ListForClaim(ctx context.Context, claimID research.ClaimID) ([]research.Conflict, error) {
	const operation = "list source conflicts for claim"
	if err := claimID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if err := requireDependency(operation, "conflict repository", service.repository); err != nil {
		return nil, err
	}
	results, err := service.repository.ListByClaim(ctx, claimID)
	return results, repositoryError(operation, err)
}

var _ ConflictResolutionService = (*conflictResolutionService)(nil)
