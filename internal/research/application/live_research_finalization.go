package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

type liveResearchFinalizationStage struct{ bundles SourceBundleService }

func NewLiveResearchFinalizationStage(bundles SourceBundleService) (LiveResearchStageService, error) {
	const operation = "configure live research finalization stage"
	if err := requireDependency(operation, "source bundle service", bundles); err != nil {
		return nil, err
	}
	return &liveResearchFinalizationStage{bundles: bundles}, nil
}

func (stage *liveResearchFinalizationStage) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	const operation = "prepare live research finalization"
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	if ctx == nil {
		return artifacts, invalid(operation, errors.New("context is nil"))
	}
	if artifacts.Bundle == nil {
		return artifacts, invalid(operation, errors.New("live research finalization has no bundle"))
	}
	stored, err := stage.bundles.Get(ctx, artifacts.Bundle.ID)
	if err != nil {
		return artifacts, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	if stored.RunID != input.Run.ID || stored.ContentHash != artifacts.Bundle.ContentHash || stored.AlgorithmVersion != research.SourceBundleAlgorithmV1 {
		return artifacts, invalid(operation, fmt.Errorf("durable bundle does not match live research run"))
	}
	bundle := cloneSourceBundleArtifact(stored)
	artifacts.Bundle = &bundle
	return artifacts, nil
}

var _ LiveResearchStageService = (*liveResearchFinalizationStage)(nil)
