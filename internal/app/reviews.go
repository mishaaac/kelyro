package app

import (
	"context"
	"errors"
	"fmt"
)

func (service *Service) executeReviews(ctx context.Context, command Command) (result Result, err error) {
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
	if store.Reviews() == nil {
		return Result{}, errors.New("review scheduler service is unavailable")
	}
	view, err := store.Reviews().List(ctx, command.ReviewsDue)
	result.Reviews = &view
	return result, err
}
