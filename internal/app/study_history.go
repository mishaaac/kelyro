package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/learning"
)

func (service *Service) executeStudyHistory(ctx context.Context, command Command) (result Result, err error) {
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
	if store.History() == nil {
		return Result{}, errors.New("study history service is unavailable")
	}
	switch command.Action {
	case ActionHistory:
		period := learning.StudyPeriodAll
		if command.HistoryToday {
			period = learning.StudyPeriodToday
		}
		view, historyErr := store.History().List(ctx, period)
		result.History = &view
		return result, historyErr
	case ActionTime:
		summary, timeErr := store.History().Time(ctx)
		result.StudyTime = &summary
		return result, timeErr
	default:
		return Result{}, fmt.Errorf("unsupported study history action %q", command.Action)
	}
}
