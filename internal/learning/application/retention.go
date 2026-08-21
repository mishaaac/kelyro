package application

import (
	"context"
	"errors"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type RetentionOption func(*retentionService)

func WithRetentionClock(now func() time.Time) RetentionOption {
	return func(service *retentionService) {
		if now != nil {
			service.now = now
		}
	}
}

type retentionService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
}

func NewRetentionService(profiles ProfileService, unitOfWork UnitOfWork, options ...RetentionOption) RetentionService {
	service := &retentionService{profiles: profiles, unitOfWork: unitOfWork, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *retentionService) State(ctx context.Context, conceptID learning.ID) (learning.RetentionState, error) {
	const operation = "get concept retention"
	if service == nil || service.profiles == nil || service.unitOfWork == nil {
		return learning.RetentionState{}, Classify(ErrorUnavailable, operation, errors.New("retention service dependencies are not configured"))
	}
	if err := conceptID.Validate(); err != nil {
		return learning.RetentionState{}, invalid(operation, err)
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.RetentionState{}, err
	}
	var state learning.RetentionState
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		var loadErr error
		state, loadErr = repositories.Retention.Get(ctx, student.ID, conceptID)
		return loadErr
	})
	if err != nil {
		return learning.RetentionState{}, repositoryError(operation, err)
	}
	return state, nil
}

func (service *retentionService) Recalculate(ctx context.Context, conceptID learning.ID) (learning.RetentionCalculation, error) {
	const operation = "recalculate concept retention"
	if service == nil || service.profiles == nil || service.unitOfWork == nil || service.now == nil {
		return learning.RetentionCalculation{}, Classify(ErrorUnavailable, operation, errors.New("retention service dependencies are not configured"))
	}
	if err := conceptID.Validate(); err != nil {
		return learning.RetentionCalculation{}, invalid(operation, err)
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.RetentionCalculation{}, err
	}
	measuredAt, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.RetentionCalculation{}, invalid(operation, err)
	}

	var calculation learning.RetentionCalculation
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		items, loadErr := repositories.Evidence.ListByConcept(ctx, student.ID, conceptID)
		if loadErr != nil {
			return loadErr
		}
		mastery, calculateErr := learning.CalculateMasteryV1(student.ID, conceptID, items)
		if calculateErr != nil {
			return Classify(ErrorInvalidState, operation, calculateErr)
		}
		calculation, calculateErr = learning.CalculateRetentionV1(mastery, items, measuredAt)
		if calculateErr != nil {
			return Classify(ErrorInvalidState, operation, calculateErr)
		}
		if saveErr := repositories.Retention.Save(ctx, calculation.State); saveErr != nil {
			return saveErr
		}
		instances, loadErr := repositories.CurriculumInstances.ListByStudent(ctx, student.ID)
		if loadErr != nil {
			return loadErr
		}
		for _, instance := range instances {
			if instance.Status != learning.CurriculumInstanceActive {
				continue
			}
			states, statesErr := repositories.InstanceConceptStates.ListByInstance(ctx, instance.ID)
			if statesErr != nil {
				return statesErr
			}
			state, present := findInstanceState(states, conceptID)
			if !present {
				continue
			}
			progression, applyErr := learning.ApplyRetentionV1(state, calculation.State)
			if applyErr != nil {
				return Classify(ErrorInvalidState, operation, applyErr)
			}
			if progression.StateChanged {
				if saveErr := repositories.InstanceConceptStates.Save(ctx, progression.State); saveErr != nil {
					return saveErr
				}
			}
		}
		return nil
	})
	if err != nil {
		return learning.RetentionCalculation{}, repositoryError(operation, err)
	}
	return calculation, nil
}

var _ RetentionService = (*retentionService)(nil)
