package app

import (
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchFinalizationForRun(store researchapp.SourceRegistryStore) (researchapp.LiveResearchStageService, researchapp.ResearchFinalizationService, error) {
	if store == nil || store.Bundles() == nil || store.Finalization() == nil {
		return nil, nil, fmt.Errorf("research finalization services are unavailable")
	}
	stage, err := researchapp.NewLiveResearchFinalizationStage(store.Bundles())
	if err != nil {
		return nil, nil, fmt.Errorf("assemble live research finalization: %w", err)
	}
	return stage, store.Finalization(), nil
}
