package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) executeResearch(ctx context.Context, command Command) (Result, error) {
	if command.ResearchOperation == "topic" || command.ResearchOperation == "status" || command.ResearchOperation == "update-scan" {
		if service.researchStores == nil {
			return Result{}, errors.New("research store is unavailable")
		}
		found, err := service.discoverWorkspace(command)
		if err != nil {
			return Result{}, err
		}
		command.Workspace = found.Root
		store, err := service.researchStores.Open(ctx, found.Root)
		if err != nil {
			return Result{}, fmt.Errorf("open research store: %w", err)
		}
		defer store.Close()
		if command.ResearchOperation == "update-scan" {
			if store.UpdateScan() == nil {
				return Result{}, errors.New("research update scan service is unavailable")
			}
			settings, settingsErr := service.resolvedConfigForWorkspace(found.Root, command.ConfigOverrides)
			if settingsErr != nil {
				return Result{}, settingsErr
			}
			gate, gateErr := service.networkGate(settings, command)
			if gateErr != nil {
				return Result{}, gateErr
			}
			now, clockErr := research.NewTimestamp(service.researchClock().UTC())
			if clockErr != nil {
				return Result{}, fmt.Errorf("research clock: %w", clockErr)
			}
			scan, scanErr := store.UpdateScan().Scan(ctx, researchapp.ResearchModeAuto, researchapp.NetworkResearchAccess{Gate: gate}, now)
			if scanErr != nil {
				return Result{}, scanErr
			}
			return Result{UpdateScan: &scan}, nil
		}
		var view ResearchCLIView
		if command.ResearchOperation == "topic" {
			view, err = service.startResearchTopic(ctx, command, store)
		} else {
			view, err = researchStatus(ctx, store, command.ResearchRunID)
		}
		if err != nil {
			return Result{}, err
		}
		return Result{ResearchView: &view}, nil
	}
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
