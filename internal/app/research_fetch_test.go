package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestServiceAssemblesProductionResearchFetchBehindResolvedPrivacy(t *testing.T) {
	t.Parallel()
	configs := &recordingConfigStore{project: config.Settings{config.KeyAllowNetwork: config.BoolValue(true)}}
	fetcher := &recordingProductionSourceFetcher{at: time.Date(2026, 8, 31, 15, 0, 0, 0, time.UTC)}
	service := NewService(nil, nil).WithConfig(configs).WithResearchFetcher(fetcher).WithResearchSourceCaches(&appSourceFetchCacheFactory{})
	store, runID := appFetchCostStore(t)
	stage, err := service.researchFetchForRun(context.Background(), Command{}, "/workspace", store, runID)
	if err != nil {
		t.Fatal(err)
	}
	source := appResearchFetchSource(t, "source.app-fetch")
	artifacts, err := stage.Execute(context.Background(), researchapp.LiveResearchStageInput{
		Mode: researchapp.ResearchModeOnline, Artifacts: researchapp.LiveResearchArtifacts{Sources: []research.Source{source}},
	})
	if err != nil || len(artifacts.FetchedSources) != 1 || artifacts.FetchedSources[0].SourceID != source.ID || fetcher.calls != 1 {
		t.Fatalf("production fetch stage = (%+v,%v), calls=%d", artifacts, err, fetcher.calls)
	}
	metadata, err := store.Costs().Metadata(context.Background(), runID)
	if err != nil || metadata.Used.FetchRequests != 1 || metadata.Used.Bytes != researchapp.DefaultLiveSourceMaximumBytes {
		t.Fatalf("production fetch cost = (%+v, %v)", metadata, err)
	}
}

func TestServiceResearchFetchPrivacyDenialNeverReachesAdapter(t *testing.T) {
	t.Parallel()
	configs := &recordingConfigStore{project: config.Settings{config.KeyAllowNetwork: config.BoolValue(false)}}
	fetcher := &recordingProductionSourceFetcher{at: time.Date(2026, 8, 31, 15, 0, 0, 0, time.UTC)}
	service := NewService(nil, nil).WithConfig(configs).WithResearchFetcher(fetcher).WithResearchSourceCaches(&appSourceFetchCacheFactory{})
	store, runID := appFetchCostStore(t)
	stage, err := service.researchFetchForRun(context.Background(), Command{}, "/workspace", store, runID)
	if err != nil {
		t.Fatal(err)
	}
	source := appResearchFetchSource(t, "source.app-fetch-blocked")
	artifacts, err := stage.Execute(context.Background(), researchapp.LiveResearchStageInput{
		Mode: researchapp.ResearchModeOnline, Artifacts: researchapp.LiveResearchArtifacts{Sources: []research.Source{source}},
	})
	if !errors.Is(err, researchapp.ErrNetworkResearchBlocked) || !errors.Is(err, privacy.ErrNetworkBlocked) ||
		len(artifacts.FetchFailures) != 1 || fetcher.calls != 0 {
		t.Fatalf("blocked production fetch = (%+v,%v), calls=%d", artifacts, err, fetcher.calls)
	}
}

func TestServiceAssemblesSnapshotStageFromWorkspaceStoreAndSourceCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	memoryStore := memory.New()
	repositories := memoryStore.Repositories()
	source := appResearchFetchSource(t, "source.app-snapshot")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	capture := researchapp.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, nil)
	cache := &recordingAppSourceFetchCache{}
	service := NewService(nil, nil).WithResearchSourceCaches(&appSourceFetchCacheFactory{cache: cache})
	stage, err := service.researchSnapshotForRun(ctx, "/workspace", &fakeSourceRegistryStore{snapshots: capture})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("fixture")
	fetchedAt, _ := research.NewTimestamp(time.Date(2026, 8, 31, 15, 0, 0, 0, time.UTC))
	fetched := researchapp.FetchedSource{
		SourceID: source.ID, Locator: source.Locator, FetchedAt: fetchedAt, Body: body, Origin: researchapp.FetchOriginLive,
		Metadata: research.FetchMetadata{StatusCode: 200, ContentType: "text/plain", ContentHash: research.CanonicalContentHashV1(body),
			ContentLength: int64(len(body)), FetchVersion: "fixture-fetch-v1"},
	}
	artifacts, err := stage.Execute(ctx, researchapp.LiveResearchStageInput{Artifacts: researchapp.LiveResearchArtifacts{
		Sources: []research.Source{source}, FetchedSources: []researchapp.FetchedSource{fetched}, FetchMaximumBytes: 4096,
	}})
	if err != nil || len(artifacts.Snapshots) != 1 || len(artifacts.NormalizationInputs) != 1 || cache.writes != 1 {
		t.Fatalf("production snapshot stage = (%+v,%v), cache writes=%d", artifacts, err, cache.writes)
	}
}

