package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/learning"
)

func (service *Service) executeMastery(ctx context.Context, command Command) (result Result, err error) {
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
	if store.Mastery() == nil {
		return Result{}, errors.New("mastery threshold service is unavailable")
	}

	var resolved = newResolvedMasteryResult(&result)
	switch command.MasteryOperation {
	case "show":
		*resolved, err = store.Mastery().Show(ctx, nil)
	case "set":
		*resolved, err = store.Mastery().SetWorkspaceOverride(ctx, command.MasteryThreshold)
	case "set-default":
		*resolved, err = store.Mastery().SetStudentDefault(ctx, command.MasteryThreshold)
	case "reset":
		*resolved, err = store.Mastery().ClearWorkspaceOverride(ctx)
	default:
		return Result{}, fmt.Errorf("unsupported mastery threshold operation %q", command.MasteryOperation)
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func newResolvedMasteryResult(result *Result) *learning.ResolvedMasteryThreshold {
	result.Mastery = &learning.ResolvedMasteryThreshold{}
	return result.Mastery
}
