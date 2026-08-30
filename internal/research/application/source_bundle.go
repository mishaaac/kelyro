package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	bundlepolicy "github.com/mishaaac/kelyro/internal/research/bundle"
)

type sourceBundleService struct {
	repository   SourceBundleRepository
	runs         ResearchRunRepository
	claims       ClaimRepository
	sources      SourceRepository
	evidence     EvidenceRepository
	trust        TrustRegistryRepository
	verification VerificationRepository
	conflicts    ConflictRepository
	freshness    FreshnessRepository
	clock        Clock
}

func NewSourceBundleService(
	repository SourceBundleRepository,
	runs ResearchRunRepository,
	claims ClaimRepository,
	sources SourceRepository,
	evidence EvidenceRepository,
	trust TrustRegistryRepository,
	verification VerificationRepository,
	conflicts ConflictRepository,
	freshness FreshnessRepository,
	clock Clock,
) SourceBundleService {
	return &sourceBundleService{
		repository: repository, runs: runs, claims: claims, sources: sources,
		evidence: evidence, trust: trust, verification: verification,
		conflicts: conflicts, freshness: freshness, clock: clock,
	}
}

func (service *sourceBundleService) Assemble(ctx context.Context, request AssembleSourceBundleRequest) (research.SourceBundle, error) {
	const operation = "assemble source bundle"
	if err := request.Validate(); err != nil {
		return research.SourceBundle{}, invalid(operation, err)
	}
	for name, dependency := range map[string]any{
		"source bundle repository": service.repository,
		"research run repository":  service.runs,
		"claim repository":         service.claims,
		"source repository":        service.sources,
		"evidence repository":      service.evidence,
		"trust repository":         service.trust,
		"verification repository":  service.verification,
		"conflict repository":      service.conflicts,
		"freshness repository":     service.freshness,
		"clock":                    service.clock,
	} {
		if err := requireDependency(operation, name, dependency); err != nil {
			return research.SourceBundle{}, err
		}
	}
	verifiedAt := service.clock.Now()
	if err := verifiedAt.Validate(); err != nil {
		return research.SourceBundle{}, invalid(operation, fmt.Errorf("clock: %w", err))
	}
	run, err := service.runs.GetRun(ctx, request.RunID)
	if err != nil {
		return research.SourceBundle{}, repositoryError(operation, err)
	}
	researchRequest, err := service.runs.GetRequest(ctx, run.RequestID)
	if err != nil {
		return research.SourceBundle{}, repositoryError(operation, err)
	}

	claimIDs := append([]research.ClaimID(nil), request.ClaimIDs...)
	sort.Slice(claimIDs, func(i, j int) bool { return claimIDs[i].String() < claimIDs[j].String() })
	claimObservations := make([]bundlepolicy.ClaimObservation, 0, len(claimIDs))
	sourceIDs := make(map[research.SourceID]struct{})
	allConflicts := make(map[research.ID]research.Conflict)
	freshnessObservations := make([]bundlepolicy.FreshnessObservation, 0, len(claimIDs))
	missingFreshness := make([]research.ClaimID, 0)
	for _, claimID := range claimIDs {
		claim, getErr := service.claims.Get(ctx, claimID)
		if getErr != nil {
			return research.SourceBundle{}, repositoryError(operation, getErr)
		}
		observation := bundlepolicy.ClaimObservation{Claim: claim}
		for _, evidenceID := range claim.EvidenceIDs {
			evidence, evidenceErr := service.evidence.Get(ctx, evidenceID)
			switch {
			case evidenceErr == nil:
				if evidence.ExtractedAt.After(verifiedAt) {
					return research.SourceBundle{}, invalid(operation, fmt.Errorf("evidence %q is after bundle", evidenceID))
				}
				if !containsSourceID(claim.SourceIDs, evidence.SourceID) {
					return research.SourceBundle{}, invalid(operation, fmt.Errorf("evidence %q source is not declared by claim", evidenceID))
				}
			case errors.Is(evidenceErr, ErrNotFound):
				observation.MissingEvidenceIDs = append(observation.MissingEvidenceIDs, evidenceID)
			default:
				return research.SourceBundle{}, repositoryError(operation, evidenceErr)
			}
		}
		verification, verificationErr := service.verification.LatestByClaim(ctx, claimID)
		switch {
		case verificationErr == nil:
			observation.Verification = &verification
		case errors.Is(verificationErr, ErrNotFound):
		default:
			return research.SourceBundle{}, repositoryError(operation, verificationErr)
		}
		claimObservations = append(claimObservations, observation)
		for _, sourceID := range claim.SourceIDs {
			sourceIDs[sourceID] = struct{}{}
		}
		conflicts, conflictErr := service.conflicts.ListByClaim(ctx, claimID)
		if conflictErr != nil {
			return research.SourceBundle{}, repositoryError(operation, conflictErr)
		}
		for _, item := range conflicts {
			allConflicts[item.ID] = item
		}
		freshnessID, idErr := research.NewID(claimID.String())
		if idErr != nil {
			return research.SourceBundle{}, invalid(operation, idErr)
		}
		record, freshnessErr := service.freshness.Get(ctx, freshnessID)
		switch {
		case freshnessErr == nil:
			freshnessObservations = append(freshnessObservations, bundlepolicy.FreshnessObservation{
				ClaimID: claimID, State: record.State, Score: record.Score,
				LastVerifiedAt: record.LastVerifiedAt, AlgorithmVersion: record.AlgorithmVersion,
			})
		case errors.Is(freshnessErr, ErrNotFound):
			missingFreshness = append(missingFreshness, claimID)
		default:
			return research.SourceBundle{}, repositoryError(operation, freshnessErr)
		}
	}

	orderedSourceIDs := make([]research.SourceID, 0, len(sourceIDs))
	for sourceID := range sourceIDs {
		orderedSourceIDs = append(orderedSourceIDs, sourceID)
	}
	sort.Slice(orderedSourceIDs, func(i, j int) bool { return orderedSourceIDs[i].String() < orderedSourceIDs[j].String() })
	sourceObservations := make([]bundlepolicy.SourceObservation, 0, len(orderedSourceIDs))
	for _, sourceID := range orderedSourceIDs {
		source, sourceErr := service.sources.Get(ctx, sourceID)
		if sourceErr != nil {
			return research.SourceBundle{}, repositoryError(operation, sourceErr)
		}
		observation := bundlepolicy.SourceObservation{Source: source}
		decision, decisionErr := service.trust.LatestDecision(ctx, sourceID)
		switch {
		case decisionErr == nil:
			observation.TrustDecision = &decision
		case errors.Is(decisionErr, ErrNotFound):
		default:
			return research.SourceBundle{}, repositoryError(operation, decisionErr)
		}
		sourceObservations = append(sourceObservations, observation)
	}

	conflicts := make([]research.Conflict, 0, len(allConflicts))
	for _, item := range allConflicts {
		conflicts = append(conflicts, item)
	}
	conflicts = bundlepolicy.LatestConflicts(conflicts)
	identity := []string{run.ID.String(), verifiedAt.Time().Format(time.RFC3339Nano)}
	for _, claimID := range claimIDs {
		identity = append(identity, claimID.String())
	}
	bundle, err := bundlepolicy.AssembleV1(bundlepolicy.Input{
		ID: stableResearchID("bundle", identity...), Request: researchRequest, Run: run,
		Claims: claimObservations, Sources: sourceObservations, Conflicts: conflicts,
		Freshness: freshnessObservations, MissingFreshness: missingFreshness, VerifiedAt: verifiedAt,
	})
	if err != nil {
		return research.SourceBundle{}, invalid(operation, err)
	}
	if err := service.repository.Append(ctx, bundle); err != nil {
		return research.SourceBundle{}, repositoryError(operation, err)
	}
	return bundle, nil
}

