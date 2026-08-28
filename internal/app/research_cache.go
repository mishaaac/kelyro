package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

func (service *Service) executeResearch(ctx context.Context, command Command) (Result, error) {
	if command.ResearchOperation == "stats" {
		if service.researchStores == nil {
			return Result{}, errors.New("research store is unavailable")
		}
		found, err := service.discoverWorkspace(command)
		if err != nil {
			return Result{}, err
		}
		store, err := service.researchStores.Open(ctx, found.Root)
		if err != nil {
			return Result{}, fmt.Errorf("open research store: %w", err)
		}
		defer store.Close()
		if store.Costs() == nil {
			return Result{}, errors.New("research cost service is unavailable")
		}
		now, err := research.NewTimestamp(service.researchClock().UTC())
		if err != nil {
			return Result{}, fmt.Errorf("research clock: %w", err)
		}
		stats, err := store.Costs().Stats(ctx, now)
		if err != nil {
			return Result{}, err
		}
		return Result{ResearchCostStats: &stats}, nil
	}
	if service.researchCaches == nil {
		return Result{}, errors.New("research cache is unavailable")
	}
	found, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	cache, err := service.researchCaches.Open(ctx, found.Root)
	if err != nil {
		return Result{}, fmt.Errorf("open research cache: %w", err)
	}
	switch command.ResearchCacheOperation {
	case "status":
		status, statusErr := cache.Status(ctx)
		if statusErr != nil {
			return Result{}, statusErr
		}
		return Result{ResearchCacheStatus: &status}, nil
	case "clear":
		cleared, clearErr := cache.Clear(ctx)
		if clearErr != nil {
			return Result{}, clearErr
		}
		return Result{ResearchCacheCleared: &cleared}, nil
	default:
		return Result{}, fmt.Errorf("unsupported research cache operation %q", command.ResearchCacheOperation)
	}
}
