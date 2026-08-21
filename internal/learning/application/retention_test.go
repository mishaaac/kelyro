package application_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestRetentionServicePersistsInjectedClockAndProjectsReviewDue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	createdAt := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students),
		application.WithProfileClock(func() time.Time { return createdAt }))
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(func() time.Time { return createdAt }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.retention"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Retain concepts", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	curriculum := instanceTestCurriculum(t, "1.0.0")
	instances := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return createdAt }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.retention"), nil }))
	instance, err := instances.Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	conceptID := testID(t, "concept.a")
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observedAt, _ := learning.NewTimestamp(createdAt.Add(time.Hour))
	evidence, err := learning.NewEvidenceWithMetadata(testID(t, "evidence.retention"), student.ID, conceptID,
		learning.EvidenceAssessment, "fixture/retention", testScore(t, .9), learning.EvidenceMetadata{
			Confidence: 1, Independence: 1, Difficulty: .5, AlgorithmVersion: "fixture/retention-v1",
		}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	state, err := instances.State(ctx, instance.ID, conceptID)
	if err != nil {
		t.Fatal(err)
	}
	state.Exposure = learning.ExposureMastered
	state.Mastery = testScore(t, .9)
	state.FirstSeenAt, state.LastSeenAt, state.MasteredAt = &observedAt, &observedAt, &observedAt
	state.UpdatedAt = observedAt
	if err := instances.SaveState(ctx, state); err != nil {
		t.Fatal(err)
	}

	clockValue := createdAt.Add(30 * 24 * time.Hour)
	service := application.NewRetentionService(profiles, store, application.WithRetentionClock(func() time.Time { return clockValue }))
	calculation, err := service.Recalculate(ctx, conceptID)
	if err != nil {
		t.Fatal(err)
	}
	if calculation.State.MeasuredAt.Time() != clockValue || calculation.State.Status != learning.RetentionOverdue {
		t.Fatalf("calculation = %+v", calculation)
	}
	persisted, err := service.State(ctx, conceptID)
	if err != nil || !reflect.DeepEqual(persisted, calculation.State) {
		t.Fatalf("persisted state = (%+v, %v), want %+v", persisted, err, calculation.State)
	}
	projected, err := instances.State(ctx, instance.ID, conceptID)
	if err != nil || projected.Exposure != learning.ExposureReviewDue || projected.Mastery != state.Mastery || projected.ReviewDueAt == nil {
		t.Fatalf("projected state = (%+v, %v)", projected, err)
	}
}
