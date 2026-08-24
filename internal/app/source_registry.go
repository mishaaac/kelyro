package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
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
	if store.Registry() == nil {
		return Result{}, errors.New("source registry service is unavailable")
	}
	switch command.SourceRegistryOperation {
	case "list":
		result.SourceRegistryEntries, err = store.Registry().List(ctx)
		if result.SourceRegistryEntries == nil {
			result.SourceRegistryEntries = make([]research.SourceRegistryEntry, 0)
		}
	case "show":
		entry, getErr := store.Registry().Get(ctx, command.SourceRegistryID)
		result.SourceRegistryEntry, err = &entry, getErr
	default:
		return Result{}, fmt.Errorf("unsupported source registry operation %q", command.SourceRegistryOperation)
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}
