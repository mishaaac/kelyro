package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestCostControlledFetchReservesBoundedUnitsBeforeLiveAdapter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fixture := networkFixture(t)
	store, runID, costs := fetchCostFixture(t, research.ResearchCostUsage{FetchRequests: 1, Bytes: fixture.fetchRequest.MaximumBytes})
	_ = store
	fetcher := &recordingSourceFetcher{fetched: fixture.fetched}
	service, err := application.NewCostControlledFetchService(
		fetcher, nil, application.NetworkResearchAccess{Gate: &recordingNetworkGate{}}, costs,
		fixedClock{now: testTimestamp(t, 12)}, runID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Fetch(ctx, application.ResearchModeOnline, fixture.fetchRequest); err != nil {
		t.Fatal(err)
	}
	metadata, err := costs.Metadata(ctx, runID)
	if err != nil || metadata.Used.FetchRequests != 1 || metadata.Used.Bytes != fixture.fetchRequest.MaximumBytes || fetcher.calls != 1 {
		t.Fatalf("live fetch cost = (%+v, %v), calls=%d", metadata, err, fetcher.calls)
	}
	if _, err := service.Fetch(ctx, application.ResearchModeOnline, fixture.fetchRequest); !errors.Is(err, application.ErrBudgetExceeded) || fetcher.calls != 1 {
		t.Fatalf("budgeted fetch error=%v calls=%d", err, fetcher.calls)
	}
}

func TestCostControlledFetchRecordsOfflineCacheSavingsWithoutUsage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fixture := networkFixture(t)
	_, runID, costs := fetchCostFixture(t, research.ResearchCostUsage{FetchRequests: 2, Bytes: fixture.fetchRequest.MaximumBytes * 2})
	cache := &recordingFetchCache{fetched: fixture.fetched}
	service, err := application.NewCostControlledFetchService(
		&recordingSourceFetcher{}, cache, application.NetworkResearchAccess{Gate: &recordingNetworkGate{err: privacy.ErrNetworkBlocked}},
		costs, fixedClock{now: testTimestamp(t, 12)}, runID,
	)
	if err != nil {
		t.Fatal(err)
	}
	fetched, err := service.Fetch(ctx, application.ResearchModeAuto, fixture.fetchRequest)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := costs.Metadata(ctx, runID)
	if err != nil || !metadata.Used.IsZero() || metadata.CacheSavings.FetchRequests != 1 || metadata.CacheSavings.Bytes != int64(len(fetched.Body)) {
		t.Fatalf("cached fetch cost = (%+v, %v)", metadata, err)
	}
}

func fetchCostFixture(t *testing.T, runLimit research.ResearchCostUsage) (*memory.Store, research.ID, application.ResearchCostService) {
	t.Helper()
	store := memory.New()
	repositories := store.Repositories()
	request, run := testRequestRun(t)
	budget := research.DefaultResearchCostBudgetV1()
	budget.PerRun = runLimit
	if budget.PerTopic.FetchRequests < runLimit.FetchRequests {
		budget.PerTopic.FetchRequests = runLimit.FetchRequests
	}
	if budget.PerTopic.Bytes < runLimit.Bytes {
		budget.PerTopic.Bytes = runLimit.Bytes
	}
	metadata := research.ResearchCostMetadata{Budget: budget, AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	run.Cost = &metadata
	if err := repositories.Runs.Create(context.Background(), request, run); err != nil {
		t.Fatal(err)
	}
	return store, run.ID, application.NewResearchCostService(repositories.Costs)
}