func (service *sourceBundleService) Get(ctx context.Context, id research.ID) (research.SourceBundle, error) {
	const operation = "get source bundle"
	if err := id.Validate(); err != nil {
		return research.SourceBundle{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "source bundle repository", service.repository); err != nil {
		return research.SourceBundle{}, err
	}
	bundle, err := service.repository.Get(ctx, id)
	return bundle, repositoryError(operation, err)
}

func (service *sourceBundleService) Export(ctx context.Context, id research.ID) ([]byte, error) {
	const operation = "export source bundle"
	bundle, err := service.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if bundle.AlgorithmVersion != research.SourceBundleAlgorithmV1 {
		return nil, invalid(operation, fmt.Errorf("legacy source bundle has no canonical v1 representation"))
	}
	encoded, err := bundle.ExportJSON()
	if err != nil {
		return nil, invalid(operation, err)
	}
	return encoded, nil
}

func (service *sourceBundleService) ListForRun(ctx context.Context, runID research.ID) ([]research.SourceBundle, error) {
	const operation = "list source bundles for research run"
	if err := runID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if err := requireDependency(operation, "source bundle repository", service.repository); err != nil {
		return nil, err
	}
	items, err := service.repository.ListByRun(ctx, runID)
	return items, repositoryError(operation, err)
}

var _ SourceBundleService = (*sourceBundleService)(nil)
