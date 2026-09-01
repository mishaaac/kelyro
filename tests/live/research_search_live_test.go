package live_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/researchsearch"
	"github.com/mishaaac/kelyro/internal/infra/secretstore"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

const liveResearchSearchEnvironment = "KELYRO_LIVE_RESEARCH_SEARCH_TESTS"

func TestLiveResearchSearchProvider(t *testing.T) {
	if os.Getenv(liveResearchSearchEnvironment) != "1" {
		t.Skipf("set %s=1 to run the external search provider smoke", liveResearchSearchEnvironment)
	}

	transportConfig := researchsearch.DefaultTransportConfig()
	transportConfig.UserAgent = "Kelyro/live-research-search-test"
	transportConfig.RequestTimeout = 15 * time.Second
	transportConfig.DialTimeout = 5 * time.Second
	transportConfig.TLSHandshakeTimeout = 5 * time.Second
	transportConfig.ResponseHeaderTimeout = 10 * time.Second
	client, err := researchsearch.NewBraveHTTPClient(transportConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.CloseIdleConnections)

	provider, readiness, err := researchsearch.NewBraveFromSecrets(client, secretstore.New())
	if err != nil {
		t.Fatalf("build live search provider (%s): %v", readiness, err)
	}
	discovery := application.NewDiscoveryService(
		provider,
		nil,
		application.NetworkResearchAccess{
			Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: true}, nil),
		},
	)

	requestID, err := research.NewID("request.live.search-provider")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	results, err := discovery.Search(ctx, application.ResearchModeOnline, application.SearchQuery{
		RequestID: requestID,
		Text:      "Go programming language official documentation",
	}, application.SearchOptions{Limit: 3})
	if err != nil {
		t.Fatalf("live search provider query failed: %v", err)
	}
	if len(results) == 0 || len(results) > 3 {
		t.Fatalf("live search provider returned %d results, want 1..3", len(results))
	}

	seen := make(map[string]struct{}, len(results))
	for index, result := range results {
		if err := result.Validate(); err != nil {
			t.Fatalf("live search result %d: %v", index, err)
		}
		if result.Provider != researchsearch.ProviderID || result.Rank < 0 ||
			result.Locator.String() == researchsearch.EndpointURL {
			t.Fatalf("live search result %d has invalid provider metadata", index)
		}
		locator := result.Locator.String()
		if _, duplicate := seen[locator]; duplicate {
			t.Fatalf("live search provider repeated result URL at position %d", index)
		}
		seen[locator] = struct{}{}
	}
}
