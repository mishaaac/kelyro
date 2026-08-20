package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type GoalLifecycleOption func(*goalLifecycleService)

func WithGoalClock(now func() time.Time) GoalLifecycleOption {
	return func(service *goalLifecycleService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithGoalIDGenerator(generate func() (learning.ID, error)) GoalLifecycleOption {
	return func(service *goalLifecycleService) {
		if generate != nil {
			service.generateID = generate
		}
	}
}

type goalLifecycleService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
	generateID func() (learning.ID, error)
}

func NewGoalLifecycleService(profiles ProfileService, unitOfWork UnitOfWork, options ...GoalLifecycleOption) GoalLifecycleService {
	service := &goalLifecycleService{profiles: profiles, unitOfWork: unitOfWork, now: time.Now, generateID: randomGoalID}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *goalLifecycleService) Show(ctx context.Context) ([]learning.LearningGoal, error) {
	const operation = "show learning goals"
	student, err := service.student(ctx, operation)
	if err != nil {
		return nil, err
	}
	var goals []learning.LearningGoal
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		listed, listErr := repositories.Goals.ListByStudent(ctx, student.ID)
		goals = listed
		return listErr
	})
	return goals, err
}

func (service *goalLifecycleService) Set(ctx context.Context, input SetGoalInput) (learning.LearningGoal, error) {
	const operation = "set learning goal"
	if _, err := learning.MasteryRequirementFromThreshold(input.MasteryThreshold); err != nil {
		return learning.LearningGoal{}, invalid(operation, err)
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	id, err := service.generateID()
	if err != nil {
		return learning.LearningGoal{}, Classify(ErrorUnavailable, operation, fmt.Errorf("generate learning goal id: %w", err))
	}
	details := learning.GoalDetails{
		Title: strings.TrimSpace(input.Title), Description: strings.TrimSpace(input.Description),
		Domain: strings.TrimSpace(input.Domain), TargetOutcome: strings.TrimSpace(input.TargetOutcome),
		StartingLevel: input.StartingLevel,
	}
	goal, err := learning.NewLearningGoal(id, student.ID, details, input.MasteryThreshold, timestamp)
	if err != nil {
		return learning.LearningGoal{}, invalid(operation, err)
	}
	goal, err = goal.Activate(timestamp)
	if err != nil {
		return learning.LearningGoal{}, invalid(operation, err)
	}
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		goals, listErr := repositories.Goals.ListByStudent(ctx, student.ID)
		if listErr != nil {
			return listErr
		}
		if err := pauseActiveGoals(ctx, repositories.Goals, goals, timestamp, learning.ID{}); err != nil {
			return err
		}
		return repositories.Goals.Create(ctx, goal)
	})
	if err != nil {
		return learning.LearningGoal{}, err
	}
	return goal, nil
}

func (service *goalLifecycleService) Pause(ctx context.Context) (learning.LearningGoal, error) {
	const operation = "pause learning goal"
	student, err := service.student(ctx, operation)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	var paused learning.LearningGoal
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		goals, listErr := repositories.Goals.ListByStudent(ctx, student.ID)
		if listErr != nil {
			return listErr
		}
		for _, goal := range goals {
			if goal.Status != learning.GoalActive {
				continue
			}
			paused, listErr = goal.Pause(timestamp)
			if listErr != nil {
				return invalid(operation, listErr)
			}
			return repositories.Goals.Update(ctx, paused)
		}
		return Classify(ErrorNotFound, operation, errors.New("no active learning goal"))
	})
	return paused, err
}

func (service *goalLifecycleService) Resume(ctx context.Context) (learning.LearningGoal, error) {
	const operation = "resume learning goal"
	student, err := service.student(ctx, operation)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	var resumed learning.LearningGoal
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		goals, listErr := repositories.Goals.ListByStudent(ctx, student.ID)
		if listErr != nil {
			return listErr
		}
		target, found := latestPausedGoal(goals)
		if !found {
			return Classify(ErrorNotFound, operation, errors.New("no paused learning goal"))
		}
		if err := pauseActiveGoals(ctx, repositories.Goals, goals, timestamp, target.ID); err != nil {
			return err
		}
		resumed, listErr = target.Activate(timestamp)
		if listErr != nil {
			return invalid(operation, listErr)
		}
		return repositories.Goals.Update(ctx, resumed)
	})
	return resumed, err
}

func (service *goalLifecycleService) student(ctx context.Context, operation string) (learning.Student, error) {
	if service == nil || service.profiles == nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, errors.New("profile service is not configured"))
	}
	return service.profiles.Show(ctx)
}

func (service *goalLifecycleService) withRepositories(ctx context.Context, operation string, work func(Repositories) error) error {
	if service == nil || service.unitOfWork == nil {
		return Classify(ErrorUnavailable, operation, errors.New("learning transaction is not configured"))
	}
	return repositoryError(operation, service.unitOfWork.WithinTransaction(ctx, work))
}

func (service *goalLifecycleService) timestamp(operation string) (learning.Timestamp, error) {
	if service == nil || service.now == nil {
		return learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("learning goal clock is not configured"))
	}
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Timestamp{}, invalid(operation, err)
	}
	return timestamp, nil
}

func pauseActiveGoals(ctx context.Context, repository GoalRepository, goals []learning.LearningGoal, at learning.Timestamp, except learning.ID) error {
	for _, goal := range goals {
		if goal.Status != learning.GoalActive || goal.ID == except {
			continue
		}
		paused, err := goal.Pause(at)
		if err != nil {
			return invalid("pause active learning goal", err)
		}
		if err := repository.Update(ctx, paused); err != nil {
			return err
		}
	}
	return nil
}

func latestPausedGoal(goals []learning.LearningGoal) (learning.LearningGoal, bool) {
	paused := make([]learning.LearningGoal, 0, len(goals))
	for _, goal := range goals {
		if goal.Status == learning.GoalPaused {
			paused = append(paused, goal)
		}
	}
	if len(paused) == 0 {
		return learning.LearningGoal{}, false
	}
	sort.Slice(paused, func(i, j int) bool {
		if paused[i].UpdatedAt != paused[j].UpdatedAt {
			return paused[i].UpdatedAt.After(paused[j].UpdatedAt)
		}
		return paused[i].ID.String() > paused[j].ID.String()
	})
	return paused[0], true
}

func randomGoalID() (learning.ID, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return learning.ID{}, err
	}
	return learning.NewID("goal." + hex.EncodeToString(bytes))
}
