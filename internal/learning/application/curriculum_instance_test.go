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

func TestCurriculumInstanceCreatesLazilyAndProtectsDefinitionIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC) }))
	goals := application.NewGoalLifecycleService(profiles, store,
		application.WithGoalClock(func() time.Time { return time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC) }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.curriculum"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Learn the fixture", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	ids := []learning.ID{testID(t, "instance.v1"), testID(t, "instance.duplicate"), testID(t, "instance.changed")}
	service := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return time.Date(2026, 8, 19, 11, 0, 0, 0, time.UTC) }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		}))
	curriculum := instanceTestCurriculum(t, "1.0.0")

	instance, err := service.Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if instance.Curriculum != curriculum.Reference || instance.Status != learning.CurriculumInstanceActive {
		t.Fatalf("Create() = %+v", instance)
	}
	states, err := service.States(ctx, instance.ID)
	if err != nil || len(states) != 0 {
		t.Fatalf("States() before first access = (%+v, %v), want empty", states, err)
	}
	state, err := service.State(ctx, instance.ID, testID(t, "concept.a"))
	if err != nil || state.Exposure != learning.ExposureNotSeen || state.CurriculumInstanceID != instance.ID {
		t.Fatalf("State() = (%+v, %v)", state, err)
	}
	states, err = service.States(ctx, instance.ID)
	if err != nil || len(states) != 1 {
		t.Fatalf("States() after first access = (%+v, %v)", states, err)
	}

	if _, err := service.Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate Create() error = %v, want conflict", err)
	}
	changed := curriculum
	changed.Title = "Mutated definition"
	if _, err := service.Create(ctx, goal.ID, changed, learning.CurriculumSourceFixture); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("changed definition Create() error = %v, want conflict", err)
	}
}

func TestCurriculumInstanceStateIsIsolatedByVersionedInstance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students),
		application.WithProfileClock(func() time.Time { return time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC) }))
	goals := application.NewGoalLifecycleService(profiles, store,
		application.WithGoalClock(func() time.Time { return time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC) }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.versions"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Versioned curriculum", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	ids := []learning.ID{testID(t, "instance.v1"), testID(t, "instance.v2")}
	service := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return time.Date(2026, 8, 19, 11, 0, 0, 0, time.UTC) }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) {
			id := ids[0]
			ids = ids[1:]
			return id, nil
		}))
	first, err := service.Create(ctx, goal.ID, instanceTestCurriculum(t, "1.0.0"), learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, goal.ID, instanceTestCurriculum(t, "2.0.0"), learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	conceptID := testID(t, "concept.a")
	firstState, err := service.State(ctx, first.ID, conceptID)
	if err != nil {
		t.Fatal(err)
	}
	seen, _ := learning.NewTimestamp(time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC))
	updated, _ := learning.NewTimestamp(time.Date(2026, 8, 19, 13, 0, 0, 0, time.UTC))
	firstState.Exposure = learning.ExposureLearning
	firstState.Mastery = testScore(t, .72)
	firstState.FirstSeenAt = &seen
	firstState.LastSeenAt = &seen
	firstState.UpdatedAt = updated
	firstState.ManualFlags = []string{"flag.needs-coaching"}
	if err := service.SaveState(ctx, firstState); err != nil {
		t.Fatal(err)
	}
	secondState, err := service.State(ctx, second.ID, conceptID)
	if err != nil {
		t.Fatal(err)
	}
	if secondState.Exposure != learning.ExposureNotSeen || secondState.Mastery.Value() != 0 || len(secondState.ManualFlags) != 0 {
		t.Fatalf("second instance inherited first state: %+v", secondState)
	}
	loaded, err := service.Get(ctx, first.ID)
	if err != nil || loaded.Curriculum.Version != "1.0.0" {
		t.Fatalf("Get(first) = (%+v, %v)", loaded, err)
	}
	instances, err := service.List(ctx)
	if err != nil || len(instances) != 2 || instances[0].Curriculum.Version != "1.0.0" || instances[1].Curriculum.Version != "2.0.0" {
		t.Fatalf("List() = (%+v, %v)", instances, err)
	}
}

func instanceTestCurriculum(t *testing.T, version string) learning.Curriculum {
	t.Helper()
	phaseID := testID(t, "phase.fixture")
	moduleID := testID(t, "module.fixture")
	lessonID := testID(t, "lesson.fixture")
	topicID := testID(t, "topic.fixture")
	conceptAID := testID(t, "concept.a")
	conceptBID := testID(t, "concept.b")
	status := learning.CurriculumStatusMetadata{State: learning.CurriculumNodeActive}
	conceptA := &learning.ConceptDefinition{
		Objectives: []string{"Understand A"}, Difficulty: learning.ConceptDifficultyFoundational,
		EstimatedEffortMinutes: 10, AssessmentExpectations: []string{"Explain A"},
	}
	conceptB := &learning.ConceptDefinition{
		Objectives: []string{"Apply B"}, Difficulty: learning.ConceptDifficultyIntermediate,
		EstimatedEffortMinutes: 15, AssessmentExpectations: []string{"Apply B correctly"},
		Prerequisites: []learning.ConceptPrerequisite{{ConceptID: conceptAID, Requirement: learning.PrerequisiteMastered}},
	}
	curriculum, err := learning.NewCurriculum(learning.CurriculumContractVersion,
		learning.CurriculumRef{ID: testID(t, "fixture.instance"), Version: version},
		"Instance fixture", "Deterministic fixture for learner curriculum instances.",
		[]learning.CurriculumNode{
			{ID: phaseID, Type: learning.CurriculumNodePhase, Title: "Phase", Description: "Phase.", Status: status, Version: version},
			{ID: moduleID, Type: learning.CurriculumNodeModule, ParentID: &phaseID, Title: "Module", Description: "Module.", Status: status, Version: version},
			{ID: lessonID, Type: learning.CurriculumNodeLesson, ParentID: &moduleID, Title: "Lesson", Description: "Lesson.", Status: status, Version: version},
			{ID: topicID, Type: learning.CurriculumNodeTopic, ParentID: &lessonID, Title: "Topic", Description: "Topic.", Status: status, Version: version},
			{ID: conceptAID, Type: learning.CurriculumNodeConcept, ParentID: &topicID, Title: "A", Description: "A.", Order: 0, Status: status, Version: version, Concept: conceptA},
			{ID: conceptBID, Type: learning.CurriculumNodeConcept, ParentID: &topicID, Title: "B", Description: "B.", Order: 1, Status: status, Version: version, Concept: conceptB},
		})
	if err != nil {
		t.Fatal(err)
	}
	return curriculum
}
