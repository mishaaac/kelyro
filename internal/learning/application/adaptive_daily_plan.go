package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type AdaptiveDailyPlanOption func(*adaptiveDailyPlanService)

func WithAdaptiveDailyPlanClock(now func() time.Time) AdaptiveDailyPlanOption {
	return func(service *adaptiveDailyPlanService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithAdaptiveDailyPlanPolicy(policy learning.DailyPlanPolicy) AdaptiveDailyPlanOption {
	return func(service *adaptiveDailyPlanService) { service.policy = policy }
}

type adaptiveDailyPlanService struct {
	profiles   ProfileService
	mastery    MasteryPolicyService
	unitOfWork UnitOfWork
	now        func() time.Time
	policy     learning.DailyPlanPolicy
}

func NewAdaptiveDailyPlanService(profiles ProfileService, mastery MasteryPolicyService, unitOfWork UnitOfWork, options ...AdaptiveDailyPlanOption) AdaptiveDailyPlanService {
	service := &adaptiveDailyPlanService{
		profiles: profiles, mastery: mastery, unitOfWork: unitOfWork,
		now: time.Now, policy: learning.DefaultDailyPlanPolicy(),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *adaptiveDailyPlanService) Today(ctx context.Context) (learning.DailyPlan, error) {
	const operation = "build adaptive daily plan"
	if service == nil || service.profiles == nil || service.mastery == nil || service.unitOfWork == nil || service.now == nil {
		return learning.DailyPlan{}, Classify(ErrorUnavailable, operation, errors.New("daily plan dependencies are not configured"))
	}
	if err := service.policy.Validate(); err != nil {
		return learning.DailyPlan{}, invalid(operation, err)
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.DailyPlan{}, repositoryError(operation, err)
	}
	generatedAt, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.DailyPlan{}, invalid(operation, err)
	}
	date, err := dailyPlanLocalDate(generatedAt, student.Profile.Timezone)
	if err != nil {
		return learning.DailyPlan{}, invalid(operation, err)
	}
	threshold, err := service.mastery.Show(ctx, nil)
	if err != nil {
		return learning.DailyPlan{}, repositoryError(operation, err)
	}

	var plan learning.DailyPlan
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		if err := requireAdaptiveDailyPlanRepositories(operation, repositories); err != nil {
			return err
		}
		goals, err := repositories.Goals.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		goal, err := activeDailyPlanGoal(goals)
		if err != nil {
			return Classify(ErrorNotFound, operation, err)
		}
		instances, err := repositories.CurriculumInstances.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		instance, err := activeDailyPlanInstance(instances, goal.ID)
		if err != nil {
			return Classify(ErrorNotFound, operation, err)
		}
		concepts, err := repositories.Curricula.PlanningConcepts(ctx, instance.Curriculum)
		if err != nil {
			return err
		}
		states, err := repositories.InstanceConceptStates.ListByInstance(ctx, instance.ID)
		if err != nil {
			return err
		}
		reviews, err := repositories.Reviews.ListDue(ctx, student.ID, generatedAt)
		if err != nil {
			return err
		}
		retention, err := repositories.Retention.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		mistakes, err := repositories.Mistakes.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		history, err := repositories.History.ListByStudent(ctx, student.ID, nil, nil)
		if err != nil {
			return err
		}
		known := make(map[learning.ID]struct{}, len(concepts))
		for _, concept := range concepts {
			known[concept.ConceptID] = struct{}{}
		}
		reviews = dailyPlanReviewsWithinCurriculum(reviews, known)
		candidate, err := learning.BuildAdaptiveDailyPlanV1(learning.DailyPlanInput{
			StudentID: student.ID, GoalID: goal.ID, CurriculumInstanceID: instance.ID,
			Curriculum: instance.Curriculum, Timezone: student.Profile.Timezone, Date: date, GeneratedAt: generatedAt,
			AvailableMinutes: student.Profile.Availability.DailyMinutes, MasteryPolicy: threshold,
			Concepts: concepts, States: states, ReviewsDue: reviews, Retention: retention,
			Mistakes: mistakes, History: history, GenerationReason: learning.DailyPlanGeneratedInitial, Policy: service.policy,
		})
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		existing, loadErr := repositories.DailyPlans.ForDate(ctx, student.ID, goal.ID, date)
		if loadErr == nil {
			if existing.PolicyVersion == candidate.PolicyVersion && existing.SourceFingerprint == candidate.SourceFingerprint {
				plan = existing
				return nil
			}
			if existing.PolicyVersion != candidate.PolicyVersion {
				candidate.GenerationReason = learning.DailyPlanGeneratedPolicyChanged
			} else {
				candidate.GenerationReason = learning.DailyPlanGeneratedSourceChanged
			}
			if err := candidate.Validate(); err != nil {
				return Classify(ErrorInvalidState, operation, err)
			}
		} else if !errors.Is(loadErr, ErrNotFound) {
			return loadErr
		}
		if err := repositories.DailyPlans.Save(ctx, candidate); err != nil {
			return err
		}
		plan = candidate
		return nil
	})
	if err != nil {
		return learning.DailyPlan{}, repositoryError(operation, err)
	}
	return plan, nil
}

func dailyPlanLocalDate(generatedAt learning.Timestamp, timezone string) (learning.Timestamp, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return learning.Timestamp{}, err
	}
	local := generatedAt.Time().In(location)
	return learning.NewTimestamp(time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location))
}

func activeDailyPlanGoal(goals []learning.LearningGoal) (learning.LearningGoal, error) {
	var active *learning.LearningGoal
	for index := range goals {
		if goals[index].Status != learning.GoalActive {
			continue
		}
		if active != nil {
			return learning.LearningGoal{}, errors.New("multiple active learning goals")
		}
		copy := goals[index]
		active = &copy
	}
	if active == nil {
		return learning.LearningGoal{}, errors.New("no active learning goal")
	}
	return *active, nil
}

func activeDailyPlanInstance(instances []learning.CurriculumInstance, goalID learning.ID) (learning.CurriculumInstance, error) {
	candidates := make([]learning.CurriculumInstance, 0)
	for _, instance := range instances {
		if instance.GoalID == goalID && instance.Status == learning.CurriculumInstanceActive {
			candidates = append(candidates, instance)
		}
	}
	if len(candidates) == 0 {
		return learning.CurriculumInstance{}, errors.New("active goal has no active curriculum instance")
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].UpdatedAt != candidates[j].UpdatedAt {
			return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
		}
		return candidates[i].ID.String() > candidates[j].ID.String()
	})
	return candidates[0], nil
}

func dailyPlanReviewsWithinCurriculum(items []learning.ReviewItem, known map[learning.ID]struct{}) []learning.ReviewItem {
	result := make([]learning.ReviewItem, 0, len(items))
	for _, item := range items {
		if _, exists := known[item.ConceptID]; exists {
			result = append(result, item)
		}
	}
	return result
}

func requireAdaptiveDailyPlanRepositories(operation string, repositories Repositories) error {
	if repositories.Goals == nil || repositories.CurriculumInstances == nil || repositories.Curricula == nil ||
		repositories.InstanceConceptStates == nil || repositories.Reviews == nil || repositories.Retention == nil ||
		repositories.Mistakes == nil || repositories.History == nil || repositories.DailyPlans == nil {
		return Classify(ErrorUnavailable, operation, fmt.Errorf("daily plan repositories are not configured"))
	}
	return nil
}

var _ AdaptiveDailyPlanService = (*adaptiveDailyPlanService)(nil)
