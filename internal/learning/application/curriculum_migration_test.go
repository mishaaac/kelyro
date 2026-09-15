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

func TestCurriculumInstanceMigrationPreservesStableStateAndArchivesSource(t *testing.T) {
	t.Parallel()
	ctx, store, profiles, goal := migrationFixture(t)
	oldCurriculum := instanceTestCurriculum(t, "1.0.0")
	create := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return time.Date(2026, 9, 14, 11, 0, 0, 0, time.UTC) }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.migration.old"), nil }))
	source, err := create.Create(ctx, goal.ID, oldCurriculum, learning.CurriculumSourcePack)
	if err != nil {
		t.Fatal(err)
	}
	state, err := create.State(ctx, source.ID, testID(t, "concept.a"))
	if err != nil {
		t.Fatal(err)
	}
	seen, _ := learning.NewTimestamp(time.Date(2026, 9, 14, 11, 15, 0, 0, time.UTC))
	state.Exposure = learning.ExposureLearning
	state.Mastery = testScore(t, .67)
	state.FirstSeenAt, state.LastSeenAt, state.UpdatedAt = &seen, &seen, seen
	if err := create.SaveState(ctx, state); err != nil {
		t.Fatal(err)
	}
	if _, err := create.State(ctx, source.ID, testID(t, "concept.b")); err != nil {
		t.Fatal(err)
	}
	newCurriculum := migrationTargetCurriculum(t, oldCurriculum)
	migrate := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return time.Date(2026, 9, 14, 13, 0, 0, 0, time.UTC) }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.migration.new"), nil }))
	impact, err := migrate.Migrate(ctx, application.CurriculumInstanceMigrationRequest{
		PlanID: "migration.plan", BackupID: "backup.pack-upgrade", OldCurriculum: oldCurriculum.Reference, NewCurriculum: newCurriculum,
		PreserveConceptIDs: []learning.ID{testID(t, "concept.a")}, InitializeUnknownConceptIDs: []learning.ID{testID(t, "concept.c")},
		RecalculateUnlockEligibility: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if impact.SourceInstancesArchived != 1 || impact.TargetInstancesCreated != 1 || impact.ConceptStatesPreserved != 1 ||
		impact.UnknownConceptStatesCreated != 1 || !impact.RecalculatedUnlockEligibility || impact.PolicyVersion != application.CurriculumInstanceMigrationPolicyV1 {
		t.Fatalf("migration impact = %+v", impact)
	}
	archived, err := migrate.Get(ctx, source.ID)
	if err != nil || archived.Status != learning.CurriculumInstanceArchived {
		t.Fatalf("source instance = %+v, %v", archived, err)
	}
	instances, err := migrate.List(ctx)
	if err != nil || len(instances) != 2 {
		t.Fatalf("instances = %+v, %v", instances, err)
	}
	targetID := testID(t, "instance.migration.new")
	preserved, err := migrate.State(ctx, targetID, testID(t, "concept.a"))
	if err != nil || preserved.Exposure != learning.ExposureLearning || preserved.Mastery.Value() != .67 || preserved.FirstSeenAt == nil || *preserved.FirstSeenAt != seen {
		t.Fatalf("preserved state = %+v, %v", preserved, err)
	}
	unknown, err := migrate.State(ctx, targetID, testID(t, "concept.c"))
	if err != nil || unknown.Exposure != learning.ExposureNotSeen || unknown.Mastery.Value() != 0 {
		t.Fatalf("unknown state = %+v, %v", unknown, err)
	}
	historical, err := migrate.State(ctx, source.ID, testID(t, "concept.b"))
	if err != nil || historical.Exposure != learning.ExposureNotSeen {
		t.Fatalf("historical source state = %+v, %v", historical, err)
	}
}

func TestCurriculumInstanceMigrationRequiresBackupAndRollsBackAtomically(t *testing.T) {
	t.Parallel()
	ctx, store, profiles, goal := migrationFixture(t)
	oldCurriculum := instanceTestCurriculum(t, "1.0.0")
	create := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.rollback.old"), nil }))
	if _, err := create.Create(ctx, goal.ID, oldCurriculum, learning.CurriculumSourcePack); err != nil {
		t.Fatal(err)
	}
	newCurriculum := migrationTargetCurriculum(t, oldCurriculum)
	request := application.CurriculumInstanceMigrationRequest{
		PlanID: "migration.rollback", OldCurriculum: oldCurriculum.Reference, NewCurriculum: newCurriculum,
		PreserveConceptIDs: []learning.ID{testID(t, "concept.a")}, InitializeUnknownConceptIDs: []learning.ID{testID(t, "concept.c")},
	}
	if _, err := create.Migrate(ctx, request); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("migration without backup error = %v", err)
	}
	request.BackupID = "backup.rollback"
	failing := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return learning.ID{}, errors.New("injected id failure") }))
	if _, err := failing.Migrate(ctx, request); err == nil {
		t.Fatal("expected migration failure")
	}
	instances, err := create.List(ctx)
	if err != nil || len(instances) != 1 || instances[0].Status != learning.CurriculumInstanceActive {
		t.Fatalf("rollback instances = %+v, %v", instances, err)
	}
	if _, err := store.Repositories().Curricula.Concept(ctx, newCurriculum.Reference, testID(t, "concept.a")); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("rolled-back curriculum lookup error = %v", err)
	}
}

func migrationFixture(t *testing.T) (context.Context, *memory.Store, application.ProfileService, learning.LearningGoal) {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students))
	goals := application.NewGoalLifecycleService(profiles, store,
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.migration"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Migrate curriculum", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	return ctx, store, profiles, goal
}

func migrationTargetCurriculum(t *testing.T, old learning.Curriculum) learning.Curriculum {
	t.Helper()
	nodes := make([]learning.CurriculumNode, 0, len(old.Nodes))
	var topicID learning.ID
	for _, node := range old.Nodes {
		if node.Type == learning.CurriculumNodeTopic {
			topicID = node.ID
		}
		if node.ID != testID(t, "concept.b") {
			nodes = append(nodes, node)
		}
	}
	conceptID := testID(t, "concept.c")
	nodes = append(nodes, learning.CurriculumNode{
		ID: conceptID, Type: learning.CurriculumNodeConcept, ParentID: &topicID, Title: "C", Description: "C.", Order: 1,
		Status: learning.CurriculumStatusMetadata{State: learning.CurriculumNodeActive}, Version: "2.0.0",
		Concept: &learning.ConceptDefinition{Objectives: []string{"Understand C"}, Difficulty: learning.ConceptDifficultyIntermediate,
			EstimatedEffortMinutes: 15, AssessmentExpectations: []string{"Explain C"}},
	})
	curriculum, err := learning.NewCurriculum(learning.CurriculumContractVersion,
		learning.CurriculumRef{ID: old.Reference.ID, Version: "2.0.0"}, old.Title, old.Description, nodes)
	if err != nil {
		t.Fatal(err)
	}
	return curriculum
}
