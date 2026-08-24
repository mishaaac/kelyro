package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestNetworkResearchServicesBlockEveryLiveBoundary(t *testing.T) {
	t.Parallel()

	fixture := networkFixture(t)
	gate := &recordingNetworkGate{err: privacy.ErrNetworkBlocked}
	search := &recordingSearchProvider{results: fixture.results}
	fetch := &recordingSourceFetcher{fetched: fixture.fetched}
	releases := &recordingReleaseLookupProvider{records: fixture.releases}
	access := application.NetworkResearchAccess{Gate: gate}

	_, searchErr := application.NewDiscoveryService(search, nil, access).
		Search(context.Background(), application.ResearchModeOnline, fixture.searchQuery)
	_, fetchErr := application.NewFetchService(fetch, nil, access).
		Fetch(context.Background(), application.ResearchModeOnline, fixture.fetchRequest)
	_, releaseErr := application.NewReleaseLookupService(releases, nil, access).
		Lookup(context.Background(), application.ResearchModeOnline, fixture.releaseQuery)

	for name, err := range map[string]error{
		"discovery": searchErr,
		"fetch":     fetchErr,
		"release":   releaseErr,
	} {
		if !errors.Is(err, application.ErrNetworkResearchBlocked) || !errors.Is(err, privacy.ErrNetworkBlocked) {
			t.Errorf("%s error = %v, want categorized privacy denial", name, err)
		}
		if kind, ok := application.KindOf(err); !ok || kind != application.ErrorNetworkResearchBlocked {
			t.Errorf("KindOf(%s) = (%q, %v)", name, kind, ok)
		}
	}
	if search.calls != 0 || fetch.calls != 0 || releases.calls != 0 {
		t.Fatalf("live calls = search:%d fetch:%d release:%d, want zero", search.calls, fetch.calls, releases.calls)
	}
	wantOperations := []string{
		string(application.NetworkOperationDiscovery),
		string(application.NetworkOperationFetch),
		string(application.NetworkOperationRelease),
	}
	if len(gate.requests) != len(wantOperations) {
		t.Fatalf("authorization requests = %+v", gate.requests)
	}
	for index, request := range gate.requests {
		if request.Operation != wantOperations[index] || request.Purpose != privacy.ExternalResource {
			t.Errorf("authorization request %d = %+v", index, request)
		}
	}
}

func TestNetworkResearchAutoUsesOfflineCacheWhenPrivacyBlocks(t *testing.T) {
	t.Parallel()

	fixture := networkFixture(t)
	gate := &recordingNetworkGate{err: privacy.ErrNetworkBlocked}
	search := &recordingSearchProvider{}
	fetch := &recordingSourceFetcher{}
	releases := &recordingReleaseLookupProvider{}
	searchCache := &recordingSearchCache{results: fixture.results}
	fetchCache := &recordingFetchCache{fetched: fixture.fetched}
	releaseCache := &recordingReleaseCache{records: fixture.releases}
	access := application.NetworkResearchAccess{Gate: gate}

	results, err := application.NewDiscoveryService(search, searchCache, access).
		Search(context.Background(), application.ResearchModeAuto, fixture.searchQuery)
	if err != nil || len(results) != 1 {
		t.Fatalf("cached Search() = (%+v, %v)", results, err)
	}
	fetched, err := application.NewFetchService(fetch, fetchCache, access).
		Fetch(context.Background(), application.ResearchModeAuto, fixture.fetchRequest)
	if err != nil || fetched.SourceID != fixture.fetchRequest.SourceID {
		t.Fatalf("cached Fetch() = (%+v, %v)", fetched, err)
	}
	if fetched.Origin != application.FetchOriginCache {
		t.Fatalf("cached Fetch() origin = %q", fetched.Origin)
	}
	records, err := application.NewReleaseLookupService(releases, releaseCache, access).
		Lookup(context.Background(), application.ResearchModeAuto, fixture.releaseQuery)
	if err != nil || len(records) != 1 {
		t.Fatalf("cached Lookup() = (%+v, %v)", records, err)
	}

	if search.calls != 0 || fetch.calls != 0 || releases.calls != 0 {
		t.Fatalf("live calls = search:%d fetch:%d release:%d, want zero", search.calls, fetch.calls, releases.calls)
	}
	if searchCache.calls != 1 || fetchCache.calls != 1 || releaseCache.calls != 1 {
		t.Fatalf("cache calls = search:%d fetch:%d release:%d, want one each", searchCache.calls, fetchCache.calls, releaseCache.calls)
	}
}

