package learning

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestKnowledgeGraphChainOperations(t *testing.T) {
	t.Parallel()

	curriculum := graphCurriculum(t, []graphConceptFixture{
		{id: "concept.a", title: "A"},
		{id: "concept.b", title: "B", prerequisites: []graphPrerequisiteFixture{{"concept.a", PrerequisiteMastered}}},
		{id: "concept.c", title: "C", prerequisites: []graphPrerequisiteFixture{{"concept.b", PrerequisiteIntroduced}}},
	})
	graph, err := NewKnowledgeGraph(curriculum)
	if err != nil {
		t.Fatal(err)
	}

	conceptA := curriculumID(t, "concept.a")
	conceptB := curriculumID(t, "concept.b")
	conceptC := curriculumID(t, "concept.c")
	prerequisites, err := graph.GetPrerequisites(conceptC)
	if err != nil || len(prerequisites) != 1 || prerequisites[0].ConceptID != conceptB || prerequisites[0].Requirement != PrerequisiteIntroduced {
		t.Fatalf("GetPrerequisites(C) = (%+v, %v)", prerequisites, err)
	}
	dependents, err := graph.GetDependents(conceptA)
	if err != nil || !reflect.DeepEqual(dependents, []ID{conceptB}) {
		t.Fatalf("GetDependents(A) = (%+v, %v)", dependents, err)
	}
	ancestors, err := graph.Ancestors(conceptC)
	if err != nil || !reflect.DeepEqual(ancestors, []ID{conceptA, conceptB}) {
		t.Fatalf("Ancestors(C) = (%+v, %v)", ancestors, err)
	}
	if got := graph.TopologicalOrder(); !reflect.DeepEqual(got, []ID{conceptA, conceptB, conceptC}) {
		t.Fatalf("TopologicalOrder() = %+v", got)
	}
	empty, err := NewStudentStateSnapshot(nil)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := graph.EvaluateIntroduction(conceptA, empty, graphPolicy(t, .85))
	if err != nil || !decision.CanIntroduce || decision.PolicyVersion != PrerequisitePolicyVersion || !strings.Contains(decision.Explanation(), "No prerequisites are required") {
		t.Fatalf("root EvaluateIntroduction() = (%+v, %v)", decision, err)
	}
}

