package researchsearch

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestFactoryBuildsOnlyConfiguredProductionProviderBehindPrivacy(t *testing.T) {
	t.Parallel()

	requests := 0
	factory := newFactory(roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("HTTP must not be reached")
	}))
	secrets := &factorySecretReader{value: "fixture-token"}
	request := validBuildRequest(t, secrets)
	result, err := factory.Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Provider == nil || result.Discovery == nil {
		t.Fatalf("assembly = %+v", result)
	}
	if _, ok := result.Provider.(*Brave); !ok {
		t.Fatalf("provider type = %T, want *Brave", result.Provider)
	}
	query := application.SearchQuery{RequestID: mustFactoryID(t, "request.factory"), Text: "Go interfaces"}
	_, err = result.Discovery.Search(context.Background(), application.ResearchModeOnline, query, application.SearchOptions{Limit: 2})
	if !errors.Is(err, application.ErrNetworkDisabled) || requests != 0 {
		t.Fatalf("privacy-gated Search() = %v, HTTP requests = %d", err, requests)
	}
	if secrets.calls != 1 {
		t.Fatalf("secret reads = %d, want one", secrets.calls)
	}
}

func TestFactoryNeverFallsBackForDisabledOrUnknownProvider(t *testing.T) {
	t.Parallel()

	secrets := &factorySecretReader{value: "fixture-token"}
	factory := newFactory(staticClient(nil))
	request := validBuildRequest(t, secrets)
	request.Settings.Provider = ""
	if result, err := factory.Build(context.Background(), request); !errors.Is(err, ErrProviderDisabled) || result.Provider != nil || result.Discovery != nil {
		t.Fatalf("disabled assembly = (%+v, %v)", result, err)
	}
	request.Settings.Provider = "unknown-provider"
	if result, err := factory.Build(context.Background(), request); !errors.Is(err, ErrProviderUnavailable) || result.Provider != nil || result.Discovery != nil {
		t.Fatalf("unknown assembly = (%+v, %v)", result, err)
	}
	if secrets.calls != 0 {
		t.Fatalf("disabled/unknown provider read Secrets %d times", secrets.calls)
	}
}

func validBuildRequest(t *testing.T, secrets application.LiveSearchSecretReader) application.LiveSearchBuildRequest {
	t.Helper()
	at, err := research.NewTimestamp(time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return application.LiveSearchBuildRequest{
		Settings: application.LiveSearchProviderSettings{Provider: ProviderID, MaxResultsPerQuery: 8, MaxQueriesPerRun: 4},
		Secrets:  secrets,
		Access:   application.NetworkResearchAccess{Gate: privacy.NewNetworkGate(privacy.Policy{}, nil)},
		Costs:    noopFactoryCostService{}, Clock: factoryClock{now: at}, RunID: mustFactoryID(t, "run.factory"),
	}
}

type factorySecretReader struct {
	value string
	calls int
}

func (reader *factorySecretReader) Get(string) (string, error) {
	reader.calls++
	return reader.value, nil
}

type factoryClock struct{ now research.Timestamp }

func (clock factoryClock) Now() research.Timestamp { return clock.now }

type noopFactoryCostService struct{}

func (noopFactoryCostService) Evaluate(context.Context, application.CostControlRequest) (application.CostControlDecision, error) {
	return application.CostControlDecision{NetworkAllowed: true}, nil
}
func (noopFactoryCostService) Metadata(context.Context, research.ID) (research.ResearchCostMetadata, error) {
	return research.ResearchCostMetadata{}, nil
}
func (noopFactoryCostService) Stats(context.Context, research.Timestamp) (application.ResearchCostStats, error) {
	return application.ResearchCostStats{}, nil
}

func mustFactoryID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
