package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type AchievementOption func(*achievementService)

func WithAchievementClock(now func() time.Time) AchievementOption {
	return func(service *achievementService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithAchievementDefinitions(definitions []learning.AchievementDefinition) AchievementOption {
	return func(service *achievementService) {
		service.definitions = append([]learning.AchievementDefinition(nil), definitions...)
	}
}

type achievementService struct {
	profiles    ProfileService
	unitOfWork  UnitOfWork
	now         func() time.Time
	definitions []learning.AchievementDefinition
}

func NewAchievementService(profiles ProfileService, unitOfWork UnitOfWork, options ...AchievementOption) AchievementService {
	service := &achievementService{
		profiles: profiles, unitOfWork: unitOfWork, now: time.Now,
		definitions: learning.FoundationAchievementDefinitions(),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *achievementService) Refresh(ctx context.Context) (AchievementRefresh, error) {
	const operation = "refresh learning achievements"
	if service == nil || service.profiles == nil || service.unitOfWork == nil || service.now == nil {
		return AchievementRefresh{}, Classify(ErrorUnavailable, operation, errors.New("achievement dependencies are not configured"))
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return AchievementRefresh{}, repositoryError(operation, err)
	}
	evaluatedAt, err := learning.NewTimestamp(service.now())
	if err != nil {
		return AchievementRefresh{}, invalid(operation, fmt.Errorf("read achievement clock: %w", err))
	}
	result := AchievementRefresh{EvaluatedAt: evaluatedAt, PolicyVersion: learning.AchievementPolicyVersion}
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		if err := requireAchievementRepositories(operation, repositories); err != nil {
			return err
		}
		for _, definition := range service.definitions {
			if err := definition.Validate(); err != nil {
				return Classify(ErrorInvalidState, operation, err)
			}
			if err := repositories.Achievements.SaveDefinition(ctx, definition); err != nil {
				return err
			}
		}
		sessions, err := repositories.StudySessions.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		events, err := repositories.History.ListByStudent(ctx, student.ID, nil, nil)
		if err != nil {
			return err
		}
		reviews, err := repositories.Reviews.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		modules, err := achievementModules(ctx, repositories, student.ID)
		if err != nil {
			return err
		}
		candidates, err := learning.EvaluateAchievementsV1(service.definitions, learning.AchievementEvaluationInput{
			StudentID: student.ID, Timezone: student.Profile.Timezone, AsOf: evaluatedAt,
			Sessions: sessions, Events: events, Reviews: reviews, Modules: modules,
			StreakPolicy: learning.DefaultStreakPolicy(),
		})
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		for _, candidate := range candidates {
			created, err := repositories.Achievements.Unlock(ctx, candidate)
			if err != nil {
				return err
			}
			if !created {
				continue
			}
			if err := recordStudyEvent(ctx, repositories.History, student.ID, learning.StudyEventAchievementUnlocked,
				candidate.ID, *candidate.UnlockedAt, nil, nil, nil); err != nil {
				return err
			}
			result.NewlyUnlocked = append(result.NewlyUnlocked, candidate)
		}
		result.Achievements, err = repositories.Achievements.ListByStudent(ctx, student.ID)
		return err
	})
	if err != nil {
		return AchievementRefresh{}, repositoryError(operation, err)
	}
	if result.Achievements == nil {
		result.Achievements = []learning.Achievement{}
	}
	if result.NewlyUnlocked == nil {
		result.NewlyUnlocked = []learning.Achievement{}
	}
	return result, nil
}

func requireAchievementRepositories(operation string, repositories Repositories) error {
	if repositories.Achievements == nil || repositories.StudySessions == nil || repositories.History == nil ||
		repositories.Reviews == nil || repositories.CurriculumInstances == nil || repositories.InstanceConceptStates == nil ||
		repositories.Curricula == nil {
		return Classify(ErrorUnavailable, operation, errors.New("achievement repositories are not configured"))
	}
	return nil
}

func achievementModules(ctx context.Context, repositories Repositories, studentID learning.ID) ([]learning.AchievementModuleProgress, error) {
	instances, err := repositories.CurriculumInstances.ListByStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	modules := make([]learning.AchievementModuleProgress, 0)
	for _, instance := range instances {
		concepts, err := repositories.Curricula.Concepts(ctx, instance.Curriculum)
		if err != nil {
			return nil, err
		}
		states, err := repositories.InstanceConceptStates.ListByInstance(ctx, instance.ID)
		if err != nil {
			return nil, err
		}
		masteredByConcept := make(map[learning.ID]*learning.Timestamp, len(states))
		for _, state := range states {
			if state.MasteredAt != nil {
				masteredAt := *state.MasteredAt
				masteredByConcept[state.ConceptID] = &masteredAt
			}
		}
		byModule := make(map[learning.ID][]learning.AchievementConceptProgress)
		for _, concept := range concepts {
			moduleID, err := repositories.Curricula.ModuleForConcept(ctx, instance.Curriculum, concept.ID)
			if err != nil {
				return nil, err
			}
			byModule[moduleID] = append(byModule[moduleID], learning.AchievementConceptProgress{
				ConceptID: concept.ID, MasteredAt: masteredByConcept[concept.ID],
			})
		}
		moduleIDs := make([]learning.ID, 0, len(byModule))
		for moduleID := range byModule {
			moduleIDs = append(moduleIDs, moduleID)
		}
		sort.Slice(moduleIDs, func(i, j int) bool { return moduleIDs[i].String() < moduleIDs[j].String() })
		for _, moduleID := range moduleIDs {
			moduleConcepts := byModule[moduleID]
			sort.Slice(moduleConcepts, func(i, j int) bool {
				return moduleConcepts[i].ConceptID.String() < moduleConcepts[j].ConceptID.String()
			})
			modules = append(modules, learning.AchievementModuleProgress{
				StudentID: studentID, CurriculumInstanceID: instance.ID, GoalID: instance.GoalID,
				Curriculum: instance.Curriculum, ModuleID: moduleID, Concepts: moduleConcepts,
			})
		}
	}
	return modules, nil
}

var _ AchievementService = (*achievementService)(nil)
