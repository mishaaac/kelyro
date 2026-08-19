package app

import (
	"context"
	"errors"
	"fmt"
)

func (service *Service) executeProfile(ctx context.Context, command Command) (result Result, err error) {
	if service.profiles == nil {
		return Result{}, errors.New("learner profile store is unavailable")
	}
	found, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	store, err := service.profiles.Open(ctx, found.Root)
	if err != nil {
		return Result{}, fmt.Errorf("open learner profile: %w", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if store.Profiles() == nil {
		return Result{}, errors.New("learner profile service is unavailable")
	}

	var studentErr error
	switch command.ProfileOperation {
	case "show":
		student, getErr := store.Profiles().Show(ctx)
		studentErr = getErr
		result.Profile = &student
	case "edit":
		student, editErr := store.Profiles().Edit(ctx, command.ProfileChanges)
		studentErr = editErr
		result.Profile = &student
	default:
		return Result{}, fmt.Errorf("unsupported profile operation %q", command.ProfileOperation)
	}
	if studentErr != nil {
		return Result{}, studentErr
	}
	return result, nil
}
