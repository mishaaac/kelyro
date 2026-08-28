package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) executeSourceRegistry(ctx context.Context, command Command) (result Result, err error) {
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
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	switch command.SourceRegistryOperation {
	case "sources-list":
		if store.Sources() == nil {
			return Result{}, errors.New("source service is unavailable")
		}
		result.Sources, err = store.Sources().List(ctx)
		if result.Sources == nil {
			result.Sources = make([]research.Source, 0)
		}
	case "source-show":
		if store.Sources() == nil {
			return Result{}, errors.New("source service is unavailable")
		}
		source, getErr := store.Sources().Get(ctx, command.SourceID)
		if getErr != nil {
			return Result{}, getErr
		}
		view := SourceCLIView{Source: source}
		snapshot, snapshotErr := store.Sources().LatestSnapshot(ctx, command.SourceID)
		if snapshotErr == nil {
			view.LatestSnapshot = &snapshot
		} else if !errors.Is(snapshotErr, researchapp.ErrNotFound) {
			return Result{}, snapshotErr
		}
		result.Source = &view
	case "conflicts":
		if store.Conflicts() == nil {
			return Result{}, errors.New("conflict service is unavailable")
		}
		result.SourceConflicts, err = store.Conflicts().ListUnresolved(ctx)
		if result.SourceConflicts == nil {
			result.SourceConflicts = make([]research.Conflict, 0)
		}
	case "list":
		if store.Registry() == nil {
			return Result{}, errors.New("source registry service is unavailable")
		}
		result.SourceRegistryEntries, err = store.Registry().List(ctx)
		if result.SourceRegistryEntries == nil {
			result.SourceRegistryEntries = make([]research.SourceRegistryEntry, 0)
		}
	case "show":
		if store.Registry() == nil {
			return Result{}, errors.New("source registry service is unavailable")
		}
		entry, getErr := store.Registry().Get(ctx, command.SourceRegistryID)
		result.SourceRegistryEntry, err = &entry, getErr
	case "trace":
		if store.Provenance() == nil {
			return Result{}, errors.New("provenance service is unavailable")
		}
		graph, traceErr := store.Provenance().Trace(ctx, command.ProvenanceClaimID)
		result.ProvenanceGraph, err = &graph, traceErr
	case "stale":
		if store.Freshness() == nil {
			return Result{}, errors.New("freshness service is unavailable")
		}
		if service.researchClock == nil {
			return Result{}, errors.New("research clock is unavailable")
		}
		asOf, timestampErr := research.NewTimestamp(service.researchClock().UTC())
		if timestampErr != nil {
			return Result{}, fmt.Errorf("research clock: %w", timestampErr)
		}
		result.StaleSources, err = store.Freshness().Due(ctx, asOf)
		if result.StaleSources == nil {
			result.StaleSources = make([]researchapp.FreshnessRecord, 0)
		}
	default:
		return Result{}, fmt.Errorf("unsupported source registry operation %q", command.SourceRegistryOperation)
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}
