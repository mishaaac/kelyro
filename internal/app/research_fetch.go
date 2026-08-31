package app

import (
	"context"
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

// researchFetchForRun assembles the production fetch stage without starting
// network work. Snapshot and cache composition remain later explicit stages.
func (service *Service) researchFetchForRun(ctx context.Context, command Command, workspaceRoot string) (researchapp.LiveResearchStageService, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if service.researchFetcher == nil {
		return nil, fmt.Errorf("research source fetcher is unavailable")
	}
	if service.configs == nil {
		return nil, fmt.Errorf("configuration store is unavailable")
	}
	settings, err := service.resolvedConfigForWorkspace(workspaceRoot, command.ConfigOverrides)
	if err != nil {
		return nil, err
	}
	gate, err := service.networkGate(settings, command)
	if err != nil {
		return nil, err
	}
	fetch := researchapp.NewFetchService(service.researchFetcher, nil, researchapp.NetworkResearchAccess{Gate: gate})
	stage, err := researchapp.NewLiveSourceFetchService(
		fetch, researchapp.DefaultResearchProcessingLimitsV1(), researchapp.DefaultLiveSourceMaximumBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("assemble live research fetch: %w", err)
	}
	return stage, nil
}
