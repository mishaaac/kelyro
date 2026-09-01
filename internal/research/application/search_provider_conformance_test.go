package application_test

import (
	"testing"

	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/research/application/searchprovidertest"
)

func TestStaticSearchProviderConformance(t *testing.T) {
	t.Parallel()
	searchprovidertest.Run(t, "static", func(_ *testing.T, results []application.SearchResult) application.SearchProvider {
		return memory.NewStaticSearchProvider(results)
	})
}
