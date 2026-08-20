package app

import (
	"context"
	"errors"
	"fmt"
)

func (service *Service) executeGoal(ctx context.Context, command Command) (result Result, err error) {
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
	if store.Goals() == nil {
		return Result{}, errors.New("learning goal service is unavailable")
	}

	switch command.GoalOperation {
	case "show":
		result.Goals, err = store.Goals().Show(ctx)
	case "set":
		goal, setErr := store.Goals().Set(ctx, command.GoalInput)
		result.Goal, err = &goal, setErr
	case "pause":
		goal, pauseErr := store.Goals().Pause(ctx)
		result.Goal, err = &goal, pauseErr
	case "resume":
		goal, resumeErr := store.Goals().Resume(ctx)
		result.Goal, err = &goal, resumeErr
	default:
		return Result{}, fmt.Errorf("unsupported learning goal operation %q", command.GoalOperation)
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}
