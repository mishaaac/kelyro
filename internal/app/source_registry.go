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
	case "transparency":
		result.SourceTransparency, err = sourceTransparency(ctx, store)
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
		view, viewErr := sourceTransparencyDetail(ctx, store, source, true)
		if viewErr != nil {
			return Result{}, viewErr
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

func sourceTransparency(ctx context.Context, store researchapp.SourceRegistryStore) ([]SourceCLIView, error) {
	if store.Sources() == nil {
		return nil, errors.New("source service is unavailable")
	}
	sources, err := store.Sources().List(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]SourceCLIView, 0, len(sources))
	for _, source := range sources {
		view, viewErr := sourceTransparencyDetail(ctx, store, source, false)
		if viewErr != nil {
			return nil, viewErr
		}
		views = append(views, view)
	}
	return views, nil
}

func sourceTransparencyDetail(ctx context.Context, store researchapp.SourceRegistryStore, source research.Source, includeSnapshot bool) (SourceCLIView, error) {
	view := SourceCLIView{Source: source}
	if includeSnapshot {
		snapshot, err := store.Sources().LatestSnapshot(ctx, source.ID)
		if err == nil {
			view.LatestSnapshot = &snapshot
		} else if !errors.Is(err, researchapp.ErrNotFound) {
			return SourceCLIView{}, err
		}
	}
	if store.TrustDecisions() == nil {
		return SourceCLIView{}, errors.New("trust decision service is unavailable")
	}
	decision, err := store.TrustDecisions().Latest(ctx, source.ID)
	if err == nil {
		view.TrustDecision = &decision
	} else if !errors.Is(err, researchapp.ErrNotFound) {
		return SourceCLIView{}, err
	}
	if store.Freshness() == nil {
		return SourceCLIView{}, errors.New("freshness service is unavailable")
	}
	subjectID, idErr := research.NewID(source.ID.String())
	if idErr != nil {
		return SourceCLIView{}, idErr
	}
	freshnessRecord, err := store.Freshness().Get(ctx, subjectID)
	if err == nil {
		view.Freshness = &freshnessRecord
	} else if !errors.Is(err, researchapp.ErrNotFound) {
		return SourceCLIView{}, err
	}
	return view, nil
}
