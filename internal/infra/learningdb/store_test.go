package learningdb

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
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

func TestFactoryRejectsNegativeStudySessionIdleTimeout(t *testing.T) {
	if _, err := NewFactory("test").WithStudySessionIdleTimeout(-time.Second).Open(context.Background(), t.TempDir()); err == nil {
		t.Fatal("Open() accepted a negative study session idle timeout")
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

func TestFactoryPersistsCompletedIntegratedSetupAcrossStoreLifetimes(t *testing.T) {
	ctx := context.Background()
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
	factory.now = func() time.Time { value := current; current = current.Add(time.Minute); return value }
	store, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}

	view := completeIntegratedSetupWithoutDiagnostic(t, ctx, store.Setup())
	if view.Setup.Status != learning.SetupCompleted || view.Setup.SetupCompletedAt == nil || view.Instance == nil {
		t.Fatalf("completed setup = %+v", view)
	}
	states, err := store.CurriculumInstances().States(ctx, *view.Setup.CurriculumInstanceID)
	if err != nil || len(states) == 0 {
		t.Fatalf("initial states = (%+v, %v)", states, err)
	}
	for _, state := range states {
		if state.Exposure != learning.ExposureNotSeen || state.Mastery.Value() != 0 {
			t.Fatalf("initial state = %+v", state)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	resumed, err := reopened.Setup().Show(ctx)
	if err != nil || resumed.Setup.Status != learning.SetupCompleted || resumed.Setup.SetupCompletedAt == nil || resumed.Instance == nil {
		t.Fatalf("reopened setup = (%+v, %v)", resumed, err)
	}
}

func TestFactoryPersistsMistakeMemoryAcrossStoreLifetimes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatal(err)
	}
	factory := NewFactory("test")
	current := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	factory.now = func() time.Time { value := current; current = current.Add(time.Minute); return value }
	store, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	setup := completeIntegratedSetupWithoutDiagnostic(t, ctx, store.Setup())
	states, err := store.CurriculumInstances().States(ctx, *setup.Setup.CurriculumInstanceID)
	if err != nil || len(states) == 0 {
		t.Fatalf("curriculum states = (%+v, %v)", states, err)
	}
	observedAt, _ := learning.NewTimestamp(current.Add(time.Hour))
	recorded, err := store.Mistakes().Record(ctx, application.RecordMistakeInput{
		ConceptID: states[0].ConceptID, Key: "fixture-misread", Category: learning.MistakeCareless,
		Summary: "Misread the deterministic fixture prompt", ObservedAt: observedAt, SourceRef: "fixture/check/persistence",
	})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	items, err := reopened.Mistakes().List(ctx)
	if err != nil || len(items) != 1 || items[0].ID != recorded.Mistake.ID {
		t.Fatalf("persisted mistakes = (%+v, %v)", items, err)
	}
	view, err := reopened.Mistakes().Get(ctx, recorded.Mistake.ID)
	if err != nil || len(view.History) != 1 || view.History[0].Type != learning.MistakeObservedEvent {
		t.Fatalf("persisted mistake history = (%+v, %v)", view, err)
	}
}

func TestFactoryPersistsAndRecoversStudySessionAcrossStoreLifetimes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatal(err)
	}
	current := time.Date(2026, time.August, 21, 9, 0, 0, 0, time.UTC)
	nextID := 0
	factory := NewFactory("test")
	factory.now = func() time.Time { return current }
	factory.studySessionIdle = 15 * time.Minute
	factory.studySessionID = func() (learning.ID, error) {
		nextID++
		return learning.NewID("session.persisted." + strconv.Itoa(nextID))
	}
	store, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	setup := completeIntegratedSetupWithoutDiagnostic(t, ctx, store.Setup())
	if setup.Instance == nil {
		t.Fatal("completed setup has no curriculum instance")
	}
	started, err := store.StudySessions().Start(ctx, setup.Instance.GoalID, setup.Instance.ID)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	current = current.Add(5 * time.Minute)
	if _, err := store.StudySessions().RecordActivity(ctx); err != nil {
		t.Fatalf("RecordActivity() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	current = current.Add(10 * time.Minute)
	reopened, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	resumable, err := reopened.StudySessions().Recover(ctx)
	if err != nil || resumable.ID != started.ID || resumable.Status != learning.StudySessionActive {
		t.Fatalf("recent recovery = (%+v, %v)", resumable, err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}

	current = current.Add(10 * time.Minute)
	reopened, err = factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.StudySessions().Recover(ctx)
	if err != nil || recovered.Status != learning.StudySessionRecovered || recovered.ActiveDuration != 20*time.Minute {
		t.Fatalf("stale recovery = (%+v, %v)", recovered, err)
	}
	replacement, err := reopened.StudySessions().Start(ctx, setup.Instance.GoalID, setup.Instance.ID)
	if err != nil || replacement.ID == started.ID {
		t.Fatalf("replacement session = (%+v, %v)", replacement, err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err = factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	active, err := reopened.StudySessions().Current(ctx)
	if err != nil || active.ID != replacement.ID {
		t.Fatalf("persisted replacement = (%+v, %v)", active, err)
	}
}

func TestDevelopmentSetupResetPreservesProfileGoalAndFoundationData(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatal(err)
	}
	factory := NewFactory("dev")
	store, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	completed := completeIntegratedSetupWithoutDiagnostic(t, ctx, store.Setup())
	goalsBefore, err := store.Goals().Show(ctx)
	if err != nil || len(goalsBefore) != 1 {
		t.Fatalf("goals before reset = (%+v, %v)", goalsBefore, err)
	}

	reset, err := store.Setup().ResetDevelopment(ctx)
	if err != nil || reset.Setup.Status != learning.SetupAwaitingOnboarding {
		t.Fatalf("reset setup = (%+v, %v)", reset, err)
	}
	student, err := store.Profiles().Show(ctx)
	if err != nil || student.Profile.DisplayName != "Ada" {
		t.Fatalf("profile after reset = (%+v, %v)", student, err)
	}
	goalsAfter, err := store.Goals().Show(ctx)
	if err != nil || len(goalsAfter) != len(goalsBefore) || goalsAfter[0].ID != goalsBefore[0].ID {
		t.Fatalf("goals after reset = (%+v, %v)", goalsAfter, err)
	}
	if _, err := store.CurriculumInstances().Get(ctx, *completed.Setup.CurriculumInstanceID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("setup curriculum after reset error = %v, want not found", err)
	}
}

func completeIntegratedSetupWithoutDiagnostic(t *testing.T, ctx context.Context, setup application.LearnerSetupService) application.LearnerSetupView {
	t.Helper()
	if _, err := setup.Start(ctx); err != nil {
		t.Fatal(err)
	}
	for _, answer := range []string{"Ada", "Understand ratios", "Mathematics", "Solve ratio problems", "beginner", "beginner", "30", "5", "practice", "0.80", "no", ""} {
		if _, err := setup.SubmitOnboarding(ctx, answer); err != nil {
			t.Fatalf("submit onboarding answer %q: %v", answer, err)
		}
	}
	view, err := setup.Confirm(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return view
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
