package application

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/mishaaac/kelyro/internal/research"
)

const LiveSourceSnapshotV1 = "live-source-snapshot-v1"

type SourceSnapshotFailure struct {
	SourceID research.SourceID
	Locator  research.SourceLocator
	Kind     ErrorKind
}

func (failure SourceSnapshotFailure) Validate() error {
	return SourceFetchFailure{SourceID: failure.SourceID, Locator: failure.Locator, Kind: failure.Kind}.Validate()
}

type LiveSourceSnapshotRequest struct {
	Fetched      []FetchedSource
	Sources      []research.Source
	MaximumBytes int64
}

type LiveSourceSnapshotResult struct {
	Snapshots           []research.SourceSnapshot
	NormalizationInputs []FetchedSource
	SnapshotFailures    []SourceSnapshotFailure
	CacheFailures       []SourceSnapshotFailure
	CacheWrites         int
	CacheHits           int
	CacheSuppressed     int
	AlgorithmVersion    string
}

func (result LiveSourceSnapshotResult) Validate() error {
	if len(result.Snapshots) == 0 && len(result.SnapshotFailures) == 0 {
		return errors.New("live source snapshot result is empty")
	}
	if len(result.Snapshots)+len(result.SnapshotFailures) > MaximumFetchesPerRun || result.CacheWrites < 0 ||
		result.CacheHits < 0 || result.CacheSuppressed < 0 || result.CacheWrites+result.CacheHits+result.CacheSuppressed > len(result.Snapshots) {
		return errors.New("live source snapshot counts are inconsistent")
	}
	seenSnapshots := make(map[research.SourceID]struct{}, len(result.Snapshots))
	for index, snapshot := range result.Snapshots {
		if err := snapshot.Validate(); err != nil {
			return fmt.Errorf("live source snapshot %d: %w", index, err)
		}
		if _, duplicate := seenSnapshots[snapshot.SourceID]; duplicate {
			return fmt.Errorf("live source snapshot repeats source %q", snapshot.SourceID)
		}
		seenSnapshots[snapshot.SourceID] = struct{}{}
	}
	seenInputs := make(map[research.SourceID]struct{}, len(result.NormalizationInputs))
	for index, input := range result.NormalizationInputs {
		if err := input.Validate(); err != nil {
			return fmt.Errorf("snapshot normalization input %d: %w", index, err)
		}
		if _, exists := seenSnapshots[input.SourceID]; !exists {
			return fmt.Errorf("snapshot normalization input %q has no snapshot", input.SourceID)
		}
		if _, duplicate := seenInputs[input.SourceID]; duplicate {
			return fmt.Errorf("snapshot normalization input repeats source %q", input.SourceID)
		}
		seenInputs[input.SourceID] = struct{}{}
	}
	for index, failure := range append(append([]SourceSnapshotFailure(nil), result.SnapshotFailures...), result.CacheFailures...) {
		if err := failure.Validate(); err != nil {
			return fmt.Errorf("live source snapshot failure %d: %w", index, err)
		}
	}
	if result.AlgorithmVersion != LiveSourceSnapshotV1 {
		return fmt.Errorf("live source snapshot algorithm must be %q", LiveSourceSnapshotV1)
	}
	return nil
}

type liveSourceSnapshotService struct {
	snapshots SnapshotCaptureService
	cache     SourceFetchCacheAdapter
}

func NewLiveSourceSnapshotService(snapshots SnapshotCaptureService, cache SourceFetchCacheAdapter) (LiveSourceSnapshotService, error) {
	const operation = "configure live source snapshot"
	if err := requireDependency(operation, "snapshot capture service", snapshots); err != nil {
		return nil, err
	}
	if err := requireDependency(operation, "source fetch cache adapter", cache); err != nil {
		return nil, err
	}
	return &liveSourceSnapshotService{snapshots: snapshots, cache: cache}, nil
}

