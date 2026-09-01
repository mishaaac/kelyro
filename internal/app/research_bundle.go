package app

import (
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchBundleForRun(store researchapp.SourceRegistryStore) (researchapp.LiveResearchStageService, error) {
	if store == nil || store.Bundles() == nil {
		return nil, fmt.Errorf("research source bundle service is unavailable")
	}
	stage, err := researchapp.NewLiveSourceBundleService(store.Bundles())
	if err != nil {
		return nil, fmt.Errorf("assemble live research source bundle: %w", err)
	}
	return stage, nil
}
