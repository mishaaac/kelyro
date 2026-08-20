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

func TestPrerequisiteServiceUsesOneInstanceStateSnapshotAndResolvedMasteryPolicy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fixture := newPrerequisiteServiceFixture(t, ctx)
	seen, _ := learning.NewTimestamp(time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC))
	state := learning.InstanceConceptState{
		CurriculumInstanceID: fixture.instance.ID,
		StudentID:            fixture.instance.StudentID,
		ConceptID:            testID(t, "concept.a"),
		Exposure:             learning.ExposureLearning,
		Mastery:              testScore(t, .80),
		FirstSeenAt:          &seen,
		LastSeenAt:           &seen,
		UpdatedAt:            seen,
	}
	if err := fixture.repositories.InstanceConceptStates.Save(ctx, state); err != nil {
		t.Fatal(err)
	}
	counting := &countingInstanceConceptStateRepository{InstanceConceptStateRepository: fixture.repositories.InstanceConceptStates}
	service := application.NewPrerequisiteService(fixture.graph, fixture.profiles, fixture.mastery, fixture.repositories.CurriculumInstances, counting)

	decision, err := service.EvaluateIntroduction(ctx, fixture.instance.ID, testID(t, "concept.b"), nil)
	if err != nil || !decision.CanIntroduce || decision.MasteryPolicy.Source != learning.MasterySourceStudentDefault {
		t.Fatalf("default EvaluateIntroduction() = (%+v, %v)", decision, err)
	}
	if counting.listCalls != 1 {
		t.Fatalf("instance state list calls = %d, want 1", counting.listCalls)
	}

	pack, err := learning.NewPackMasteryOverride(.85, .80, .90)
	if err != nil {
		t.Fatal(err)
	}
	decision, err = service.EvaluateIntroduction(ctx, fixture.instance.ID, testID(t, "concept.b"), &pack)
	if err != nil || decision.CanIntroduce || decision.MasteryPolicy.Source != learning.MasterySourcePackOverride {
		t.Fatalf("pack EvaluateIntroduction() = (%+v, %v)", decision, err)
	}
	if len(decision.Checks) != 1 || decision.Checks[0].Reason != learning.PrerequisiteBelowMastery {
		t.Fatalf("pack checks = %+v", decision.Checks)
	}
	if counting.listCalls != 2 {
		t.Fatalf("instance state list calls after second evaluation = %d, want 2", counting.listCalls)
	}
}

func TestPrerequisiteServiceClassifiesInvalidConceptAndMissingDependencies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fixture := newPrerequisiteServiceFixture(t, ctx)
	service := application.NewPrerequisiteService(fixture.graph, fixture.profiles, fixture.mastery,
		fixture.repositories.CurriculumInstances, fixture.repositories.InstanceConceptStates)

	_, err := service.EvaluateIntroduction(ctx, fixture.instance.ID, testID(t, "concept.unknown"), nil)
	if !errors.Is(err, application.ErrInvalidState) || !errors.Is(err, learning.ErrUnknownCurriculumConcept) {
		t.Fatalf("unknown concept error = %v", err)
	}
	otherCurriculum := instanceTestCurriculum(t, "2.0.0")
	if err := fixture.repositories.Definitions.Install(ctx, otherCurriculum); err != nil {
		t.Fatal(err)
	}
	otherInstance, err := learning.NewCurriculumInstance(
		testID(t, "instance.other-version"), fixture.instance.StudentID, fixture.instance.GoalID,
		otherCurriculum.Reference, learning.CurriculumSourceFixture, fixture.instance.CreatedAt,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.repositories.CurriculumInstances.Create(ctx, otherInstance); err != nil {
		t.Fatal(err)
	}
	if _, err := service.EvaluateIntroduction(ctx, otherInstance.ID, testID(t, "concept.b"), nil); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("mismatched graph version error = %v", err)
	}

	unavailable := application.NewPrerequisiteService(nil, fixture.profiles, fixture.mastery,
		fixture.repositories.CurriculumInstances, fixture.repositories.InstanceConceptStates)
	if _, err := unavailable.EvaluateIntroduction(ctx, fixture.instance.ID, testID(t, "concept.b"), nil); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("missing graph error = %v", err)
	}
}

type countingInstanceConceptStateRepository struct {
	application.InstanceConceptStateRepository
	listCalls int
}

func (repository *countingInstanceConceptStateRepository) ListByInstance(ctx context.Context, instanceID learning.ID) ([]learning.InstanceConceptState, error) {
	repository.listCalls++
	return repository.InstanceConceptStateRepository.ListByInstance(ctx, instanceID)
}

type prerequisiteServiceFixture struct {
	repositories application.Repositories
	profiles     application.ProfileService
	mastery      application.MasteryPolicyService
	graph        *learning.KnowledgeGraph
	instance     learning.CurriculumInstance
}

func newPrerequisiteServiceFixture(t *testing.T, ctx context.Context) prerequisiteServiceFixture {
	t.Helper()
	store := memory.New()
	repositories := store.Repositories()
	clock := masteryPolicyClock()
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students), application.WithProfileClock(clock))
	mastery := application.NewMasteryPolicyService(profiles, repositories.Mastery, application.WithMasteryPolicyClock(clock))
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(clock),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.prerequisites"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Prerequisites", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	curriculum := instanceTestCurriculum(t, "1.0.0")
	graph, err := learning.NewKnowledgeGraph(curriculum)
	if err != nil {
		t.Fatal(err)
	}
	instances := application.NewCurriculumInstanceService(profiles, store, application.WithCurriculumInstanceClock(clock),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.prerequisites"), nil }))
	instance, err := instances.Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	return prerequisiteServiceFixture{repositories: repositories, profiles: profiles, mastery: mastery, graph: graph, instance: instance}
}
