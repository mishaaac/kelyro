package app

import (
	"context"
	"errors"
	"fmt"

	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
)

func (service *Service) executeStudySession(ctx context.Context, command Command) (result Result, err error) {
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
	if store.StudySessions() == nil {
		return Result{}, errors.New("study session service is unavailable")
	}

	var sessionErr error
	switch command.SessionOperation {
	case "status":
		session, currentErr := store.StudySessions().Current(ctx)
		if errors.Is(currentErr, learningapp.ErrNotFound) {
			return Result{Message: "No active study session."}, nil
		}
		result.StudySession, sessionErr = &session, currentErr
	case "stop":
		session, stopErr := store.StudySessions().Stop(ctx)
		result.StudySession, sessionErr = &session, stopErr
	default:
		return Result{}, fmt.Errorf("unsupported study session operation %q", command.SessionOperation)
	}
	if sessionErr != nil {
		return Result{}, sessionErr
	}
	return result, nil
}
