package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestConceptAtomizerV1PreservesAlreadyAtomicCandidate(t *testing.T) {
	t.Parallel()
	candidate, evidence := atomizerCandidate(t, false)
	hint := atomizationHint(t, candidate, "concept.http-request", "HTTP request", candidate.ClaimRefs, atomicCriteria())

	result, err := NewConceptAtomizerV1(curriculum.NewAtomicConceptPolicyV1()).Atomize(context.Background(), ConceptAtomizationRequest{
		Candidate: candidate, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
		CandidateCriteria: atomicCriteria(), DomainHints: []curriculum.ConceptAtomizationHint{hint},
	})
	if err != nil {
		t.Fatalf("Atomize() error = %v", err)
	}
	if len(result.Concepts) != 1 || len(result.SplitReasons) != 0 || result.Concepts[0].ID.String() != "concept.http-request" || result.AlgorithmVersion != curriculum.AtomizerVersionV1 {
		t.Fatalf("atomic result = %+v", result)
	}
}

func TestConceptAtomizerV1SplitsBroadCandidateWithOverlappingEvidence(t *testing.T) {
	t.Parallel()
	candidate, evidence := atomizerCandidate(t, true)
	broad := atomicCriteria()
	broad.IndependentlyAssessableParts = []string{"request method", "request target"}
	hints := []curriculum.ConceptAtomizationHint{
		atomizationHint(t, candidate, "concept.request-target", "Request target", candidate.ClaimRefs, atomicCriteria()),
		atomizationHint(t, candidate, "concept.request-method", "Request method", candidate.ClaimRefs[:1], atomicCriteria()),
	}
	request := ConceptAtomizationRequest{Candidate: candidate, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, CandidateCriteria: broad, DomainHints: hints}
	service := NewConceptAtomizerV1(curriculum.NewAtomicConceptPolicyV1())
	result, err := service.Atomize(context.Background(), request)
	if err != nil {
		t.Fatalf("Atomize() error = %v", err)
	}
	if len(result.Concepts) != 2 || result.Concepts[0].ID.String() != "concept.request-method" || len(result.SplitReasons) != 1 {
		t.Fatalf("split result = %+v", result)
	}
	if len(result.ClaimMapping) != 2 || len(result.ClaimMapping[0].ConceptIDs) != 2 {
		t.Fatalf("overlapping claim mapping = %+v", result.ClaimMapping)
	}

	request.DomainHints[0], request.DomainHints[1] = request.DomainHints[1], request.DomainHints[0]
	repeated, err := service.Atomize(context.Background(), request)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered atomization differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestConceptAtomizerV1RejectsBroadCandidateWithoutEvidenceToSplit(t *testing.T) {
	t.Parallel()
	candidate, evidence := atomizerCandidate(t, true)
	broad := atomicCriteria()
	broad.IndependentlyAssessableParts = []string{"request method", "request target"}

	_, err := NewConceptAtomizerV1(curriculum.NewAtomicConceptPolicyV1()).Atomize(context.Background(), ConceptAtomizationRequest{
		Candidate: candidate, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, CandidateCriteria: broad,
	})
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "no evidence-backed atomization hints") {
		t.Fatalf("Atomize() error = %v", err)
	}
}

func TestConceptAtomizerV1RejectsNonAtomicChildAndUnmappedClaim(t *testing.T) {
	t.Parallel()
	candidate, evidence := atomizerCandidate(t, true)
	broad := atomicCriteria()
	broad.IndependentlyAssessableParts = []string{"request method", "request target"}
	nonAtomic := atomicCriteria()
	nonAtomic.IndependentlyAssessableParts = []string{"one", "two"}
	hints := []curriculum.ConceptAtomizationHint{
		atomizationHint(t, candidate, "concept.one", "One", candidate.ClaimRefs[:1], nonAtomic),
		atomizationHint(t, candidate, "concept.two", "Two", candidate.ClaimRefs[1:], atomicCriteria()),
	}
	service := NewConceptAtomizerV1(curriculum.NewAtomicConceptPolicyV1())
	_, err := service.Atomize(context.Background(), ConceptAtomizationRequest{Candidate: candidate, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, CandidateCriteria: broad, DomainHints: hints})
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "not atomic") {
		t.Fatalf("non-atomic child error = %v", err)
	}

	hints[0].Criteria = atomicCriteria()
	hints[1].ClaimRefs = candidate.ClaimRefs[:1]
	_, err = service.Atomize(context.Background(), ConceptAtomizationRequest{Candidate: candidate, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, CandidateCriteria: broad, DomainHints: hints})
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "do not map every candidate claim") {
		t.Fatalf("unmapped claim error = %v", err)
	}
}

func atomizerCandidate(t *testing.T, twoClaims bool) (curriculum.ConceptCandidate, curriculum.CurriculumEvidenceSet) {
	t.Helper()
	evidence, _ := decompositionEvidence(t)
	evidence.Claims[0].Kind = curriculum.EvidenceClaimDefinition
	evidence.Claims[0].Scope = "HTTP request"
	if twoClaims {
		evidence.Claims = append(evidence.Claims, candidateClaim(t, evidence, "claim.http.behavior", curriculum.EvidenceClaimBehavior, "HTTP request", "", curriculum.ConceptCurrent, "A request carries a method and target."))
	}
	set, err := NewConceptCandidateExtractorV1().Extract(context.Background(), ConceptCandidateExtractionRequest{EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}})
	if err != nil {
		t.Fatal(err)
	}
	return set.Candidates[0], evidence
}

func atomizationHint(t *testing.T, candidate curriculum.ConceptCandidate, id, title string, refs []curriculum.EvidenceRef, criteria curriculum.AtomicConceptCriteria) curriculum.ConceptAtomizationHint {
	t.Helper()
	conceptID, err := curriculum.NewConceptID(id)
	if err != nil {
		t.Fatal(err)
	}
	return curriculum.ConceptAtomizationHint{
		CandidateID: candidate.ID, ConceptID: conceptID, Title: title,
		Definition: title + " is independently explainable, practicable and assessable.", Version: "concept-v1",
		Difficulty: curriculum.DifficultyFoundational, Foundational: true,
		ClaimRefs: append([]curriculum.EvidenceRef(nil), refs...), Criteria: criteria,
	}
}

func atomicCriteria() curriculum.AtomicConceptCriteria {
	return curriculum.AtomicConceptCriteria{
		Named: curriculum.AtomicityCriterionSatisfied, Defined: curriculum.AtomicityCriterionSatisfied,
		PrerequisiteBoundary: curriculum.AtomicityCriterionSatisfied, Explainable: curriculum.AtomicityCriterionSatisfied,
		Practicable: curriculum.AtomicityCriterionSatisfied, Assessable: curriculum.AtomicityCriterionSatisfied,
		Evidenced: curriculum.AtomicityCriterionSatisfied, MeaningfulStandalone: curriculum.AtomicityCriterionSatisfied,
	}
}
