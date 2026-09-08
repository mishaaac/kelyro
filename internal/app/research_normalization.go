package app

import (
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchNormalizationForRun(store researchapp.SourceRegistryStore) (researchapp.LiveResearchStageService, error) {
	if service.researchNormalizer == nil {
		return nil, fmt.Errorf("research source normalizer is unavailable")
	}
	stage, err := researchapp.NewLiveSourceNormalizationService(service.researchNormalizer)
	if err != nil {
		return nil, fmt.Errorf("assemble live research normalization: %w", err)
	}
	if store == nil || store.Sources() == nil {
		return nil, fmt.Errorf("research source classification store is unavailable")
	}
	classified, err := researchapp.NewLiveSourceClassificationStage(stage, researchapp.NewDeterministicSourceClassifierV1(), store.Sources())
	if err != nil {
		return nil, fmt.Errorf("assemble live research source classification: %w", err)
	}
	return classified, nil
}
