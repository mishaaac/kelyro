package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestLearningAnalyticsServiceCalculatesFromPrimarySources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	createdAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students),
		application.WithProfileClock(func() time.Time { return createdAt }))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// This legacy materialized row is intentionally stale. Learning Analytics
	// v1 must never use it as the source of truth.
	staleCapturedAt := fixtureTimestamp(t, createdAt.Add(time.Hour))
	if err := repositories.Analytics.Append(ctx, learning.AnalyticsSnapshot{
		StudentID: student.ID, CapturedAt: staleCapturedAt, StudyMinutes: 999,
		SessionsCompleted: 99, ConceptsIntroduced: 99, ConceptsMastered: 99, ReviewsDue: 99,
	}); err != nil {
		t.Fatal(err)
	}

	goals := application.NewGoalLifecycleService(profiles, store,
		application.WithGoalClock(func() time.Time { return createdAt.Add(2 * time.Hour) }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.analytics-service"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Analytics fixture", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	instances := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return createdAt.Add(3 * time.Hour) }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.analytics-service"), nil }))
	instance, err := instances.Create(ctx, goal.ID, instanceTestCurriculum(t, "analytics-v1"), learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	state, err := instances.State(ctx, instance.ID, testID(t, "concept.a"))
	if err != nil {
		t.Fatal(err)
	}
	seenAt := fixtureTimestamp(t, createdAt.Add(4*time.Hour))
	state.Exposure = learning.ExposureLearning
	state.Mastery = testScore(t, .55)
	state.FirstSeenAt, state.LastSeenAt = &seenAt, &seenAt
	state.UpdatedAt = seenAt
	if err := instances.SaveState(ctx, state); err != nil {
		t.Fatal(err)
	}

	service := application.NewLearningAnalyticsService(profiles, store,
		application.WithLearningAnalyticsClock(func() time.Time { return now }))
	snapshot, err := service.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Progress.ConceptsIntroduced.Value != 1 || snapshot.Progress.ConceptsLearning.Value != 1 ||
		snapshot.Progress.ConceptsMastered.Value != 0 || snapshot.Time.Total.Value != 0 {
		t.Fatalf("snapshot used non-primary facts: %+v", snapshot)
	}
	if snapshot.Mastery.AverageKnown.Value == nil || snapshot.Mastery.AverageKnown.Value.Value() != .55 ||
		!snapshot.CapturedAt.Time().Equal(now) || snapshot.PolicyVersion != learning.LearningAnalyticsPolicyVersion {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}
