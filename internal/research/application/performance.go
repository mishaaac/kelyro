package application

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/mishaaac/kelyro/internal/research"
)

const (
	ResearchProcessingLimitsV1             = "research-processing-limits-v1"
	MaximumConcurrentDiscovery             = 16
	MaximumConcurrentFetch                 = 32
	MaximumDiscoveryQueriesPerRun          = 100
	MaximumDiscoveryCandidatesPerRun       = 500
	MaximumFetchesPerRun                   = 200
	MaximumClaimsPerRun                    = 5_000
	MaximumFetchedBytesPerRun        int64 = 64 << 20
)

// ResearchProcessingLimits controls one bounded in-process run. Custom values
// may reduce but never raise the immutable v1 hard ceilings.
type ResearchProcessingLimits struct {
	MaxConcurrentDiscovery int
	MaxConcurrentFetch     int
	MaxDiscoveryQueries    int
	MaxCandidates          int
	MaxFetches             int
	MaxClaims              int
	MaxFetchedBytes        int64
	AlgorithmVersion       string
}

func DefaultResearchProcessingLimitsV1() ResearchProcessingLimits {
	return ResearchProcessingLimits{
		MaxConcurrentDiscovery: 4,
		MaxConcurrentFetch:     8,
		MaxDiscoveryQueries:    MaximumDiscoveryQueriesPerRun,
		MaxCandidates:          MaximumDiscoveryCandidatesPerRun,
		MaxFetches:             MaximumFetchesPerRun,
		MaxClaims:              MaximumClaimsPerRun,
		MaxFetchedBytes:        MaximumFetchedBytesPerRun,
		AlgorithmVersion:       ResearchProcessingLimitsV1,
	}
}

func (limits ResearchProcessingLimits) Validate() error {
	if limits.AlgorithmVersion != ResearchProcessingLimitsV1 {
		return fmt.Errorf("research processing limits algorithm must be %q", ResearchProcessingLimitsV1)
	}
	for _, item := range []struct {
		name       string
		value, max int
	}{
		{"concurrent discovery", limits.MaxConcurrentDiscovery, MaximumConcurrentDiscovery},
		{"concurrent fetch", limits.MaxConcurrentFetch, MaximumConcurrentFetch},
		{"discovery queries", limits.MaxDiscoveryQueries, MaximumDiscoveryQueriesPerRun},
		{"discovery candidates", limits.MaxCandidates, MaximumDiscoveryCandidatesPerRun},
		{"fetches", limits.MaxFetches, MaximumFetchesPerRun},
		{"claims", limits.MaxClaims, MaximumClaimsPerRun},
	} {
		if item.value < 1 || item.value > item.max {
			return fmt.Errorf("research maximum %s must be between 1 and %d", item.name, item.max)
		}
	}
	if limits.MaxFetchedBytes < 1 || limits.MaxFetchedBytes > MaximumFetchedBytesPerRun {
		return fmt.Errorf("research maximum fetched bytes must be between 1 and %d", MaximumFetchedBytesPerRun)
	}
	return nil
}

type DiscoveryWorkItem struct {
	Query   SearchQuery
	Options SearchOptions
}

func (item DiscoveryWorkItem) Validate() error {
	if err := item.Query.Validate(); err != nil {
		return err
	}
	return item.Options.Validate()
}

// ResearchProcessingRequest describes independently prepared work for one run.
// Fetch inputs are explicit because discovery candidates remain non-evidence
// until a caller selects and registers them.
type ResearchProcessingRequest struct {
	RunID       research.ID
	Mode        ResearchMode
	Discoveries []DiscoveryWorkItem
	Fetches     []FetchRequest
	ClaimCount  int
}

type ResearchProcessingResult struct {
	Discoveries      [][]SearchResult
	Fetches          []FetchedSource
	CandidateCount   int
	FetchedBytes     int64
	AlgorithmVersion string
}

type ResearchProcessingService interface {
	Process(context.Context, ResearchProcessingRequest) (ResearchProcessingResult, error)
}

type researchProcessingService struct {
	discovery DiscoveryService
	fetch     FetchService
	limits    ResearchProcessingLimits
}

func NewResearchProcessingService(discovery DiscoveryService, fetch FetchService, limits ResearchProcessingLimits) (ResearchProcessingService, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	return &researchProcessingService{discovery: discovery, fetch: fetch, limits: limits}, nil
}

