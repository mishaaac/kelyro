package app

import (
	"context"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

// researchSearchForRun is the production composition boundary consumed by the
// future orchestrator. Assembly is explicit and does not execute a query.
func (service *Service) researchSearchForRun(
	ctx context.Context,
	command Command,
	workspaceRoot string,
	runID research.ID,
	costs researchapp.ResearchCostService,
) (researchapp.LiveSearchBuildResult, error) {
	if service.researchSearch == nil {
		return researchapp.LiveSearchBuildResult{}, fmt.Errorf("research search factory is unavailable")
	}
	if service.configs == nil {
		return researchapp.LiveSearchBuildResult{}, fmt.Errorf("configuration store is unavailable")
	}
	if service.secrets == nil {
		return researchapp.LiveSearchBuildResult{}, fmt.Errorf("secret store is unavailable")
	}
	settings, err := service.resolvedConfigForWorkspace(workspaceRoot, command.ConfigOverrides)
	if err != nil {
		return researchapp.LiveSearchBuildResult{}, err
	}
	search, err := config.ResearchSearchFromResolved(settings)
	if err != nil {
		return researchapp.LiveSearchBuildResult{}, err
	}
	gate, err := service.networkGate(settings, command)
	if err != nil {
		return researchapp.LiveSearchBuildResult{}, err
	}
	return service.researchSearch.Build(ctx, researchapp.LiveSearchBuildRequest{
		Settings: researchapp.LiveSearchProviderSettings{
			Provider: search.Provider, MaxResultsPerQuery: search.MaxResultsPerQuery,
			MaxQueriesPerRun: search.MaxQueriesPerRun,
		},
		Secrets: service.secrets,
		Access:  researchapp.NetworkResearchAccess{Gate: gate},
		Costs:   costs,
		Clock:   researchSearchClock{now: service.researchClock},
		RunID:   runID,
	})
}

type researchSearchClock struct{ now func() time.Time }

func (clock researchSearchClock) Now() research.Timestamp {
	if clock.now == nil {
		return research.Timestamp{}
	}
	timestamp, err := research.NewTimestamp(clock.now().UTC())
	if err != nil {
		return research.Timestamp{}
	}
	return timestamp
}
