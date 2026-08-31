package app

import (
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchEvidenceForRun(store researchapp.SourceRegistryStore) (researchapp.LiveResearchStageService, error) {
	if store == nil || store.Evidence() == nil {
		return nil, fmt.Errorf("research evidence repository is unavailable")
	}
	stage, err := researchapp.NewLiveEvidenceExtractionService(
		researchapp.NewDeterministicEvidenceExtractorV1(), store.Evidence(), researchSearchClock{now: service.researchClock},
	)
	if err != nil {
		return nil, fmt.Errorf("assemble live research evidence extraction: %w", err)
	}
	return stage, nil
}
