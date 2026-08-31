package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestMapSearchResultsToSourceCandidatesPreservesUntrustedDiscoveryMetadata(t *testing.T) {
	t.Parallel()
	published := testTimestamp(t, 8)
	discovered := testTimestamp(t, 10)
	query := application.SearchQuery{RequestID: testID(t, "request.candidate-map"), Text: "  Go\n interfaces docs "}
	results := []application.SearchResult{
		{
			Title: "  Interface\n documentation ", Locator: discoveryLocator(t, "HTTPS://DOCS.EXAMPLE.TEST/interfaces#methods"),
			Snippet: " Defines\tinterface types. ", Provider: " brave\nsearch ", Rank: 3, PublishedHint: &published,
		},
		{
			Title: "Duplicate URL", Locator: discoveryLocator(t, "https://docs.example.test/interfaces#examples"),
			Provider: "brave search", Rank: 7,
		},
	}

	candidates, err := application.MapSearchResultsToSourceCandidatesV1(context.Background(), application.SourceCandidateMappingInput{
		Query: query, Results: results, DiscoveredAt: discovered,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 {
		t.Fatalf("candidates = %+v, want one per result before Step 18", candidates)
	}
	first := candidates[0]
	if first.Locator.String() != "https://docs.example.test/interfaces" || first.AlgorithmVersion != application.SourceCandidateMapperV1 || len(first.Discoveries) != 1 {
		t.Fatalf("mapped candidate = %+v", first)
	}
	discovery := first.Discoveries[0]
	if discovery.Query.RequestID != query.RequestID || discovery.Query.Text != "Go interfaces docs" || discovery.Title != "Interface documentation" ||
		discovery.Snippet != "Defines interface types." || discovery.Provider != "brave search" || discovery.Rank != 3 ||
		discovery.DiscoveredAt.Time() != discovered.Time() || discovery.PublishedHint == nil || discovery.PublishedHint.Time() != published.Time() {
		t.Fatalf("mapped discovery = %+v", discovery)
	}
	if discovery.PublishedHint == results[0].PublishedHint {
		t.Fatal("candidate retained provider-owned timestamp pointer")
	}
	results[0].PublishedHint = nil
	if candidates[0].Discoveries[0].PublishedHint == nil {
		t.Fatal("candidate changed with provider result")
	}
	if err := first.Validate(); err != nil {
		t.Fatalf("candidate validation = %v", err)
	}
}

func TestMapSearchResultsToSourceCandidatesRejectsInvalidOrUnboundedInput(t *testing.T) {
	t.Parallel()
	query := application.SearchQuery{RequestID: testID(t, "request.candidate-invalid"), Text: "docs"}
	valid := application.SearchResult{Title: "Docs", Locator: discoveryLocator(t, "https://example.test/docs"), Provider: "fixture", Rank: 0}
	oversized := make([]application.SearchResult, application.MaximumSearchResults+1)
	for index := range oversized {
		oversized[index] = valid
	}
	if _, err := application.MapSearchResultsToSourceCandidatesV1(context.Background(), application.SourceCandidateMappingInput{
		Query: query, Results: oversized, DiscoveredAt: testTimestamp(t, 10),
	}); err == nil {
		t.Fatal("mapper accepted unbounded results")
	}
	invalid := valid
	invalid.Rank = -1
	if _, err := application.MapSearchResultsToSourceCandidatesV1(context.Background(), application.SourceCandidateMappingInput{
		Query: query, Results: []application.SearchResult{invalid}, DiscoveredAt: testTimestamp(t, 10),
	}); err == nil {
		t.Fatal("mapper accepted invalid provider metadata")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := application.MapSearchResultsToSourceCandidatesV1(cancelled, application.SourceCandidateMappingInput{
		Query: query, Results: []application.SearchResult{valid}, DiscoveredAt: testTimestamp(t, 10),
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled mapping error = %v", err)
	}
}
