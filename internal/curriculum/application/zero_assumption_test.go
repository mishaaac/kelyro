package application

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestZeroAssumptionAuditV1PassesEvidenceBackedFoundationPath(t *testing.T) {
	t.Parallel()
	request, root, target := zeroAssumptionFixture(t)
	request.Graph = compileAuditGraph(t, request.Concepts, []curriculum.Prerequisite{{
		ConceptID: target.ID, RequiredConceptID: root.ID, Kind: curriculum.PrerequisiteHard,
		EvidenceRefs: append([]curriculum.EvidenceRef(nil), root.EvidenceRefs...),
	}})

	result, err := NewZeroAssumptionAuditV1().Audit(context.Background(), request)
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if !result.Passed || len(result.Violations) != 0 || result.AuditedRequirements != 1 || result.AlgorithmVersion != curriculum.ZeroAssumptionAuditVersionV1 {
		t.Fatalf("audit result = %+v", result)
	}

	reordered := request
	reordered.Concepts = []curriculum.Concept{target, root}
	repeated, err := NewZeroAssumptionAuditV1().Audit(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered audit differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestZeroAssumptionAuditV1FindsMissingFoundationAndPrerequisite(t *testing.T) {
	t.Parallel()
	request, root, target := zeroAssumptionFixture(t)
	missingID, err := curriculum.NewConceptID("concept.terminal")
	if err != nil {
		t.Fatal(err)
	}
	request.Baseline.Requirements = append(request.Baseline.Requirements, curriculum.FoundationRequirement{
		ID: curriculumID(t, "foundation.terminal"), ConceptID: missingID,
		TargetCompetencyIDs: []curriculum.ID{request.Competencies.Competencies[0].ID},
		Reason:              "Terminal operation is relevant to the declared command-line goal.",
		EvidenceRefs:        append([]curriculum.EvidenceRef(nil), root.EvidenceRefs...),
	})
	request.Graph = compileAuditGraph(t, request.Concepts, nil)

	result, err := NewZeroAssumptionAuditV1().Audit(context.Background(), request)
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if result.Passed || len(result.Violations) != 2 {
		t.Fatalf("audit result = %+v", result)
	}
	codes := map[curriculum.ZeroAssumptionViolationCode]curriculum.ConceptID{}
	for _, violation := range result.Violations {
		codes[violation.Code] = violation.FoundationConceptID
		if violation.TargetConceptID != target.ID || len(violation.EvidenceRefs) == 0 {
			t.Fatalf("violation = %+v", violation)
		}
	}
	if codes[curriculum.ZeroAssumptionMissingFoundation] != missingID || codes[curriculum.ZeroAssumptionMissingPrerequisite] != root.ID {
		t.Fatalf("violation codes = %+v", codes)
	}
}

func TestZeroAssumptionAuditV1HonorsExplicitNonZeroBaseline(t *testing.T) {
	t.Parallel()
	request, root, _ := zeroAssumptionFixture(t)
	request.Baseline.LearnerProfile = curriculum.LearnerProfileSomeExperience
	request.Baseline.AssumedConceptIDs = []curriculum.ConceptID{root.ID}
	request.Concepts = request.Concepts[1:]
	request.Graph = compileAuditGraph(t, request.Concepts, nil)

	result, err := NewZeroAssumptionAuditV1().Audit(context.Background(), request)
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if !result.Passed || result.AssumedRequirements != 1 {
		t.Fatalf("audit result = %+v", result)
	}

	request.Baseline.LearnerProfile = curriculum.LearnerProfileZero
	_, err = NewZeroAssumptionAuditV1().Audit(context.Background(), request)
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("zero baseline with assumptions error = %v", err)
	}
}

func TestZeroAssumptionAuditV1HonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewZeroAssumptionAuditV1().Audit(ctx, ZeroAssumptionAuditRequest{})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Audit() error = %v", err)
	}
}

func zeroAssumptionFixture(t *testing.T) (ZeroAssumptionAuditRequest, curriculum.Concept, curriculum.Concept) {
	t.Helper()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, false)
	profile := decompositionProfile(t, reference, goal.Outcomes)
	root := graphConcept(t, "concept.files", true)
	root.EvidenceRefs = []curriculum.EvidenceRef{reference}
	target := graphConcept(t, "concept.http-server", false)
	target.EvidenceRefs = []curriculum.EvidenceRef{reference}
	competency := curriculum.Competency{
		ID: curriculumID(t, "competency.http"), AreaID: profile.Areas[0].ID, Area: profile.Areas[0].Name,
		OutcomeID: goal.Outcomes[0].ID, ExpectedLevel: curriculum.CompetencyApply,
		EvidenceRefs: []curriculum.EvidenceRef{reference}, ConceptRefs: []curriculum.ConceptID{target.ID},
	}
	matrix := curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: goal.ID, Competencies: []curriculum.Competency{competency}}
	baseline := curriculum.AssumptionBaseline{
		ID: curriculumID(t, "baseline.http-zero"), Version: "http-zero-v1",
		DomainProfileID: profile.ID, DomainProfileVersion: profile.Version, LearnerProfile: curriculum.LearnerProfileZero,
		Requirements: []curriculum.FoundationRequirement{{
			ID: curriculumID(t, "foundation.files"), ConceptID: root.ID,
			TargetCompetencyIDs: []curriculum.ID{competency.ID},
			Reason:              "Files and directories are relevant to serving local content.", EvidenceRefs: []curriculum.EvidenceRef{reference},
		}},
	}
	concepts := []curriculum.Concept{root, target}
	return ZeroAssumptionAuditRequest{
		DomainProfile: profile, Baseline: baseline, Competencies: matrix, Concepts: concepts,
		Graph: compileAuditGraph(t, concepts, nil), EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	}, root, target
}
