package app

import (
	"context"
	"errors"
	"fmt"

	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
)

func (service *Service) executeOnboarding(ctx context.Context, command Command) (result Result, err error) {
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
	if store.Onboarding() == nil {
		return Result{}, errors.New("onboarding service is unavailable")
	}

	var viewErr error
	var viewResult = newOnboardingResult(&result)
	switch command.OnboardingOperation {
	case "show":
		*viewResult, viewErr = store.Onboarding().Show(ctx)
	case "start":
		*viewResult, viewErr = store.Onboarding().Start(ctx)
	case "submit":
		*viewResult, viewErr = store.Onboarding().Submit(ctx, command.OnboardingAnswer)
	case "back":
		*viewResult, viewErr = store.Onboarding().Back(ctx)
	case "cancel":
		*viewResult, viewErr = store.Onboarding().Cancel(ctx)
	case "confirm":
		confirmation, confirmErr := store.Onboarding().Confirm(ctx)
		viewResult.Interview = confirmation.View
		result.Goal = &confirmation.Goal
		viewErr = confirmErr
	default:
		return Result{}, fmt.Errorf("unsupported onboarding operation %q", command.OnboardingOperation)
	}
	if viewErr != nil {
		return Result{}, viewErr
	}
	return result, nil
}

func newOnboardingResult(result *Result) *learningapp.OnboardingView {
	result.Onboarding = &learningapp.OnboardingView{}
	return result.Onboarding
}
