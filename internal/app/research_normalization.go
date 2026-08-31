package app

import (
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchNormalizationForRun() (researchapp.LiveResearchStageService, error) {
	if service.researchNormalizer == nil {
		return nil, fmt.Errorf("research source normalizer is unavailable")
	}
	stage, err := researchapp.NewLiveSourceNormalizationService(service.researchNormalizer)
	if err != nil {
		return nil, fmt.Errorf("assemble live research normalization: %w", err)
	}
	return stage, nil
}
