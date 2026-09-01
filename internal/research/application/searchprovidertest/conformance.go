// Package searchprovidertest provides the reusable conformance suite for the
// application-owned SearchProvider port.
package searchprovidertest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

// Factory constructs a provider whose deterministic result set is supplied by
// the suite. Adapters may translate these results to their native fixture
// representation, but the returned value must be the real implementation.
type Factory func(*testing.T, []application.SearchResult) application.SearchProvider

// Run executes the provider-neutral SearchProvider contract. Vendor-specific
// request, response, transport, and error behavior belongs in adapter tests.
func Run(t *testing.T, providerID string, factory Factory) {
	t.Helper()
	if providerID == "" {
		t.Fatal("conformance provider ID is required")
	}
	if factory == nil {
		t.Fatal("conformance provider factory is required")
	}

	t.Run("valid bounded results", func(t *testing.T) {
		fixtures := fixtureResults(t, providerID, 3)
		provider := factory(t, fixtures)
		query, options := fixtureRequest(t, 2)

		results, err := provider.Search(context.Background(), query, options)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(results) != options.Limit {
			t.Fatalf("Search() returned %d results, want bounded limit %d", len(results), options.Limit)
		}
		for index, result := range results {
			if err := result.Validate(); err != nil {
				t.Fatalf("result %d is invalid: %v", index, err)
			}
			assertResult(t, result, fixtures[index])
		}
		if options.DesiredKind == nil || *options.DesiredKind != research.SourceSpecification ||
			options.TargetVersion == nil || options.TargetVersion.String() != "v1.24" {
			t.Fatalf("Search() mutated options: %+v", options)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		provider := factory(t, nil)
		query, options := fixtureRequest(t, 1)
		results, err := provider.Search(context.Background(), query, options)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("Search() results = %+v, want empty", results)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		provider := factory(t, fixtureResults(t, providerID, 1))
		query, options := fixtureRequest(t, 1)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := provider.Search(ctx, query, options); !errors.Is(err, context.Canceled) {
			t.Fatalf("Search() error = %v, want context.Canceled", err)
		}
	})

	t.Run("result ownership", func(t *testing.T) {
		fixtures := fixtureResults(t, providerID, 1)
		provider := factory(t, fixtures)
		query, options := fixtureRequest(t, 1)
		first, err := provider.Search(context.Background(), query, options)
		if err != nil {
			t.Fatal(err)
		}
		first[0].Title = "caller mutation"
		firstPublished := first[0].PublishedHint

		second, err := provider.Search(context.Background(), query, options)
		if err != nil {
			t.Fatal(err)
		}
		assertResult(t, second[0], fixtures[0])
		if firstPublished == second[0].PublishedHint {
			t.Fatal("Search() reused caller-visible timestamp storage")
		}
	})
}

func fixtureRequest(t *testing.T, limit int) (application.SearchQuery, application.SearchOptions) {
	t.Helper()
	requestID, err := research.NewID("request.search-provider-conformance")
	if err != nil {
		t.Fatal(err)
	}
	version, err := research.NewSourceVersion("v1.24")
	if err != nil {
		t.Fatal(err)
	}
	kind := research.SourceSpecification
	return application.SearchQuery{RequestID: requestID, Text: "Go interfaces"}, application.SearchOptions{
		DesiredKind: &kind, TargetVersion: &version, Limit: limit,
	}
}

func fixtureResults(t *testing.T, providerID string, count int) []application.SearchResult {
	t.Helper()
	results := make([]application.SearchResult, count)
	for index := range results {
		locator, err := research.NewSourceLocator(fmt.Sprintf("https://example.test/conformance/%c", 'a'+index))
		if err != nil {
			t.Fatal(err)
		}
		published, err := research.NewTimestamp(time.Date(2026, time.September, index+1, 12, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		results[index] = application.SearchResult{
			Title: fmt.Sprintf("Conformance result %c", 'A'+index), Locator: locator,
			Snippet: "Provider-neutral candidate.", Provider: providerID, Rank: index, PublishedHint: &published,
		}
	}
	return results
}

func assertResult(t *testing.T, got, want application.SearchResult) {
	t.Helper()
	if got.Title != want.Title || got.Locator != want.Locator || got.Snippet != want.Snippet ||
		got.Provider != want.Provider || got.Rank != want.Rank || got.CacheHit || got.CacheStale || got.CacheWarning != "" ||
		got.PublishedHint == nil || want.PublishedHint == nil || !got.PublishedHint.Time().Equal(want.PublishedHint.Time()) {
		t.Fatalf("result = %+v, want %+v", got, want)
	}
}
