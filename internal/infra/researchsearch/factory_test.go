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
	"github.com/mishaaac/kelyro/internal/storage"
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

func TestFactoryProbesReadinessWithoutNetworkOrUnnecessarySecretReads(t *testing.T) {
	t.Parallel()

	httpCalls := 0
	factory := newFactory(roundTripFunc(func(*http.Request) (*http.Response, error) {
		httpCalls++
		return nil, errors.New("readiness must not call HTTP")
	}))
	settings := application.LiveSearchProviderSettings{Provider: ProviderID, MaxResultsPerQuery: 8, MaxQueriesPerRun: 4}
	for _, test := range []struct {
		name      string
		provider  string
		secret    string
		secretErr error
		want      application.LiveSearchReadiness
		wantReads int
	}{
		{
			name: "configured", provider: ProviderID, secret: "fixture-token", wantReads: 1,
			want: application.LiveSearchReadiness{Provider: application.LiveSearchProviderConfigured, Credential: application.LiveSearchCredentialAvailable},
		},
		{
			name: "missing credential", provider: ProviderID, secretErr: storage.ErrSecretNotFound, wantReads: 1,
			want: application.LiveSearchReadiness{Provider: application.LiveSearchProviderConfigured, Credential: application.LiveSearchCredentialMissing},
		},
		{
			name: "invalid credential", provider: ProviderID, secret: "invalid token", wantReads: 1,
			want: application.LiveSearchReadiness{Provider: application.LiveSearchProviderConfigured, Credential: application.LiveSearchCredentialInvalid},
		},
		{
			name: "disabled", provider: "", wantReads: 0,
			want: application.LiveSearchReadiness{Provider: application.LiveSearchProviderDisabled, Credential: application.LiveSearchCredentialNotApplicable},
		},
		{
			name: "unknown", provider: "unknown", wantReads: 0,
			want: application.LiveSearchReadiness{Provider: application.LiveSearchProviderUnavailable, Credential: application.LiveSearchCredentialNotApplicable},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &factorySecretReader{value: test.secret, err: test.secretErr}
			settings.Provider = test.provider
			got := factory.Probe(context.Background(), settings, reader)
			if got != test.want || reader.calls != test.wantReads {
				t.Fatalf("Probe() = %+v, reads = %d, want %+v/%d", got, reader.calls, test.want, test.wantReads)
			}
		})
	}
	if httpCalls != 0 {
		t.Fatalf("readiness HTTP calls = %d, want zero", httpCalls)
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
	err   error
	calls int
}

func (reader *factorySecretReader) Get(string) (string, error) {
	reader.calls++
	return reader.value, reader.err
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
