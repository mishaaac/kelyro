package application

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/mishaaac/kelyro/internal/research"
)

const (
	LiveSourceFetchV1                   = "live-source-fetch-v1"
	DefaultLiveSourceMaximumBytes int64 = 2 << 20
)

type SourceFetchFailure struct {
	SourceID research.SourceID
	Locator  research.SourceLocator
	Kind     ErrorKind
}

func (failure SourceFetchFailure) Validate() error {
	if err := failure.SourceID.Validate(); err != nil {
		return fmt.Errorf("source fetch failure: %w", err)
	}
	if err := failure.Locator.Validate(); err != nil {
		return fmt.Errorf("source fetch failure locator: %w", err)
	}
	switch failure.Kind {
	case ErrorNotFound, ErrorConflict, ErrorInvalidState, ErrorUnavailable, ErrorPersistenceFailure,
		ErrorExternalFailure, ErrorNetworkResearchBlocked, ErrorBudgetExceeded:
		return nil
	default:
		return fmt.Errorf("invalid source fetch failure kind %q", failure.Kind)
	}
}

type LiveSourceFetchRequest struct {
	Mode    ResearchMode
	Sources []research.Source
}

type LiveSourceFetchResult struct {
	Fetched          []FetchedSource
	Failures         []SourceFetchFailure
	RequestedCount   int
	FetchedBytes     int64
	MaximumBytesEach int64
	AlgorithmVersion string
}

func (result LiveSourceFetchResult) Validate() error {
	if result.RequestedCount < 1 || result.RequestedCount > MaximumFetchesPerRun ||
		len(result.Fetched)+len(result.Failures) != result.RequestedCount {
		return errors.New("live source fetch counts are inconsistent")
	}
	if result.FetchedBytes < 0 || result.FetchedBytes > MaximumFetchedBytesPerRun || result.MaximumBytesEach < 1 ||
		result.MaximumBytesEach > MaximumFetchedBytesPerRun {
		return errors.New("live source fetch byte bounds are invalid")
	}
	var fetchedBytes int64
	seen := make(map[research.SourceID]struct{}, result.RequestedCount)
	for index, fetched := range result.Fetched {
		if err := fetched.Validate(); err != nil {
			return fmt.Errorf("live fetched source %d: %w", index, err)
		}
		if int64(len(fetched.Body)) > result.MaximumBytesEach {
			return fmt.Errorf("live fetched source %q exceeds allocated bytes", fetched.SourceID)
		}
		if _, duplicate := seen[fetched.SourceID]; duplicate {
			return fmt.Errorf("live source fetch repeats source %q", fetched.SourceID)
		}
		seen[fetched.SourceID] = struct{}{}
		fetchedBytes += int64(len(fetched.Body))
	}
	for index, failure := range result.Failures {
		if err := failure.Validate(); err != nil {
			return fmt.Errorf("live source fetch failure %d: %w", index, err)
		}
		if _, duplicate := seen[failure.SourceID]; duplicate {
			return fmt.Errorf("live source fetch repeats source %q", failure.SourceID)
		}
		seen[failure.SourceID] = struct{}{}
	}
	if fetchedBytes != result.FetchedBytes {
		return errors.New("live source fetched byte summary is inconsistent")
	}
	if result.AlgorithmVersion != LiveSourceFetchV1 {
		return fmt.Errorf("live source fetch algorithm must be %q", LiveSourceFetchV1)
	}
	return nil
}

type liveSourceFetchService struct {
	fetch                 FetchService
	limits                ResearchProcessingLimits
	maximumBytesPerSource int64
}

func NewLiveSourceFetchService(fetch FetchService, limits ResearchProcessingLimits, maximumBytesPerSource int64) (LiveSourceFetchService, error) {
	const operation = "configure live source fetch"
	if err := requireDependency(operation, "privacy-gated fetch service", fetch); err != nil {
		return nil, err
	}
	if err := limits.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if maximumBytesPerSource < 1 || maximumBytesPerSource > limits.MaxFetchedBytes {
		return nil, invalid(operation, fmt.Errorf("source fetch maximum bytes must be between 1 and %d", limits.MaxFetchedBytes))
	}
	return &liveSourceFetchService{fetch: fetch, limits: limits, maximumBytesPerSource: maximumBytesPerSource}, nil
}

