package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestCostControlledDiscoveryReservesBoundedSearchBeforeProvider(t *testing.T) {
	t.Parallel()

	fixture := newLiveSearchCostFixture(t, research.ResearchCostUsage{SearchRequests: 2, ProviderAPICalls: 3})
	provider := &costControlledSearchProvider{providerCalls: 2, results: fixture.results}
	service, err := application.NewCostControlledDiscoveryService(
		provider, nil, fixture.access(true), fixture.costs, fixedClock{now: fixture.at},
		fixture.policy(8),
	)
	if err != nil {
		t.Fatal(err)
	}

	results, err := service.Search(context.Background(), application.ResearchModeOnline, fixture.query, application.SearchOptions{Limit: 8})
	if err != nil || len(results) != 1 || provider.searches != 1 || provider.apiCalls != 2 {
		t.Fatalf("Search() = (%+v, %v), searches/API calls = %d/%d", results, err, provider.searches, provider.apiCalls)
	}
	metadata, err := fixture.costs.Metadata(context.Background(), fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Used.SearchRequests != 1 || metadata.Used.ProviderAPICalls != 2 {
		t.Fatalf("reserved usage = %+v", metadata.Used)
	}
}

func TestCostControlledDiscoveryStopsBeforeProviderAtRunBudget(t *testing.T) {
	t.Parallel()

	fixture := newLiveSearchCostFixture(t, research.ResearchCostUsage{SearchRequests: 1, ProviderAPICalls: 2})
	provider := &costControlledSearchProvider{providerCalls: 2, results: fixture.results}
	service, err := application.NewCostControlledDiscoveryService(
		provider, nil, fixture.access(true), fixture.costs, fixedClock{now: fixture.at},
		fixture.policy(8),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Search(context.Background(), application.ResearchModeOnline, fixture.query, application.SearchOptions{Limit: 8}); err != nil {
		t.Fatal(err)
	}
	_, err = service.Search(context.Background(), application.ResearchModeOnline, fixture.query, application.SearchOptions{Limit: 8})
	if !errors.Is(err, application.ErrBudgetExceeded) {
		t.Fatalf("second Search() error = %v, want budget_exceeded", err)
	}
	if provider.searches != 1 || provider.apiCalls != 2 {
		t.Fatalf("provider searches/API calls = %d/%d, want 1/2", provider.searches, provider.apiCalls)
	}
	metadata, err := fixture.costs.Metadata(context.Background(), fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if !metadata.StoppedByBudget || metadata.StopScope != research.ResearchBudgetRun ||
		metadata.Used.SearchRequests != 1 || metadata.Used.ProviderAPICalls != 2 {
		t.Fatalf("budget metadata = %+v", metadata)
	}
}

func TestCostControlledDiscoveryRejectsUnboundedResultsAndPrivacyBeforeReservation(t *testing.T) {
	t.Parallel()

	fixture := newLiveSearchCostFixture(t, research.ResearchCostUsage{SearchRequests: 2, ProviderAPICalls: 2})
	provider := &costControlledSearchProvider{providerCalls: 1, results: fixture.results}
	service, err := application.NewCostControlledDiscoveryService(
		provider, nil, fixture.access(true), fixture.costs, fixedClock{now: fixture.at},
		fixture.policy(4),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Search(context.Background(), application.ResearchModeOnline, fixture.query, application.SearchOptions{Limit: 5}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("oversize Search() error = %v", err)
	}

	blocked, err := application.NewCostControlledDiscoveryService(
		provider, nil, fixture.access(false), fixture.costs, fixedClock{now: fixture.at},
		fixture.policy(4),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blocked.Search(context.Background(), application.ResearchModeOnline, fixture.query, application.SearchOptions{Limit: 4}); !errors.Is(err, application.ErrNetworkDisabled) {
		t.Fatalf("blocked Search() error = %v", err)
	}
	metadata, err := fixture.costs.Metadata(context.Background(), fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if !metadata.Used.IsZero() || provider.searches != 0 || provider.apiCalls != 0 {
		t.Fatalf("blocked usage/provider = %+v/%d/%d", metadata.Used, provider.searches, provider.apiCalls)
	}
}

func TestCostControlledDiscoveryStopsPaginationAtProviderCallBudget(t *testing.T) {
	t.Parallel()

	fixture := newLiveSearchCostFixture(t, research.ResearchCostUsage{SearchRequests: 2, ProviderAPICalls: 1})
	provider := &costControlledSearchProvider{providerCalls: 2, results: fixture.results}
	service, err := application.NewCostControlledDiscoveryService(
		provider, nil, fixture.access(true), fixture.costs, fixedClock{now: fixture.at},
		fixture.policy(8),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Search(context.Background(), application.ResearchModeOnline, fixture.query, application.SearchOptions{Limit: 8})
	if !errors.Is(err, application.ErrBudgetExceeded) {
		t.Fatalf("Search() error = %v, want budget_exceeded", err)
	}
	if provider.searches != 0 || provider.apiCalls != 1 {
		t.Fatalf("provider searches/API calls = %d/%d, want 0/1", provider.searches, provider.apiCalls)
	}
	metadata, err := fixture.costs.Metadata(context.Background(), fixture.runID)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Used.SearchRequests != 1 || metadata.Used.ProviderAPICalls != 1 || !metadata.StoppedByBudget {
		t.Fatalf("budget metadata = %+v", metadata)
	}
}

type liveSearchCostFixture struct {
	runID   research.ID
	at      research.Timestamp
	query   application.SearchQuery
	results []application.SearchResult
	costs   application.ResearchCostService
}

func newLiveSearchCostFixture(t *testing.T, runLimit research.ResearchCostUsage) liveSearchCostFixture {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	runID := testID(t, "run.live-search-cost")
	requestID := testID(t, "request.live-search-cost")
	at, err := research.NewTimestamp(time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	budget := research.DefaultResearchCostBudgetV1()
	budget.PerRun.SearchRequests = runLimit.SearchRequests
	budget.PerRun.ProviderAPICalls = runLimit.ProviderAPICalls
	metadata := research.ResearchCostMetadata{Budget: budget, AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	topic, err := research.NewResearchTopic("live-search-cost", "software", "go")
	if err != nil {
		t.Fatal(err)
	}
	request := research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: at}
	run := research.ResearchRun{ID: runID, RequestID: requestID, Status: research.ResearchRunRunning, StartedAt: at, Cost: &metadata}
	if err := application.NewResearchService(store.Repositories().Runs).Start(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	locator, err := research.NewSourceLocator("https://example.test/docs")
	if err != nil {
		t.Fatal(err)
	}
	return liveSearchCostFixture{
		runID: runID, at: at,
		query:   application.SearchQuery{RequestID: requestID, Text: "Go interfaces"},
		results: []application.SearchResult{{Title: "Docs", Locator: locator, Provider: "fixture", Rank: 0}},
		costs:   application.NewResearchCostService(store.Repositories().Costs),
	}
}

func (fixture liveSearchCostFixture) access(allowed bool) application.NetworkResearchAccess {
	return application.NetworkResearchAccess{Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: allowed}, nil)}
}

func (fixture liveSearchCostFixture) policy(maxResults int) application.LiveSearchCostPolicy {
	return application.LiveSearchCostPolicy{
		RunID: fixture.runID, MaxResultsPerQuery: maxResults,
		AlgorithmVersion: application.LiveSearchCostPolicyV1,
	}
}

type costControlledSearchProvider struct {
	providerCalls int
	results       []application.SearchResult
	searches      int
	apiCalls      int
}

func (provider *costControlledSearchProvider) Search(context.Context, application.SearchQuery, application.SearchOptions) ([]application.SearchResult, error) {
	provider.searches++
	return provider.results, nil
}

func (provider *costControlledSearchProvider) SearchWithCostControl(
	ctx context.Context,
	_ application.SearchQuery,
	_ application.SearchOptions,
	authorize application.ProviderCallAuthorizer,
) ([]application.SearchResult, error) {
	for range provider.providerCalls {
		if err := authorize(ctx); err != nil {
			return nil, err
		}
		provider.apiCalls++
	}
	provider.searches++
	return provider.results, nil
}

var _ application.CostControlledSearchProvider = (*costControlledSearchProvider)(nil)
