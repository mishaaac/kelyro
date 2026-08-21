package app

import (
	"context"
	"errors"
	"fmt"
)

// executeDashboard exposes the Student Core read model to presentation
// adapters without making it a public CLI command in this implementation step.
func (service *Service) executeDashboard(ctx context.Context, command Command) (result Result, err error) {
	if service.profiles == nil {
		return Result{}, errors.New("student core store is unavailable")
	}
	found, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	store, err := service.profiles.Open(ctx, found.Root)
	if err != nil {
		return Result{}, fmt.Errorf("open student core: %w", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if store.Dashboard() == nil {
		return Result{}, errors.New("progress dashboard service is unavailable")
	}
	dashboard, err := store.Dashboard().Show(ctx)
	result.Dashboard = &dashboard
	return result, err
}
