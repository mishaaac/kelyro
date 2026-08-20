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

func TestGoalLifecycleKeepsOneActiveGoalAndHistory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	profiles := application.NewProfileService(
		application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC) }),
	)
	times := []time.Time{
		time.Date(2026, 8, 19, 13, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 19, 14, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 19, 16, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 19, 17, 0, 0, 0, time.UTC),
	}
	ids := []string{"goal.go", "goal.git"}
	service := application.NewGoalLifecycleService(
		profiles, store,
		application.WithGoalClock(func() time.Time {
			value := times[0]
			times = times[1:]
			return value
		}),
		application.WithGoalIDGenerator(func() (learning.ID, error) {
			value := ids[0]
			ids = ids[1:]
			return learning.NewID(value)
		}),
	)

	first, err := service.Set(ctx, goalInput(t, "Backend Engineer with Go", "Software engineering"))
	if err != nil || first.Status != learning.GoalActive {
		t.Fatalf("first Set() = (%+v, %v)", first, err)
	}
	second, err := service.Set(ctx, goalInput(t, "Master Git", "Developer tools"))
	if err != nil || second.Status != learning.GoalActive {
		t.Fatalf("second Set() = (%+v, %v)", second, err)
	}
	goals, err := service.Show(ctx)
	if err != nil || len(goals) != 2 || countGoalStatus(goals, learning.GoalActive) != 1 || countGoalStatus(goals, learning.GoalPaused) != 1 {
		t.Fatalf("Show() after replacement = (%+v, %v)", goals, err)
	}
	resumed, err := service.Resume(ctx)
	if err != nil || resumed.ID != first.ID || resumed.Status != learning.GoalActive {
		t.Fatalf("Resume() with another active goal = (%+v, %v)", resumed, err)
	}
	paused, err := service.Pause(ctx)
	if err != nil || paused.ID != first.ID || paused.Status != learning.GoalPaused {
		t.Fatalf("Pause() = (%+v, %v)", paused, err)
	}
	resumed, err = service.Resume(ctx)
	if err != nil || resumed.ID != first.ID || resumed.Status != learning.GoalActive {
		t.Fatalf("Resume() = (%+v, %v)", resumed, err)
	}
	goals, err = service.Show(ctx)
	if err != nil || len(goals) != 2 || countGoalStatus(goals, learning.GoalActive) != 1 {
		t.Fatalf("final Show() = (%+v, %v)", goals, err)
	}
}

func TestGoalSetRollsBackPausedGoalWhenCreateConflicts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students))
	id, _ := learning.NewID("goal.same")
	service := application.NewGoalLifecycleService(
		profiles, store,
		application.WithGoalClock(func() time.Time { return time.Date(2026, 8, 19, 13, 0, 0, 0, time.UTC) }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return id, nil }),
	)
	if _, err := service.Set(ctx, goalInput(t, "First", "General")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Set(ctx, goalInput(t, "Second", "General")); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("second Set() error = %v, want conflict", err)
	}
	goals, err := service.Show(ctx)
	if err != nil || len(goals) != 1 || goals[0].Status != learning.GoalActive || goals[0].Title != "First" {
		t.Fatalf("goals after rollback = (%+v, %v)", goals, err)
	}
}

func TestGoalSetRejectsThresholdOutsideProgressionPolicyRange(t *testing.T) {
	t.Parallel()
	store := memory.New()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students))
	service := application.NewGoalLifecycleService(profiles, store)
	input := goalInput(t, "Invalid threshold", "General")
	input.MasteryThreshold, _ = learning.NewMasteryThreshold(.49)
	if _, err := service.Set(context.Background(), input); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("Set() error = %v, want invalid state", err)
	}
}

func goalInput(t *testing.T, title, domain string) application.SetGoalInput {
	t.Helper()
	threshold, err := learning.NewMasteryThreshold(.8)
	if err != nil {
		t.Fatal(err)
	}
	return application.SetGoalInput{
		Title: title, Domain: domain, TargetOutcome: "Demonstrate the target skill",
		StartingLevel: learning.ExperienceBeginner, MasteryThreshold: threshold,
	}
}

func countGoalStatus(goals []learning.LearningGoal, status learning.GoalStatus) int {
	count := 0
	for _, goal := range goals {
		if goal.Status == status {
			count++
		}
	}
	return count
}
