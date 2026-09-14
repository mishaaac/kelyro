package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestBeginnerSimulatorV1ReportsVocabularyToolAndAssumptionJumps(t *testing.T) {
	t.Parallel()
	request := brokenBeginnerFixture(t)

	result, err := NewBeginnerSimulatorV1().Simulate(context.Background(), request)
	if err != nil {
		t.Fatalf("Simulate() error = %v", err)
	}
	if result.Passed || len(result.Steps) != 3 || len(result.Gaps) != 3 || result.AlgorithmVersion != curriculum.BeginnerSimulationVersionV1 {
		t.Fatalf("simulation = %+v", result)
	}
	actualKinds := make([]curriculum.BeginnerGapKind, len(result.Gaps))
	for index, gap := range result.Gaps {
		actualKinds[index] = gap.Kind
		if gap.ConceptID.String() != "concept.02-use" {
			t.Fatalf("gap = %+v", gap)
		}
	}
	// Gaps follow deterministic check order: vocabulary, tool, assumption.
	wantKinds := []curriculum.BeginnerGapKind{curriculum.BeginnerGapVocabulary, curriculum.BeginnerGapTool, curriculum.BeginnerGapAssumption}
	if !reflect.DeepEqual(actualKinds, wantKinds) {
		t.Fatalf("gap kinds = %v, want %v", actualKinds, wantKinds)
	}

	reordered := request
	reordered.Concepts = []curriculum.Concept{request.Concepts[2], request.Concepts[0], request.Concepts[1]}
	reordered.Tools = []curriculum.ToolRequirement{request.Tools[0]}
	repeated, err := NewBeginnerSimulatorV1().Simulate(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered simulation differs: %v\nfirst=%+v\nsecond=%+v", err, result, repeated)
	}
}

func TestBeginnerSimulatorV1PassesWhenNeedsAreIntroducedFirst(t *testing.T) {
	t.Parallel()
	request := brokenBeginnerFixture(t)
	rootID := request.Graph.TopologicalOrder[0]
	request.Vocabulary = buildAuditVocabulary(t, request.Concepts, []curriculum.VocabularyDefinition{{
		Term: "socket", CanonicalConceptID: rootID, IntroducedBy: rootID, Scope: "networking",
	}}, []curriculum.VocabularyUse{{Term: "socket", UsedAt: request.Graph.TopologicalOrder[1]}}, nil)
	request.Tools[0].IntroducedAt = &rootID
	request.Assumptions[0].SatisfiedBy = &rootID

	result, err := NewBeginnerSimulatorV1().Simulate(context.Background(), request)
	if err != nil || !result.Passed || len(result.Gaps) != 0 || len(result.IntroducedVocabulary) != 1 || len(result.IntroducedToolIDs) != 1 || len(result.ResolvedAssumptionIDs) != 1 {
		t.Fatalf("simulation = %+v / %v", result, err)
	}
}

func brokenBeginnerFixture(t *testing.T) BeginnerSimulationRequest {
	t.Helper()
	root := graphConcept(t, "concept.01-foundation", true)
	use := graphConcept(t, "concept.02-use", false)
	late := graphConcept(t, "concept.03-late", false)
	concepts := []curriculum.Concept{root, use, late}
	graph := compileAuditGraph(t, concepts, []curriculum.Prerequisite{
		graphPrerequisite(use, root, curriculum.PrerequisiteHard),
		graphPrerequisite(late, use, curriculum.PrerequisiteHard),
	})
	matrix := curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: curriculumID(t, "goal.beginner"), Competencies: []curriculum.Competency{{
		ID: curriculumID(t, "competency.beginner"), AreaID: curriculumID(t, "area.beginner"), Area: "Beginner",
		OutcomeID: curriculumID(t, "outcome.beginner"), ExpectedLevel: curriculum.CompetencyApply,
		ConceptRefs: []curriculum.ConceptID{root.ID, use.ID, late.ID},
	}}}
	hierarchy, err := NewCurriculumHierarchyBuilderV1().Build(context.Background(), CurriculumHierarchyBuildRequest{Competencies: matrix, Concepts: concepts, Graph: graph})
	if err != nil {
		t.Fatal(err)
	}
	vocabulary := buildAuditVocabulary(t, concepts, []curriculum.VocabularyDefinition{{
		Term: "socket", CanonicalConceptID: late.ID, IntroducedBy: late.ID, Scope: "networking",
	}}, []curriculum.VocabularyUse{{Term: "socket", UsedAt: use.ID}}, nil)
	toolID := curriculumID(t, "tool.debugger")
	assumptionID := curriculumID(t, "assumption.process")
	return BeginnerSimulationRequest{
		Concepts: concepts, Graph: graph, Hierarchy: hierarchy, Vocabulary: vocabulary,
		Tools:       []curriculum.ToolRequirement{{ID: toolID, Purpose: "Inspect a process.", Level: curriculum.ToolRequired, IntroducedAt: &late.ID}},
		ToolUses:    []curriculum.BeginnerToolUse{{ToolID: toolID, UsedAt: use.ID}},
		Assumptions: []curriculum.BeginnerAssumption{{ID: assumptionID, RequiredAt: use.ID, SatisfiedBy: &late.ID, Reason: "Process basics are assumed before introduction."}},
	}
}
