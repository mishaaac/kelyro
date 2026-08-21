package app

import (
	"context"
	"errors"
	"fmt"
)

func (service *Service) executeAchievements(ctx context.Context, command Command) (result Result, err error) {
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
	if store.Achievements() == nil {
		return Result{}, errors.New("achievement service is unavailable")
	}
	refreshed, err := store.Achievements().Refresh(ctx)
	result.Achievements = &refreshed
	return result, err
}