func (service *liveSourceFetchService) FetchSources(ctx context.Context, request LiveSourceFetchRequest) (LiveSourceFetchResult, error) {
	const operation = "fetch live research sources"
	if ctx == nil {
		return LiveSourceFetchResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveSourceFetchResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.Mode.Validate(); err != nil {
		return LiveSourceFetchResult{}, invalid(operation, err)
	}
	if len(request.Sources) == 0 || len(request.Sources) > service.limits.MaxFetches {
		return LiveSourceFetchResult{}, invalid(operation, fmt.Errorf("source count must be between 1 and %d", service.limits.MaxFetches))
	}
	seen := make(map[research.SourceID]struct{}, len(request.Sources))
	for index, source := range request.Sources {
		if err := source.Validate(); err != nil {
			return LiveSourceFetchResult{}, invalid(operation, fmt.Errorf("source %d: %w", index, err))
		}
		if _, duplicate := seen[source.ID]; duplicate {
			return LiveSourceFetchResult{}, invalid(operation, fmt.Errorf("source %q is repeated", source.ID))
		}
		seen[source.ID] = struct{}{}
	}
	maximumEach := min(service.maximumBytesPerSource, service.limits.MaxFetchedBytes/int64(len(request.Sources)))
	requests := make([]FetchRequest, len(request.Sources))
	for index, source := range request.Sources {
		requests[index] = FetchRequest{SourceID: source.ID, Locator: source.Locator, MaximumBytes: maximumEach}
	}
	outputs, failures := service.fetchAll(ctx, request.Mode, requests)
	result := LiveSourceFetchResult{RequestedCount: len(requests), MaximumBytesEach: maximumEach, AlgorithmVersion: LiveSourceFetchV1}
	var firstFailure error
	for index, fetched := range outputs {
		if failures[index] != nil {
			if firstFailure == nil {
				firstFailure = failures[index]
			}
			kind, ok := KindOf(failures[index])
			if !ok {
				kind = ErrorUnavailable
			}
			result.Failures = append(result.Failures, SourceFetchFailure{SourceID: requests[index].SourceID, Locator: requests[index].Locator, Kind: kind})
			continue
		}
		result.Fetched = append(result.Fetched, cloneFetchedSource(fetched))
		result.FetchedBytes += int64(len(fetched.Body))
	}
	if err := result.Validate(); err != nil {
		return LiveSourceFetchResult{}, invalid(operation, err)
	}
	if err := ctx.Err(); err != nil {
		return cloneLiveSourceFetchResult(result), Classify(ErrorUnavailable, operation, err)
	}
	if len(result.Fetched) == 0 {
		return cloneLiveSourceFetchResult(result), boundaryError(ErrorUnavailable, operation, firstFailure)
	}
	return cloneLiveSourceFetchResult(result), nil
}

func (service *liveSourceFetchService) fetchAll(ctx context.Context, mode ResearchMode, requests []FetchRequest) ([]FetchedSource, []error) {
	outputs := make([]FetchedSource, len(requests))
	failures := make([]error, len(requests))
	jobs := make(chan int, len(requests))
	for index := range requests {
		jobs <- index
	}
	close(jobs)
	workers := min(service.limits.MaxConcurrentFetch, len(requests))
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			for index := range jobs {
				if err := ctx.Err(); err != nil {
					failures[index] = err
					continue
				}
				outputs[index], failures[index] = service.fetch.Fetch(ctx, mode, requests[index])
			}
		}()
	}
	wait.Wait()
	return outputs, failures
}

func cloneFetchedSource(fetched FetchedSource) FetchedSource {
	clone := fetched
	clone.Body = append([]byte(nil), fetched.Body...)
	return clone
}

func cloneLiveSourceFetchResult(result LiveSourceFetchResult) LiveSourceFetchResult {
	clone := result
	clone.Fetched = make([]FetchedSource, len(result.Fetched))
	for index, fetched := range result.Fetched {
		clone.Fetched[index] = cloneFetchedSource(fetched)
	}
	clone.Failures = append([]SourceFetchFailure(nil), result.Failures...)
	return clone
}

func (service *liveSourceFetchService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.FetchSources(ctx, LiveSourceFetchRequest{Mode: input.Mode, Sources: input.Artifacts.Sources})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.FetchedSources = make([]FetchedSource, len(result.Fetched))
	for index, fetched := range result.Fetched {
		artifacts.FetchedSources[index] = cloneFetchedSource(fetched)
	}
	artifacts.FetchFailures = append([]SourceFetchFailure(nil), result.Failures...)
	artifacts.FetchMaximumBytes = result.MaximumBytesEach
	return artifacts, err
}

var (
	_ LiveSourceFetchService   = (*liveSourceFetchService)(nil)
	_ LiveResearchStageService = (*liveSourceFetchService)(nil)
)
