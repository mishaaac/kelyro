package memory

import (
	"context"

	"github.com/mishaaac/kelyro/internal/research/application"
)

// StaticSearchProvider is a deterministic, network-free SearchProvider for
// development and tests. Production discovery adapters remain replaceable
// behind the application-owned SearchProvider contract.
type StaticSearchProvider struct {
	results []application.SearchResult
}

var _ application.SearchProvider = (*StaticSearchProvider)(nil)

func NewStaticSearchProvider(results []application.SearchResult) *StaticSearchProvider {
	return &StaticSearchProvider{results: cloneSearchResults(results)}
}

func (provider *StaticSearchProvider) Search(
	ctx context.Context,
	_ application.SearchQuery,
	_ application.SearchOptions,
) ([]application.SearchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return cloneSearchResults(provider.results), nil
}

func cloneSearchResults(results []application.SearchResult) []application.SearchResult {
	cloned := make([]application.SearchResult, len(results))
	for index, result := range results {
		cloned[index] = result
		if result.PublishedHint != nil {
			published := *result.PublishedHint
			cloned[index].PublishedHint = &published
		}
	}
	return cloned
}
