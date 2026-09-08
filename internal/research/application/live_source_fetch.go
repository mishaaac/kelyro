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
	LiveSourceFetchV2                   = "live-source-fetch-v2"
	LiveSourceFetchAllocationV1         = "live-source-fetch-allocation-v1"
	DefaultLiveSourceMaximumBytes int64 = 2 << 20
)

type LiveSourceFetchAllocation struct {
	MaximumFetches   int
	MaximumBytes     int64
	AlgorithmVersion string
}

func (allocation LiveSourceFetchAllocation) Validate(limits ResearchProcessingLimits) error {
	if allocation.MaximumFetches < 1 || allocation.MaximumFetches > limits.MaxFetches {
		return fmt.Errorf("allocated fetches must be between 1 and %d", limits.MaxFetches)
	}
	if allocation.MaximumBytes < 1 || allocation.MaximumBytes > limits.MaxFetchedBytes {
		return fmt.Errorf("allocated fetch bytes must be between 1 and %d", limits.MaxFetchedBytes)
	}
	if allocation.AlgorithmVersion != LiveSourceFetchAllocationV1 {
		return fmt.Errorf("live fetch allocation algorithm must be %q", LiveSourceFetchAllocationV1)
	}
	return nil
}

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
	if result.AlgorithmVersion != LiveSourceFetchV1 && result.AlgorithmVersion != LiveSourceFetchV2 {
		return fmt.Errorf("invalid live source fetch algorithm %q", result.AlgorithmVersion)
	}
	return nil
}

type liveSourceFetchService struct {
	fetch                 FetchService
	limits                ResearchProcessingLimits
	maximumBytesPerSource int64
	allocation            LiveSourceFetchAllocation
	algorithmVersion      string
}

func NewLiveSourceFetchService(fetch FetchService, limits ResearchProcessingLimits, maximumBytesPerSource int64, allocations ...LiveSourceFetchAllocation) (LiveSourceFetchService, error) {
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
	if len(allocations) > 1 {
		return nil, invalid(operation, errors.New("more than one live fetch allocation was provided"))
	}
	allocation := LiveSourceFetchAllocation{
		MaximumFetches: limits.MaxFetches, MaximumBytes: limits.MaxFetchedBytes,
		AlgorithmVersion: LiveSourceFetchAllocationV1,
	}
	algorithmVersion := LiveSourceFetchV1
	if len(allocations) == 1 {
		allocation = allocations[0]
		algorithmVersion = LiveSourceFetchV2
	}
	if err := allocation.Validate(limits); err != nil {
		return nil, invalid(operation, err)
	}
	return &liveSourceFetchService{
		fetch: fetch, limits: limits, maximumBytesPerSource: maximumBytesPerSource,
		allocation: allocation, algorithmVersion: algorithmVersion,
	}, nil
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
	selectedCount := min(len(request.Sources), service.allocation.MaximumFetches, int(service.allocation.MaximumBytes))
	maximumEach := min(service.maximumBytesPerSource, service.allocation.MaximumBytes/int64(selectedCount))
	requests := make([]FetchRequest, selectedCount)
	for index, source := range request.Sources[:selectedCount] {
		requests[index] = FetchRequest{SourceID: source.ID, Locator: source.Locator, MaximumBytes: maximumEach}
	}
	outputs, failures := service.fetchAll(ctx, request.Mode, requests)
	result := LiveSourceFetchResult{
		RequestedCount: len(request.Sources), MaximumBytesEach: maximumEach,
		AlgorithmVersion: service.algorithmVersion,
	}
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
	for _, source := range request.Sources[selectedCount:] {
		result.Failures = append(result.Failures, SourceFetchFailure{
			SourceID: source.ID, Locator: source.Locator, Kind: ErrorBudgetExceeded,
		})
	}
	if result.FetchedBytes > service.allocation.MaximumBytes {
		return LiveSourceFetchResult{}, invalid(operation, errors.New("fetched bytes exceed live allocation"))
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
	artifacts.FetchAlgorithmVersion = result.AlgorithmVersion
	return artifacts, err
}

var (
	_ LiveSourceFetchService   = (*liveSourceFetchService)(nil)
	_ LiveResearchStageService = (*liveSourceFetchService)(nil)
)
