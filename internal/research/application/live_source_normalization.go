package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

const LiveSourceNormalizationV1 = "live-source-normalization-v1"

type SourceNormalizationFailure struct {
	SourceID research.SourceID
	Locator  research.SourceLocator
	Kind     ErrorKind
}

func (failure SourceNormalizationFailure) Validate() error {
	return SourceFetchFailure{SourceID: failure.SourceID, Locator: failure.Locator, Kind: failure.Kind}.Validate()
}

type LiveSourceNormalizationRequest struct {
	Inputs    []FetchedSource
	Snapshots []research.SourceSnapshot
}

type LiveSourceNormalizationResult struct {
	Sources          []NormalizedSource
	Failures         []SourceNormalizationFailure
	AlgorithmVersion string
}

func (result LiveSourceNormalizationResult) Validate() error {
	if len(result.Sources) == 0 && len(result.Failures) == 0 {
		return errors.New("live source normalization result is empty")
	}
	if len(result.Sources)+len(result.Failures) > MaximumFetchesPerRun {
		return errors.New("live source normalization count exceeds run maximum")
	}
	seen := make(map[research.SourceID]struct{}, len(result.Sources)+len(result.Failures))
	for index, source := range result.Sources {
		if err := source.Validate(); err != nil {
			return fmt.Errorf("normalized source %d: %w", index, err)
		}
		if _, duplicate := seen[source.SourceID]; duplicate {
			return fmt.Errorf("normalization repeats source %q", source.SourceID)
		}
		seen[source.SourceID] = struct{}{}
	}
	for index, failure := range result.Failures {
		if err := failure.Validate(); err != nil {
			return fmt.Errorf("source normalization failure %d: %w", index, err)
		}
		if _, duplicate := seen[failure.SourceID]; duplicate {
			return fmt.Errorf("normalization repeats source %q", failure.SourceID)
		}
		seen[failure.SourceID] = struct{}{}
	}
	if result.AlgorithmVersion != LiveSourceNormalizationV1 {
		return fmt.Errorf("live source normalization algorithm must be %q", LiveSourceNormalizationV1)
	}
	return nil
}

type liveSourceNormalizationService struct {
	normalizer SourceNormalizer
}

func NewLiveSourceNormalizationService(normalizer SourceNormalizer) (LiveSourceNormalizationService, error) {
	const operation = "configure live source normalization"
	if err := requireDependency(operation, "source normalizer", normalizer); err != nil {
		return nil, err
	}
	return &liveSourceNormalizationService{normalizer: normalizer}, nil
}