func TestFetchServiceAcceptsHardenedRedirectLocatorAndMarksLiveOrigin(t *testing.T) {
	t.Parallel()
	fixture := networkFixture(t)
	redirected := fixture.fetched
	redirected.Locator = testLocator(t, "redirected")
	service := application.NewFetchService(
		&recordingSourceFetcher{fetched: redirected}, nil,
		application.NetworkResearchAccess{Gate: &recordingNetworkGate{}},
	)

	result, err := service.Fetch(context.Background(), application.ResearchModeOnline, fixture.fetchRequest)
	if err != nil {
		t.Fatal(err)
	}
	if result.Locator != redirected.Locator || result.Origin != application.FetchOriginLive {
		t.Fatalf("redirected live result = %+v", result)
	}
}

func TestNetworkResearchModesKeepOfflineOfflineAndAllowAuthorizedLiveCalls(t *testing.T) {
	t.Parallel()

	fixture := networkFixture(t)
	gate := &recordingNetworkGate{}
	live := &recordingSearchProvider{results: fixture.results}
	cache := &recordingSearchCache{results: fixture.results}
	service := application.NewDiscoveryService(live, cache, application.NetworkResearchAccess{Gate: gate})

	if _, err := service.Search(context.Background(), application.ResearchModeOffline, fixture.searchQuery); err != nil {
		t.Fatalf("offline Search() error = %v", err)
	}
	if live.calls != 0 || cache.calls != 1 || len(gate.requests) != 0 {
		t.Fatalf("offline calls = live:%d cache:%d gate:%d", live.calls, cache.calls, len(gate.requests))
	}

	if _, err := service.Search(context.Background(), application.ResearchModeOnline, fixture.searchQuery); err != nil {
		t.Fatalf("online Search() error = %v", err)
	}
	if live.calls != 1 || cache.calls != 1 || len(gate.requests) != 1 {
		t.Fatalf("online calls = live:%d cache:%d gate:%d", live.calls, cache.calls, len(gate.requests))
	}

	if _, err := service.Search(context.Background(), application.ResearchMode("invented"), fixture.searchQuery); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("invalid mode error = %v, want invalid_state", err)
	}
	if live.calls != 1 || cache.calls != 1 || len(gate.requests) != 1 {
		t.Fatal("invalid mode reached a gate, provider, or cache")
	}
}

func TestNetworkResearchAutoWithoutCachedFallbackReturnsCategorizedBlock(t *testing.T) {
	t.Parallel()

	fixture := networkFixture(t)
	live := &recordingSearchProvider{results: fixture.results}
	service := application.NewDiscoveryService(live, nil, application.NetworkResearchAccess{
		Gate: &recordingNetworkGate{err: privacy.ErrNetworkBlocked},
	})
	_, err := service.Search(context.Background(), application.ResearchModeAuto, fixture.searchQuery)
	if !errors.Is(err, application.ErrNetworkResearchBlocked) || live.calls != 0 {
		t.Fatalf("Search() error = %v calls = %d", err, live.calls)
	}
}

func TestNetworkResearchCacheMissAndFailureKeepDistinctCategories(t *testing.T) {
	t.Parallel()

	fixture := networkFixture(t)
	blocked := application.NetworkResearchAccess{Gate: &recordingNetworkGate{err: privacy.ErrNetworkBlocked}}
	cacheMiss := application.Classify(application.ErrorNotFound, "read search cache", errors.New("fixture miss"))
	_, err := application.NewDiscoveryService(nil, &recordingSearchCache{err: cacheMiss}, blocked).
		Search(context.Background(), application.ResearchModeAuto, fixture.searchQuery)
	if kind, ok := application.KindOf(err); !ok || kind != application.ErrorNetworkResearchBlocked {
		t.Fatalf("cache miss KindOf() = (%q, %v), error = %v", kind, ok, err)
	}

	cacheFailure := errors.New("cache database failed")
	_, err = application.NewDiscoveryService(nil, &recordingSearchCache{err: cacheFailure}, blocked).
		Search(context.Background(), application.ResearchModeAuto, fixture.searchQuery)
	if !errors.Is(err, application.ErrPersistenceFailure) || !errors.Is(err, cacheFailure) {
		t.Fatalf("cache failure error = %v, want persistence_failure", err)
	}
}

