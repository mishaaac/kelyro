package app

import (
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchEvidenceForRun(store researchapp.SourceRegistryStore) (researchapp.LiveResearchStageService, error) {
	if store == nil || store.Evidence() == nil || store.Claims() == nil || store.Citations() == nil ||
		store.TrustRepository() == nil || store.Registry() == nil || store.Freshness() == nil ||
		store.Releases() == nil || store.Deprecations() == nil {
		return nil, fmt.Errorf("research extraction repositories are unavailable")
	}
	clock := researchSearchClock{now: service.researchClock}
	evidence, err := researchapp.NewLiveEvidenceExtractionService(
		researchapp.NewDeterministicEvidenceExtractorV1(), store.Evidence(), clock,
	)
	if err != nil {
		return nil, fmt.Errorf("assemble live research evidence extraction: %w", err)
	}
	claims, err := researchapp.NewLiveClaimExtractionService(
		researchapp.NewDeterministicClaimExtractorV1(), store.Evidence(), store.Claims(), store.Citations(), clock,
	)
	if err != nil {
		return nil, fmt.Errorf("assemble live research claim extraction: %w", err)
	}
	trust, err := researchapp.NewLiveTrustEvaluationService(store.TrustRepository(), store.Registry(), clock)
	if err != nil {
		return nil, fmt.Errorf("assemble live research trust evaluation: %w", err)
	}
	temporal, err := researchapp.NewLiveTemporalEvaluationService(
		store.Freshness(), trust, store.TrustRepository(), store.Releases(), store.Deprecations(), clock,
	)
	if err != nil {
		return nil, fmt.Errorf("assemble live research temporal evaluation: %w", err)
	}
	stage, err := researchapp.NewLiveResearchExtractionStage(evidence, claims, temporal)
	if err != nil {
		return nil, fmt.Errorf("assemble live research extraction: %w", err)
	}
	return stage, nil
}