func (service *liveSourceNormalizationService) NormalizeSources(ctx context.Context, request LiveSourceNormalizationRequest) (LiveSourceNormalizationResult, error) {
	const operation = "normalize live research sources"
	if ctx == nil {
		return LiveSourceNormalizationResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveSourceNormalizationResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if len(request.Inputs) == 0 || len(request.Inputs) > MaximumFetchesPerRun {
		return LiveSourceNormalizationResult{}, invalid(operation, fmt.Errorf("normalization input count must be between 1 and %d", MaximumFetchesPerRun))
	}

	snapshots := make(map[research.SourceID]research.SourceSnapshot, len(request.Snapshots))
	for index, snapshot := range request.Snapshots {
		if err := snapshot.Validate(); err != nil {
			return LiveSourceNormalizationResult{}, invalid(operation, fmt.Errorf("source snapshot %d: %w", index, err))
		}
		if _, duplicate := snapshots[snapshot.SourceID]; duplicate {
			return LiveSourceNormalizationResult{}, invalid(operation, fmt.Errorf("source snapshot %q is repeated", snapshot.SourceID))
		}
		snapshots[snapshot.SourceID] = snapshot
	}
	seen := make(map[research.SourceID]struct{}, len(request.Inputs))
	for index, input := range request.Inputs {
		if err := input.Validate(); err != nil {
			return LiveSourceNormalizationResult{}, invalid(operation, fmt.Errorf("normalization input %d: %w", index, err))
		}
		if _, duplicate := seen[input.SourceID]; duplicate {
			return LiveSourceNormalizationResult{}, invalid(operation, fmt.Errorf("normalization input %q is repeated", input.SourceID))
		}
		seen[input.SourceID] = struct{}{}
		snapshot, exists := snapshots[input.SourceID]
		if !exists || snapshot.Locator != input.Locator || snapshot.Fetch.ContentHash == "" ||
			snapshot.Fetch.ContentHash != input.Metadata.ContentHash {
			return LiveSourceNormalizationResult{}, invalid(operation, fmt.Errorf("normalization input %q does not match its durable snapshot", input.SourceID))
		}
	}

	result := LiveSourceNormalizationResult{AlgorithmVersion: LiveSourceNormalizationV1}
	var firstFailure error
	for _, input := range request.Inputs {
		if err := ctx.Err(); err != nil {
			return cloneLiveSourceNormalizationResult(result), Classify(ErrorUnavailable, operation, err)
		}
		normalized, err := service.normalizer.Normalize(ctx, cloneFetchedSource(input))
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return cloneLiveSourceNormalizationResult(result), Classify(ErrorUnavailable, operation, err)
			}
			if firstFailure == nil {
				firstFailure = err
			}
			result.Failures = append(result.Failures, normalizationFailure(input, err))
			continue
		}
		if normalized.SourceID != input.SourceID || normalized.Locator != input.Locator {
			err = errors.New("normalizer output does not match input identity")
			if firstFailure == nil {
				firstFailure = err
			}
			result.Failures = append(result.Failures, normalizationFailure(input, err))
			continue
		}
		if err := normalized.Validate(); err != nil {
			if firstFailure == nil {
				firstFailure = err
			}
			result.Failures = append(result.Failures, normalizationFailure(input, err))
			continue
		}
		result.Sources = append(result.Sources, cloneNormalizedSource(normalized))
	}
	if err := result.Validate(); err != nil {
		return LiveSourceNormalizationResult{}, invalid(operation, err)
	}
	if len(result.Sources) == 0 {
		return cloneLiveSourceNormalizationResult(result), boundaryError(ErrorInvalidState, operation, firstFailure)
	}
	return cloneLiveSourceNormalizationResult(result), nil
}

func normalizationFailure(input FetchedSource, err error) SourceNormalizationFailure {
	kind, ok := KindOf(err)
	if !ok {
		kind = ErrorInvalidState
	}
	return SourceNormalizationFailure{SourceID: input.SourceID, Locator: input.Locator, Kind: kind}
}

func cloneNormalizedSource(source NormalizedSource) NormalizedSource {
	clone := source
	if source.CanonicalLocator != nil {
		locator := *source.CanonicalLocator
		clone.CanonicalLocator = &locator
	}
	clone.Headings = make([]NormalizedHeading, len(source.Headings))
	for index, heading := range source.Headings {
		clone.Headings[index] = heading
		clone.Headings[index].Path = append([]string(nil), heading.Path...)
	}
	clone.TextSegments = append([]string(nil), source.TextSegments...)
	clone.CodeBlocks = append([]NormalizedCodeBlock(nil), source.CodeBlocks...)
	clone.Links = append([]NormalizedLink(nil), source.Links...)
	if source.PublishedAt != nil {
		published := *source.PublishedAt
		clone.PublishedAt = &published
	}
	if source.UpdatedAt != nil {
		updated := *source.UpdatedAt
		clone.UpdatedAt = &updated
	}
	clone.VersionHints = append([]string(nil), source.VersionHints...)
	return clone
}

func cloneLiveSourceNormalizationResult(result LiveSourceNormalizationResult) LiveSourceNormalizationResult {
	clone := result
	clone.Sources = make([]NormalizedSource, len(result.Sources))
	for index, source := range result.Sources {
		clone.Sources[index] = cloneNormalizedSource(source)
	}
	clone.Failures = append([]SourceNormalizationFailure(nil), result.Failures...)
	return clone
}

func (service *liveSourceNormalizationService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.NormalizeSources(ctx, LiveSourceNormalizationRequest{
		Inputs: input.Artifacts.NormalizationInputs, Snapshots: input.Artifacts.Snapshots,
	})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.NormalizedSources = make([]NormalizedSource, len(result.Sources))
	for index, source := range result.Sources {
		artifacts.NormalizedSources[index] = cloneNormalizedSource(source)
	}
	artifacts.NormalizationFailures = append([]SourceNormalizationFailure(nil), result.Failures...)
	return artifacts, err
}

var (
	_ LiveSourceNormalizationService = (*liveSourceNormalizationService)(nil)
	_ LiveResearchStageService       = (*liveSourceNormalizationService)(nil)
)
