package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	registrypolicy "github.com/mishaaac/kelyro/internal/research/registry"
	verificationpolicy "github.com/mishaaac/kelyro/internal/research/verification"
)

type verificationService struct {
	repository VerificationRepository
	claims     ClaimRepository
	sources    SourceRepository
	trust      TrustRegistryRepository
	registry   SourceRegistryRepository
	conflicts  ConflictRepository
	clock      Clock
}

func NewVerificationService(
	repository VerificationRepository,
	claims ClaimRepository,
	sources SourceRepository,
	trust TrustRegistryRepository,
	registry SourceRegistryRepository,
	conflicts ConflictRepository,
	clock Clock,
) VerificationService {
	return &verificationService{
		repository: repository, claims: claims, sources: sources, trust: trust,
		registry: registry, conflicts: conflicts, clock: clock,
	}
}

func (service *verificationService) Verify(
	ctx context.Context,
	claimID research.ClaimID,
) (research.VerificationResult, error) {
	const operation = "verify claim from multiple sources"
	if err := claimID.Validate(); err != nil {
		return research.VerificationResult{}, invalid(operation, err)
	}
	for name, dependency := range map[string]any{
		"verification repository":    service.repository,
		"claim repository":           service.claims,
		"source repository":          service.sources,
		"trust repository":           service.trust,
		"source registry repository": service.registry,
		"conflict repository":        service.conflicts,
		"clock":                      service.clock,
	} {
		if err := requireDependency(operation, name, dependency); err != nil {
			return research.VerificationResult{}, err
		}
	}
	verifiedAt := service.clock.Now()
	if err := verifiedAt.Validate(); err != nil {
		return research.VerificationResult{}, invalid(operation, fmt.Errorf("clock: %w", err))
	}
	claim, err := service.claims.Get(ctx, claimID)
	if err != nil {
		return research.VerificationResult{}, repositoryError(operation, err)
	}
	entries, err := service.registry.List(ctx)
	if err != nil {
		return research.VerificationResult{}, repositoryError(operation, err)
	}
	catalog, err := registrypolicy.NewCatalog(entries)
	if err != nil {
		return research.VerificationResult{}, invalid(operation, err)
	}

	sourceIDs := append([]research.SourceID(nil), claim.SourceIDs...)
	sort.Slice(sourceIDs, func(i, j int) bool { return sourceIDs[i].String() < sourceIDs[j].String() })
	observations := make([]verificationpolicy.Observation, 0, len(sourceIDs))
	for index, sourceID := range sourceIDs {
		source, getErr := service.sources.Get(ctx, sourceID)
		if getErr != nil {
			return research.VerificationResult{}, repositoryError(operation, getErr)
		}
		var decision *research.TrustDecision
		storedDecision, decisionErr := service.trust.LatestDecision(ctx, sourceID)
		switch {
		case decisionErr == nil:
			copy := storedDecision
			decision = &copy
		case errors.Is(decisionErr, ErrNotFound):
			// Missing reviewed trust data is an explicit unknown authority metric,
			// not an invented tier and not a repository failure.
		default:
			return research.VerificationResult{}, repositoryError(operation, decisionErr)
		}
		observation := verificationpolicy.Observation{Source: source, TrustDecision: decision}
		entry, matched, matchErr := catalog.MatchLocator(source.Locator)
		if matchErr != nil {
			return research.VerificationResult{}, invalid(operation, matchErr)
		}
		if matched {
			if entry.LastReviewedAt.After(verifiedAt) {
				return research.VerificationResult{}, invalid(operation,
					fmt.Errorf("source registry entry for observation %d is after verification", index))
			}
			status := entry.Status
			observation.RegistryOrganization = entry.Organization
			observation.RegistryStatus = &status
		}
		observations = append(observations, observation)
	}
	conflicts, err := service.conflicts.ListByClaim(ctx, claimID)
	if err != nil {
		return research.VerificationResult{}, repositoryError(operation, err)
	}
	identity := []string{claimID.String(), verifiedAt.Time().Format(time.RFC3339Nano)}
	for _, sourceID := range sourceIDs {
		identity = append(identity, sourceID.String())
	}
	result, err := verificationpolicy.Verify(verificationpolicy.Input{
		ID: stableResearchID("verification", identity...), Claim: claim,
		Observations: observations, Conflicts: conflicts, VerifiedAt: verifiedAt,
	})
	if err != nil {
		return research.VerificationResult{}, invalid(operation, err)
	}
	if err := service.repository.Append(ctx, result); err != nil {
		return research.VerificationResult{}, repositoryError(operation, err)
	}
	return result, nil
}

func (service *verificationService) Get(ctx context.Context, id research.ID) (research.VerificationResult, error) {
	const operation = "get verification result"
	if err := id.Validate(); err != nil {
		return research.VerificationResult{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "verification repository", service.repository); err != nil {
		return research.VerificationResult{}, err
	}
	result, err := service.repository.Get(ctx, id)
	return result, repositoryError(operation, err)
}

func (service *verificationService) Latest(ctx context.Context, claimID research.ClaimID) (research.VerificationResult, error) {
	const operation = "get latest verification result"
	if err := claimID.Validate(); err != nil {
		return research.VerificationResult{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "verification repository", service.repository); err != nil {
		return research.VerificationResult{}, err
	}
	result, err := service.repository.LatestByClaim(ctx, claimID)
	return result, repositoryError(operation, err)
}

var _ VerificationService = (*verificationService)(nil)