func TestKnowledgeGraphDiamondIsDeterministic(t *testing.T) {
	t.Parallel()

	graph, err := NewKnowledgeGraph(graphCurriculum(t, []graphConceptFixture{
		{id: "concept.d", title: "D", prerequisites: []graphPrerequisiteFixture{{"concept.c", PrerequisiteMastered}, {"concept.b", PrerequisiteMastered}}},
		{id: "concept.c", title: "C", prerequisites: []graphPrerequisiteFixture{{"concept.a", PrerequisiteIntroduced}}},
		{id: "concept.b", title: "B", prerequisites: []graphPrerequisiteFixture{{"concept.a", PrerequisiteMastered}}},
		{id: "concept.a", title: "A"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	a := curriculumID(t, "concept.a")
	b := curriculumID(t, "concept.b")
	c := curriculumID(t, "concept.c")
	d := curriculumID(t, "concept.d")
	if got := graph.TopologicalOrder(); !reflect.DeepEqual(got, []ID{a, b, c, d}) {
		t.Fatalf("diamond TopologicalOrder() = %+v", got)
	}
	if got, _ := graph.GetDependents(a); !reflect.DeepEqual(got, []ID{b, c}) {
		t.Fatalf("diamond GetDependents(A) = %+v", got)
	}
	if got, _ := graph.Ancestors(d); !reflect.DeepEqual(got, []ID{a, b, c}) {
		t.Fatalf("diamond Ancestors(D) = %+v", got)
	}
	prerequisites, _ := graph.GetPrerequisites(d)
	if got := []ID{prerequisites[0].ConceptID, prerequisites[1].ConceptID}; !reflect.DeepEqual(got, []ID{b, c}) {
		t.Fatalf("diamond direct prerequisites = %+v", got)
	}
}

func TestPrerequisiteEngineSeparatesIntroducedAndMasteredRequirements(t *testing.T) {
	t.Parallel()

	graph, err := NewKnowledgeGraph(graphCurriculum(t, []graphConceptFixture{
		{id: "concept.exposure", title: "Exposure"},
		{id: "concept.mastery", title: "Mastery"},
		{id: "concept.missing", title: "Missing"},
		{id: "concept.target", title: "Target", prerequisites: []graphPrerequisiteFixture{
			{"concept.mastery", PrerequisiteMastered},
			{"concept.exposure", PrerequisiteIntroduced},
			{"concept.missing", PrerequisiteMastered},
		}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewStudentStateSnapshot([]ConceptState{
		graphConceptState(t, "concept.exposure", ExposureLearning, .10),
		// Exposure and mastery remain independent: exact calculated mastery
		// satisfies threshold-v1 even when exposure is not_seen.
		graphConceptState(t, "concept.mastery", ExposureNotSeen, .85),
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := graph.EvaluateIntroduction(curriculumID(t, "concept.target"), snapshot, graphPolicy(t, .85))
	if err != nil {
		t.Fatal(err)
	}
	if decision.CanIntroduce {
		t.Fatal("target with missing prerequisite was introducible")
	}
	if len(decision.Checks) != 3 || !decision.Checks[0].Satisfied || !decision.Checks[1].Satisfied || decision.Checks[2].Satisfied {
		t.Fatalf("checks = %+v", decision.Checks)
	}
	missing, err := graph.MissingPrerequisites(curriculumID(t, "concept.target"), snapshot, graphPolicy(t, .85))
	if err != nil || len(missing) != 1 || missing[0].ConceptID != curriculumID(t, "concept.missing") || missing[0].Reason != PrerequisiteMissingState {
		t.Fatalf("MissingPrerequisites() = (%+v, %v)", missing, err)
	}
	explanation := decision.Explanation()
	for _, want := range []string{
		"Target is locked.",
		"✓ Exposure — introduced (learning)",
		"✓ Mastery — 85% (requires 85%)",
		"✗ Missing — no student state (requires 85% mastery)",
	} {
		if !strings.Contains(explanation, want) {
			t.Errorf("Explanation() = %q, want containing %q", explanation, want)
		}
	}
}

func TestPrerequisiteEngineUsesInclusiveMasteryBoundaryAndMissingExposure(t *testing.T) {
	t.Parallel()

	graph, err := NewKnowledgeGraph(graphCurriculum(t, []graphConceptFixture{
		{id: "concept.a", title: "A"},
		{id: "concept.target", title: "Target", prerequisites: []graphPrerequisiteFixture{{"concept.a", PrerequisiteMastered}}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	target := curriculumID(t, "concept.target")
	policy := graphPolicy(t, .85)
	for _, test := range []struct {
		score float64
		want  bool
	}{
		{.849, false},
		{.85, true},
		{.90, true},
	} {
		snapshot, snapshotErr := NewStudentStateSnapshot([]ConceptState{graphConceptState(t, "concept.a", ExposureNotSeen, test.score)})
		if snapshotErr != nil {
			t.Fatal(snapshotErr)
		}
		got, evaluateErr := graph.CanIntroduce(target, snapshot, policy)
		if evaluateErr != nil || got != test.want {
			t.Errorf("CanIntroduce(score=%v) = (%v, %v), want %v", test.score, got, evaluateErr, test.want)
		}
	}

	introducedGraph, err := NewKnowledgeGraph(graphCurriculum(t, []graphConceptFixture{
		{id: "concept.a", title: "A"},
		{id: "concept.target", title: "Target", prerequisites: []graphPrerequisiteFixture{{"concept.a", PrerequisiteIntroduced}}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	notSeen, _ := NewStudentStateSnapshot([]ConceptState{graphConceptState(t, "concept.a", ExposureNotSeen, 1)})
	allowed, err := introducedGraph.CanIntroduce(target, notSeen, policy)
	if err != nil || allowed {
		t.Fatalf("introduced prerequisite accepted not_seen high mastery = (%v, %v)", allowed, err)
	}
}

func TestKnowledgeGraphRejectsCycleAndUnknownConcept(t *testing.T) {
	t.Parallel()

	curriculum := graphCurriculum(t, []graphConceptFixture{
		{id: "concept.a", title: "A"},
		{id: "concept.b", title: "B", prerequisites: []graphPrerequisiteFixture{{"concept.a", PrerequisiteMastered}}},
	})
	conceptB := curriculumID(t, "concept.b")
	for index := range curriculum.Nodes {
		if curriculum.Nodes[index].ID == curriculumID(t, "concept.a") {
			curriculum.Nodes[index].Concept.Prerequisites = []ConceptPrerequisite{{ConceptID: conceptB, Requirement: PrerequisiteMastered}}
		}
	}
	if _, err := NewKnowledgeGraph(curriculum); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("NewKnowledgeGraph(cycle) error = %v", err)
	}

	validGraph, err := NewKnowledgeGraph(graphCurriculum(t, []graphConceptFixture{{id: "concept.a", title: "A"}}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = validGraph.GetPrerequisites(curriculumID(t, "concept.unknown"))
	if !errors.Is(err, ErrUnknownCurriculumConcept) {
		t.Fatalf("unknown concept error = %v", err)
	}
}

func TestKnowledgeGraphHandlesThousandsOfConcepts(t *testing.T) {
	t.Parallel()

	const conceptCount = 3000
	concepts := make([]graphConceptFixture, 0, conceptCount)
	for index := 0; index < conceptCount; index++ {
		concept := graphConceptFixture{id: fmt.Sprintf("concept.large.%04d", index), title: fmt.Sprintf("Concept %d", index)}
		if index > 0 {
			concept.prerequisites = []graphPrerequisiteFixture{{fmt.Sprintf("concept.large.%04d", index-1), PrerequisiteMastered}}
		}
		concepts = append(concepts, concept)
	}
	graph, err := NewKnowledgeGraph(graphCurriculum(t, concepts))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(graph.TopologicalOrder()); got != conceptCount {
		t.Fatalf("large graph topological count = %d", got)
	}
	ancestors, err := graph.Ancestors(curriculumID(t, fmt.Sprintf("concept.large.%04d", conceptCount-1)))
	if err != nil || len(ancestors) != conceptCount-1 {
		t.Fatalf("large graph ancestors = %d, %v", len(ancestors), err)
	}
}

func TestStudentStateSnapshotRejectsMixedOrDuplicateState(t *testing.T) {
	t.Parallel()

	first := graphConceptState(t, "concept.a", ExposureLearning, .5)
	duplicate := first
	if _, err := NewStudentStateSnapshot([]ConceptState{first, duplicate}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate snapshot error = %v", err)
	}
	otherStudent := graphConceptState(t, "concept.b", ExposureLearning, .5)
	otherStudent.StudentID = curriculumID(t, "student.other")
	if _, err := NewStudentStateSnapshot([]ConceptState{first, otherStudent}); err == nil || !strings.Contains(err.Error(), "mixes students") {
		t.Fatalf("mixed snapshot error = %v", err)
	}
}

type graphConceptFixture struct {
	id            string
	title         string
	prerequisites []graphPrerequisiteFixture
}

type graphPrerequisiteFixture struct {
	id          string
	requirement PrerequisiteRequirement
}

func graphCurriculum(t *testing.T, concepts []graphConceptFixture) Curriculum {
	t.Helper()
	phaseID := curriculumID(t, "phase.graph")
	moduleID := curriculumID(t, "module.graph")
	lessonID := curriculumID(t, "lesson.graph")
	topicID := curriculumID(t, "topic.graph")
	nodes := []CurriculumNode{
		{ID: phaseID, Type: CurriculumNodePhase, Title: "Graph", Description: "Graph fixture phase.", Order: 0, Status: activeCurriculumStatus(), Version: "1.0.0"},
		{ID: moduleID, Type: CurriculumNodeModule, ParentID: &phaseID, Title: "Graph", Description: "Graph fixture module.", Order: 0, Status: activeCurriculumStatus(), Version: "1.0.0"},
		{ID: lessonID, Type: CurriculumNodeLesson, ParentID: &moduleID, Title: "Graph", Description: "Graph fixture lesson.", Order: 0, Status: activeCurriculumStatus(), Version: "1.0.0"},
		{ID: topicID, Type: CurriculumNodeTopic, ParentID: &lessonID, Title: "Graph", Description: "Graph fixture topic.", Order: 0, Status: activeCurriculumStatus(), Version: "1.0.0"},
	}
	for index, fixture := range concepts {
		prerequisites := make([]ConceptPrerequisite, 0, len(fixture.prerequisites))
		for _, prerequisite := range fixture.prerequisites {
			prerequisites = append(prerequisites, ConceptPrerequisite{ConceptID: curriculumID(t, prerequisite.id), Requirement: prerequisite.requirement})
		}
		definition := ConceptDefinition{
			Objectives: []string{"Understand " + fixture.title}, Prerequisites: prerequisites,
			Difficulty: ConceptDifficultyFoundational, EstimatedEffortMinutes: 10,
			AssessmentExpectations: []string{"Explain " + fixture.title},
		}
		nodes = append(nodes, CurriculumNode{
			ID: curriculumID(t, fixture.id), Type: CurriculumNodeConcept, ParentID: &topicID,
			Title: fixture.title, Description: "Graph fixture concept.", Order: index,
			Status: activeCurriculumStatus(), Version: "1.0.0", Concept: &definition,
		})
	}
	curriculum, err := NewCurriculum(
		CurriculumContractVersion,
		CurriculumRef{ID: curriculumID(t, "fixture.graph"), Version: "1.0.0"},
		"Graph fixture",
		"A deterministic prerequisite graph fixture.",
		nodes,
	)
	if err != nil {
		t.Fatalf("NewCurriculum() error = %v", err)
	}
	return curriculum
}

func graphPolicy(t *testing.T, threshold float64) ResolvedMasteryThreshold {
	t.Helper()
	requirement, err := NewMasteryRequirement(threshold)
	if err != nil {
		t.Fatal(err)
	}
	return ResolvedMasteryThreshold{
		Requirement: requirement, Source: MasterySourceStudentDefault,
		PolicyVersion: MasteryThresholdPolicyVersion,
	}
}

func graphConceptState(t *testing.T, concept string, exposure ExposureState, mastery float64) ConceptState {
	t.Helper()
	studentID := curriculumID(t, "student.graph")
	conceptID := curriculumID(t, concept)
	score, err := NewMasteryScore(mastery)
	if err != nil {
		t.Fatal(err)
	}
	timestamp, err := NewTimestamp(time.Date(2026, time.August, 19, 18, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	state := ConceptState{StudentID: studentID, ConceptID: conceptID, Exposure: exposure, Mastery: score, UpdatedAt: timestamp}
	if exposure != ExposureNotSeen {
		state.IntroducedAt = &timestamp
	}
	if err := state.Validate(); err != nil {
		t.Fatal(err)
	}
	return state
}
