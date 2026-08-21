package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type StreakOption func(*streakService)

func WithStreakClock(now func() time.Time) StreakOption {
	return func(service *streakService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithStreakPolicy(policy learning.StreakPolicy) StreakOption {
	return func(service *streakService) { service.policy = policy }
}

type streakService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
	policy     learning.StreakPolicy
}

func NewStreakService(profiles ProfileService, unitOfWork UnitOfWork, options ...StreakOption) StreakService {
	service := &streakService{profiles: profiles, unitOfWork: unitOfWork, now: time.Now, policy: learning.DefaultStreakPolicy()}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *streakService) Show(ctx context.Context) (learning.Streak, error) {
	const operation = "calculate study streak"
	if service == nil || service.profiles == nil || service.unitOfWork == nil || service.now == nil {
		return learning.Streak{}, Classify(ErrorUnavailable, operation, errors.New("streak dependencies are not configured"))
	}
	if err := service.policy.Validate(); err != nil {
		return learning.Streak{}, invalid(operation, err)
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.Streak{}, repositoryError(operation, err)
	}
	asOf, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Streak{}, invalid(operation, fmt.Errorf("read streak clock: %w", err))
	}
	var streak learning.Streak
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		if repositories.History == nil || repositories.StudySessions == nil || repositories.Streaks == nil {
			return Classify(ErrorUnavailable, operation, errors.New("streak repositories are not configured"))
		}
		events, listErr := repositories.History.ListByStudent(ctx, student.ID, nil, nil)
		if listErr != nil {
			return listErr
		}
		sessions, listErr := repositories.StudySessions.ListByStudent(ctx, student.ID)
		if listErr != nil {
			return listErr
		}
		streak, listErr = learning.CalculateStreakV1(learning.StreakCalculationInput{
			StudentID: student.ID, Events: events, Sessions: sessions, Timezone: student.Profile.Timezone,
			AsOf: asOf, Policy: service.policy,
		})
		if listErr != nil {
			return Classify(ErrorInvalidState, operation, listErr)
		}
		return repositories.Streaks.Save(ctx, streak)
	})
	if err != nil {
		return learning.Streak{}, repositoryError(operation, err)
	}
	return streak, nil
}

var _ StreakService = (*streakService)(nil)
