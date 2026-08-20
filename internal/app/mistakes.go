package app

import (
	"context"
	"errors"
	"fmt"
)

func (service *Service) executeMistakes(ctx context.Context, command Command) (result Result, err error) {
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
	if store.Mistakes() == nil {
		return Result{}, errors.New("mistake memory service is unavailable")
	}

	switch command.MistakeOperation {
	case "list":
		result.Mistakes, err = store.Mistakes().List(ctx)
	case "show":
		view, showErr := store.Mistakes().Get(ctx, command.MistakeID)
		result.Mistake, err = &view, showErr
	default:
		return Result{}, fmt.Errorf("unsupported mistake memory operation %q", command.MistakeOperation)
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}
