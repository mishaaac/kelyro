//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/infra/researchcachefs"
	"github.com/mishaaac/kelyro/internal/infra/researchdb"
	"github.com/mishaaac/kelyro/internal/infra/researchfetch"
	"github.com/mishaaac/kelyro/internal/infra/researchhttp"
	"github.com/mishaaac/kelyro/internal/infra/researchnormalize"
	"github.com/mishaaac/kelyro/internal/infra/workspacefs"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/storage"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestResearchTopicQueryToBundleEndToEnd(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	workspaces := workspacefs.New("query-to-bundle-e2e")
	if _, err := workspaces.Init(root, workspace.InitOptions{}); err != nil {
		t.Fatal(err)
	}
	fixture := newQueryToBundleFixture(t)
	stores := researchdb.NewFactory("query-to-bundle-e2e")
	seedQueryToBundleSources(t, ctx, stores, root, fixture)

	httpConfig := researchhttp.DefaultConfig()
	httpConfig.UserAgent = "Kelyro/query-to-bundle-e2e"
	httpConfig.RequestTimeout = 3 * time.Second
	httpConfig.DialTimeout = time.Second
	httpConfig.MinimumIntervalPerHost = time.Millisecond
	httpClient, err := researchhttp.NewLoopbackFixtureClient(httpConfig, fixture.contentHosts, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(httpClient.CloseIdleConnections)

	service := app.NewService(workspaces, func() (string, error) { return root, nil }).
		WithConfig(&queryToBundleConfigStore{}).
		WithSecrets(&queryToBundleSecretStore{values: map[string]string{queryToBundleSecretName: "fixture-token"}}).
		WithResearchStores(stores).
		WithResearchCaches(researchcachefs.NewFactory()).
		WithResearchSearch(&queryToBundleSearchFactory{client: fixture.searchAPI.Client(), endpoint: fixture.searchAPI.URL}).
		WithResearchFetcher(researchfetch.New(httpClient)).
		WithResearchNormalizer(researchnormalize.New())

	result, err := service.Execute(ctx, app.Command{
		Action: app.ActionResearch, Workspace: root, ResearchOperation: "topic", ResearchTopic: "Fixture API",
		ConfigOverrides: config.Settings{
			config.KeyAllowNetwork:                     config.BoolValue(true),
			config.KeyResearchSearchProvider:           config.StringValue(queryToBundleProviderID),
			config.KeyResearchSearchMaxResultsPerQuery: config.NumberValue(2),
			config.KeyResearchSearchMaxQueriesPerRun:   config.NumberValue(1),
		},
	})
	if err != nil {
		t.Fatalf("research topic: %v", err)
	}
	if result.ResearchView == nil || result.ResearchView.Execution == nil {
		t.Fatalf("research topic view = %+v", result.ResearchView)
	}
	view := result.ResearchView
	if view.Run.Status != research.ResearchRunCompleted || view.DiscoveryPending ||
		view.Execution.Disposition != application.ResearchQueueConsumeCompleted || view.Bundle == nil {
		t.Fatalf("terminal research view = %+v", view)
	}
	artifacts := view.Execution.Orchestration.Artifacts
	if len(artifacts.SearchResults) != 2 || len(artifacts.Sources) != 2 || len(artifacts.FetchedSources) != 2 ||
		len(artifacts.Snapshots) != 2 || len(artifacts.NormalizedSources) != 2 || len(artifacts.Evidence) < 2 ||
		len(artifacts.Claims) != 2 || len(artifacts.Verifications) != 2 || artifacts.Bundle == nil ||
		len(artifacts.ProvenanceGraphs) != len(artifacts.Claims) {
		t.Fatalf("query-to-bundle artifacts = %+v", artifacts)
	}
	for _, verification := range artifacts.Verifications {
		if verification.Status != research.VerificationVerified && verification.Status != research.VerificationVerifiedCaveat {
			t.Fatalf("verification %q status = %s", verification.ClaimID, verification.Status)
		}
	}
	if fixture.searchCalls.Load() != 1 || fixture.contentCalls[0].Load() != 1 || fixture.contentCalls[1].Load() != 1 {
		t.Fatalf("search/content calls = %d/[%d,%d]", fixture.searchCalls.Load(), fixture.contentCalls[0].Load(), fixture.contentCalls[1].Load())
	}

	store, err := stores.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	durableRun, err := store.Research().Run(ctx, view.Run.ID)
	if err != nil || durableRun.Status != research.ResearchRunCompleted || durableRun.CompletedAt == nil {
		t.Fatalf("durable run = (%+v, %v)", durableRun, err)
	}
	bundles, err := store.Bundles().ListForRun(ctx, durableRun.ID)
	if err != nil || len(bundles) != 1 || bundles[0].ID != view.Bundle.ID || bundles[0].ContentHash == "" {
		t.Fatalf("durable bundles = (%+v, %v)", bundles, err)
	}
	for _, claimID := range bundles[0].ClaimIDs {
		graph, err := store.Provenance().Trace(ctx, claimID)
		if err != nil {
			t.Fatalf("trace Claim %q: %v", claimID, err)
		}
		assertCompleteQueryToBundleProvenance(t, graph, view.Request.ID, durableRun.ID, bundles[0].ID)
	}
}

