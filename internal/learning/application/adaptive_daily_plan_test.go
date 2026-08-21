package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestAdaptiveDailyPlanServicePersistsReusesAndRegeneratesExplicitly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	createdAt := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	current := time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students),
		application.WithProfileClock(func() time.Time { return createdAt }))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	goals := application.NewGoalLifecycleService(profiles, store,
		application.WithGoalClock(func() time.Time { return createdAt.Add(time.Hour) }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.daily-service"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Daily planning", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	instances := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return createdAt.Add(2 * time.Hour) }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.daily-service"), nil }))
	instance, err := instances.Create(ctx, goal.ID, instanceTestCurriculum(t, "daily-plan-v1"), learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	mastery := application.NewMasteryPolicyService(profiles, repositories.Mastery,
		application.WithMasteryPolicyClock(func() time.Time { return createdAt.Add(3 * time.Hour) }))
	service := application.NewAdaptiveDailyPlanService(profiles, mastery, store,
		application.WithAdaptiveDailyPlanClock(func() time.Time { return current }))

	first, err := service.Today(ctx)
	if err != nil || first.GenerationReason != learning.DailyPlanGeneratedInitial || first.Status != learning.DailyPlanReady ||
		len(first.Items) != 1 || first.Items[0].ConceptIDs[0] != testID(t, "concept.a") {
		t.Fatalf("first Today() = (%+v, %v)", first, err)
	}
	stored, err := repositories.DailyPlans.ForDate(ctx, student.ID, goal.ID, first.Date)
	if err != nil || stored.SourceFingerprint != first.SourceFingerprint {
		t.Fatalf("stored plan = (%+v, %v)", stored, err)
	}

	current = current.Add(time.Hour)
	unchanged, err := service.Today(ctx)
	if err != nil || unchanged.CreatedAt != first.CreatedAt || unchanged.SourceFingerprint != first.SourceFingerprint {
		t.Fatalf("unchanged Today() = (%+v, %v), want reuse %+v", unchanged, err, first)
	}

	state, err := instances.State(ctx, instance.ID, testID(t, "concept.a"))
	if err != nil {
		t.Fatal(err)
	}
	masteredAt := fixtureTimestamp(t, current.Add(30*time.Minute))
	state.Exposure = learning.ExposureMastered
	state.Mastery = testScore(t, .9)
	state.FirstSeenAt, state.LastSeenAt, state.MasteredAt = &masteredAt, &masteredAt, &masteredAt
	state.UpdatedAt = masteredAt
	if err := instances.SaveState(ctx, state); err != nil {
		t.Fatal(err)
	}
	current = current.Add(time.Hour)
	regenerated, err := service.Today(ctx)
	if err != nil || regenerated.GenerationReason != learning.DailyPlanGeneratedSourceChanged ||
		regenerated.SourceFingerprint == first.SourceFingerprint || len(regenerated.Items) != 1 ||
		regenerated.Items[0].ConceptIDs[0] != testID(t, "concept.b") || !regenerated.CreatedAt.After(first.CreatedAt) {
		t.Fatalf("regenerated Today() = (%+v, %v)", regenerated, err)
	}
}

func TestAdaptiveDailyPlanServiceRequiresActiveGoal(t *testing.T) {
	t.Parallel()
	store := memory.New()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC) }))
	mastery := application.NewMasteryPolicyService(profiles, store.Repositories().Mastery,
		application.WithMasteryPolicyClock(func() time.Time { return time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC) }))
	service := application.NewAdaptiveDailyPlanService(profiles, mastery, store,
		application.WithAdaptiveDailyPlanClock(func() time.Time { return time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC) }))
	if _, err := service.Today(context.Background()); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Today() without active goal error = %v, want not found", err)
	}
}
