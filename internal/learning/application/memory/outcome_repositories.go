package memory

import (
	"context"
	"sort"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

type reviewRepository struct{ store *Store }

func (repository reviewRepository) GetSchedule(ctx context.Context, studentID, conceptID learning.ID) (learning.ReviewSchedule, error) {
	if err := contextError("get memory review schedule", ctx); err != nil {
		return learning.ReviewSchedule{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	schedule, exists := repository.store.schedules[studentConceptKey{student: studentID, concept: conceptID}]
	if !exists {
		return learning.ReviewSchedule{}, notFound("get memory review schedule")
	}
	return cloneSchedule(schedule), nil
}

func (repository reviewRepository) SaveSchedule(ctx context.Context, schedule learning.ReviewSchedule) error {
	if err := contextError("save memory review schedule", ctx); err != nil {
		return err
	}
	if err := schedule.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "save memory review schedule", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.schedules[studentConceptKey{student: schedule.StudentID, concept: schedule.ConceptID}] = cloneSchedule(schedule)
	return nil
}

func (repository reviewRepository) CreateItem(ctx context.Context, item learning.ReviewItem) error {
	if err := contextError("create memory review item", ctx); err != nil {
		return err
	}
	if err := item.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "create memory review item", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.reviewItems[item.ID]; exists {
		return conflict("create memory review item")
	}
	if item.Status == learning.ReviewPending {
		for _, existing := range repository.store.reviewItems {
			if existing.StudentID == item.StudentID && existing.ConceptID == item.ConceptID && existing.Status == learning.ReviewPending {
				return conflict("create memory review item")
			}
		}
	}
	repository.store.reviewItems[item.ID] = cloneReviewItem(item)
	return nil
}

func (repository reviewRepository) GetItem(ctx context.Context, id learning.ID) (learning.ReviewItem, error) {
	if err := contextError("get memory review item", ctx); err != nil {
		return learning.ReviewItem{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	item, exists := repository.store.reviewItems[id]
	if !exists {
		return learning.ReviewItem{}, notFound("get memory review item")
	}
	return cloneReviewItem(item), nil
}

func (repository reviewRepository) UpdateItem(ctx context.Context, item learning.ReviewItem) error {
	if err := contextError("update memory review item", ctx); err != nil {
		return err
	}
	if err := item.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "update memory review item", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.reviewItems[item.ID]; !exists {
		return notFound("update memory review item")
	}
	if item.Status == learning.ReviewPending {
		for id, existing := range repository.store.reviewItems {
			if id != item.ID && existing.StudentID == item.StudentID && existing.ConceptID == item.ConceptID && existing.Status == learning.ReviewPending {
				return conflict("update memory review item")
			}
		}
	}
	repository.store.reviewItems[item.ID] = cloneReviewItem(item)
	return nil
}

func (repository reviewRepository) PendingByConcept(ctx context.Context, studentID, conceptID learning.ID) (learning.ReviewItem, error) {
	if err := contextError("get memory pending review", ctx); err != nil {
		return learning.ReviewItem{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	for _, item := range repository.store.reviewItems {
		if item.StudentID == studentID && item.ConceptID == conceptID && item.Status == learning.ReviewPending {
			return cloneReviewItem(item), nil
		}
	}
	return learning.ReviewItem{}, notFound("get memory pending review")
}

func (repository reviewRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.ReviewItem, error) {
	if err := contextError("list memory review items", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.ReviewItem, 0)
	for _, item := range repository.store.reviewItems {
		if item.StudentID == studentID {
			items = append(items, cloneReviewItem(item))
		}
	}
	sortReviewItems(items)
	return items, nil
}

func (repository reviewRepository) ListDue(ctx context.Context, studentID learning.ID, asOf learning.Timestamp) ([]learning.ReviewItem, error) {
	if err := contextError("list memory due reviews", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.ReviewItem, 0)
	for _, item := range repository.store.reviewItems {
		if item.StudentID == studentID && item.Status == learning.ReviewPending && !item.DueAt.After(asOf) {
			items = append(items, cloneReviewItem(item))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return reviewItemLess(items[i], items[j])
	})
	return items, nil
}

func sortReviewItems(items []learning.ReviewItem) {
	sort.Slice(items, func(i, j int) bool { return reviewItemLess(items[i], items[j]) })
}

func reviewItemLess(left, right learning.ReviewItem) bool {
	if left.DueAt == right.DueAt {
		return left.ID.String() < right.ID.String()
	}
	return left.DueAt.Before(right.DueAt)
}

type streakRepository struct{ store *Store }

func (repository streakRepository) Get(ctx context.Context, studentID learning.ID) (learning.Streak, error) {
	if err := contextError("get memory streak", ctx); err != nil {
		return learning.Streak{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	streak, exists := repository.store.streaks[studentID]
	if !exists {
		return learning.Streak{}, notFound("get memory streak")
	}
	return cloneStreak(streak), nil
}

func (repository streakRepository) Save(ctx context.Context, streak learning.Streak) error {
	if err := contextError("save memory streak", ctx); err != nil {
		return err
	}
	if err := streak.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "save memory streak", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.streaks[streak.StudentID] = cloneStreak(streak)
	return nil
}

type achievementRepository struct{ store *Store }

func (repository achievementRepository) SaveDefinition(ctx context.Context, definition learning.AchievementDefinition) error {
	if err := contextError("save memory achievement definition", ctx); err != nil {
		return err
	}
	if err := definition.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "save memory achievement definition", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.achievementDefinitions[definition.ID] = definition
	return nil
}

func (repository achievementRepository) ListDefinitions(ctx context.Context) ([]learning.AchievementDefinition, error) {
	if err := contextError("list memory achievement definitions", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.AchievementDefinition, 0, len(repository.store.achievementDefinitions))
	for _, definition := range repository.store.achievementDefinitions {
		items = append(items, definition)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID.String() < items[j].ID.String() })
	return items, nil
}

func (repository achievementRepository) Get(ctx context.Context, id learning.ID) (learning.Achievement, error) {
	if err := contextError("get memory achievement", ctx); err != nil {
		return learning.Achievement{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	achievement, exists := repository.store.achievements[id]
	if !exists {
		return learning.Achievement{}, notFound("get memory achievement")
	}
	return cloneAchievement(achievement), nil
}

func (repository achievementRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.Achievement, error) {
	if err := contextError("list memory achievements", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.Achievement, 0)
	for _, achievement := range repository.store.achievements {
		if achievement.StudentID == studentID {
			items = append(items, cloneAchievement(achievement))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID.String() < items[j].ID.String() })
	return items, nil
}

func (repository achievementRepository) Save(ctx context.Context, achievement learning.Achievement) error {
	if err := contextError("save memory achievement", ctx); err != nil {
		return err
	}
	if err := achievement.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "save memory achievement", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.achievements[achievement.ID] = cloneAchievement(achievement)
	return nil
}

func (repository achievementRepository) Unlock(ctx context.Context, achievement learning.Achievement) (bool, error) {
	if err := contextError("unlock memory achievement", ctx); err != nil {
		return false, err
	}
	if err := achievement.Validate(); err != nil {
		return false, application.Classify(application.ErrorInvalidState, "unlock memory achievement", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	for _, existing := range repository.store.achievements {
		if existing.StudentID == achievement.StudentID && existing.Key == achievement.Key {
			return false, nil
		}
	}
	if _, exists := repository.store.achievements[achievement.ID]; exists {
		return false, conflict("unlock memory achievement")
	}
	repository.store.achievements[achievement.ID] = cloneAchievement(achievement)
	return true, nil
}

func (repository achievementRepository) AppendMilestone(ctx context.Context, milestone learning.Milestone) error {
	if err := contextError("append memory milestone", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.milestones[milestone.ID]; exists {
		return conflict("append memory milestone")
	}
	repository.store.milestones[milestone.ID] = milestone
	return nil
}

func (repository achievementRepository) ListMilestones(ctx context.Context, studentID, goalID learning.ID) ([]learning.Milestone, error) {
	if err := contextError("list memory milestones", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.Milestone, 0)
	for _, milestone := range repository.store.milestones {
		if milestone.StudentID == studentID && milestone.GoalID == goalID {
			items = append(items, milestone)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ReachedAt == items[j].ReachedAt {
			return items[i].ID.String() < items[j].ID.String()
		}
		return items[i].ReachedAt.Before(items[j].ReachedAt)
	})
	return items, nil
}

type analyticsRepository struct{ store *Store }

func (repository analyticsRepository) Append(ctx context.Context, snapshot learning.AnalyticsSnapshot) error {
	if err := contextError("append memory analytics", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	items := repository.store.analytics[snapshot.StudentID]
	for _, existing := range items {
		if existing.CapturedAt == snapshot.CapturedAt {
			return conflict("append memory analytics")
		}
	}
	repository.store.analytics[snapshot.StudentID] = append(items, snapshot)
	return nil
}

func (repository analyticsRepository) Latest(ctx context.Context, studentID learning.ID) (learning.AnalyticsSnapshot, error) {
	if err := contextError("get latest memory analytics", ctx); err != nil {
		return learning.AnalyticsSnapshot{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := repository.store.analytics[studentID]
	if len(items) == 0 {
		return learning.AnalyticsSnapshot{}, notFound("get latest memory analytics")
	}
	latest := items[0]
	for _, item := range items[1:] {
		if item.CapturedAt.After(latest.CapturedAt) {
			latest = item
		}
	}
	return latest, nil
}

type dailyPlanRepository struct{ store *Store }

func (repository dailyPlanRepository) Save(ctx context.Context, plan learning.DailyPlan) error {
	if err := contextError("save memory daily plan", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.dailyPlans[dailyPlanKey(plan.StudentID, plan.GoalID, plan.Date)] = cloneDailyPlan(plan)
	return nil
}

func (repository dailyPlanRepository) ForDate(ctx context.Context, studentID, goalID learning.ID, date learning.Timestamp) (learning.DailyPlan, error) {
	if err := contextError("get memory daily plan", ctx); err != nil {
		return learning.DailyPlan{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	plan, exists := repository.store.dailyPlans[dailyPlanKey(studentID, goalID, date)]
	if !exists {
		return learning.DailyPlan{}, notFound("get memory daily plan")
	}
	return cloneDailyPlan(plan), nil
}

func dailyPlanKey(studentID, goalID learning.ID, date learning.Timestamp) planKey {
	return planKey{student: studentID, goal: goalID, date: date.Time().UnixNano()}
}
