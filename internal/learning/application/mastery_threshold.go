package application

import (
	"context"
	"errors"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type MasteryPolicyOption func(*masteryPolicyService)

func WithMasteryPolicyClock(now func() time.Time) MasteryPolicyOption {
	return func(service *masteryPolicyService) {
		if now != nil {
			service.now = now
		}
	}
}

type masteryPolicyService struct {
	profiles ProfileService
	settings MasteryThresholdRepository
	now      func() time.Time
}

func NewMasteryPolicyService(profiles ProfileService, settings MasteryThresholdRepository, options ...MasteryPolicyOption) MasteryPolicyService {
	service := &masteryPolicyService{profiles: profiles, settings: settings, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *masteryPolicyService) Show(ctx context.Context, pack *learning.PackMasteryOverride) (learning.ResolvedMasteryThreshold, error) {
	const operation = "show mastery threshold"
	settings, err := service.load(ctx, operation)
	if err != nil {
		return learning.ResolvedMasteryThreshold{}, err
	}
	resolved, err := learning.ResolveMasteryThreshold(settings, pack)
	if err != nil {
		return learning.ResolvedMasteryThreshold{}, invalid(operation, err)
	}
	return resolved, nil
}

func (service *masteryPolicyService) SetStudentDefault(ctx context.Context, threshold learning.MasteryThreshold) (learning.ResolvedMasteryThreshold, error) {
	return service.update(ctx, "set student mastery threshold", func(settings learning.MasteryThresholdSettings, at learning.Timestamp) (learning.MasteryThresholdSettings, error) {
		return settings.SetStudentDefault(threshold, at)
	})
}

func (service *masteryPolicyService) SetWorkspaceOverride(ctx context.Context, threshold learning.MasteryThreshold) (learning.ResolvedMasteryThreshold, error) {
	return service.update(ctx, "set workspace mastery threshold", func(settings learning.MasteryThresholdSettings, at learning.Timestamp) (learning.MasteryThresholdSettings, error) {
		return settings.SetWorkspaceOverride(threshold, at)
	})
}

func (service *masteryPolicyService) ClearWorkspaceOverride(ctx context.Context) (learning.ResolvedMasteryThreshold, error) {
	return service.update(ctx, "clear workspace mastery threshold", func(settings learning.MasteryThresholdSettings, at learning.Timestamp) (learning.MasteryThresholdSettings, error) {
		return settings.ClearWorkspaceOverride(at)
	})
}

func (service *masteryPolicyService) update(ctx context.Context, operation string, change func(learning.MasteryThresholdSettings, learning.Timestamp) (learning.MasteryThresholdSettings, error)) (learning.ResolvedMasteryThreshold, error) {
	settings, err := service.load(ctx, operation)
	if err != nil {
		return learning.ResolvedMasteryThreshold{}, err
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return learning.ResolvedMasteryThreshold{}, err
	}
	settings, err = change(settings, timestamp)
	if err != nil {
		return learning.ResolvedMasteryThreshold{}, invalid(operation, err)
	}
	if err := service.settings.Save(ctx, settings); err != nil {
		return learning.ResolvedMasteryThreshold{}, repositoryError(operation, err)
	}
	resolved, err := learning.ResolveMasteryThreshold(settings, nil)
	if err != nil {
		return learning.ResolvedMasteryThreshold{}, invalid(operation, err)
	}
	return resolved, nil
}

func (service *masteryPolicyService) load(ctx context.Context, operation string) (learning.MasteryThresholdSettings, error) {
	if service == nil || service.profiles == nil || service.settings == nil {
		return learning.MasteryThresholdSettings{}, Classify(ErrorUnavailable, operation, errors.New("mastery policy dependencies are not configured"))
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.MasteryThresholdSettings{}, err
	}
	settings, err := service.settings.Get(ctx, student.ID)
	if err == nil {
		if validateErr := settings.Validate(); validateErr != nil {
			return learning.MasteryThresholdSettings{}, repositoryError(operation, validateErr)
		}
		return settings, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return learning.MasteryThresholdSettings{}, repositoryError(operation, err)
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return learning.MasteryThresholdSettings{}, err
	}
	settings, err = learning.NewMasteryThresholdSettings(student.ID, timestamp)
	if err != nil {
		return learning.MasteryThresholdSettings{}, invalid(operation, err)
	}
	if err := service.settings.Save(ctx, settings); err != nil {
		return learning.MasteryThresholdSettings{}, repositoryError(operation, err)
	}
	return settings, nil
}

func (service *masteryPolicyService) timestamp(operation string) (learning.Timestamp, error) {
	if service == nil || service.now == nil {
		return learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("mastery policy clock is not configured"))
	}
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Timestamp{}, invalid(operation, err)
	}
	return timestamp, nil
}
