package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestKnowledgeGraphCompilerV1CompilesTopologyComponentsReachabilityAndCriticalPath(t *testing.T) {
	t.Parallel()
	root := graphConcept(t, "concept.root", true)
	middle := graphConcept(t, "concept.middle", false)
	leaf := graphConcept(t, "concept.leaf", false)
	orphan := graphConcept(t, "concept.orphan", false)
	prerequisites := []curriculum.Prerequisite{
		graphPrerequisite(leaf, middle, curriculum.PrerequisiteHard),
		graphPrerequisite(middle, root, curriculum.PrerequisiteExposureOnly),
	}

	result, err := NewKnowledgeGraphCompilerV1().Compile(context.Background(), KnowledgeGraphCompilationRequest{
		Concepts: []curriculum.Concept{leaf, orphan, root, middle}, Prerequisites: prerequisites,
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if result.AlgorithmVersion != curriculum.KnowledgeGraphCompilerVersionV1 || result.ConsumptionContract != curriculum.CurriculumConsumptionContractV1 {
		t.Fatalf("graph versions = %+v", result)
	}
	assertConceptIDs(t, result.TopologicalOrder, "concept.orphan", "concept.root", "concept.middle", "concept.leaf")
	assertConceptIDs(t, result.RootConceptIDs, "concept.orphan", "concept.root")
	assertConceptIDs(t, result.UnreachableConceptIDs, "concept.orphan")
	assertConceptIDs(t, result.CriticalPath.ConceptIDs, "concept.root", "concept.middle", "concept.leaf")
	if result.CriticalPath.EdgeCount != 2 || len(result.Components) != 2 {
		t.Fatalf("graph metadata = %+v", result)
	}
	if len(result.ConsumptionPrerequisites) != 2 || result.ConsumptionPrerequisites[0].Requirement != curriculum.ConsumptionPrerequisiteMastered || result.ConsumptionPrerequisites[1].Requirement != curriculum.ConsumptionPrerequisiteIntroduced {
		t.Fatalf("consumption prerequisites = %+v", result.ConsumptionPrerequisites)
	}

	reordered, err := NewKnowledgeGraphCompilerV1().Compile(context.Background(), KnowledgeGraphCompilationRequest{
		Concepts:      []curriculum.Concept{middle, root, orphan, leaf},
		Prerequisites: []curriculum.Prerequisite{prerequisites[1], prerequisites[0]},
	})
	if err != nil || !reflect.DeepEqual(result, reordered) {
		t.Fatalf("reordered compilation differs: %+v / %+v / %v", result, reordered, err)
	}
}

func TestKnowledgeGraphCompilerV1RejectsCycle(t *testing.T) {
	t.Parallel()
	first := graphConcept(t, "concept.first", false)
	second := graphConcept(t, "concept.second", false)
	_, err := NewKnowledgeGraphCompilerV1().Compile(context.Background(), KnowledgeGraphCompilationRequest{
		Concepts: []curriculum.Concept{first, second},
		Prerequisites: []curriculum.Prerequisite{
			graphPrerequisite(first, second, curriculum.PrerequisiteHard),
			graphPrerequisite(second, first, curriculum.PrerequisiteHard),
		},
	})
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("Compile() error = %v, want invalid input", err)
	}
}

func TestKnowledgeGraphCompilerV1ConsumptionProjectionOmitsRecommendationsAndPrefersMastery(t *testing.T) {
	t.Parallel()
	root := graphConcept(t, "concept.root", true)
	leaf := graphConcept(t, "concept.leaf", false)
	result, err := NewKnowledgeGraphCompilerV1().Compile(context.Background(), KnowledgeGraphCompilationRequest{
		Concepts: []curriculum.Concept{root, leaf},
		Prerequisites: []curriculum.Prerequisite{
			graphPrerequisite(leaf, root, curriculum.PrerequisiteRecommended),
			graphPrerequisite(leaf, root, curriculum.PrerequisiteVocabulary),
			graphPrerequisite(leaf, root, curriculum.PrerequisiteHard),
		},
	})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(result.ConsumptionPrerequisites) != 1 || result.ConsumptionPrerequisites[0].Requirement != curriculum.ConsumptionPrerequisiteMastered {
		t.Fatalf("consumption projection = %+v", result.ConsumptionPrerequisites)
	}
}

func TestKnowledgeGraphCompilerV1LargeGraph(t *testing.T) {
	t.Parallel()
	const size = 5000
	concepts := make([]curriculum.Concept, size)
	prerequisites := make([]curriculum.Prerequisite, 0, size-1)
	for index := range concepts {
		concepts[index] = graphConcept(t, fmt.Sprintf("concept.%04d", index), index == 0)
		if index > 0 {
			prerequisites = append(prerequisites, graphPrerequisite(concepts[index], concepts[index-1], curriculum.PrerequisiteHard))
		}
	}
	result, err := NewKnowledgeGraphCompilerV1().Compile(context.Background(), KnowledgeGraphCompilationRequest{Concepts: concepts, Prerequisites: prerequisites})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if len(result.TopologicalOrder) != size || len(result.UnreachableConceptIDs) != 0 || len(result.Components) != 1 || result.CriticalPath.EdgeCount != size-1 {
		t.Fatalf("large graph metadata: order=%d unreachable=%d components=%d critical=%d", len(result.TopologicalOrder), len(result.UnreachableConceptIDs), len(result.Components), result.CriticalPath.EdgeCount)
	}
}

func graphConcept(t *testing.T, rawID string, foundational bool) curriculum.Concept {
	t.Helper()
	concept := granularityConcept(t, rawID, curriculum.AtomicityAtomic)
	concept.Foundational = foundational
	return concept
}

func graphPrerequisite(concept, required curriculum.Concept, kind curriculum.PrerequisiteKind) curriculum.Prerequisite {
	return curriculum.Prerequisite{ConceptID: concept.ID, RequiredConceptID: required.ID, Kind: kind}
}

func assertConceptIDs(t *testing.T, values []curriculum.ConceptID, expected ...string) {
	t.Helper()
	actual := make([]string, len(values))
	for index, id := range values {
		actual[index] = id.String()
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("concept IDs = %v, want %v", actual, expected)
	}
}
