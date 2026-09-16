package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestPrerequisiteExpansionV1ReportsMissingRoot(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	concept := expansionConcept(t, "concept.orphan", false, reference)

	result, err := NewPrerequisiteExpansionV1().Expand(context.Background(), PrerequisiteExpansionRequest{
		Concepts: []curriculum.Concept{concept}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	})
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(result.AddedConcepts) != 0 || len(result.ExpandedPrerequisites) != 0 || len(result.UnresolvedGaps) != 1 || result.UnresolvedGaps[0].Code != curriculum.PrerequisiteGapMissingRoot {
		t.Fatalf("missing-root result = %+v", result)
	}
}

func TestPrerequisiteExpansionV1RecursivelyAddsEvidenceBackedConcepts(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	handler := expansionConcept(t, "concept.handler", false, reference)
	function := expansionConcept(t, "concept.function", false, reference)
	value := expansionConcept(t, "concept.value", true, reference)
	semantics := []curriculum.ConceptPrerequisiteSemantic{
		prerequisiteSemantic(function, value, curriculum.PrerequisiteHard, reference, "Values are required to understand functions."),
		prerequisiteSemantic(handler, function, curriculum.PrerequisiteHard, reference, "Functions are required to implement handlers."),
	}
	request := PrerequisiteExpansionRequest{
		Concepts: []curriculum.Concept{handler}, AvailableConcepts: []curriculum.Concept{value, function},
		Semantics: semantics, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	}
	service := NewPrerequisiteExpansionV1()
	result, err := service.Expand(context.Background(), request)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if result.AlgorithmVersion != curriculum.PrerequisiteExpansionVersionV1 || len(result.AddedConcepts) != 2 || len(result.ExpandedPrerequisites) != 2 || len(result.UnresolvedGaps) != 0 {
		t.Fatalf("recursive result = %+v", result)
	}
	if result.AddedConcepts[0].ID.String() != "concept.function" || result.AddedConcepts[1].ID.String() != "concept.value" {
		t.Fatalf("added concept order = %+v", result.AddedConcepts)
	}

	reversed := request
	reversed.AvailableConcepts = []curriculum.Concept{function, value}
	reversed.Semantics = []curriculum.ConceptPrerequisiteSemantic{semantics[1], semantics[0]}
	repeated, err := service.Expand(context.Background(), reversed)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered expansion differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestPrerequisiteExpansionV1PreventsCycle(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	first := expansionConcept(t, "concept.first", false, reference)
	second := expansionConcept(t, "concept.second", false, reference)
	semantics := []curriculum.ConceptPrerequisiteSemantic{
		prerequisiteSemantic(first, second, curriculum.PrerequisiteHard, reference, "Second precedes first."),
		prerequisiteSemantic(second, first, curriculum.PrerequisiteHard, reference, "This reverse edge would be cyclic."),
	}

	result, err := NewPrerequisiteExpansionV1().Expand(context.Background(), PrerequisiteExpansionRequest{
		Concepts: []curriculum.Concept{first}, AvailableConcepts: []curriculum.Concept{second},
		Semantics: semantics, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	})
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(result.AddedConcepts) != 1 || len(result.ExpandedPrerequisites) != 1 || len(result.UnresolvedGaps) != 1 || result.UnresolvedGaps[0].Code != curriculum.PrerequisiteGapCyclePrevented {
		t.Fatalf("cycle-prevention result = %+v", result)
	}
	if result.ExpandedPrerequisites[0].ConceptID != first.ID || result.ExpandedPrerequisites[0].RequiredConceptID != second.ID {
		t.Fatalf("preserved acyclic edge = %+v", result.ExpandedPrerequisites)
	}
}

func TestPrerequisiteExpansionV1ReportsUnavailablePrerequisite(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	current := expansionConcept(t, "concept.current", false, reference)
	missing := expansionConcept(t, "concept.missing", true, reference)

	result, err := NewPrerequisiteExpansionV1().Expand(context.Background(), PrerequisiteExpansionRequest{
		Concepts: []curriculum.Concept{current}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
		Semantics: []curriculum.ConceptPrerequisiteSemantic{
			prerequisiteSemantic(current, missing, curriculum.PrerequisiteRecommended, reference, "Missing prerequisite is recommended."),
		},
	})
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(result.UnresolvedGaps) != 1 || result.UnresolvedGaps[0].Code != curriculum.PrerequisiteGapConceptUnavailable || result.UnresolvedGaps[0].RequiredConceptID == nil || *result.UnresolvedGaps[0].RequiredConceptID != missing.ID {
		t.Fatalf("unavailable prerequisite result = %+v", result)
	}
}

func TestPrerequisiteExpansionV1RejectsCycleInExistingGraph(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	first := expansionConcept(t, "concept.first", false, reference)
	second := expansionConcept(t, "concept.second", false, reference)
	_, err := NewPrerequisiteExpansionV1().Expand(context.Background(), PrerequisiteExpansionRequest{
		Concepts: []curriculum.Concept{first, second}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
		Prerequisites: []curriculum.Prerequisite{
			{ConceptID: first.ID, RequiredConceptID: second.ID, Kind: curriculum.PrerequisiteHard, EvidenceRefs: []curriculum.EvidenceRef{reference}},
			{ConceptID: second.ID, RequiredConceptID: first.ID, Kind: curriculum.PrerequisiteHard, EvidenceRefs: []curriculum.EvidenceRef{reference}},
		},
	})
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("Expand() error = %v, want invalid existing cycle", err)
	}
}

func TestPrerequisiteExpansionV1HandlesDeepExpansionIteratively(t *testing.T) {
	t.Parallel()
	const size = 10_000
	evidence, reference := decompositionEvidence(t)
	concepts := make([]curriculum.Concept, size)
	semantics := make([]curriculum.ConceptPrerequisiteSemantic, 0, size-1)
	for index := range concepts {
		concepts[index] = expansionConcept(t, fmt.Sprintf("concept.deep.%05d", index), index == 0, reference)
		if index > 0 {
			semantics = append(semantics, prerequisiteSemantic(concepts[index], concepts[index-1], curriculum.PrerequisiteHard, reference, "Generated deep dependency."))
		}
	}
	result, err := NewPrerequisiteExpansionV1().Expand(context.Background(), PrerequisiteExpansionRequest{
		Concepts: []curriculum.Concept{concepts[size-1]}, AvailableConcepts: concepts[:size-1],
		Semantics: semantics, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AddedConcepts) != size-1 || len(result.ExpandedPrerequisites) != size-1 || len(result.UnresolvedGaps) != 0 {
		t.Fatalf("deep expansion: added=%d prerequisites=%d gaps=%d", len(result.AddedConcepts), len(result.ExpandedPrerequisites), len(result.UnresolvedGaps))
	}
}

func expansionConcept(t *testing.T, rawID string, foundational bool, evidence curriculum.EvidenceRef) curriculum.Concept {
	t.Helper()
	concept := granularityConcept(t, rawID, curriculum.AtomicityAtomic)
	concept.Foundational = foundational
	concept.EvidenceRefs = []curriculum.EvidenceRef{evidence}
	return concept
}
