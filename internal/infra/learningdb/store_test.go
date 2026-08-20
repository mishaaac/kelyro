package learningdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/curriculumyaml"
	"github.com/mishaaac/kelyro/internal/infra/diagnosticjson"
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

func TestFactoryPersistsMasteryThresholdAcrossStoreLifetimes(t *testing.T) {
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
	custom, _ := learning.NewMasteryThreshold(.77)
	set, err := store.Mastery().SetWorkspaceOverride(context.Background(), custom)
	if err != nil || set.Source != learning.MasterySourceWorkspaceOverride {
		t.Fatalf("SetWorkspaceOverride() = (%+v, %v)", set, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	resolved, err := reopened.Mastery().Show(context.Background(), nil)
	if err != nil || resolved.Source != learning.MasterySourceWorkspaceOverride || resolved.Requirement.Mode != learning.MasteryModeCustom || resolved.Requirement.Threshold.Value() != .77 {
		t.Fatalf("persisted mastery = (%+v, %v)", resolved, err)
	}
}

func TestFactoryReopensCurriculumInstanceAndIsolatedConceptState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.Open(filepath.Join("..", "..", "..", "testdata", "curricula", "foundation-demo", "curriculum.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	curriculum, err := curriculumyaml.Load(fixture)
	if closeErr := fixture.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}
	diagnosticFixture, err := os.Open(filepath.Join("..", "..", "..", "testdata", "curricula", "foundation-demo", "diagnostic.json"))
	if err != nil {
		t.Fatal(err)
	}
	diagnostic, err := diagnosticjson.Load(diagnosticFixture)
	if closeErr := diagnosticFixture.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		t.Fatal(err)
	}

	factory := NewFactory("test")
	factory.now = func() time.Time { return time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC) }
	goalID, _ := learning.NewID("goal.curriculum-persisted")
	instanceID, _ := learning.NewID("instance.curriculum-persisted")
	factory.goalID = func() (learning.ID, error) { return goalID, nil }
	factory.curriculumInstanceID = func() (learning.ID, error) { return instanceID, nil }
	store, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	threshold, _ := learning.NewMasteryThreshold(.8)
	goal, err := store.Goals().Set(ctx, application.SetGoalInput{
		Title: "Understand ratios", Domain: "Mathematics", TargetOutcome: "Solve ratio problems",
		StartingLevel: learning.ExperienceBeginner, MasteryThreshold: threshold,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := store.CurriculumInstances().Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	conceptID, _ := learning.NewID("concept.ratio-meaning")
	state, err := store.CurriculumInstances().State(ctx, instance.ID, conceptID)
	if err != nil {
		t.Fatal(err)
	}
	seen, _ := learning.NewTimestamp(time.Date(2026, time.August, 19, 15, 30, 0, 0, time.UTC))
	updated, _ := learning.NewTimestamp(time.Date(2026, time.August, 19, 16, 0, 0, 0, time.UTC))
	score, _ := learning.NewMasteryScore(.64)
	state.Exposure = learning.ExposureLearning
	state.Mastery = score
	state.FirstSeenAt = &seen
	state.LastSeenAt = &seen
	state.ManualFlags = []string{"flag.needs-example"}
	state.UpdatedAt = updated
	if err := store.CurriculumInstances().SaveState(ctx, state); err != nil {
		t.Fatal(err)
	}
	diagnosticView, err := store.Diagnostics().Start(ctx, instance.ID, diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	diagnosticView, err = store.Diagnostics().Submit(ctx, diagnosticView.Attempt.ID, diagnostic, []string{"multiplicative"})
	if err != nil || len(diagnosticView.Attempt.Observations) != 1 {
		t.Fatalf("diagnostic checkpoint = (%+v, %v)", diagnosticView, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	loaded, err := reopened.CurriculumInstances().Get(ctx, instance.ID)
	if err != nil || loaded.Curriculum != curriculum.Reference || loaded.GoalID != goal.ID {
		t.Fatalf("reopened instance = (%+v, %v)", loaded, err)
	}
	loadedState, err := reopened.CurriculumInstances().State(ctx, instance.ID, conceptID)
	if err != nil || loadedState.Exposure != learning.ExposureLearning || loadedState.Mastery.Value() != .64 ||
		len(loadedState.ManualFlags) != 1 || loadedState.ManualFlags[0] != "flag.needs-example" {
		t.Fatalf("reopened state = (%+v, %v)", loadedState, err)
	}
	resumed, err := reopened.Diagnostics().Resume(ctx, diagnosticView.Attempt.ID, diagnostic)
	if err != nil || resumed.Item == nil || resumed.Item.ID.String() != "item.ratio-words" || len(resumed.Attempt.Observations) != 1 {
		t.Fatalf("reopened diagnostic = (%+v, %v)", resumed, err)
	}
}