func TestResearchTopicPrivacyDisabledEndToEnd(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	workspaces := workspacefs.New("privacy-disabled-e2e")
	if _, err := workspaces.Init(root, workspace.InitOptions{}); err != nil {
		t.Fatal(err)
	}
	fixture := newQueryToBundleFixture(t)
	stores := researchdb.NewFactory("privacy-disabled-e2e")

	httpConfig := researchhttp.DefaultConfig()
	httpConfig.UserAgent = "Kelyro/privacy-disabled-e2e"
	httpConfig.RequestTimeout = 3 * time.Second
	httpConfig.DialTimeout = time.Second
	httpConfig.MinimumIntervalPerHost = time.Millisecond
	httpClient, err := researchhttp.NewLoopbackFixtureClient(httpConfig, fixture.contentHosts, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(httpClient.CloseIdleConnections)

	searchFactory := &queryToBundleSearchFactory{client: fixture.searchAPI.Client(), endpoint: fixture.searchAPI.URL}
	service := app.NewService(workspaces, func() (string, error) { return root, nil }).
		WithConfig(&queryToBundleConfigStore{}).
		WithSecrets(&queryToBundleSecretStore{values: map[string]string{queryToBundleSecretName: "fixture-token"}}).
		WithResearchStores(stores).
		WithResearchCaches(researchcachefs.NewFactory()).
		WithResearchSearch(searchFactory).
		WithResearchFetcher(researchfetch.New(httpClient)).
		WithResearchNormalizer(researchnormalize.New())

	_, err = service.Execute(ctx, app.Command{
		Action: app.ActionResearch, Workspace: root, ResearchOperation: "topic", ResearchTopic: "Fixture API",
		ConfigOverrides: config.Settings{
			config.KeyAllowNetwork:                     config.BoolValue(false),
			config.KeyResearchSearchProvider:           config.StringValue(queryToBundleProviderID),
			config.KeyResearchSearchMaxResultsPerQuery: config.NumberValue(2),
			config.KeyResearchSearchMaxQueriesPerRun:   config.NumberValue(1),
		},
	})
	if !errors.Is(err, application.ErrNetworkResearchBlocked) {
		t.Fatalf("research topic error = %v, want network research blocked", err)
	}
	if searchFactory.buildCalls.Load() != 1 || searchFactory.provider == nil {
		t.Fatalf("search factory build/provider = %d/%v", searchFactory.buildCalls.Load(), searchFactory.provider != nil)
	}
	if searchFactory.provider.calls.Load() != 0 || fixture.searchCalls.Load() != 0 ||
		fixture.contentCalls[0].Load() != 0 || fixture.contentCalls[1].Load() != 0 {
		t.Fatalf(
			"provider/search/content calls = %d/%d/[%d,%d], want all zero",
			searchFactory.provider.calls.Load(), fixture.searchCalls.Load(),
			fixture.contentCalls[0].Load(), fixture.contentCalls[1].Load(),
		)
	}

	store, err := stores.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	durableRun, err := store.Research().Run(ctx, searchFactory.runID)
	if err != nil || durableRun.Status != research.ResearchRunFailed || durableRun.CompletedAt == nil {
		t.Fatalf("durable run = (%+v, %v)", durableRun, err)
	}
	audits, err := store.Research().AuditTrail(ctx, durableRun.ID)
	if err != nil || len(audits) == 0 {
		t.Fatalf("durable audit trail = (%+v, %v)", audits, err)
	}
	terminal := audits[len(audits)-1]
	if terminal.Outcome != research.ResearchRunFailed || terminal.NetworkAllowed || terminal.Execution == nil ||
		terminal.Execution.FailureKind != string(application.ErrorNetworkResearchBlocked) ||
		terminal.Execution.ResultCount != 0 || terminal.Execution.FetchCount != 0 || len(terminal.Execution.Providers) != 0 {
		t.Fatalf("terminal privacy audit = %+v", terminal)
	}
	bundles, err := store.Bundles().ListForRun(ctx, durableRun.ID)
	if err != nil || len(bundles) != 0 {
		t.Fatalf("durable bundles = (%+v, %v), want none", bundles, err)
	}
}

const (
	queryToBundleProviderID = "fixture-search-api"
	queryToBundleSecretName = "research.search.fixture-search-api.api_key"
)

type queryToBundleFixture struct {
	searchAPI    *httptest.Server
	content      *httptest.Server
	locators     []research.SourceLocator
	contentHosts []string
	searchCalls  atomic.Int32
	contentCalls [2]atomic.Int32
}

func newQueryToBundleFixture(t *testing.T) *queryToBundleFixture {
	t.Helper()
	fixture := &queryToBundleFixture{contentHosts: []string{"docs-one.fixture.test", "docs-two.fixture.test"}}
	fixture.content = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch request.URL.Path {
		case "/reference":
			fixture.contentCalls[0].Add(1)
			_, _ = io.WriteString(writer, `<html><head><title>Fixture API reference</title></head><body><h1>Fixture API</h1><p>Fixture API is a stable interface.</p></body></html>`)
		case "/guide":
			fixture.contentCalls[1].Add(1)
			_, _ = io.WriteString(writer, `<html><head><title>Fixture API guide</title></head><body><h1>Fixture API</h1><p>Fixture API is a stable contract.</p></body></html>`)
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(fixture.content.Close)
	contentURL, err := url.Parse(fixture.content.URL)
	if err != nil {
		t.Fatal(err)
	}
	for index, path := range []string{"/reference", "/guide"} {
		locator, err := research.NewSourceLocator("http://" + fixture.contentHosts[index] + ":" + contentURL.Port() + path)
		if err != nil {
			t.Fatal(err)
		}
		fixture.locators = append(fixture.locators, locator)
	}

	fixture.searchAPI = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fixture.searchCalls.Add(1)
		if request.Method != http.MethodGet || strings.TrimSpace(request.URL.Query().Get("q")) == "" ||
			request.Header.Get("X-Fixture-Token") != "fixture-token" {
			http.Error(writer, "invalid fixture search request", http.StatusBadRequest)
			return
		}
		limit, err := strconv.Atoi(request.URL.Query().Get("limit"))
		if err != nil || limit < 1 || limit > len(fixture.locators) {
			http.Error(writer, "invalid fixture search limit", http.StatusBadRequest)
			return
		}
		response := queryToBundleSearchResponse{Results: make([]queryToBundleSearchResult, limit)}
		for index := range response.Results {
			response.Results[index] = queryToBundleSearchResult{
				Title: fmt.Sprintf("Fixture API source %d", index+1), URL: fixture.locators[index].String(),
				Snippet: "Controlled query-to-bundle candidate.",
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(response)
	}))
	t.Cleanup(fixture.searchAPI.Close)
	return fixture
}

func seedQueryToBundleSources(t *testing.T, ctx context.Context, stores *researchdb.Factory, root string, fixture *queryToBundleFixture) {
	t.Helper()
	store, err := stores.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close seeded research store: %v", err)
		}
	}()
	at, err := research.NewTimestamp(time.Now().UTC().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for index, locator := range fixture.locators {
		source := research.Source{
			ID:   mustSourceID(t, fmt.Sprintf("source.e2e.query-to-bundle.%d", index+1)),
			Kind: research.SourceOfficialDocumentation, Locator: locator, TemporalScope: research.SourceTemporalCurrent,
			Metadata: research.SourceMetadata{
				Title: fmt.Sprintf("Fixture API source %d", index+1), Publisher: fmt.Sprintf("Fixture Organization %d", index+1),
				Language: "en", UpdatedAt: &at,
			},
			CreatedAt: at,
		}
		if err := store.Sources().Register(ctx, source); err != nil {
			t.Fatal(err)
		}
		domain, err := research.NewCanonicalDomain(fixture.contentHosts[index])
		if err != nil {
			t.Fatal(err)
		}
		entry := research.SourceRegistryEntry{
			ID:               mustID(t, fmt.Sprintf("registry.e2e.query-to-bundle.%d", index+1)),
			Organization:     fmt.Sprintf("Fixture Organization %d", index+1),
			CanonicalDomains: []research.CanonicalDomain{domain},
			SourceKinds:      []research.SourceKind{research.SourceOfficialDocumentation},
			AuthorityHints: []research.RegistryAuthorityHint{{
				SourceKind: research.SourceOfficialDocumentation, Tier: research.AuthorityTierB, Reason: "Controlled E2E official source.",
			}},
			ResearchDomains: []string{"*"}, TopicPatterns: []string{"*"}, Status: research.RegistryTrusted,
			AddedAt: at, LastReviewedAt: at,
		}
		if err := store.Registry().Save(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
}

type queryToBundleSearchFactory struct {
	client     *http.Client
	endpoint   string
	buildCalls atomic.Int32
	runID      research.ID
	provider   *queryToBundleSearchProvider
}

func (factory *queryToBundleSearchFactory) Build(ctx context.Context, request application.LiveSearchBuildRequest) (application.LiveSearchBuildResult, error) {
	if err := request.Validate(); err != nil {
		return application.LiveSearchBuildResult{}, err
	}
	if request.Settings.Provider != queryToBundleProviderID || factory == nil || factory.client == nil {
		return application.LiveSearchBuildResult{}, errors.New("fixture search provider is unavailable")
	}
	token, err := request.Secrets.Get(queryToBundleSecretName)
	if err != nil {
		return application.LiveSearchBuildResult{}, err
	}
	provider := &queryToBundleSearchProvider{client: factory.client, endpoint: factory.endpoint, token: token}
	factory.buildCalls.Add(1)
	factory.runID = request.RunID
	factory.provider = provider
	discovery, err := application.NewCostControlledDiscoveryService(
		provider, nil, request.Access, request.Costs, request.Clock,
		application.LiveSearchCostPolicy{
			RunID: request.RunID, MaxResultsPerQuery: request.Settings.MaxResultsPerQuery,
			AlgorithmVersion: application.LiveSearchCostPolicyV1,
		},
	)
	if err != nil {
		return application.LiveSearchBuildResult{}, err
	}
	return application.LiveSearchBuildResult{
		Provider: provider, Discovery: discovery, ProviderID: queryToBundleProviderID, AdapterVersion: "fixture-search-api-v1",
	}, nil
}

type queryToBundleSearchProvider struct {
	client   *http.Client
	endpoint string
	token    string
	calls    atomic.Int32
}

func (provider *queryToBundleSearchProvider) Search(ctx context.Context, query application.SearchQuery, options application.SearchOptions) ([]application.SearchResult, error) {
	provider.calls.Add(1)
	return provider.search(ctx, query, options, nil)
}

func (provider *queryToBundleSearchProvider) SearchWithCostControl(
	ctx context.Context,
	query application.SearchQuery,
	options application.SearchOptions,
	authorize application.ProviderCallAuthorizer,
) ([]application.SearchResult, error) {
	provider.calls.Add(1)
	if authorize == nil {
		return nil, errors.New("fixture provider authorizer is unavailable")
	}
	return provider.search(ctx, query, options, authorize)
}

func (provider *queryToBundleSearchProvider) search(
	ctx context.Context,
	query application.SearchQuery,
	options application.SearchOptions,
	authorize application.ProviderCallAuthorizer,
) ([]application.SearchResult, error) {
	if err := query.Validate(); err != nil {
		return nil, err
	}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(provider.endpoint)
	if err != nil {
		return nil, err
	}
	parameters := endpoint.Query()
	parameters.Set("q", query.Text)
	parameters.Set("limit", strconv.Itoa(options.Limit))
	endpoint.RawQuery = parameters.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Fixture-Token", provider.token)
	if authorize != nil {
		if err := authorize(ctx); err != nil {
			return nil, err
		}
	}
	response, err := provider.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fixture search status %d", response.StatusCode)
	}
	var payload queryToBundleSearchResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	results := make([]application.SearchResult, 0, min(len(payload.Results), options.Limit))
	for index, item := range payload.Results {
		if len(results) == options.Limit {
			break
		}
		locator, err := research.NewSourceLocator(item.URL)
		if err != nil {
			return nil, err
		}
		result := application.SearchResult{
			Title: item.Title, Locator: locator, Snippet: item.Snippet, Provider: queryToBundleProviderID, Rank: index,
		}
		if err := result.Validate(); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

type queryToBundleSearchResponse struct {
	Results []queryToBundleSearchResult `json:"results"`
}

type queryToBundleSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

func assertCompleteQueryToBundleProvenance(t *testing.T, graph research.ProvenanceGraph, requestID, runID, bundleID research.ID) {
	t.Helper()
	if err := graph.Validate(); err != nil {
		t.Fatalf("provenance graph: %v", err)
	}
	want := map[research.ProvenanceNodeKind]bool{
		research.ProvenanceRequest: false, research.ProvenanceRun: false, research.ProvenanceQuery: false,
		research.ProvenanceDiscoveredSource: false, research.ProvenanceSource: false, research.ProvenanceSnapshot: false,
		research.ProvenanceEvidence: false, research.ProvenanceClaim: false, research.ProvenanceSourceBundle: false,
	}
	for _, node := range graph.Nodes {
		if _, required := want[node.Kind]; required {
			want[node.Kind] = true
		}
		if node.Kind == research.ProvenanceRequest && node.ID != requestID {
			t.Fatalf("provenance request = %q, want %q", node.ID, requestID)
		}
		if node.Kind == research.ProvenanceRun && node.ID != runID {
			t.Fatalf("provenance run = %q, want %q", node.ID, runID)
		}
		if node.Kind == research.ProvenanceSourceBundle && node.ID != bundleID {
			t.Fatalf("provenance bundle = %q, want %q", node.ID, bundleID)
		}
	}
	for kind, found := range want {
		if !found {
			t.Fatalf("provenance graph lacks %s: %+v", kind, graph)
		}
	}
}

type queryToBundleConfigStore struct{}

func (*queryToBundleConfigStore) GlobalPath() (string, error)          { return "e2e-global.toml", nil }
func (*queryToBundleConfigStore) ProjectPath(string) (string, error)   { return "e2e-project.toml", nil }
func (*queryToBundleConfigStore) LoadGlobal() (config.Settings, error) { return config.Settings{}, nil }
func (*queryToBundleConfigStore) LoadProject(string) (config.Settings, error) {
	return config.Settings{}, nil
}
func (*queryToBundleConfigStore) SaveGlobal(config.Settings) error              { return nil }
func (*queryToBundleConfigStore) SaveProject(string, config.Settings) error     { return nil }
func (*queryToBundleConfigStore) SetGlobal(string, config.Value) error          { return nil }
func (*queryToBundleConfigStore) SetProject(string, string, config.Value) error { return nil }

type queryToBundleSecretStore struct{ values map[string]string }

func (store *queryToBundleSecretStore) Get(name string) (string, error) {
	value, exists := store.values[name]
	if !exists {
		return "", storage.ErrSecretNotFound
	}
	return value, nil
}
func (store *queryToBundleSecretStore) Set(name, value string) error {
	if store.values == nil {
		store.values = make(map[string]string)
	}
	store.values[name] = value
	return nil
}
func (store *queryToBundleSecretStore) Delete(name string) error {
	delete(store.values, name)
	return nil
}
func (store *queryToBundleSecretStore) Status() ([]storage.SecretStatus, error) { return nil, nil }
func (*queryToBundleSecretStore) Availability() error                           { return nil }

var (
	_ application.LiveSearchProviderFactory    = (*queryToBundleSearchFactory)(nil)
	_ application.CostControlledSearchProvider = (*queryToBundleSearchProvider)(nil)
	_ config.Store                             = (*queryToBundleConfigStore)(nil)
	_ storage.SecretStore                      = (*queryToBundleSecretStore)(nil)
)
