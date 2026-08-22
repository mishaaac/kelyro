package application

import (
	"context"
	"errors"
	"reflect"
	"sort"

	"github.com/mishaaac/kelyro/internal/learning"
)

func recalculateReviewSchedules(ctx context.Context, repositories Repositories, studentID learning.ID, calculatedAt learning.Timestamp,
	data recalculationData, retention map[learning.ID]learning.RetentionState, dryRun bool, impact *RecalculationImpact) ([]learning.ReviewItem, error) {
	critical := make(map[learning.ID]bool)
	contexts := make(map[learning.ID]learning.ConceptState)
	activeInstances := make(map[learning.ID]struct{})
	for _, instance := range data.instances {
		if instance.Status != learning.CurriculumInstanceActive {
			continue
		}
		activeInstances[instance.ID] = struct{}{}
		planning, err := repositories.Curricula.PlanningConcepts(ctx, instance.Curriculum)
		if err != nil {
			return nil, err
		}
		for _, concept := range planning {
			for _, prerequisiteID := range concept.PrerequisiteIDs {
				critical[prerequisiteID] = true
			}
		}
	}
	for instanceID, states := range data.states {
		if _, active := activeInstances[instanceID]; !active {
			continue
		}
		for _, state := range states {
			if state.Exposure == learning.ExposureNotSeen {
				continue
			}
			current, exists := contexts[state.ConceptID]
			if !exists || state.UpdatedAt.After(current.UpdatedAt) {
				contexts[state.ConceptID] = state.ConceptState()
			}
		}
	}

	projected := append([]learning.ReviewItem(nil), data.reviews...)
	grouped := groupReviewItems(projected)
	conceptIDs := make([]learning.ID, 0, len(contexts))
	for conceptID := range contexts {
		conceptIDs = append(conceptIDs, conceptID)
	}
	sort.Slice(conceptIDs, func(i, j int) bool { return conceptIDs[i].String() < conceptIDs[j].String() })
	for _, conceptID := range conceptIDs {
		state, exists := retention[conceptID]
		if !exists {
			continue
		}
		schedule, scheduled, err := learning.ScheduleReviewV1(learning.ReviewSchedulingInput{
			Concept: contexts[conceptID], Retention: state, History: grouped[conceptID],
			CriticalPrerequisite: critical[conceptID], ScheduledAt: calculatedAt,
		})
		if err != nil {
			return nil, Classify(ErrorInvalidState, "recalculate review schedule", err)
		}
		if !scheduled {
			continue
		}
		previousSchedule, loadErr := repositories.Reviews.GetSchedule(ctx, studentID, conceptID)
		scheduleChanged := true
		if loadErr == nil {
			candidate := schedule
			candidate.UpdatedAt = previousSchedule.UpdatedAt
			if reflect.DeepEqual(previousSchedule, candidate) {
				schedule, scheduleChanged = previousSchedule, false
			}
		} else if !errors.Is(loadErr, ErrNotFound) {
			return nil, loadErr
		}
		if scheduleChanged {
			impact.ReviewSchedulesChanged++
			if !dryRun {
				if err := repositories.Reviews.SaveSchedule(ctx, schedule); err != nil {
					return nil, err
				}
			}
		}

		pending := findPendingReview(grouped[conceptID])
		if pending != nil {
			var aligned learning.ReviewItem
			if pending.AlgorithmVersion == learning.ReviewSchedulerVersion {
				aligned, err = learning.AlignReviewItemV1(*pending, schedule)
			} else {
				aligned, err = learning.NewReviewItemV1(pending.ID, schedule, pending.CreatedAt)
			}
			if err != nil {
				return nil, Classify(ErrorInvalidState, "align recalculated review", err)
			}
			if !reflect.DeepEqual(*pending, aligned) {
				impact.ReviewItemsChanged++
				if !dryRun {
					if err := repositories.Reviews.UpdateItem(ctx, aligned); err != nil {
						return nil, err
					}
				}
				replaceReviewItem(projected, aligned)
				grouped[conceptID] = replaceReviewItem(grouped[conceptID], aligned)
			}
			continue
		}
		id, err := learning.NewReviewItemIDV1(studentID, conceptID, schedule.DueAt, len(grouped[conceptID]))
		if err != nil {
			return nil, Classify(ErrorInvalidState, "identify recalculated review", err)
		}
		item, err := learning.NewReviewItemV1(id, schedule, calculatedAt)
		if err != nil {
			return nil, Classify(ErrorInvalidState, "create recalculated review", err)
		}
		impact.ReviewItemsChanged++
		if !dryRun {
			if err := repositories.Reviews.CreateItem(ctx, item); err != nil {
				return nil, err
			}
		}
		projected = append(projected, item)
		grouped[conceptID] = append(grouped[conceptID], item)
	}
	return projected, nil
}

func replaceReviewItem(items []learning.ReviewItem, replacement learning.ReviewItem) []learning.ReviewItem {
	for index := range items {
		if items[index].ID == replacement.ID {
			items[index] = replacement
			return items
		}
	}
	return items
}

