package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestPracticeCoverageV1RequiresCompatibleExpectationForImportantConcept(t *testing.T) {
	t.Parallel()
	request, _, concept, reference := practiceCoverageFixture(t)
	request.Expectations = []curriculum.PracticeExpectation{
		{ID: curriculumID(t, "practice.recall"), RequirementID: request.Requirements[0].ID, Kind: curriculum.PracticeRecall, Reason: "Recall alone is below the apply target.", EvidenceRefs: []curriculum.EvidenceRef{reference}},
		{ID: curriculumID(t, "practice.apply"), RequirementID: request.Requirements[0].ID, Kind: curriculum.PracticeApply, Reason: "Apply the concept in a bounded scenario.", EvidenceRefs: []curriculum.EvidenceRef{reference}},
	}

	report, err := NewPracticeCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	result := report.Results[0]
	if result.Status != curriculum.CoverageCovered || !reflect.DeepEqual(result.CompatibleExpectationIDs, []curriculum.ID{request.Expectations[1].ID}) || !reflect.DeepEqual(result.IncompatibleExpectationIDs, []curriculum.ID{request.Expectations[0].ID}) {
		t.Fatalf("practice result = %+v", result)
	}
	if len(report.CoverageRequirements) != 1 || len(report.CoverageSupports) != 1 || report.CoverageSupports[0].ArtifactRefs[0] != request.Expectations[1].ID || report.CoverageSupports[0].ConceptIDs[0] != concept.ID {
		t.Fatalf("coverage bridge = %+v / %+v", report.CoverageRequirements, report.CoverageSupports)
	}
	if report.CompatibilityVersion != curriculum.PracticeCompatibilityVersionV1 || report.AlgorithmVersion != curriculum.PracticeCoverageVersionV1 {
		t.Fatalf("practice versions = %+v", report)
	}

	reordered := request
	reordered.Expectations = []curriculum.PracticeExpectation{request.Expectations[1], request.Expectations[0]}
	repeated, err := NewPracticeCoverageV1().Analyze(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(report, repeated) {
		t.Fatalf("reordered practice coverage differs: %+v / %+v / %v", report, repeated, err)
	}
}

func TestPracticeCoverageV1ReportsMissingForIncompatibleExpectation(t *testing.T) {
	t.Parallel()
	request, _, _, reference := practiceCoverageFixture(t)
	request.Expectations = []curriculum.PracticeExpectation{{
		ID: curriculumID(t, "practice.recall-only"), RequirementID: request.Requirements[0].ID,
		Kind: curriculum.PracticeRecall, Reason: "Recall does not reach apply.", EvidenceRefs: []curriculum.EvidenceRef{reference},
	}}
	report, err := NewPracticeCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.Results[0].Status != curriculum.CoverageMissing || len(report.Results[0].IncompatibleExpectationIDs) != 1 || len(report.CoverageSupports) != 0 {
		t.Fatalf("practice report = %+v", report)
	}
}

func TestPracticeCoverageV1RejectsUnmappedImportantConcept(t *testing.T) {
	t.Parallel()
	request, competency, _, _ := practiceCoverageFixture(t)
	request.Competencies.Competencies[0].ConceptRefs = nil
	_, err := NewPracticeCoverageV1().Analyze(context.Background(), request)
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "not mapped to competency") {
		t.Fatalf("Analyze(%s) error = %v", competency.ID, err)
	}
}

func practiceCoverageFixture(t *testing.T) (PracticeCoverageRequest, curriculum.Competency, curriculum.Concept, curriculum.EvidenceRef) {
	t.Helper()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, false)
	concept := graphConcept(t, "concept.request-flow", false)
	concept.EvidenceRefs = []curriculum.EvidenceRef{reference}
	competency := curriculum.Competency{
		ID: curriculumID(t, "competency.request-flow"), AreaID: curriculumID(t, "area.http"), Area: "HTTP",
		OutcomeID: goal.Outcomes[0].ID, ExpectedLevel: curriculum.CompetencyApply,
		EvidenceRefs: []curriculum.EvidenceRef{reference}, ConceptRefs: []curriculum.ConceptID{concept.ID},
	}
	requirement := curriculum.PracticeRequirement{
		ID: curriculumID(t, "practice.requirement.request-flow"), ConceptID: concept.ID, CompetencyID: competency.ID,
		Reason: "This important concept must be practiced at the expected competency level.", EvidenceRefs: []curriculum.EvidenceRef{reference},
	}
	return PracticeCoverageRequest{
		Competencies: curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: goal.ID, Competencies: []curriculum.Competency{competency}},
		Concepts:     []curriculum.Concept{concept}, ImportantConceptIDs: []curriculum.ConceptID{concept.ID},
		Requirements: []curriculum.PracticeRequirement{requirement}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	}, competency, concept, reference
}
