package application_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestSearchProviderContractSupportsRequiredAndOptionalFields(t *testing.T) {
	t.Parallel()

	desiredKind := research.SourceOfficialDocumentation
	targetVersion, err := research.NewSourceVersion("1.24")
	if err != nil {
		t.Fatal(err)
	}
	published := testTimestamp(t, 9)
	query := application.SearchQuery{RequestID: testID(t, "request.search-contract"), Text: "Go interfaces"}
	options := application.SearchOptions{DesiredKind: &desiredKind, TargetVersion: &targetVersion, Limit: 8}
	result := application.SearchResult{
		Title:    "The Go Programming Language Specification",
		Locator:  discoveryLocator(t, "https://go.dev/ref/spec"),
		Provider: "contract-fixture", Rank: 3, PublishedHint: &published,
	}

	if err := query.Validate(); err != nil {
		t.Fatalf("valid query: %v", err)
	}
	if err := options.Validate(); err != nil {
		t.Fatalf("valid options: %v", err)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result with optional snippet omitted: %v", err)
	}
	result.Snippet = "Interfaces define type sets."
	if err := result.Validate(); err != nil {
		t.Fatalf("result with optional snippet present: %v", err)
	}
}

func TestSearchProviderContractRejectsMissingOrInvalidFields(t *testing.T) {
	t.Parallel()

	validRequestID := testID(t, "request.search-contract-invalid")
	for _, test := range []struct {
		name  string
		query application.SearchQuery
	}{
		{name: "request ID", query: application.SearchQuery{Text: "docs"}},
		{name: "query text", query: application.SearchQuery{RequestID: validRequestID}},
		{name: "query control", query: application.SearchQuery{RequestID: validRequestID, Text: "docs\x1b[2J"}},
		{name: "query bytes", query: application.SearchQuery{RequestID: validRequestID, Text: strings.Repeat("q", research.MaximumDiscoveryQueryBytes+1)}},
	} {
		t.Run("query "+test.name, func(t *testing.T) {
			if err := test.query.Validate(); err == nil {
				t.Fatal("invalid query passed validation")
			}
		})
	}

	invalidKind := research.SourceKind("invented")
	invalidVersion := research.SourceVersion("")
	for _, test := range []struct {
		name    string
		options application.SearchOptions
	}{
		{name: "zero limit", options: application.SearchOptions{}},
		{name: "oversize limit", options: application.SearchOptions{Limit: application.MaximumSearchResults + 1}},
		{name: "desired kind", options: application.SearchOptions{DesiredKind: &invalidKind, Limit: 1}},
		{name: "target version", options: application.SearchOptions{TargetVersion: &invalidVersion, Limit: 1}},
	} {
		t.Run("options "+test.name, func(t *testing.T) {
			if err := test.options.Validate(); err == nil {
				t.Fatal("invalid options passed validation")
			}
		})
	}

	valid := application.SearchResult{
		Title: "Documentation", Locator: discoveryLocator(t, "https://example.test/docs"),
		Provider: "contract-fixture", Rank: 0,
	}
	invalidTimestamp := research.Timestamp{}
	for _, test := range []struct {
		name   string
		mutate func(*application.SearchResult)
	}{
		{name: "title", mutate: func(result *application.SearchResult) { result.Title = "" }},
		{name: "locator", mutate: func(result *application.SearchResult) { result.Locator = research.SourceLocator{} }},
		{name: "provider", mutate: func(result *application.SearchResult) { result.Provider = "" }},
		{name: "rank", mutate: func(result *application.SearchResult) { result.Rank = -1 }},
		{name: "published hint", mutate: func(result *application.SearchResult) { result.PublishedHint = &invalidTimestamp }},
		{name: "snippet whitespace", mutate: func(result *application.SearchResult) { result.Snippet = "  fragment  " }},
		{name: "title control", mutate: func(result *application.SearchResult) { result.Title = "docs\x1b[2J" }},
		{name: "snippet control", mutate: func(result *application.SearchResult) { result.Snippet = "docs\x00" }},
		{name: "provider control", mutate: func(result *application.SearchResult) { result.Provider = "fixture\x1b" }},
		{name: "title bytes", mutate: func(result *application.SearchResult) {
			result.Title = strings.Repeat("t", research.MaximumDiscoveryTitleBytes+1)
		}},
		{name: "snippet bytes", mutate: func(result *application.SearchResult) {
			result.Snippet = strings.Repeat("s", research.MaximumDiscoverySnippetBytes+1)
		}},
		{name: "provider bytes", mutate: func(result *application.SearchResult) {
			result.Provider = strings.Repeat("p", research.MaximumDiscoveryProviderBytes+1)
		}},
	} {
		t.Run("result "+test.name, func(t *testing.T) {
			result := valid
			test.mutate(&result)
			if err := result.Validate(); err == nil {
				t.Fatal("invalid result passed validation")
			}
		})
	}
}

func TestSearchProviderContractCrossesDiscoveryBoundary(t *testing.T) {
	t.Parallel()

	desiredKind := research.SourceSpecification
	targetVersion, err := research.NewSourceVersion("go1.24")
	if err != nil {
		t.Fatal(err)
	}
	published := testTimestamp(t, 8)
	provider := &searchProviderContractProbe{results: []application.SearchResult{{
		Title:   "  Language\n specification ",
		Locator: discoveryLocator(t, "https://SPEC.example.test/current#Interface_types"),
		Snippet: " Type\tsets. ", Provider: " reference\tapi ", Rank: 5, PublishedHint: &published,
	}}}
	query := application.SearchQuery{RequestID: testID(t, "request.search-contract-boundary"), Text: "  Go\n interfaces "}
	options := application.SearchOptions{DesiredKind: &desiredKind, TargetVersion: &targetVersion, Limit: 4}
	service := application.NewDiscoveryService(provider, nil, application.NetworkResearchAccess{
		Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: true}, nil),
	})

	results, err := service.Search(context.Background(), application.ResearchModeOnline, query, options)
	if err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || provider.query.RequestID != query.RequestID || provider.query.Text != "Go interfaces" {
		t.Fatalf("provider query = %+v, calls = %d", provider.query, provider.calls)
	}
	if provider.options.Limit != 4 || provider.options.DesiredKind == nil || *provider.options.DesiredKind != desiredKind ||
		provider.options.TargetVersion == nil || *provider.options.TargetVersion != targetVersion {
		t.Fatalf("provider options = %+v", provider.options)
	}
	if provider.options.DesiredKind == options.DesiredKind || provider.options.TargetVersion == options.TargetVersion {
		t.Fatal("provider received caller-owned option pointers")
	}
	if len(results) != 1 || results[0].Title != "Language specification" ||
		results[0].Locator.String() != "https://spec.example.test/current" ||
		results[0].Snippet != "Type sets." || results[0].Provider != "reference api" || results[0].Rank != 5 ||
		results[0].PublishedHint == nil || results[0].PublishedHint.Time() != published.Time() {
		t.Fatalf("contract result = %+v", results)
	}
}

type searchProviderContractProbe struct {
	results []application.SearchResult
	query   application.SearchQuery
	options application.SearchOptions
	calls   int
}

func (provider *searchProviderContractProbe) Search(
	_ context.Context,
	query application.SearchQuery,
	options application.SearchOptions,
) ([]application.SearchResult, error) {
	provider.calls++
	provider.query = query
	provider.options = options
	return provider.results, nil
}

var _ application.SearchProvider = (*searchProviderContractProbe)(nil)
