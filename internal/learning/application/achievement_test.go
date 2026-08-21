package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestAchievementServiceRecalculatesHistoryAndNeverUnlocksTwice(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	base := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return base }))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	goals := application.NewGoalLifecycleService(profiles, store,
		application.WithGoalClock(func() time.Time { return base }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.achievement-service"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Achievement fixture", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	instances := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return base }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.achievement-service"), nil }))
	instance, err := instances.Create(ctx, goal.ID, instanceTestCurriculum(t, "1.0.0"), learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	session := completedStudySession(t, "session.achievement-service", student.ID, goal.ID, instance.ID, base.Add(time.Hour), 20*time.Minute)
	if err := store.Repositories().StudySessions.Create(ctx, session); err != nil {
		t.Fatal(err)
	}
	masteredAt := []time.Time{base.AddDate(0, 0, 1), base.AddDate(0, 0, 2)}
	for index, conceptID := range []learning.ID{testID(t, "concept.a"), testID(t, "concept.b")} {
		state, err := instances.State(ctx, instance.ID, conceptID)
		if err != nil {
			t.Fatal(err)
		}
		observed := fixtureTimestamp(t, masteredAt[index])
		state.Exposure = learning.ExposureMastered
		state.Mastery = testScore(t, .9)
		state.FirstSeenAt, state.LastSeenAt, state.MasteredAt = &observed, &observed, &observed
		state.UpdatedAt = observed
		if err := instances.SaveState(ctx, state); err != nil {
			t.Fatal(err)
		}
	}

	now := base.AddDate(0, 0, 10)
	service := application.NewAchievementService(profiles, store, application.WithAchievementClock(func() time.Time { return now }))
	first, err := service.Refresh(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.NewlyUnlocked) != 3 || len(first.Achievements) != 3 {
		t.Fatalf("first Refresh() = %+v, want first session, first concept and module", first)
	}
	if *first.NewlyUnlocked[0].UnlockedAt == first.EvaluatedAt {
		t.Fatalf("historical unlock was stamped at recalculation time: %+v", first.NewlyUnlocked[0])
	}
	second, err := service.Refresh(ctx)
	if err != nil || len(second.NewlyUnlocked) != 0 || len(second.Achievements) != 3 {
		t.Fatalf("idempotent Refresh() = (%+v, %v)", second, err)
	}
	events, err := store.Repositories().History.ListByStudent(ctx, student.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	unlockedEvents := 0
	for _, event := range events {
		if event.Type == learning.StudyEventAchievementUnlocked {
			unlockedEvents++
		}
	}
	if unlockedEvents != 3 {
		t.Fatalf("achievement history events = %d, want 3", unlockedEvents)
	}
	definitions, err := store.Repositories().Achievements.ListDefinitions(ctx)
	if err != nil || len(definitions) != len(learning.FoundationAchievementDefinitions()) {
		t.Fatalf("persisted definitions = (%+v, %v)", definitions, err)
	}
}
