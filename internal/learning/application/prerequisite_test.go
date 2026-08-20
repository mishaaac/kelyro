package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestPrerequisiteServiceUsesOneStateSnapshotAndResolvedMasteryPolicy(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	clock := masteryPolicyClock()
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students), application.WithProfileClock(clock))
	mastery := application.NewMasteryPolicyService(profiles, repositories.Mastery, application.WithMasteryPolicyClock(clock))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}

	graph := prerequisiteApplicationGraph(t)
	state := learning.ConceptState{
		StudentID: student.ID, ConceptID: testID(t, "concept.a"),
		Exposure: learning.ExposureLearning, Mastery: testScore(t, .80),
		IntroducedAt: timestampPointer(testTimestamp(t, 10)), UpdatedAt: testTimestamp(t, 10),
	}
	if err := repositories.Concepts.Save(ctx, state); err != nil {
		t.Fatal(err)
	}
	counting := &countingConceptStateRepository{ConceptStateRepository: repositories.Concepts}
	service := application.NewPrerequisiteService(graph, profiles, mastery, counting)

	decision, err := service.EvaluateIntroduction(ctx, testID(t, "concept.b"), nil)
	if err != nil || !decision.CanIntroduce || decision.MasteryPolicy.Source != learning.MasterySourceStudentDefault {
		t.Fatalf("default EvaluateIntroduction() = (%+v, %v)", decision, err)
	}
	if counting.listCalls != 1 {
		t.Fatalf("concept state list calls = %d, want 1", counting.listCalls)
	}

	pack, err := learning.NewPackMasteryOverride(.85, .80, .90)
	if err != nil {
		t.Fatal(err)
	}
	decision, err = service.EvaluateIntroduction(ctx, testID(t, "concept.b"), &pack)
	if err != nil || decision.CanIntroduce || decision.MasteryPolicy.Source != learning.MasterySourcePackOverride {
		t.Fatalf("pack EvaluateIntroduction() = (%+v, %v)", decision, err)
	}
	if len(decision.Checks) != 1 || decision.Checks[0].Reason != learning.PrerequisiteBelowMastery {
		t.Fatalf("pack checks = %+v", decision.Checks)
	}
	if counting.listCalls != 2 {
		t.Fatalf("concept state list calls after second evaluation = %d, want 2", counting.listCalls)
	}
}

func TestPrerequisiteServiceClassifiesInvalidConceptAndMissingDependencies(t *testing.T) {
	t.Parallel()

	store := memory.New()
	repositories := store.Repositories()
	clock := masteryPolicyClock()
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students), application.WithProfileClock(clock))
	mastery := application.NewMasteryPolicyService(profiles, repositories.Mastery, application.WithMasteryPolicyClock(clock))
	service := application.NewPrerequisiteService(prerequisiteApplicationGraph(t), profiles, mastery, repositories.Concepts)

	_, err := service.EvaluateIntroduction(context.Background(), testID(t, "concept.unknown"), nil)
	if !errors.Is(err, application.ErrInvalidState) || !errors.Is(err, learning.ErrUnknownCurriculumConcept) {
		t.Fatalf("unknown concept error = %v", err)
	}

	unavailable := application.NewPrerequisiteService(nil, profiles, mastery, repositories.Concepts)
	if _, err := unavailable.EvaluateIntroduction(context.Background(), testID(t, "concept.b"), nil); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("missing graph error = %v", err)
	}
}

type countingConceptStateRepository struct {
	application.ConceptStateRepository
	listCalls int
}

func (repository *countingConceptStateRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.ConceptState, error) {
	repository.listCalls++
	return repository.ConceptStateRepository.ListByStudent(ctx, studentID)
}

func prerequisiteApplicationGraph(t *testing.T) *learning.KnowledgeGraph {
	t.Helper()
	phaseID := testID(t, "phase.graph")
	moduleID := testID(t, "module.graph")
	lessonID := testID(t, "lesson.graph")
	topicID := testID(t, "topic.graph")
	conceptAID := testID(t, "concept.a")
	conceptBID := testID(t, "concept.b")
	status := learning.CurriculumStatusMetadata{State: learning.CurriculumNodeActive}
	conceptDefinition := func(objective string) *learning.ConceptDefinition {
		return &learning.ConceptDefinition{
			Objectives: []string{objective}, Difficulty: learning.ConceptDifficultyFoundational,
			EstimatedEffortMinutes: 10, AssessmentExpectations: []string{"Explain the concept"},
		}
	}
	conceptB := conceptDefinition("Apply B")
	conceptB.Prerequisites = []learning.ConceptPrerequisite{{ConceptID: conceptAID, Requirement: learning.PrerequisiteMastered}}
	curriculum, err := learning.NewCurriculum(
		learning.CurriculumContractVersion,
		learning.CurriculumRef{ID: testID(t, "fixture.application-graph"), Version: "1.0.0"},
		"Application graph",
		"Application prerequisite fixture.",
		[]learning.CurriculumNode{
			{ID: phaseID, Type: learning.CurriculumNodePhase, Title: "Phase", Description: "Phase.", Order: 0, Status: status, Version: "1.0.0"},
			{ID: moduleID, Type: learning.CurriculumNodeModule, ParentID: &phaseID, Title: "Module", Description: "Module.", Order: 0, Status: status, Version: "1.0.0"},
			{ID: lessonID, Type: learning.CurriculumNodeLesson, ParentID: &moduleID, Title: "Lesson", Description: "Lesson.", Order: 0, Status: status, Version: "1.0.0"},
			{ID: topicID, Type: learning.CurriculumNodeTopic, ParentID: &lessonID, Title: "Topic", Description: "Topic.", Order: 0, Status: status, Version: "1.0.0"},
			{ID: conceptAID, Type: learning.CurriculumNodeConcept, ParentID: &topicID, Title: "A", Description: "A.", Order: 0, Status: status, Version: "1.0.0", Concept: conceptDefinition("Understand A")},
			{ID: conceptBID, Type: learning.CurriculumNodeConcept, ParentID: &topicID, Title: "B", Description: "B.", Order: 1, Status: status, Version: "1.0.0", Concept: conceptB},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := learning.NewKnowledgeGraph(curriculum)
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func timestampPointer(timestamp learning.Timestamp) *learning.Timestamp {
	return &timestamp
}