type recordingProductionSourceFetcher struct {
	at    time.Time
	calls int
}

func (fetcher *recordingProductionSourceFetcher) Fetch(_ context.Context, request researchapp.FetchRequest) (researchapp.FetchedSource, error) {
	fetcher.calls++
	body := []byte("fixture")
	at, _ := research.NewTimestamp(fetcher.at)
	return researchapp.FetchedSource{
		SourceID: request.SourceID, Locator: request.Locator, FetchedAt: at, Body: body, Origin: researchapp.FetchOriginLive,
		Metadata: research.FetchMetadata{StatusCode: 200, ContentType: "text/plain", ContentHash: research.CanonicalContentHashV1(body),
			ContentLength: int64(len(body)), FetchVersion: "fixture-fetch-v1"},
	}, nil
}

func appResearchFetchSource(t *testing.T, idValue string) research.Source {
	t.Helper()
	id, err := research.NewSourceID(idValue)
	if err != nil {
		t.Fatal(err)
	}
	locator, err := research.NewSourceLocator("https://docs.example.test/" + idValue)
	if err != nil {
		t.Fatal(err)
	}
	at, err := research.NewTimestamp(time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return research.Source{ID: id, Kind: research.SourceOther, Locator: locator, TemporalScope: research.SourceTemporalCurrent,
		Metadata: research.SourceMetadata{Title: "Fixture"}, CreatedAt: at}
}

var _ researchapp.SourceFetcher = (*recordingProductionSourceFetcher)(nil)

func appFetchCostStore(t *testing.T) (*fakeSourceRegistryStore, research.ID) {
	t.Helper()
	memoryStore := memory.New()
	repositories := memoryStore.Repositories()
	topic, _ := research.NewResearchTopic("fetch accounting", "software", "Go")
	requestID, _ := research.NewID("request.app-fetch-cost")
	runID, _ := research.NewID("run.app-fetch-cost")
	at, _ := research.NewTimestamp(time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC))
	cost := research.ResearchCostMetadata{Budget: research.DefaultResearchCostBudgetV1(), AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	run := research.ResearchRun{ID: runID, RequestID: requestID, Status: research.ResearchRunRunning, StartedAt: at, Cost: &cost}
	request := research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: at}
	if err := repositories.Runs.Create(context.Background(), request, run); err != nil {
		t.Fatal(err)
	}
	return &fakeSourceRegistryStore{costs: researchapp.NewResearchCostService(repositories.Costs)}, runID
}

type appSourceFetchCacheFactory struct {
	cache researchapp.SourceFetchCacheAdapter
}

func (factory *appSourceFetchCacheFactory) OpenSourceFetchCache(context.Context, string) (researchapp.SourceFetchCacheAdapter, error) {
	if factory.cache != nil {
		return factory.cache, nil
	}
	return appSourceFetchCache{}, nil
}

type appSourceFetchCache struct{}

func (appSourceFetchCache) FetchCached(context.Context, researchapp.FetchRequest) (researchapp.FetchedSource, error) {
	return researchapp.FetchedSource{}, researchapp.Classify(researchapp.ErrorNotFound, "fixture source cache", errors.New("cache miss"))
}

func (appSourceFetchCache) CacheFetched(context.Context, researchapp.FetchRequest, researchapp.FetchedSource) error {
	return nil
}

type recordingAppSourceFetchCache struct{ writes int }

func (*recordingAppSourceFetchCache) FetchCached(context.Context, researchapp.FetchRequest) (researchapp.FetchedSource, error) {
	return researchapp.FetchedSource{}, researchapp.Classify(researchapp.ErrorNotFound, "fixture source cache", errors.New("cache miss"))
}

func (cache *recordingAppSourceFetchCache) CacheFetched(context.Context, researchapp.FetchRequest, researchapp.FetchedSource) error {
	cache.writes++
	return nil
}
