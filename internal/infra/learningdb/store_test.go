package learningdb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/platform"
)

func TestFactoryPersistsProfileAcrossStoreLifetimes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatal(err)
	}
	factory := NewFactory("test")
	factory.now = func() time.Time { return time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC) }
	store, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	name := "Ada"
	if _, err := store.Profiles().Edit(context.Background(), application.ProfileChanges{DisplayName: &name}); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	student, err := reopened.Profiles().Show(context.Background())
	if err != nil || student.Profile.DisplayName != "Ada" {
		t.Fatalf("persisted profile = (%+v, %v)", student.Profile, err)
	}
}

func TestFactoryPersistsLearningGoalHistoryAcrossStoreLifetimes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatal(err)
	}
	factory := NewFactory("test")
	factory.now = func() time.Time { return time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC) }
	id, _ := learning.NewID("goal.persisted")
	factory.goalID = func() (learning.ID, error) { return id, nil }
	store, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	threshold, _ := learning.NewMasteryThreshold(.8)
	created, err := store.Goals().Set(context.Background(), application.SetGoalInput{
		Title: "Learn differential equations", Domain: "Mathematics",
		TargetOutcome: "Solve first-order differential equations",
		StartingLevel: learning.ExperienceBeginner, MasteryThreshold: threshold,
	})
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if _, err := store.Goals().Pause(context.Background()); err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	goals, err := reopened.Goals().Show(context.Background())
	if err != nil || len(goals) != 1 || goals[0].ID != created.ID || goals[0].Status != learning.GoalPaused {
		t.Fatalf("persisted goals = (%+v, %v)", goals, err)
	}
}

func TestFactoryPersistsOnboardingCheckpointAcrossStoreLifetimes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatal(err)
	}
	factory := NewFactory("test")
	current := time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC)
	factory.now = func() time.Time {
		value := current
		current = current.Add(time.Minute)
		return value
	}
	store, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Onboarding().Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Onboarding().Submit(context.Background(), "Ada"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	view, err := reopened.Onboarding().Show(context.Background())
	if err != nil || view.Question.ID != application.OnboardingGoalTitleQuestion || view.Interview.Answers[application.OnboardingDisplayNameQuestion] != "Ada" {
		t.Fatalf("resumed onboarding = (%+v, %v)", view, err)
	}
}
