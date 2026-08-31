package application_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestLiveSourceSnapshotPersistsMetadataCachesBodiesAndPreservesNormalizationInput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	first := testSource(t, "live-snapshot-first")
	second := testSource(t, "live-snapshot-no-store")
	for _, source := range []research.Source{first, second} {
		if err := repositories.Sources.Create(ctx, source); err != nil {
			t.Fatal(err)
		}
	}
	firstFetched := fetchedFixture(t, first, 12, http.StatusOK, `"first"`, []byte("first body"))
	firstFetched.Locator = discoveryLocator(t, "https://cdn.example.test/final-first")
	secondFetched := fetchedFixture(t, second, 13, http.StatusOK, `"second"`, []byte("second body"))
	secondFetched.NoStore = true
	cache := newSnapshotCache()
	service, err := application.NewLiveSourceSnapshotService(
		application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, nil), cache,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.SnapshotSources(ctx, application.LiveSourceSnapshotRequest{
		Fetched: []application.FetchedSource{firstFetched, secondFetched}, Sources: []research.Source{first, second}, MaximumBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Snapshots) != 2 || len(result.NormalizationInputs) != 2 || result.CacheWrites != 1 ||
		result.CacheSuppressed != 1 || result.CacheHits != 0 || len(result.CacheFailures) != 0 ||
		result.AlgorithmVersion != application.LiveSourceSnapshotV1 {
		t.Fatalf("live snapshot result = %+v", result)
	}
	if string(result.NormalizationInputs[0].Body) != "first body" || string(result.NormalizationInputs[1].Body) != "second body" {
		t.Fatalf("normalization inputs = %+v", result.NormalizationInputs)
	}
	for _, snapshot := range result.Snapshots {
		stored, getErr := repositories.Snapshots.Get(ctx, snapshot.ID)
		if getErr != nil || stored.Fetch.ContentHash == "" || stored.Fetch.ContentLength == 0 {
			t.Fatalf("durable snapshot = (%+v,%v)", stored, getErr)
		}
	}
	if cache.writeCount != 1 || cache.values[first.ID].NoStore || len(cache.requests) != 1 || cache.requests[0].Locator != first.Locator {
		t.Fatalf("cache writes=%d values=%+v", cache.writeCount, cache.values)
	}
	result.NormalizationInputs[0].Body[0] = 'X'
	if string(firstFetched.Body) != "first body" || string(cache.values[first.ID].Body) != "first body" {
		t.Fatal("snapshot stage leaked fetched body ownership")
	}
}

func TestLiveSourceSnapshotReusesDurableHistoryForOfflineCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "live-snapshot-cache")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	live := fetchedFixture(t, source, 12, http.StatusOK, `"cache"`, []byte("cached body"))
	capture := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, nil)
	durable, err := capture.CaptureFetched(ctx, live, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: 4096, BodyPolicy: application.SnapshotNormalizedExcerpt,
	})
	if err != nil {
		t.Fatal(err)
	}
	cached := live
	cached.Origin = application.FetchOriginCache
	cached.CacheHit = true
	service, err := application.NewLiveSourceSnapshotService(capture, newSnapshotCache())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.SnapshotSources(ctx, application.LiveSourceSnapshotRequest{Fetched: []application.FetchedSource{cached}, Sources: []research.Source{source}, MaximumBytes: 4096})
	if err != nil || len(result.Snapshots) != 1 || result.Snapshots[0].ID != durable.Snapshot.ID || result.CacheHits != 1 ||
		len(result.NormalizationInputs) != 1 || result.NormalizationInputs[0].Origin != application.FetchOriginCache {
		t.Fatalf("cached snapshot reuse = (%+v,%v)", result, err)
	}
	history, err := repositories.Snapshots.ListBySource(ctx, source.ID)
	if err != nil || len(history) != 1 {
		t.Fatalf("cached snapshot invented history = (%+v,%v)", history, err)
	}

	orphanStore := memory.New()
	orphanRepositories := orphanStore.Repositories()
	if err := orphanRepositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	orphanService, _ := application.NewLiveSourceSnapshotService(
		application.NewSnapshotCaptureService(orphanRepositories.Sources, orphanRepositories.Snapshots, nil), newSnapshotCache(),
	)
	orphanResult, err := orphanService.SnapshotSources(ctx, application.LiveSourceSnapshotRequest{Fetched: []application.FetchedSource{cached}, Sources: []research.Source{source}, MaximumBytes: 4096})
	if err == nil || len(orphanResult.SnapshotFailures) != 1 || len(orphanResult.Snapshots) != 0 {
		t.Fatalf("orphan cached snapshot = (%+v,%v)", orphanResult, err)
	}
}

func TestLiveSourceSnapshotRevalidationUsesCachedBodyWithoutDuplicatingSnapshot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "live-snapshot-304")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	live := fetchedFixture(t, source, 12, http.StatusOK, `"stable"`, []byte("stable body"))
	cache := newSnapshotCache()
	cache.values[source.ID] = cloneSnapshotFetched(live)
	capture := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, nil)
	if _, err := capture.CaptureFetched(ctx, live, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: 4096, BodyPolicy: application.SnapshotNormalizedExcerpt,
	}); err != nil {
		t.Fatal(err)
	}
	revalidated := fetchedFixture(t, source, 13, http.StatusNotModified, `"stable"`, nil)
	service, _ := application.NewLiveSourceSnapshotService(capture, cache)
	result, err := service.SnapshotSources(ctx, application.LiveSourceSnapshotRequest{Fetched: []application.FetchedSource{revalidated}, Sources: []research.Source{source}, MaximumBytes: 4096})
	if err != nil || len(result.Snapshots) != 1 || result.Snapshots[0].Fetch.StatusCode != http.StatusNotModified ||
		result.CacheHits != 1 || len(result.NormalizationInputs) != 1 || string(result.NormalizationInputs[0].Body) != "stable body" {
		t.Fatalf("304 snapshot result = (%+v,%v)", result, err)
	}
	history, err := repositories.Snapshots.ListBySource(ctx, source.ID)
	if err != nil || len(history) != 2 {
		t.Fatalf("304 snapshot history = (%+v,%v)", history, err)
	}
}

type snapshotCache struct {
	values     map[research.SourceID]application.FetchedSource
	writeCount int
	requests   []application.FetchRequest
	fail       error
}

func newSnapshotCache() *snapshotCache {
	return &snapshotCache{values: make(map[research.SourceID]application.FetchedSource)}
}

func (cache *snapshotCache) CacheFetched(_ context.Context, request application.FetchRequest, fetched application.FetchedSource) error {
	if cache.fail != nil {
		return cache.fail
	}
	cache.writeCount++
	cache.requests = append(cache.requests, request)
	cache.values[fetched.SourceID] = cloneSnapshotFetched(fetched)
	return nil
}

func (cache *snapshotCache) FetchCached(_ context.Context, request application.FetchRequest) (application.FetchedSource, error) {
	fetched, exists := cache.values[request.SourceID]
	if !exists {
		return application.FetchedSource{}, application.Classify(application.ErrorNotFound, "snapshot cache fixture", errors.New("cache miss"))
	}
	fetched = cloneSnapshotFetched(fetched)
	fetched.Origin = application.FetchOriginCache
	fetched.CacheHit = true
	fetched.CacheStale = false
	fetched.CacheWarning = ""
	return fetched, nil
}

func cloneSnapshotFetched(fetched application.FetchedSource) application.FetchedSource {
	fetched.Body = append([]byte(nil), fetched.Body...)
	return fetched
}

var _ application.SourceFetchCacheAdapter = (*snapshotCache)(nil)
