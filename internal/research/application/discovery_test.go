package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestDiscoveryNormalizesDeduplicatesAndPreservesProviderRanks(t *testing.T) {
	t.Parallel()

	published := testTimestamp(t, 9)
	desiredKind := research.SourceOfficialDocumentation
	targetVersion, err := research.NewSourceVersion("1.24")
	if err != nil {
		t.Fatal(err)
	}
	provider := &capturingSearchProvider{results: []application.SearchResult{
		{
			Title: "  Official\n documentation ", Locator: discoveryLocator(t, "https://DOCS.example.test/guide#overview"),
			Snippet: " Use\tthe guide. ", Provider: " fixture\nsearch ", Rank: 7, PublishedHint: &published,
		},
		{
			Title: "Duplicate deep link", Locator: discoveryLocator(t, "https://docs.example.test/guide#api"),
			Provider: "fixture search", Rank: 2,
		},
		{
			Title: " Specification ", Locator: discoveryLocator(t, "https://spec.example.test/current"),
			Provider: " fixture search ", Rank: 41,
		},
		{
			Title: "Beyond requested limit", Locator: discoveryLocator(t, "https://example.test/extra"),
			Provider: "fixture search", Rank: 1,
		},
	}}
	query := application.SearchQuery{
		RequestID: testID(t, "request.discovery.normalize"),
		Text:      "  official\n  documentation\t1.24 ",
	}
	options := application.SearchOptions{
		DesiredKind: &desiredKind, TargetVersion: &targetVersion, Limit: 2,
	}
	service := application.NewDiscoveryService(
		provider, nil,
		application.NetworkResearchAccess{Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: true}, nil)},
	)

	results, err := service.Search(context.Background(), application.ResearchModeOnline, query, options)
	if err != nil {
		t.Fatal(err)
	}
	if provider.query.Text != "official documentation 1.24" || provider.options.Limit != options.Limit {
		t.Fatalf("provider input = (%+v, %+v)", provider.query, provider.options)
	}
	if provider.options.DesiredKind == nil || *provider.options.DesiredKind != desiredKind ||
		provider.options.TargetVersion == nil || *provider.options.TargetVersion != targetVersion {
		t.Fatalf("provider options = %+v", provider.options)
	}
	if provider.options.DesiredKind == options.DesiredKind || provider.options.TargetVersion == options.TargetVersion {
		t.Fatal("provider received caller-owned option pointers")
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v, want two unique candidates", results)
	}
	if results[0].Title != "Official documentation" || results[0].Snippet != "Use the guide." ||
		results[0].Provider != "fixture search" || results[0].Locator.String() != "https://docs.example.test/guide" {
		t.Fatalf("normalized first result = %+v", results[0])
	}
	if results[0].Rank != 7 || results[1].Rank != 41 {
		t.Fatalf("ranks = [%d, %d], want provider ranks [7, 41]", results[0].Rank, results[1].Rank)
	}
	if results[0].PublishedHint == nil || results[0].PublishedHint.Time() != published.Time() {
		t.Fatalf("published hint = %+v", results[0].PublishedHint)
	}
}

func TestDiscoveryRejectsInvalidOptionsAndUnboundedProviderOutput(t *testing.T) {
	t.Parallel()

	query := application.SearchQuery{RequestID: testID(t, "request.discovery.bounds"), Text: "docs"}
	provider := &capturingSearchProvider{}
	service := application.NewDiscoveryService(
		provider, nil,
		application.NetworkResearchAccess{Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: true}, nil)},
	)
	if _, err := service.Search(context.Background(), application.ResearchModeOnline, query, application.SearchOptions{
		Limit: application.MaximumSearchResults + 1,
	}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("oversize option error = %v, want invalid_state", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want zero", provider.calls)
	}

	result := application.SearchResult{
		Title: "Documentation", Locator: discoveryLocator(t, "https://example.test/docs"), Provider: "fixture", Rank: 0,
	}
	provider.results = make([]application.SearchResult, application.MaximumSearchResults+1)
	for index := range provider.results {
		provider.results[index] = result
	}
	_, err := service.Search(context.Background(), application.ResearchModeOnline, query, application.SearchOptions{Limit: 5})
	if !errors.Is(err, application.ErrExternalFailure) || !strings.Contains(err.Error(), "more than 100 results") {
		t.Fatalf("unbounded provider error = %v", err)
	}
}

func TestStaticSearchProviderIsNetworkFreeAndDefensive(t *testing.T) {
	t.Parallel()

	published := testTimestamp(t, 9)
	original := []application.SearchResult{{
		Title: "Fixture", Locator: discoveryLocator(t, "https://example.test/fixture"),
		Provider: "static", Rank: 3, PublishedHint: &published,
	}}
	provider := memory.NewStaticSearchProvider(original)
	original[0].Title = "mutated"
	query := application.SearchQuery{RequestID: testID(t, "request.discovery.static"), Text: "fixture"}
	options := application.SearchOptions{Limit: 5}

	first, err := provider.Search(context.Background(), query, options)
	if err != nil {
		t.Fatal(err)
	}
	first[0].Title = "changed"
	second, err := provider.Search(context.Background(), query, options)
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Title != "Fixture" || second[0].PublishedHint == first[0].PublishedHint {
		t.Fatalf("defensive fixture results = %+v", second)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Search(canceled, query, options); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Search() error = %v", err)
	}
}

type capturingSearchProvider struct {
	results []application.SearchResult
	err     error
	calls   int
	query   application.SearchQuery
	options application.SearchOptions
}

func (provider *capturingSearchProvider) Search(
	_ context.Context,
	query application.SearchQuery,
	options application.SearchOptions,
) ([]application.SearchResult, error) {
	provider.calls++
	provider.query = query
	provider.options = options
	return provider.results, provider.err
}

func discoveryLocator(t *testing.T, value string) research.SourceLocator {
	t.Helper()
	locator, err := research.NewSourceLocator(value)
	if err != nil {
		t.Fatal(err)
	}
	return locator
}
