package app

import (
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchProvenanceForRun(store researchapp.SourceRegistryStore) (researchapp.LiveResearchStageService, error) {
	if store == nil || store.Provenance() == nil {
		return nil, fmt.Errorf("research provenance service is unavailable")
	}
	stage, err := researchapp.NewLiveResearchProvenanceService(store.Provenance(), researchSearchClock{now: service.researchClock})
	if err != nil {
		return nil, fmt.Errorf("preserve live research provenance: %w", err)
	}
	return stage, nil
}