func (service *maintenanceRecalculationService) recalculateDailyPlan(ctx context.Context, repositories Repositories, student learning.Student,
	threshold learning.ResolvedMasteryThreshold, calculatedAt learning.Timestamp, data recalculationData,
	retention map[learning.ID]learning.RetentionState, reviews []learning.ReviewItem, dryRun bool, impact *RecalculationImpact) error {
	goal, found := maintenanceActiveGoal(data.goals)
	if !found {
		return nil
	}
	instance, found := maintenanceActiveInstance(data.instances, goal.ID)
	if !found {
		return nil
	}
	date, err := dailyPlanLocalDate(calculatedAt, student.Profile.Timezone)
	if err != nil {
		return Classify(ErrorInvalidState, "calculate maintenance plan date", err)
	}
	planning, err := repositories.Curricula.PlanningConcepts(ctx, instance.Curriculum)
	if err != nil {
		return err
	}
	known := make(map[learning.ID]struct{}, len(planning))
	for _, concept := range planning {
		known[concept.ConceptID] = struct{}{}
	}
	due := make([]learning.ReviewItem, 0)
	for _, item := range reviews {
		if _, exists := known[item.ConceptID]; exists && item.Status == learning.ReviewPending && !item.DueAt.After(calculatedAt) {
			due = append(due, item)
		}
	}
	retentionItems := make([]learning.RetentionState, 0, len(retention))
	for _, item := range retention {
		retentionItems = append(retentionItems, item)
	}
	sort.Slice(retentionItems, func(i, j int) bool {
		return retentionItems[i].ConceptID.String() < retentionItems[j].ConceptID.String()
	})
	mistakes, err := repositories.Mistakes.ListByStudent(ctx, student.ID)
	if err != nil {
		return err
	}
	history, err := repositories.History.ListByStudent(ctx, student.ID, nil, nil)
	if err != nil {
		return err
	}
	candidate, err := service.algorithms.DailyPlan.Build(learning.DailyPlanInput{
		StudentID: student.ID, GoalID: goal.ID, CurriculumInstanceID: instance.ID,
		Curriculum: instance.Curriculum, Timezone: student.Profile.Timezone, Date: date, GeneratedAt: calculatedAt,
		AvailableMinutes: student.Profile.Availability.DailyMinutes, MasteryPolicy: threshold,
		Concepts: planning, States: data.states[instance.ID], ReviewsDue: due, Retention: retentionItems,
		Mistakes: mistakes, History: history, GenerationReason: learning.DailyPlanGeneratedInitial,
		Policy: learning.DefaultDailyPlanPolicy(),
	})
	if err != nil {
		return Classify(ErrorInvalidState, "recalculate daily plan", err)
	}
	if candidate.PolicyVersion != service.algorithms.DailyPlan.Version() {
		return Classify(ErrorInvalidState, "recalculate daily plan", errors.New("daily plan result version does not match configured algorithm"))
	}
	existing, loadErr := repositories.DailyPlans.ForDate(ctx, student.ID, goal.ID, date)
	if loadErr == nil {
		impact.Previous.DailyPlan = appendVersion(impact.Previous.DailyPlan, existing.PolicyVersion)
		if existing.PolicyVersion == candidate.PolicyVersion && existing.SourceFingerprint == candidate.SourceFingerprint {
			return nil
		}
		if existing.PolicyVersion != candidate.PolicyVersion {
			candidate.GenerationReason = learning.DailyPlanGeneratedPolicyChanged
		} else {
			candidate.GenerationReason = learning.DailyPlanGeneratedSourceChanged
		}
		if err := candidate.Validate(); err != nil {
			return Classify(ErrorInvalidState, "validate recalculated daily plan", err)
		}
	} else if !errors.Is(loadErr, ErrNotFound) {
		return loadErr
	}
	impact.DailyPlansChanged++
	if !dryRun {
		return repositories.DailyPlans.Save(ctx, candidate)
	}
	return nil
}

func maintenanceActiveGoal(goals []learning.LearningGoal) (learning.LearningGoal, bool) {
	for _, goal := range goals {
		if goal.Status == learning.GoalActive {
			return goal, true
		}
	}
	return learning.LearningGoal{}, false
}

func maintenanceActiveInstance(instances []learning.CurriculumInstance, goalID learning.ID) (learning.CurriculumInstance, bool) {
	candidates := make([]learning.CurriculumInstance, 0)
	for _, instance := range instances {
		if instance.GoalID == goalID && instance.Status == learning.CurriculumInstanceActive {
			candidates = append(candidates, instance)
		}
	}
	if len(candidates) == 0 {
		return learning.CurriculumInstance{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].UpdatedAt != candidates[j].UpdatedAt {
			return candidates[i].UpdatedAt.After(candidates[j].UpdatedAt)
		}
		return candidates[i].ID.String() > candidates[j].ID.String()
	})
	return candidates[0], true
}