func (service *liveSourceSnapshotService) SnapshotSources(ctx context.Context, request LiveSourceSnapshotRequest) (LiveSourceSnapshotResult, error) {
	const operation = "snapshot live research sources"
	if ctx == nil {
		return LiveSourceSnapshotResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveSourceSnapshotResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if len(request.Fetched) == 0 || len(request.Fetched) > MaximumFetchesPerRun {
		return LiveSourceSnapshotResult{}, invalid(operation, fmt.Errorf("fetched source count must be between 1 and %d", MaximumFetchesPerRun))
	}
	if request.MaximumBytes < 1 || request.MaximumBytes > MaximumFetchedBytesPerRun {
		return LiveSourceSnapshotResult{}, invalid(operation, errors.New("snapshot maximum bytes are invalid"))
	}
	sources := make(map[research.SourceID]research.Source, len(request.Sources))
	for index, source := range request.Sources {
		if err := source.Validate(); err != nil {
			return LiveSourceSnapshotResult{}, invalid(operation, fmt.Errorf("registered source %d: %w", index, err))
		}
		if _, duplicate := sources[source.ID]; duplicate {
			return LiveSourceSnapshotResult{}, invalid(operation, fmt.Errorf("registered source %q is repeated", source.ID))
		}
		sources[source.ID] = source
	}
	seen := make(map[research.SourceID]struct{}, len(request.Fetched))
	for index, fetched := range request.Fetched {
		if err := fetched.Validate(); err != nil {
			return LiveSourceSnapshotResult{}, invalid(operation, fmt.Errorf("fetched source %d: %w", index, err))
		}
		if int64(len(fetched.Body)) > request.MaximumBytes {
			return LiveSourceSnapshotResult{}, invalid(operation, fmt.Errorf("fetched source %d exceeds snapshot maximum", index))
		}
		if _, duplicate := seen[fetched.SourceID]; duplicate {
			return LiveSourceSnapshotResult{}, invalid(operation, fmt.Errorf("fetched source %q is repeated", fetched.SourceID))
		}
		if _, exists := sources[fetched.SourceID]; !exists {
			return LiveSourceSnapshotResult{}, invalid(operation, fmt.Errorf("fetched source %q is not registered in stage artifacts", fetched.SourceID))
		}
		seen[fetched.SourceID] = struct{}{}
	}

	result := LiveSourceSnapshotResult{AlgorithmVersion: LiveSourceSnapshotV1}
	var firstFailure error
	for _, fetched := range request.Fetched {
		if err := ctx.Err(); err != nil {
			return cloneLiveSourceSnapshotResult(result), Classify(ErrorUnavailable, operation, err)
		}
		captureRequest := SnapshotCaptureRequest{SourceID: fetched.SourceID, MaximumBytes: request.MaximumBytes, BodyPolicy: SnapshotNormalizedExcerpt}
		capture, err := service.snapshots.CaptureFetched(ctx, fetched, captureRequest)
		if err != nil {
			if firstFailure == nil {
				firstFailure = err
			}
			result.SnapshotFailures = append(result.SnapshotFailures, snapshotFailure(fetched, err))
			continue
		}
		result.Snapshots = append(result.Snapshots, capture.Snapshot)
		if capture.NormalizationInput != nil {
			result.NormalizationInputs = append(result.NormalizationInputs, cloneFetchedSource(*capture.NormalizationInput))
		}
		cacheRequest := FetchRequest{SourceID: fetched.SourceID, Locator: sources[fetched.SourceID].Locator, MaximumBytes: request.MaximumBytes}
		switch {
		case fetched.Origin == FetchOriginCache:
			result.CacheHits++
		case fetched.NoStore:
			result.CacheSuppressed++
		case fetched.Metadata.StatusCode == http.StatusNotModified:
			cached, cacheErr := service.cache.FetchCached(ctx, cacheRequest)
			if cacheErr != nil {
				result.CacheFailures = append(result.CacheFailures, snapshotFailure(fetched, cacheErr))
				continue
			}
			reused, reuseErr := service.snapshots.CaptureFetched(ctx, cached, captureRequest)
			if reuseErr != nil || reused.Snapshot.ID != capture.Snapshot.ID || reused.NormalizationInput == nil {
				if reuseErr == nil {
					reuseErr = errors.New("cached 304 body does not match durable revalidation")
				}
				result.CacheFailures = append(result.CacheFailures, snapshotFailure(fetched, reuseErr))
				continue
			}
			result.CacheHits++
			result.NormalizationInputs = append(result.NormalizationInputs, cloneFetchedSource(*reused.NormalizationInput))
		case len(fetched.Body) > 0:
			if cacheErr := service.cache.CacheFetched(ctx, cacheRequest, fetched); cacheErr != nil {
				result.CacheFailures = append(result.CacheFailures, snapshotFailure(fetched, cacheErr))
			} else {
				result.CacheWrites++
			}
		}
	}
	if err := result.Validate(); err != nil {
		return LiveSourceSnapshotResult{}, invalid(operation, err)
	}
	if len(result.Snapshots) == 0 {
		return cloneLiveSourceSnapshotResult(result), boundaryError(ErrorPersistenceFailure, operation, firstFailure)
	}
	return cloneLiveSourceSnapshotResult(result), nil
}

func snapshotFailure(fetched FetchedSource, err error) SourceSnapshotFailure {
	kind, ok := KindOf(err)
	if !ok {
		kind = ErrorUnavailable
	}
	return SourceSnapshotFailure{SourceID: fetched.SourceID, Locator: fetched.Locator, Kind: kind}
}

func cloneLiveSourceSnapshotResult(result LiveSourceSnapshotResult) LiveSourceSnapshotResult {
	clone := result
	clone.Snapshots = append([]research.SourceSnapshot(nil), result.Snapshots...)
	clone.NormalizationInputs = make([]FetchedSource, len(result.NormalizationInputs))
	for index, input := range result.NormalizationInputs {
		clone.NormalizationInputs[index] = cloneFetchedSource(input)
	}
	clone.SnapshotFailures = append([]SourceSnapshotFailure(nil), result.SnapshotFailures...)
	clone.CacheFailures = append([]SourceSnapshotFailure(nil), result.CacheFailures...)
	return clone
}

func (service *liveSourceSnapshotService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.SnapshotSources(ctx, LiveSourceSnapshotRequest{
		Fetched: input.Artifacts.FetchedSources, Sources: input.Artifacts.Sources, MaximumBytes: input.Artifacts.FetchMaximumBytes,
	})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.Snapshots = append([]research.SourceSnapshot(nil), result.Snapshots...)
	artifacts.NormalizationInputs = make([]FetchedSource, len(result.NormalizationInputs))
	for index, fetched := range result.NormalizationInputs {
		artifacts.NormalizationInputs[index] = cloneFetchedSource(fetched)
	}
	artifacts.SnapshotFailures = append([]SourceSnapshotFailure(nil), result.SnapshotFailures...)
	artifacts.CacheFailures = append([]SourceSnapshotFailure(nil), result.CacheFailures...)
	return artifacts, err
}

var (
	_ LiveSourceSnapshotService = (*liveSourceSnapshotService)(nil)
	_ LiveResearchStageService  = (*liveSourceSnapshotService)(nil)
)
