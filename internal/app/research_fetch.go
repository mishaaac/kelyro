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
	if service.researchSourceCaches == nil {
		return nil, fmt.Errorf("research source cache is unavailable")
	}
	settings, err := service.resolvedConfigForWorkspace(workspaceRoot, command.ConfigOverrides)
	if err != nil {
		return nil, err
	}
	gate, err := service.networkGate(settings, command)
	if err != nil {
		return nil, err
	}
	cache, err := service.researchSourceCaches.OpenSourceFetchCache(ctx, workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("open research source cache: %w", err)
	}
	fetch := researchapp.NewFetchService(service.researchFetcher, cache, researchapp.NetworkResearchAccess{Gate: gate})
	stage, err := researchapp.NewLiveSourceFetchService(
		fetch, researchapp.DefaultResearchProcessingLimitsV1(), researchapp.DefaultLiveSourceMaximumBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("assemble live research fetch: %w", err)
	}
	return stage, nil
}

func (service *Service) researchSnapshotForRun(ctx context.Context, workspaceRoot string, store researchapp.SourceRegistryStore) (researchapp.LiveResearchStageService, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store == nil || store.Snapshots() == nil {
		return nil, fmt.Errorf("research snapshot capture is unavailable")
	}
	if service.researchSourceCaches == nil {
		return nil, fmt.Errorf("research source cache is unavailable")
	}
	cache, err := service.researchSourceCaches.OpenSourceFetchCache(ctx, workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("open research source cache: %w", err)
	}
	stage, err := researchapp.NewLiveSourceSnapshotService(store.Snapshots(), cache)
	if err != nil {
		return nil, fmt.Errorf("assemble live research snapshot: %w", err)
	}
	return stage, nil
}
