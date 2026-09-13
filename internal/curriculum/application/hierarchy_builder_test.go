package application

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCurriculumHierarchyBuilderV1BuildsStableUXProjection(t *testing.T) {
	t.Parallel()
	root := graphConcept(t, "concept.foundation", true)
	root.Difficulty = curriculum.DifficultyIntroductory
	service := graphConcept(t, "concept.service", false)
	service.Difficulty = curriculum.DifficultyIntermediate
	deploy := graphConcept(t, "concept.deploy", false)
	deploy.Difficulty = curriculum.DifficultyAdvanced
	concepts := []curriculum.Concept{deploy, root, service}
	edges := []curriculum.Prerequisite{
		graphPrerequisite(service, root, curriculum.PrerequisiteHard),
		graphPrerequisite(deploy, service, curriculum.PrerequisiteHard),
	}
	graph := compileAuditGraph(t, concepts, edges)
	areaCore := curriculumID(t, "area.core")
	areaOps := curriculumID(t, "area.ops")
	matrix := curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: curriculumID(t, "goal.backend"), Competencies: []curriculum.Competency{
		{ID: curriculumID(t, "competency.service"), AreaID: areaCore, Area: "Core", OutcomeID: curriculumID(t, "outcome.build"), ExpectedLevel: curriculum.CompetencyApply, ConceptRefs: []curriculum.ConceptID{root.ID, service.ID}},
		{ID: curriculumID(t, "competency.deploy"), AreaID: areaOps, Area: "Operations", OutcomeID: curriculumID(t, "outcome.operate"), ExpectedLevel: curriculum.CompetencyOperate, ConceptRefs: []curriculum.ConceptID{deploy.ID}},
	}}
	request := CurriculumHierarchyBuildRequest{Competencies: matrix, Concepts: concepts, Graph: graph, PracticeContext: []curriculum.PracticeContextAssignment{
		{ConceptID: service.ID, Context: "Build a service"},
		{ConceptID: deploy.ID, Context: "Operate a service"},
	}}

	result, err := NewCurriculumHierarchyBuilderV1().Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.AlgorithmVersion != curriculum.CurriculumHierarchyBuilderVersionV1 || len(result.Phases) != 3 || len(result.Modules) != 3 || len(result.Lessons) != 3 || len(result.Topics) != 3 {
		t.Fatalf("hierarchy = %+v", result)
	}
	assertHierarchyConcepts(t, result, []curriculum.ConceptID{root.ID, service.ID, deploy.ID})

	// The service->foundation edge crosses hierarchy modules/phases and remains
	// exclusively owned by the graph artifact.
	foundCrossModule := false
	for _, prerequisite := range graph.Prerequisites {
		foundCrossModule = foundCrossModule || (prerequisite.ConceptID == service.ID && prerequisite.RequiredConceptID == root.ID)
	}
	if !foundCrossModule {
		t.Fatalf("cross-module prerequisite changed: %+v", graph.Prerequisites)
	}

	reordered := request
	reordered.Concepts = []curriculum.Concept{service, deploy, root}
	reordered.Competencies.Competencies = []curriculum.Competency{matrix.Competencies[1], matrix.Competencies[0]}
	reordered.PracticeContext = []curriculum.PracticeContextAssignment{request.PracticeContext[1], request.PracticeContext[0]}
	repeated, err := NewCurriculumHierarchyBuilderV1().Build(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered hierarchy differs: %v\nfirst=%+v\nsecond=%+v", err, result, repeated)
	}
}

func TestCurriculumHierarchyBuilderV1PreservesTwoThousandConcepts(t *testing.T) {
	t.Parallel()
	const count = 2000
	concepts := make([]curriculum.Concept, 0, count)
	refs := make([]curriculum.ConceptID, 0, count)
	for index := 0; index < count; index++ {
		concept := graphConcept(t, fmt.Sprintf("concept.scale.%04d", index), index == 0)
		concept.Difficulty = curriculum.DifficultyFoundational
		concepts = append(concepts, concept)
		refs = append(refs, concept.ID)
	}
	graph := compileAuditGraph(t, concepts, nil)
	matrix := curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: curriculumID(t, "goal.scale"), Competencies: []curriculum.Competency{{
		ID: curriculumID(t, "competency.scale"), AreaID: curriculumID(t, "area.scale"), Area: "Scale",
		OutcomeID: curriculumID(t, "outcome.scale"), ExpectedLevel: curriculum.CompetencyUnderstand, ConceptRefs: refs,
	}}}

	result, err := NewCurriculumHierarchyBuilderV1().Build(context.Background(), CurriculumHierarchyBuildRequest{Competencies: matrix, Concepts: concepts, Graph: graph})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.Topics) != 1 || len(result.Topics[0].ConceptIDs) != count {
		t.Fatalf("hierarchy imposed a concept limit: topics=%d concepts=%d", len(result.Topics), len(result.Topics[0].ConceptIDs))
	}
	assertHierarchyConcepts(t, result, refs)
}

func assertHierarchyConcepts(t *testing.T, hierarchy curriculum.CurriculumHierarchy, expected []curriculum.ConceptID) {
	t.Helper()
	seen := make(map[curriculum.ConceptID]int, len(expected))
	for _, topic := range hierarchy.Topics {
		for _, id := range topic.ConceptIDs {
			seen[id]++
		}
	}
	for _, id := range expected {
		if seen[id] != 1 {
			t.Fatalf("concept %q appears %d times", id, seen[id])
		}
	}
}
