package app

import (
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchVerificationForRun(store researchapp.SourceRegistryStore) (researchapp.LiveResearchStageService, error) {
	if store == nil || store.Verifications() == nil || store.Diversity() == nil {
		return nil, fmt.Errorf("research verification service is unavailable")
	}
	stage, err := researchapp.NewLiveMultiSourceVerificationService(store.Verifications(), store.Diversity())
	if err != nil {
		return nil, fmt.Errorf("assemble live research verification: %w", err)
	}
	return stage, nil
}
