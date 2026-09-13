package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestFirstPrinciplesExpansionV1AddsEvidenceBackedRootAndEdges(t *testing.T) {
	t.Parallel()
	request, root, target := firstPrinciplesFixture(t, true)

	result, err := NewFirstPrinciplesExpansionV1().Expand(context.Background(), request)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(result.ExpandedRootConcepts) != 1 || result.ExpandedRootConcepts[0].ID != root.ID || len(result.ExpandedPrerequisites) != 1 || len(result.UnresolvedResearchNeeds) != 0 {
		t.Fatalf("expansion = %+v", result)
	}
	edge := result.ExpandedPrerequisites[0]
	if edge.ConceptID != target.ID || edge.RequiredConceptID != root.ID || edge.Kind != curriculum.PrerequisiteHard || result.AlgorithmVersion != curriculum.FirstPrinciplesExpansionVersionV1 {
		t.Fatalf("expanded edge = %+v", edge)
	}

	reordered := request
	reordered.EvidenceSets = append([]curriculum.CurriculumEvidenceSet(nil), request.EvidenceSets...)
	repeated, err := NewFirstPrinciplesExpansionV1().Expand(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("repeated expansion differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestFirstPrinciplesExpansionV1EmitsResearchNeedWithoutVerifiedEvidence(t *testing.T) {
	t.Parallel()
	request, _, _ := firstPrinciplesFixture(t, true)
	request.Candidates[0].Concept.EvidenceRefs = nil

	result, err := NewFirstPrinciplesExpansionV1().Expand(context.Background(), request)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(result.ExpandedRootConcepts) != 0 || len(result.ExpandedPrerequisites) != 0 || len(result.UnresolvedResearchNeeds) != 1 || result.UnresolvedResearchNeeds[0].Code != curriculum.FirstPrinciplesEvidenceInsufficient {
		t.Fatalf("expansion = %+v", result)
	}
}

func TestFirstPrinciplesExpansionV1EmitsResearchNeedWithoutCandidate(t *testing.T) {
	t.Parallel()
	request, _, _ := firstPrinciplesFixture(t, false)

	result, err := NewFirstPrinciplesExpansionV1().Expand(context.Background(), request)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(result.UnresolvedResearchNeeds) != 1 || result.UnresolvedResearchNeeds[0].Code != curriculum.FirstPrinciplesCandidateUnavailable {
		t.Fatalf("expansion = %+v", result)
	}
}

func TestFirstPrinciplesExpansionV1DoesNotRewriteExistingNonFoundation(t *testing.T) {
	t.Parallel()
	request, root, target := firstPrinciplesFixture(t, true)
	root.Foundational = false
	request.Concepts = []curriculum.Concept{root, target}

	result, err := NewFirstPrinciplesExpansionV1().Expand(context.Background(), request)
	if err != nil {
		t.Fatalf("Expand() error = %v", err)
	}
	if len(result.ExpandedRootConcepts) != 0 || len(result.UnresolvedResearchNeeds) != 1 || result.UnresolvedResearchNeeds[0].Code != curriculum.FirstPrinciplesImmutableConflict {
		t.Fatalf("expansion = %+v", result)
	}
}

func firstPrinciplesFixture(t *testing.T, withCandidate bool) (FirstPrinciplesExpansionRequest, curriculum.Concept, curriculum.Concept) {
	t.Helper()
	auditRequest, root, target := zeroAssumptionFixture(t)
	auditRequest.Concepts = []curriculum.Concept{target}
	auditRequest.Graph = compileAuditGraph(t, auditRequest.Concepts, nil)
	audit, err := NewZeroAssumptionAuditV1().Audit(context.Background(), auditRequest)
	if err != nil {
		t.Fatal(err)
	}
	request := FirstPrinciplesExpansionRequest{
		AuditResult: audit, Concepts: auditRequest.Concepts, EvidenceSets: auditRequest.EvidenceSets,
	}
	if withCandidate {
		request.Candidates = []curriculum.FirstPrinciplesCandidate{{
			RequirementID: auditRequest.Baseline.Requirements[0].ID, Concept: root,
			PrerequisiteKind:         curriculum.PrerequisiteHard,
			PrerequisiteEvidenceRefs: append([]curriculum.EvidenceRef(nil), root.EvidenceRefs...),
			Reason:                   "The verified foundation closes the zero-assumption gap.",
		}}
	}
	return request, root, target
}
