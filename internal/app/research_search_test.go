package app

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func TestServiceAssemblesResearchSearchFromResolvedProductionBoundaries(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 8, 30, 12, 30, 0, 0, time.UTC)
	runID, _ := research.NewID("run.app-search-wiring")
	secrets := &recordingSecretStore{values: map[string]string{"fixture": "secret"}}
	factory := &recordingLiveSearchFactory{}
	costs := &fakeResearchCostService{}
	configs := &recordingConfigStore{project: config.Settings{
		config.KeyAllowNetwork:                     config.BoolValue(true),
		config.KeyResearchSearchProvider:           config.StringValue("brave"),
		config.KeyResearchSearchMaxResultsPerQuery: config.NumberValue(3),
		config.KeyResearchSearchMaxQueriesPerRun:   config.NumberValue(2),
	}}
	service := NewService(nil, nil).
		WithConfig(configs).
		WithSecrets(secrets).
		WithResearchSearch(factory).
		WithResearchClock(func() time.Time { return at })

	_, err := service.researchSearchForRun(context.Background(), Command{}, "/workspace", runID, costs)
	if err != nil {
		t.Fatal(err)
	}
	request := factory.request
	if factory.calls != 1 || request.Settings.Provider != "brave" || request.Settings.MaxResultsPerQuery != 3 || request.Settings.MaxQueriesPerRun != 2 {
		t.Fatalf("factory request/settings = %+v, calls = %d", request, factory.calls)
	}
	if request.Secrets != secrets || request.Costs != costs || request.RunID != runID || !request.Clock.Now().Time().Equal(at) {
		t.Fatalf("factory dependencies = %+v", request)
	}
	if err := request.Access.Gate.Authorize(context.Background(), privacy.Request{Operation: "research.discovery", Purpose: privacy.ExternalResource}); err != nil {
		t.Fatalf("resolved privacy gate denied configured network: %v", err)
	}
}

type recordingLiveSearchFactory struct {
	request   researchapp.LiveSearchBuildRequest
	readiness researchapp.LiveSearchReadiness
	calls     int
	probes    int
}

func (factory *recordingLiveSearchFactory) Probe(_ context.Context, _ researchapp.LiveSearchProviderSettings, _ researchapp.LiveSearchSecretReader) researchapp.LiveSearchReadiness {
	factory.probes++
	return factory.readiness
}

func (factory *recordingLiveSearchFactory) Build(_ context.Context, request researchapp.LiveSearchBuildRequest) (researchapp.LiveSearchBuildResult, error) {
	factory.calls++
	factory.request = request
	return researchapp.LiveSearchBuildResult{}, nil
}
