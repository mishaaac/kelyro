package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestLiveSourceFetchUsesPrivacyGatedServiceAndAllowsPartialFailure(t *testing.T) {
	t.Parallel()
	first := testSource(t, "live-fetch-first")
	second := testSource(t, "live-fetch-second")
	fetcher := &selectiveSourceFetcher{failures: map[research.SourceID]error{second.ID: errors.New("fixture transport failure")}, at: testTimestamp(t, 12)}
	gate := &recordingNetworkGate{}
	fetch := application.NewFetchService(fetcher, nil, application.NetworkResearchAccess{Gate: gate})
	limits := application.DefaultResearchProcessingLimitsV1()
	limits.MaxConcurrentFetch = 1
	service, err := application.NewLiveSourceFetchService(fetch, limits, application.DefaultLiveSourceMaximumBytes)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.FetchSources(context.Background(), application.LiveSourceFetchRequest{
		Mode: application.ResearchModeOnline, Sources: []research.Source{first, second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequestedCount != 2 || len(result.Fetched) != 1 || len(result.Failures) != 1 || result.Fetched[0].SourceID != first.ID ||
		result.Failures[0].SourceID != second.ID || result.Failures[0].Kind != application.ErrorExternalFailure ||
		result.FetchedBytes != int64(len("body:"+first.ID.String())) || result.AlgorithmVersion != application.LiveSourceFetchV1 {
		t.Fatalf("partial fetch result = %+v", result)
	}
	if len(gate.requests) != 2 || fetcher.callCount() != 2 {
		t.Fatalf("fetch boundaries gate=%d adapter=%d", len(gate.requests), fetcher.callCount())
	}
	for _, request := range fetcher.requestsCopy() {
		if request.MaximumBytes != application.DefaultLiveSourceMaximumBytes {
			t.Fatalf("fetch maximum bytes = %d", request.MaximumBytes)
		}
	}
	result.Fetched[0].Body[0] = 'X'
	again, err := service.FetchSources(context.Background(), application.LiveSourceFetchRequest{
		Mode: application.ResearchModeOnline, Sources: []research.Source{first},
	})
	if err != nil || string(again.Fetched[0].Body) != "body:"+first.ID.String() {
		t.Fatalf("fetch result leaked body ownership = (%q,%v)", again.Fetched[0].Body, err)
	}
}

func TestLiveSourceFetchAllBlockedIsTerminalAndNeverCallsAdapter(t *testing.T) {
	t.Parallel()
	first := testSource(t, "live-fetch-blocked-first")
	second := testSource(t, "live-fetch-blocked-second")
	fetcher := &selectiveSourceFetcher{at: testTimestamp(t, 12)}
	gate := &recordingNetworkGate{err: privacy.ErrNetworkBlocked}
	limits := application.DefaultResearchProcessingLimitsV1()
	limits.MaxConcurrentFetch = 1
	service, err := application.NewLiveSourceFetchService(
		application.NewFetchService(fetcher, nil, application.NetworkResearchAccess{Gate: gate}),
		limits, application.DefaultLiveSourceMaximumBytes,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.FetchSources(context.Background(), application.LiveSourceFetchRequest{
		Mode: application.ResearchModeOnline, Sources: []research.Source{first, second},
	})
	if !errors.Is(err, application.ErrNetworkResearchBlocked) || !errors.Is(err, privacy.ErrNetworkBlocked) ||
		len(result.Fetched) != 0 || len(result.Failures) != 2 || fetcher.callCount() != 0 {
		t.Fatalf("blocked fetch result = (%+v,%v), adapter calls=%d", result, err, fetcher.callCount())
	}
	for _, failure := range result.Failures {
		if failure.Kind != application.ErrorNetworkResearchBlocked {
			t.Fatalf("blocked failure = %+v", failure)
		}
	}
}

func TestLiveSourceFetchEnforcesWholeRunAllocationAndInputBounds(t *testing.T) {
	t.Parallel()
	limits := application.DefaultResearchProcessingLimitsV1()
	limits.MaxConcurrentFetch = 1
	limits.MaxFetchedBytes = 10
	fetcher := &selectiveSourceFetcher{at: testTimestamp(t, 12)}
	service, err := application.NewLiveSourceFetchService(
		application.NewFetchService(fetcher, nil, application.NetworkResearchAccess{Gate: &recordingNetworkGate{}}), limits, 8,
	)
	if err != nil {
		t.Fatal(err)
	}
	first := testSource(t, "live-fetch-budget-first")
	second := testSource(t, "live-fetch-budget-second")
	result, err := service.FetchSources(context.Background(), application.LiveSourceFetchRequest{
		Mode: application.ResearchModeOnline, Sources: []research.Source{first, second},
	})
	if err == nil || len(result.Failures) != 2 || result.MaximumBytesEach != 5 {
		t.Fatalf("allocated fetch result = (%+v,%v)", result, err)
	}
	for _, request := range fetcher.requestsCopy() {
		if request.MaximumBytes != 5 {
			t.Fatalf("allocated request = %+v", request)
		}
	}
	if _, err := service.FetchSources(context.Background(), application.LiveSourceFetchRequest{Mode: application.ResearchModeOnline}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("empty source error = %v", err)
	}
	if _, err := service.FetchSources(context.Background(), application.LiveSourceFetchRequest{
		Mode: application.ResearchModeOnline, Sources: []research.Source{first, first},
	}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("duplicate source error = %v", err)
	}
}

func TestLiveSourceFetchV2SelectsInOrderWithinDurableAllocation(t *testing.T) {
	t.Parallel()
	limits := application.DefaultResearchProcessingLimitsV1()
	limits.MaxConcurrentFetch = 2
	limits.MaxFetchedBytes = 100
	fetcher := &selectiveSourceFetcher{at: testTimestamp(t, 12)}
	service, err := application.NewLiveSourceFetchService(
		application.NewFetchService(fetcher, nil, application.NetworkResearchAccess{Gate: &recordingNetworkGate{}}),
		limits, 80, application.LiveSourceFetchAllocation{
			MaximumFetches: 2, MaximumBytes: 100, AlgorithmVersion: application.LiveSourceFetchAllocationV1,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	first := testSource(t, "live-fetch-v2-first")
	second := testSource(t, "live-fetch-v2-second")
	third := testSource(t, "live-fetch-v2-third")
	result, err := service.FetchSources(context.Background(), application.LiveSourceFetchRequest{
		Mode: application.ResearchModeOnline, Sources: []research.Source{first, second, third},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.AlgorithmVersion != application.LiveSourceFetchV2 || result.RequestedCount != 3 ||
		len(result.Fetched) != 2 || len(result.Failures) != 1 || result.MaximumBytesEach != 50 ||
		result.Failures[0].SourceID != third.ID || result.Failures[0].Kind != application.ErrorBudgetExceeded {
		t.Fatalf("allocated v2 fetch result = %+v", result)
	}
	requests := fetcher.requestsCopy()
	if len(requests) != 2 || requests[0].SourceID != first.ID && requests[1].SourceID != first.ID ||
		requests[0].SourceID != second.ID && requests[1].SourceID != second.ID {
		t.Fatalf("allocated v2 requests = %+v", requests)
	}
	for _, request := range requests {
		if request.MaximumBytes != 50 {
			t.Fatalf("allocated v2 request = %+v", request)
		}
	}
}

type selectiveSourceFetcher struct {
	mu       sync.Mutex
	failures map[research.SourceID]error
	at       research.Timestamp
	requests []application.FetchRequest
}

func (fetcher *selectiveSourceFetcher) Fetch(_ context.Context, request application.FetchRequest) (application.FetchedSource, error) {
	fetcher.mu.Lock()
	fetcher.requests = append(fetcher.requests, request)
	failure := fetcher.failures[request.SourceID]
	fetcher.mu.Unlock()
	if failure != nil {
		return application.FetchedSource{}, failure
	}
	body := []byte("body:" + request.SourceID.String())
	return application.FetchedSource{
		SourceID: request.SourceID, Locator: request.Locator, FetchedAt: fetcher.at,
		Metadata: research.FetchMetadata{StatusCode: 200, ContentType: "text/plain", ContentHash: research.CanonicalContentHashV1(body),
			ContentLength: int64(len(body)), FetchVersion: "fixture-fetch-v1"},
		Body: body, Origin: application.FetchOriginLive,
	}, nil
}

func (fetcher *selectiveSourceFetcher) callCount() int {
	fetcher.mu.Lock()
	defer fetcher.mu.Unlock()
	return len(fetcher.requests)
}

func (fetcher *selectiveSourceFetcher) requestsCopy() []application.FetchRequest {
	fetcher.mu.Lock()
	defer fetcher.mu.Unlock()
	return append([]application.FetchRequest(nil), fetcher.requests...)
}

var _ application.SourceFetcher = (*selectiveSourceFetcher)(nil)
