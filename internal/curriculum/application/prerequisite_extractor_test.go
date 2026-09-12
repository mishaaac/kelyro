package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestPrerequisiteExtractorV1DerivesDeterministicChainsAndDiamonds(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	concepts := []curriculum.Concept{
		granularityConcept(t, "concept.d", curriculum.AtomicityAtomic),
		granularityConcept(t, "concept.c", curriculum.AtomicityAtomic),
		granularityConcept(t, "concept.b", curriculum.AtomicityAtomic),
		granularityConcept(t, "concept.a", curriculum.AtomicityAtomic),
	}
	byID := conceptIndex(concepts)
	semantics := []curriculum.ConceptPrerequisiteSemantic{
		prerequisiteSemantic(byID["concept.c"], byID["concept.b"], curriculum.PrerequisiteVocabulary, reference, "B defines vocabulary used by C."),
		prerequisiteSemantic(byID["concept.b"], byID["concept.d"], curriculum.PrerequisiteExposureOnly, reference, "D provides prior exposure for B."),
		prerequisiteSemantic(byID["concept.a"], byID["concept.c"], curriculum.PrerequisiteRecommended, reference, "C is recommended before A."),
		prerequisiteSemantic(byID["concept.c"], byID["concept.d"], curriculum.PrerequisiteToolDependency, reference, "D introduces the tool used by C."),
		prerequisiteSemantic(byID["concept.a"], byID["concept.b"], curriculum.PrerequisiteHard, reference, "B is required before A."),
	}
	request := PrerequisiteExtractionRequest{Concepts: concepts, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, Semantics: semantics}
	service := NewPrerequisiteExtractorV1()
	result, err := service.Extract(context.Background(), request)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if result.AlgorithmVersion != curriculum.PrerequisiteExtractorVersionV1 || len(result.Derivations) != 5 || len(result.Edges()) != 5 {
		t.Fatalf("extraction = %+v", result)
	}
	if got := result.Derivations[0].Prerequisite; got.ConceptID.String() != "concept.a" || got.RequiredConceptID.String() != "concept.b" || got.Kind != curriculum.PrerequisiteHard {
		t.Fatalf("first deterministic edge = %+v", got)
	}

	reversed := request
	reversed.Semantics = append([]curriculum.ConceptPrerequisiteSemantic(nil), request.Semantics...)
	for left, right := 0, len(reversed.Semantics)-1; left < right; left, right = left+1, right-1 {
		reversed.Semantics[left], reversed.Semantics[right] = reversed.Semantics[right], reversed.Semantics[left]
	}
	repeated, err := service.Extract(context.Background(), reversed)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered extraction differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestPrerequisiteExtractorV1RejectsMissingConceptAndEvidence(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	concept := granularityConcept(t, "concept.current", curriculum.AtomicityAtomic)
	missing := granularityConcept(t, "concept.missing", curriculum.AtomicityAtomic)
	request := PrerequisiteExtractionRequest{
		Concepts: []curriculum.Concept{concept}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
		Semantics: []curriculum.ConceptPrerequisiteSemantic{prerequisiteSemantic(concept, missing, curriculum.PrerequisiteHard, reference, "Missing concept is required.")},
	}
	_, err := NewPrerequisiteExtractorV1().Extract(context.Background(), request)
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "missing required concept") {
		t.Fatalf("missing concept error = %v", err)
	}

	request.Concepts = []curriculum.Concept{concept, missing}
	request.Semantics[0].EvidenceRefs[0].ClaimID = curriculumID(t, "claim.unknown")
	_, err = NewPrerequisiteExtractorV1().Extract(context.Background(), request)
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "unavailable evidence") {
		t.Fatalf("missing evidence error = %v", err)
	}
}

func prerequisiteSemantic(concept, required curriculum.Concept, kind curriculum.PrerequisiteKind, evidence curriculum.EvidenceRef, reason string) curriculum.ConceptPrerequisiteSemantic {
	return curriculum.ConceptPrerequisiteSemantic{
		ConceptID: concept.ID, RequiredConceptID: required.ID, Kind: kind,
		EvidenceRefs: []curriculum.EvidenceRef{evidence}, Reason: reason,
	}
}

func conceptIndex(values []curriculum.Concept) map[string]curriculum.Concept {
	result := make(map[string]curriculum.Concept, len(values))
	for _, concept := range values {
		result[concept.ID.String()] = concept
	}
	return result
}
