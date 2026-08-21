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

func TestStreakServiceRecalculatesAndRepairsMaterializedState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	createdAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	now := time.Date(2026, 8, 22, 17, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students),
		application.WithProfileClock(func() time.Time { return createdAt }))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	timezone := "America/Lima"
	student, err = profiles.Edit(ctx, application.ProfileChanges{Timezone: &timezone})
	if err != nil {
		t.Fatal(err)
	}

	goalID, instanceID, conceptID := testID(t, "goal.streak-service"), testID(t, "instance.streak-service"), testID(t, "concept.streak-service")
	event := mustStudyEvent(t, "history.streak-service", student.ID, learning.StudyEventEvidenceRecorded,
		"source.streak-service", time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC), &goalID, &instanceID, &conceptID)
	if err := repositories.History.Record(ctx, event); err != nil {
		t.Fatal(err)
	}
	session := completedStudySession(t, "session.streak-service", student.ID, goalID, instanceID,
		time.Date(2026, 8, 21, 17, 0, 0, 0, time.UTC), 10*time.Minute)
	if err := repositories.StudySessions.Create(ctx, session); err != nil {
		t.Fatal(err)
	}
	staleAt := fixtureTimestamp(t, time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	stale := learning.Streak{StudentID: student.ID, CurrentDays: 9, LongestDays: 9, LastStudyAt: &staleAt,
		PolicyVersion: learning.LegacyStreakPolicyVersion}
	if err := repositories.Streaks.Save(ctx, stale); err != nil {
		t.Fatal(err)
	}

	service := application.NewStreakService(profiles, store, application.WithStreakClock(func() time.Time { return now }))
	result, err := service.Show(ctx)
	if err != nil || result.CurrentDays != 2 || result.LongestDays != 2 || result.TotalActiveDays != 2 ||
		result.LastActiveLocalDate == nil || result.LastActiveLocalDate.String() != "2026-08-21" ||
		result.Timezone != timezone || result.PolicyVersion != learning.StreakPolicyVersion {
		t.Fatalf("Show() = (%+v, %v)", result, err)
	}
	stored, err := repositories.Streaks.Get(ctx, student.ID)
	if err != nil || !reflect.DeepEqual(stored, result) {
		t.Fatalf("stored streak = (%+v, %v), want %+v", stored, err, result)
	}
	again, err := service.Show(ctx)
	if err != nil || !reflect.DeepEqual(again, result) {
		t.Fatalf("idempotent Show() = (%+v, %v), want %+v", again, err, result)
	}
}
