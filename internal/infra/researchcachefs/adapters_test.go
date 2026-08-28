package researchcachefs

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestOfflineAdapterFeedsDiscoveryFetchAndReleaseWithoutLiveCalls(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	clock := &testClock{now: fsTimestamp(t, time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))}
	cache, err := NewFactory().WithClock(clock).Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	adapter := NewOfflineAdapter(cache)

	requestID, _ := research.NewID("request.offline-cache")
	query := application.SearchQuery{RequestID: requestID, Text: "portable docs"}
	options := application.SearchOptions{Limit: 5}
	locator, _ := research.NewSourceLocator("https://docs.example.test/portable")
	searchResults := []application.SearchResult{{Title: "Portable docs", Locator: locator, Provider: "fixture", Rank: 1}}
	if err := adapter.CacheSearch(ctx, query, options, searchResults); err != nil {
		t.Fatal(err)
	}

	sourceID, _ := research.NewSourceID("source.offline-cache")
	fetchRequest := application.FetchRequest{SourceID: sourceID, Locator: locator, MaximumBytes: 4096}
	fetched := application.FetchedSource{
		SourceID: sourceID, Locator: locator, FetchedAt: clock.now, Body: []byte("bounded"), Origin: application.FetchOriginLive,
		Metadata: research.FetchMetadata{StatusCode: 200, ContentType: "text/plain", ContentHash: "sha256:bounded", ContentLength: 7, FetchVersion: "fetch/v1"},
	}
	if err := adapter.CacheFetched(ctx, fetchRequest, fetched); err != nil {
		t.Fatal(err)
	}

	technologyID, _ := research.NewID("technology.offline-cache")
	releaseID, _ := research.NewID("release.offline-cache")
	version, _ := research.NewVersionIdentifier("2.1.0")
	releaseQuery := application.ReleaseLookupQuery{TechnologyID: technologyID, Channel: research.ReleaseStable}
	releases := []research.ReleaseRecord{{
		ID: releaseID, TechnologyID: technologyID, Version: version, Channel: research.ReleaseStable,
		Status: research.ReleaseCurrent, SourceIDs: []research.SourceID{sourceID}, ReleasedAt: &clock.now, VerifiedAt: clock.now,
	}}
	if err := adapter.CacheReleases(ctx, releaseQuery, releases); err != nil {
		t.Fatal(err)
	}
	if err := adapter.CacheNormalized(ctx, "source.offline-cache", []byte(`{"segments":["bounded"]}`)); err != nil {
		t.Fatal(err)
	}

	clock.now = fsTimestamp(t, clock.now.Time().Add(8*24*time.Hour))
	searchLive := &countingSearchProvider{}
	gotSearch, err := application.NewDiscoveryService(searchLive, adapter, application.NetworkResearchAccess{}).
		Search(ctx, application.ResearchModeOffline, query, options)
	if err != nil || len(gotSearch) != 1 || !gotSearch[0].CacheHit || !gotSearch[0].CacheStale || gotSearch[0].CacheWarning != application.CacheWarningStaleOffline {
		t.Fatalf("offline search = (%+v,%v)", gotSearch, err)
	}
	fetchLive := &countingFetcher{}
	gotFetch, err := application.NewFetchService(fetchLive, adapter, application.NetworkResearchAccess{}).
		Fetch(ctx, application.ResearchModeOffline, fetchRequest)
	if err != nil || !gotFetch.CacheHit || !gotFetch.CacheStale || gotFetch.CacheWarning != application.CacheWarningStaleOffline || string(gotFetch.Body) != "bounded" {
		t.Fatalf("offline fetch = (%+v,%v)", gotFetch, err)
	}
	releaseLive := &countingReleaseProvider{}
	gotReleases, err := application.NewReleaseLookupService(releaseLive, adapter, application.NetworkResearchAccess{}).
		Lookup(ctx, application.ResearchModeOffline, releaseQuery)
	if err != nil || len(gotReleases) != 1 || gotReleases[0].ID != releaseID {
		t.Fatalf("offline releases = (%+v,%v)", gotReleases, err)
	}
	normalized, err := adapter.LoadNormalized(ctx, "source.offline-cache")
	if err != nil || !normalized.Hit || !normalized.Stale || normalized.Warning != application.CacheWarningStaleOffline {
		t.Fatalf("offline normalized cache = (%+v,%v)", normalized, err)
	}
	if searchLive.calls != 0 || fetchLive.calls != 0 || releaseLive.calls != 0 {
		t.Fatalf("offline reached live providers: search=%d fetch=%d release=%d", searchLive.calls, fetchLive.calls, releaseLive.calls)
	}
}

type countingSearchProvider struct{ calls int }

func (provider *countingSearchProvider) Search(context.Context, application.SearchQuery, application.SearchOptions) ([]application.SearchResult, error) {
	provider.calls++
	return nil, nil
}

type countingFetcher struct{ calls int }

func (fetcher *countingFetcher) Fetch(context.Context, application.FetchRequest) (application.FetchedSource, error) {
	fetcher.calls++
	return application.FetchedSource{}, nil
}

type countingReleaseProvider struct{ calls int }

func (provider *countingReleaseProvider) LookupReleases(context.Context, application.ReleaseLookupQuery) ([]research.ReleaseRecord, error) {
	provider.calls++
	return nil, nil
}