type networkTestFixture struct {
	searchQuery  application.SearchQuery
	results      []application.SearchResult
	fetchRequest application.FetchRequest
	fetched      application.FetchedSource
	releaseQuery application.ReleaseLookupQuery
	releases     []research.ReleaseRecord
}

func networkFixture(t *testing.T) networkTestFixture {
	t.Helper()
	source := testSource(t, "network")
	fetchRequest := application.FetchRequest{
		SourceID: source.ID, Locator: source.Locator, MaximumBytes: 4096,
	}
	technologyID := testID(t, "technology.network")
	release := testRelease(t, source.ID)
	release.TechnologyID = technologyID
	return networkTestFixture{
		searchQuery: application.SearchQuery{
			RequestID: testID(t, "request.network"), Text: "official documentation", Limit: 5,
		},
		results: []application.SearchResult{{
			Title: "Official documentation", Locator: source.Locator, Provider: "fixture", Rank: 0,
		}},
		fetchRequest: fetchRequest,
		fetched: application.FetchedSource{
			SourceID: source.ID, Locator: source.Locator, FetchedAt: testTimestamp(t, 10),
			Metadata: research.FetchMetadata{
				StatusCode: 200, ContentType: "text/html", ContentHash: "sha256:fixture",
				ContentLength: 7, FetchVersion: fixtureVersion,
			},
			Body: []byte("fixture"),
		},
		releaseQuery: application.ReleaseLookupQuery{TechnologyID: technologyID, Channel: research.ReleaseStable},
		releases:     []research.ReleaseRecord{release},
	}
}

type recordingNetworkGate struct {
	err      error
	requests []privacy.Request
}

func (gate *recordingNetworkGate) Authorize(_ context.Context, request privacy.Request) error {
	gate.requests = append(gate.requests, request)
	return gate.err
}

type recordingSearchProvider struct {
	results []application.SearchResult
	err     error
	calls   int
}

func (provider *recordingSearchProvider) Search(context.Context, application.SearchQuery) ([]application.SearchResult, error) {
	provider.calls++
	return provider.results, provider.err
}

type recordingSearchCache struct {
	results []application.SearchResult
	err     error
	calls   int
}

func (cache *recordingSearchCache) SearchCached(context.Context, application.SearchQuery) ([]application.SearchResult, error) {
	cache.calls++
	return cache.results, cache.err
}

type recordingSourceFetcher struct {
	fetched application.FetchedSource
	err     error
	calls   int
}

func (fetcher *recordingSourceFetcher) Fetch(context.Context, application.FetchRequest) (application.FetchedSource, error) {
	fetcher.calls++
	return fetcher.fetched, fetcher.err
}

type recordingFetchCache struct {
	fetched application.FetchedSource
	err     error
	calls   int
}

func (cache *recordingFetchCache) FetchCached(context.Context, application.FetchRequest) (application.FetchedSource, error) {
	cache.calls++
	return cache.fetched, cache.err
}

type recordingReleaseLookupProvider struct {
	records []research.ReleaseRecord
	err     error
	calls   int
}

func (provider *recordingReleaseLookupProvider) LookupReleases(context.Context, application.ReleaseLookupQuery) ([]research.ReleaseRecord, error) {
	provider.calls++
	return provider.records, provider.err
}

type recordingReleaseCache struct {
	records []research.ReleaseRecord
	err     error
	calls   int
}

func (cache *recordingReleaseCache) LookupCachedReleases(context.Context, application.ReleaseLookupQuery) ([]research.ReleaseRecord, error) {
	cache.calls++
	return cache.records, cache.err
}
