package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

// liveSourceClassificationStage decorates the existing normalization stage so
// classification can use fetched content while preserving orchestrator order.
type liveSourceClassificationStage struct {
	normalization LiveResearchStageService
	classifier    SourceClassifier
	sources       SourceService
}

func NewLiveSourceClassificationStage(normalization LiveResearchStageService, classifier SourceClassifier, sources SourceService) (LiveResearchStageService, error) {
	const operation = "configure live source classification"
	for _, dependency := range []struct {
		name  string
		value any
	}{{"normalization stage", normalization}, {"source classifier", classifier}, {"source service", sources}} {
		if err := requireDependency(operation, dependency.name, dependency.value); err != nil {
			return nil, err
		}
	}
	return &liveSourceClassificationStage{normalization: normalization, classifier: classifier, sources: sources}, nil
}

func (stage *liveSourceClassificationStage) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	const operation = "classify normalized live research sources"
	artifacts, err := stage.normalization.Execute(ctx, input)
	if err != nil {
		return artifacts, err
	}
	if ctx == nil {
		return artifacts, invalid(operation, errors.New("context is nil"))
	}
	sourceIndexes := make(map[research.SourceID]int, len(artifacts.Sources))
	for index, source := range artifacts.Sources {
		if err := source.Validate(); err != nil {
			return artifacts, invalid(operation, fmt.Errorf("classification Source %d: %w", index, err))
		}
		sourceIndexes[source.ID] = index
	}
	artifacts.SourceClassifications = make([]SourceClassification, 0, len(artifacts.NormalizedSources))
	classifiedNormalized := make([]NormalizedSource, 0, len(artifacts.NormalizedSources))
	for index, normalized := range artifacts.NormalizedSources {
		if err := ctx.Err(); err != nil {
			return artifacts, Classify(ErrorUnavailable, operation, err)
		}
		sourceIndex, exists := sourceIndexes[normalized.SourceID]
		if !exists {
			return artifacts, invalid(operation, fmt.Errorf("normalized source %d has no registered Source", index))
		}
		source := artifacts.Sources[sourceIndex]
		if source.Locator != normalized.Locator {
			// Redirect aliases require their own durable identity workflow. Keep
			// this run safe and useful by treating only that input as a partial
			// normalization failure instead of relabeling a different locator.
			artifacts.NormalizationFailures = append(artifacts.NormalizationFailures, SourceNormalizationFailure{
				SourceID: normalized.SourceID, Locator: normalized.Locator, Kind: ErrorInvalidState,
			})
			continue
		}
		classification, classifyErr := stage.classifier.Classify(SourceClassificationRequest{Source: source, Normalized: normalized})
		if classifyErr != nil {
			return artifacts, invalid(operation, classifyErr)
		}
		if classification.Kind != source.Kind {
			if persistErr := stage.sources.ClassifyKind(ctx, source.ID, classification.Kind); persistErr != nil {
				return artifacts, boundaryError(ErrorPersistenceFailure, operation, persistErr)
			}
			source.Kind = classification.Kind
			artifacts.Sources[sourceIndex] = source
		}
		artifacts.SourceClassifications = append(artifacts.SourceClassifications, classification)
		classifiedNormalized = append(classifiedNormalized, normalized)
	}
	artifacts.NormalizedSources = classifiedNormalized
	if len(artifacts.NormalizedSources) == 0 {
		return artifacts, invalid(operation, errors.New("no normalized source retained a safe registered identity"))
	}
	return artifacts, nil
}

var _ LiveResearchStageService = (*liveSourceClassificationStage)(nil)
