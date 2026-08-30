package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/diversity"
	registrypolicy "github.com/mishaaac/kelyro/internal/research/registry"
)

type sourceDiversityService struct {
	claims   ClaimRepository
	sources  SourceRepository
	trust    TrustRegistryRepository
	registry SourceRegistryRepository
}

func NewSourceDiversityService(
	claims ClaimRepository,
	sources SourceRepository,
	trust TrustRegistryRepository,
	registry SourceRegistryRepository,
) SourceDiversityService {
	return &sourceDiversityService{claims: claims, sources: sources, trust: trust, registry: registry}
}

func (service *sourceDiversityService) Assess(
	ctx context.Context,
	request AssessSourceDiversityRequest,
) (diversity.Assessment, error) {
	const operation = "assess source diversity"
	if err := request.Validate(); err != nil {
		return diversity.Assessment{}, invalid(operation, err)
	}
	for name, dependency := range map[string]any{
		"claim repository": service.claims, "source repository": service.sources,
		"trust repository": service.trust, "source registry repository": service.registry,
	} {
		if err := requireDependency(operation, name, dependency); err != nil {
			return diversity.Assessment{}, err
		}
	}
	claim, err := service.claims.Get(ctx, request.ClaimID)
	if err != nil {
		return diversity.Assessment{}, repositoryError(operation, err)
	}
	if len(request.Annotations) != len(claim.SourceIDs) {
		return diversity.Assessment{}, invalid(operation, fmt.Errorf("annotations must cover every claim source"))
	}
	claimSourceIDs := make(map[research.SourceID]struct{}, len(claim.SourceIDs))
	for _, sourceID := range claim.SourceIDs {
		claimSourceIDs[sourceID] = struct{}{}
	}
	annotations := append([]SourceDiversityAnnotation(nil), request.Annotations...)
	sort.Slice(annotations, func(i, j int) bool { return annotations[i].SourceID.String() < annotations[j].SourceID.String() })
	entries, err := service.registry.List(ctx)
	if err != nil {
		return diversity.Assessment{}, repositoryError(operation, err)
	}
	catalog, err := registrypolicy.NewCatalog(entries)
	if err != nil {
		return diversity.Assessment{}, invalid(operation, err)
	}
	reviews := make([]diversity.SourceReview, 0, len(annotations))
	for _, annotation := range annotations {
		if _, exists := claimSourceIDs[annotation.SourceID]; !exists {
			return diversity.Assessment{}, invalid(operation, fmt.Errorf("annotation source %q is not declared by claim", annotation.SourceID))
		}
		source, sourceErr := service.sources.Get(ctx, annotation.SourceID)
		if sourceErr != nil {
			return diversity.Assessment{}, repositoryError(operation, sourceErr)
		}
		var decision *research.TrustDecision
		storedDecision, decisionErr := service.trust.LatestDecision(ctx, annotation.SourceID)
		switch {
		case decisionErr == nil:
			copy := storedDecision
			decision = &copy
		case errors.Is(decisionErr, ErrNotFound):
		default:
			return diversity.Assessment{}, repositoryError(operation, decisionErr)
		}
		organization := ""
		entry, matched, matchErr := catalog.MatchLocator(source.Locator)
		if matchErr != nil {
			return diversity.Assessment{}, invalid(operation, matchErr)
		}
		if matched && registrypolicy.AppliesTo(entry, claim.Topic, source.Kind) {
			organization = entry.Organization
		}
		reviews = append(reviews, diversity.SourceReview{
			Source: source, TrustDecision: decision, Organization: organization,
			DependencyGroup: annotation.DependencyGroup, Perspective: annotation.Perspective,
			TechnicalRole: annotation.TechnicalRole,
		})
	}
	assessment, err := (diversity.PolicyV1{}).Assess(diversity.Input{Claim: claim, Sources: reviews})
	if err != nil {
		return diversity.Assessment{}, invalid(operation, err)
	}
	return assessment, nil
}

var _ SourceDiversityService = (*sourceDiversityService)(nil)
