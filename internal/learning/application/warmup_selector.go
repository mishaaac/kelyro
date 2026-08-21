package application

import (
	"context"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type WarmUpSelectorOption func(*warmUpSelectorService)

func WithWarmUpSelectorClock(now func() time.Time) WarmUpSelectorOption {
	return func(service *warmUpSelectorService) {
		if now != nil {
			service.now = now
		}
	}
}

type warmUpSelectorService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
}

func NewWarmUpSelectorService(profiles ProfileService, unitOfWork UnitOfWork, options ...WarmUpSelectorOption) WarmUpSelectorService {
	service := &warmUpSelectorService{profiles: profiles, unitOfWork: unitOfWork, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *warmUpSelectorService) Select(ctx context.Context, request WarmUpRequest) (learning.WarmUpPlan, error) {
	const operation = "select contextual warm-up"
	if err := request.Lesson.Validate(); err != nil {
		return learning.WarmUpPlan{}, invalid(operation, err)
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.WarmUpPlan{}, repositoryError(operation, err)
	}
	generatedAt, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.WarmUpPlan{}, invalid(operation, err)
	}
	availableMinutes := request.AvailableMinutes
	if availableMinutes < 0 || availableMinutes > 24*60 {
		return learning.WarmUpPlan{}, invalid(operation, fmt.Errorf("warm-up available minutes must be within 0..1440"))
	}
	if availableMinutes > student.Profile.Availability.DailyMinutes {
		availableMinutes = student.Profile.Availability.DailyMinutes
	}

	var plan learning.WarmUpPlan
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		concepts, err := repositories.Curricula.Concepts(ctx, request.Lesson.Curriculum)
		if err != nil {
			return err
		}
		known := make(map[learning.ID]struct{}, len(concepts))
		for _, concept := range concepts {
			known[concept.ID] = struct{}{}
		}
		dueReviews, err := repositories.Reviews.ListDue(ctx, student.ID, generatedAt)
		if err != nil {
			return err
		}
		mistakes, err := repositories.Mistakes.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		dueReviews = reviewsWithinCurriculum(dueReviews, known)
		mistakes = mistakesWithinCurriculum(mistakes, known)
		plan, err = learning.SelectWarmUpV1(learning.WarmUpSelectionInput{
			StudentID: student.ID, Lesson: request.Lesson, Concepts: concepts,
			DueReviews: dueReviews, Mistakes: mistakes, RecentConceptIDs: request.RecentConceptIDs,
			AvailableMinutes: availableMinutes, GeneratedAt: generatedAt,
		})
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		return nil
	})
	if err != nil {
		return learning.WarmUpPlan{}, repositoryError(operation, err)
	}
	return plan, nil
}

func reviewsWithinCurriculum(items []learning.ReviewItem, known map[learning.ID]struct{}) []learning.ReviewItem {
	filtered := make([]learning.ReviewItem, 0, len(items))
	for _, item := range items {
		if _, exists := known[item.ConceptID]; exists {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func mistakesWithinCurriculum(items []learning.Mistake, known map[learning.ID]struct{}) []learning.Mistake {
	filtered := make([]learning.Mistake, 0, len(items))
	for _, item := range items {
		if _, exists := known[item.ConceptID]; exists {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

var _ WarmUpSelectorService = (*warmUpSelectorService)(nil)
