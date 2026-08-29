package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestResearchProcessingUsesSeparateBoundedPoolsAndStableOrdering(t *testing.T) {
	t.Parallel()
	discovery := newPoolDiscovery(4)
	fetch := newPoolFetch(3)
	limits := application.DefaultResearchProcessingLimitsV1()
	limits.MaxConcurrentDiscovery = 4
	limits.MaxConcurrentFetch = 3
	service, err := application.NewResearchProcessingService(discovery, fetch, limits)
	if err != nil {
		t.Fatal(err)
	}
	request := processingRequest(t, 12, 9, 9)
	result, err := service.Process(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if discovery.maximum.Load() != 4 || fetch.maximum.Load() != 3 {
		t.Fatalf("maximum concurrency discovery=%d fetch=%d", discovery.maximum.Load(), fetch.maximum.Load())
	}
	if discovery.active.Load() != 0 || fetch.active.Load() != 0 {
		t.Fatalf("workers leaked active calls discovery=%d fetch=%d", discovery.active.Load(), fetch.active.Load())
	}
	if len(result.Discoveries) != 12 || len(result.Fetches) != 9 || result.CandidateCount != 12 || result.FetchedBytes != 9 ||
		result.AlgorithmVersion != application.ResearchProcessingLimitsV1 {
		t.Fatalf("processing result = %+v", result)
	}
	for index := range result.Discoveries {
		if got := result.Discoveries[index][0].Title; got != fmt.Sprintf("query-%03d", index) {
			t.Fatalf("discovery order %d = %q", index, got)
		}
	}
	for index := range result.Fetches {
		if got := result.Fetches[index].SourceID.String(); got != fmt.Sprintf("source.batch-%03d", index) {
			t.Fatalf("fetch order %d = %q", index, got)
		}
	}
}

func TestResearchProcessingCancellationReleasesWorkers(t *testing.T) {
	t.Parallel()
	discovery := &blockingDiscovery{started: make(chan struct{}, 8)}
	service, err := application.NewResearchProcessingService(discovery, nil, application.DefaultResearchProcessingLimitsV1())
	if err != nil {
		t.Fatal(err)
	}
	request := processingRequest(t, 8, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, processErr := service.Process(ctx, request)
		done <- processErr
	}()
	for range 4 {
		select {
		case <-discovery.started:
		case <-time.After(time.Second):
			t.Fatal("bounded workers did not start")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, application.ErrUnavailable) {
			t.Fatalf("canceled processing error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled processing did not return")
	}
	if discovery.active.Load() != 0 {
		t.Fatalf("active discovery calls after cancellation = %d", discovery.active.Load())
	}
}

func TestResearchProcessingEnforcesRunBudgetsBeforeWork(t *testing.T) {
	t.Parallel()
	discovery := newPoolDiscovery(1)
	fetch := newPoolFetch(1)
	limits := application.DefaultResearchProcessingLimitsV1()
	limits.MaxDiscoveryQueries = 2
	limits.MaxCandidates = 1
	limits.MaxFetches = 2
	limits.MaxClaims = 3
	limits.MaxFetchedBytes = 2
	service, err := application.NewResearchProcessingService(discovery, fetch, limits)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		request application.ResearchProcessingRequest
	}{
		{name: "queries", request: processingRequest(t, 3, 0, 0)},
		{name: "fetches", request: processingRequest(t, 0, 3, 0)},
		{name: "claims", request: processingRequest(t, 1, 0, 4)},
		{name: "requested bytes", request: processingRequest(t, 0, 2, 0)},
	}
	tests[3].request.Fetches[0].MaximumBytes = 2
	tests[3].request.Fetches[1].MaximumBytes = 1
	for _, test := range tests {
		if _, err := service.Process(context.Background(), test.request); !errors.Is(err, application.ErrInvalidState) {
			t.Errorf("%s budget error = %v", test.name, err)
		}
	}
	if discovery.calls.Load() != 0 || fetch.calls.Load() != 0 {
		t.Fatalf("invalid workload reached dependencies discovery=%d fetch=%d", discovery.calls.Load(), fetch.calls.Load())
	}

	request := processingRequest(t, 2, 0, 0)
	if _, err := service.Process(context.Background(), request); !errors.Is(err, application.ErrInvalidState) || !strings.Contains(err.Error(), "candidates") {
		t.Fatalf("aggregate candidate budget error = %v", err)
	}
}

func TestResearchProcessingReportsLowestFailingInputDeterministically(t *testing.T) {
	t.Parallel()
	discovery := &failingDiscovery{fail: map[string]error{"query-001": errors.New("one"), "query-004": errors.New("four")}}
	service, err := application.NewResearchProcessingService(discovery, nil, application.DefaultResearchProcessingLimitsV1())
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Process(context.Background(), processingRequest(t, 6, 0, 0))
	if !errors.Is(err, application.ErrUnavailable) || !strings.Contains(err.Error(), "discovery item 1") {
		t.Fatalf("deterministic error = %v", err)
	}
}

func TestResearchProcessingLimitConfigurationCannotRaiseHardCeilings(t *testing.T) {
	t.Parallel()
	limits := application.DefaultResearchProcessingLimitsV1()
	limits.MaxConcurrentFetch = application.MaximumConcurrentFetch + 1
	if _, err := application.NewResearchProcessingService(nil, nil, limits); err == nil {
		t.Fatal("processing service accepted concurrency above hard ceiling")
	}
}

func TestReleaseIngestionBatchBoundsMatchRunClaimCeiling(t *testing.T) {
	t.Parallel()
	if application.MaximumClaimsPerIngestionBatch != application.MaximumClaimsPerRun {
		t.Fatalf("claim batch ceiling = %d, run ceiling = %d", application.MaximumClaimsPerIngestionBatch, application.MaximumClaimsPerRun)
	}
	batch := application.ReleaseIngestionBatch{
		Claims: make([]research.Claim, application.MaximumClaimsPerIngestionBatch+1),
	}
	if err := batch.ValidateBounds(); err == nil || !strings.Contains(err.Error(), "claims") {
		t.Fatalf("oversized claim batch error = %v", err)
	}
}

func BenchmarkResearchProcessingStep45Fixture(b *testing.B) {
	discovery := &fixtureDiscovery{}
	fetch := &fixtureFetch{}
	service, err := application.NewResearchProcessingService(discovery, fetch, application.DefaultResearchProcessingLimitsV1())
	if err != nil {
		b.Fatal(err)
	}
	request := processingRequest(b, 100, 200, 5_000)
	for index := range request.Discoveries {
		request.Discoveries[index].Options.Limit = 5
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		result, err := service.Process(context.Background(), request)
		if err != nil {
			b.Fatal(err)
		}
		if result.CandidateCount != 500 || len(result.Fetches) != 200 {
			b.Fatalf("fixture result candidates=%d fetches=%d", result.CandidateCount, len(result.Fetches))
		}
	}
}

type testingTB interface {
	Helper()
	Fatal(...any)
}

func processingRequest(tb testingTB, discoveries, fetches, claims int) application.ResearchProcessingRequest {
	tb.Helper()
	runID, err := research.NewID("run.performance-fixture")
	if err != nil {
		tb.Fatal(err)
	}
	requestID, err := research.NewID("request.performance-fixture")
	if err != nil {
		tb.Fatal(err)
	}
	request := application.ResearchProcessingRequest{RunID: runID, Mode: application.ResearchModeOnline, ClaimCount: claims}
	for index := range discoveries {
		request.Discoveries = append(request.Discoveries, application.DiscoveryWorkItem{
			Query:   application.SearchQuery{RequestID: requestID, Text: fmt.Sprintf("query-%03d", index)},
			Options: application.SearchOptions{Limit: 1},
		})
	}
	for index := range fetches {
		sourceID, sourceErr := research.NewSourceID(fmt.Sprintf("source.batch-%03d", index))
		if sourceErr != nil {
			tb.Fatal(sourceErr)
		}
		locator, locatorErr := research.NewSourceLocator(fmt.Sprintf("https://host-%03d.example.test/source", index%10))
		if locatorErr != nil {
			tb.Fatal(locatorErr)
		}
		request.Fetches = append(request.Fetches, application.FetchRequest{SourceID: sourceID, Locator: locator, MaximumBytes: 1})
	}
	return request
}

type concurrencyProbe struct {
	active  atomic.Int64
	maximum atomic.Int64
	calls   atomic.Int64
	gate    chan struct{}
	once    sync.Once
	want    int64
}

func newConcurrencyProbe(want int) concurrencyProbe {
	return concurrencyProbe{gate: make(chan struct{}), want: int64(want)}
}

func (probe *concurrencyProbe) enter() {
	probe.calls.Add(1)
	active := probe.active.Add(1)
	for {
		maximum := probe.maximum.Load()
		if active <= maximum || probe.maximum.CompareAndSwap(maximum, active) {
			break
		}
	}
	if active == probe.want {
		probe.once.Do(func() { close(probe.gate) })
	}
	<-probe.gate
}

func (probe *concurrencyProbe) leave() { probe.active.Add(-1) }

type poolDiscovery struct{ concurrencyProbe }

func newPoolDiscovery(want int) *poolDiscovery {
	return &poolDiscovery{concurrencyProbe: newConcurrencyProbe(want)}
}

func (service *poolDiscovery) Search(_ context.Context, _ application.ResearchMode, query application.SearchQuery, _ application.SearchOptions) ([]application.SearchResult, error) {
	service.enter()
	defer service.leave()
	return []application.SearchResult{{Title: query.Text}}, nil
}

type poolFetch struct{ concurrencyProbe }

func newPoolFetch(want int) *poolFetch {
	return &poolFetch{concurrencyProbe: newConcurrencyProbe(want)}
}

func (service *poolFetch) Fetch(_ context.Context, _ application.ResearchMode, request application.FetchRequest) (application.FetchedSource, error) {
	service.enter()
	defer service.leave()
	return application.FetchedSource{SourceID: request.SourceID, Body: []byte("x")}, nil
}

type blockingDiscovery struct {
	started chan struct{}
	active  atomic.Int64
}

func (service *blockingDiscovery) Search(ctx context.Context, _ application.ResearchMode, _ application.SearchQuery, _ application.SearchOptions) ([]application.SearchResult, error) {
	service.active.Add(1)
	defer service.active.Add(-1)
	service.started <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}

type failingDiscovery struct{ fail map[string]error }

func (service *failingDiscovery) Search(_ context.Context, _ application.ResearchMode, query application.SearchQuery, _ application.SearchOptions) ([]application.SearchResult, error) {
	return nil, service.fail[query.Text]
}

type fixtureDiscovery struct{}

func (*fixtureDiscovery) Search(_ context.Context, _ application.ResearchMode, query application.SearchQuery, options application.SearchOptions) ([]application.SearchResult, error) {
	results := make([]application.SearchResult, options.Limit)
	for index := range results {
		results[index].Title = query.Text
	}
	return results, nil
}

type fixtureFetch struct{}

func (*fixtureFetch) Fetch(_ context.Context, _ application.ResearchMode, request application.FetchRequest) (application.FetchedSource, error) {
	return application.FetchedSource{SourceID: request.SourceID, Body: []byte("x")}, nil
}

var _ application.DiscoveryService = (*poolDiscovery)(nil)
var _ application.FetchService = (*poolFetch)(nil)