func (service *researchProcessingService) Process(ctx context.Context, request ResearchProcessingRequest) (ResearchProcessingResult, error) {
	const operation = "process bounded research run"
	if err := service.validateRequest(request); err != nil {
		return ResearchProcessingResult{}, invalid(operation, err)
	}
	if ctx == nil {
		return ResearchProcessingResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return ResearchProcessingResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if len(request.Discoveries) > 0 {
		if err := requireDependency(operation, "discovery service", service.discovery); err != nil {
			return ResearchProcessingResult{}, err
		}
	}
	if len(request.Fetches) > 0 {
		if err := requireDependency(operation, "fetch service", service.fetch); err != nil {
			return ResearchProcessingResult{}, err
		}
	}

	result := ResearchProcessingResult{AlgorithmVersion: ResearchProcessingLimitsV1}
	discoveries, err := boundedMap(ctx, service.limits.MaxConcurrentDiscovery, request.Discoveries,
		func(ctx context.Context, item DiscoveryWorkItem) ([]SearchResult, error) {
			return service.discovery.Search(ctx, request.Mode, item.Query, item.Options)
		})
	if err != nil {
		return ResearchProcessingResult{}, indexedWorkError(operation, "discovery", err)
	}
	result.Discoveries = discoveries
	for _, candidates := range discoveries {
		if len(candidates) > service.limits.MaxCandidates-result.CandidateCount {
			return ResearchProcessingResult{}, invalid(operation,
				fmt.Errorf("discovery candidates exceed run limit %d", service.limits.MaxCandidates))
		}
		result.CandidateCount += len(candidates)
	}

	fetches, err := boundedMap(ctx, service.limits.MaxConcurrentFetch, request.Fetches,
		func(ctx context.Context, item FetchRequest) (FetchedSource, error) {
			return service.fetch.Fetch(ctx, request.Mode, item)
		})
	if err != nil {
		return ResearchProcessingResult{}, indexedWorkError(operation, "fetch", err)
	}
	result.Fetches = fetches
	for _, fetched := range fetches {
		bytes := int64(len(fetched.Body))
		if bytes > service.limits.MaxFetchedBytes-result.FetchedBytes {
			return ResearchProcessingResult{}, invalid(operation,
				fmt.Errorf("fetched bytes exceed run limit %d", service.limits.MaxFetchedBytes))
		}
		result.FetchedBytes += bytes
	}
	return result, nil
}

func (service *researchProcessingService) validateRequest(request ResearchProcessingRequest) error {
	if err := request.RunID.Validate(); err != nil {
		return fmt.Errorf("research processing run: %w", err)
	}
	if err := request.Mode.Validate(); err != nil {
		return err
	}
	if len(request.Discoveries) == 0 && len(request.Fetches) == 0 {
		return errors.New("research processing request has no discovery or fetch work")
	}
	if len(request.Discoveries) > service.limits.MaxDiscoveryQueries {
		return fmt.Errorf("discovery queries exceed run limit %d", service.limits.MaxDiscoveryQueries)
	}
	if len(request.Fetches) > service.limits.MaxFetches {
		return fmt.Errorf("fetches exceed run limit %d", service.limits.MaxFetches)
	}
	if request.ClaimCount < 0 || request.ClaimCount > service.limits.MaxClaims {
		return fmt.Errorf("claims exceed run limit %d", service.limits.MaxClaims)
	}
	var requestedBytes int64
	requestedCandidates := 0
	for index, item := range request.Discoveries {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("discovery item %d: %w", index, err)
		}
		if item.Options.Limit > service.limits.MaxCandidates-requestedCandidates {
			return fmt.Errorf("requested discovery candidates exceed run limit %d", service.limits.MaxCandidates)
		}
		requestedCandidates += item.Options.Limit
	}
	for index, item := range request.Fetches {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("fetch item %d: %w", index, err)
		}
		if item.MaximumBytes > service.limits.MaxFetchedBytes-requestedBytes {
			return fmt.Errorf("requested fetch bytes exceed run limit %d", service.limits.MaxFetchedBytes)
		}
		requestedBytes += item.MaximumBytes
	}
	return nil
}

type indexedWorkFailure struct {
	index int
	err   error
}

func (failure indexedWorkFailure) Error() string { return failure.err.Error() }
func (failure indexedWorkFailure) Unwrap() error { return failure.err }

func boundedMap[I, O any](ctx context.Context, concurrency int, inputs []I, work func(context.Context, I) (O, error)) ([]O, error) {
	outputs := make([]O, len(inputs))
	if len(inputs) == 0 {
		return outputs, nil
	}
	jobs := make(chan int, len(inputs))
	for index := range inputs {
		jobs <- index
	}
	close(jobs)
	errorsByIndex := make([]error, len(inputs))
	workers := min(concurrency, len(inputs))
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			for index := range jobs {
				if err := ctx.Err(); err != nil {
					errorsByIndex[index] = err
					continue
				}
				outputs[index], errorsByIndex[index] = work(ctx, inputs[index])
			}
		}()
	}
	wait.Wait()
	for index, err := range errorsByIndex {
		if err != nil {
			return nil, indexedWorkFailure{index: index, err: err}
		}
	}
	return outputs, nil
}

func indexedWorkError(operation, kind string, err error) error {
	var failure indexedWorkFailure
	if !errors.As(err, &failure) {
		return boundaryError(ErrorUnavailable, operation, err)
	}
	cause := fmt.Errorf("%s item %d: %w", kind, failure.index, failure.err)
	if classified, ok := KindOf(failure.err); ok {
		return Classify(classified, operation, cause)
	}
	return boundaryError(ErrorUnavailable, operation, cause)
}

var _ ResearchProcessingService = (*researchProcessingService)(nil)
