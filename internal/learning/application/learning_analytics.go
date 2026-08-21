package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type LearningAnalyticsOption func(*learningAnalyticsService)

func WithLearningAnalyticsClock(now func() time.Time) LearningAnalyticsOption {
	return func(service *learningAnalyticsService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithLearningAnalyticsPolicy(policy learning.LearningAnalyticsPolicy) LearningAnalyticsOption {
	return func(service *learningAnalyticsService) { service.policy = policy }
}

type learningAnalyticsService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
	policy     learning.LearningAnalyticsPolicy
}

func NewLearningAnalyticsService(profiles ProfileService, unitOfWork UnitOfWork, options ...LearningAnalyticsOption) LearningAnalyticsService {
	service := &learningAnalyticsService{
		profiles: profiles, unitOfWork: unitOfWork, now: time.Now,
		policy: learning.DefaultLearningAnalyticsPolicy(),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *learningAnalyticsService) Snapshot(ctx context.Context) (learning.LearningAnalyticsSnapshot, error) {
	const operation = "calculate learning analytics"
	if service == nil || service.profiles == nil || service.unitOfWork == nil || service.now == nil {
		return learning.LearningAnalyticsSnapshot{}, Classify(ErrorUnavailable, operation, errors.New("learning analytics dependencies are not configured"))
	}
	if err := service.policy.Validate(); err != nil {
		return learning.LearningAnalyticsSnapshot{}, invalid(operation, err)
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.LearningAnalyticsSnapshot{}, repositoryError(operation, err)
	}
	capturedAt, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.LearningAnalyticsSnapshot{}, invalid(operation, fmt.Errorf("read learning analytics clock: %w", err))
	}
	var snapshot learning.LearningAnalyticsSnapshot
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		if err := requireLearningAnalyticsRepositories(operation, repositories); err != nil {
			return err
		}
		instances, err := repositories.CurriculumInstances.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		states := make([]learning.InstanceConceptState, 0)
		for _, instance := range instances {
			instanceStates, err := repositories.InstanceConceptStates.ListByInstance(ctx, instance.ID)
			if err != nil {
				return err
			}
			states = append(states, instanceStates...)
		}
		retention, err := repositories.Retention.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		reviews, err := repositories.Reviews.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		sessions, err := repositories.StudySessions.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		events, err := repositories.History.ListByStudent(ctx, student.ID, nil, nil)
		if err != nil {
			return err
		}
		snapshot, err = learning.CalculateLearningAnalyticsV1(learning.LearningAnalyticsInput{
			StudentID: student.ID, Timezone: student.Profile.Timezone, AsOf: capturedAt,
			ConceptStates: states, RetentionStates: retention, Reviews: reviews,
			Sessions: sessions, Events: events, Policy: service.policy,
		})
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		return nil
	})
	if err != nil {
		return learning.LearningAnalyticsSnapshot{}, repositoryError(operation, err)
	}
	return snapshot, nil
}

func requireLearningAnalyticsRepositories(operation string, repositories Repositories) error {
	if repositories.CurriculumInstances == nil || repositories.InstanceConceptStates == nil || repositories.Retention == nil ||
		repositories.Reviews == nil || repositories.StudySessions == nil || repositories.History == nil {
		return Classify(ErrorUnavailable, operation, errors.New("learning analytics repositories are not configured"))
	}
	return nil
}

var _ LearningAnalyticsService = (*learningAnalyticsService)(nil)
