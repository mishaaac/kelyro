package app

import (
	"context"
	"errors"
	"fmt"

	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
)

func (service *Service) executeSetup(ctx context.Context, command Command) (result Result, err error) {
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
	if store.Setup() == nil {
		return Result{}, errors.New("learner setup service is unavailable")
	}
	var viewErr error
	result.Setup = new(learningapp.LearnerSetupView)
	switch command.SetupOperation {
	case "show", "status":
		*result.Setup, viewErr = store.Setup().Show(ctx)
	case "start":
		*result.Setup, viewErr = store.Setup().Start(ctx)
	case "onboarding-submit":
		answer := ""
		if len(command.SetupAnswers) > 0 {
			answer = command.SetupAnswers[0]
		}
		*result.Setup, viewErr = store.Setup().SubmitOnboarding(ctx, answer)
	case "back":
		*result.Setup, viewErr = store.Setup().Back(ctx)
	case "cancel":
		*result.Setup, viewErr = store.Setup().Cancel(ctx)
	case "confirm":
		*result.Setup, viewErr = store.Setup().Confirm(ctx)
	case "diagnostic-submit":
		*result.Setup, viewErr = store.Setup().SubmitDiagnostic(ctx, command.SetupAnswers)
	case "diagnostic-skip":
		*result.Setup, viewErr = store.Setup().SkipDiagnostic(ctx)
	case "reset":
		*result.Setup, viewErr = store.Setup().ResetDevelopment(ctx)
	default:
		return Result{}, fmt.Errorf("unsupported setup operation %q", command.SetupOperation)
	}
	if viewErr != nil {
		return Result{}, viewErr
	}
	return result, nil
}
