package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCoverageEngineV1ReportsEveryDimensionWithoutSingleScore(t *testing.T) {
	t.Parallel()
	request := coverageFixture(t)
	result, err := NewCoverageEngineV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.AlgorithmVersion != curriculum.CoverageEngineVersionV1 || len(result.Dimensions) != 8 {
		t.Fatalf("coverage report = %+v", result)
	}
	if coverageDimension(t, result, curriculum.CoverageCompetency).Status != curriculum.CoveragePartial {
		t.Fatalf("competency coverage = %+v", coverageDimension(t, result, curriculum.CoverageCompetency))
	}
	if coverageDimension(t, result, curriculum.CoverageConcept).Status != curriculum.CoveragePartial {
		t.Fatalf("concept coverage = %+v", coverageDimension(t, result, curriculum.CoverageConcept))
	}
	if coverageDimension(t, result, curriculum.CoverageEvidence).Status != curriculum.CoverageCovered {
		t.Fatalf("evidence coverage = %+v", coverageDimension(t, result, curriculum.CoverageEvidence))
	}
	if coverageDimension(t, result, curriculum.CoverageTheory).Status != curriculum.CoverageCovered {
		t.Fatalf("theory coverage = %+v", coverageDimension(t, result, curriculum.CoverageTheory))
	}
	if coverageDimension(t, result, curriculum.CoveragePractice).Status != curriculum.CoverageMissing {
		t.Fatalf("practice coverage = %+v", coverageDimension(t, result, curriculum.CoveragePractice))
	}
	for _, dimension := range []curriculum.CoverageDimension{curriculum.CoverageProduction, curriculum.CoverageSecurity, curriculum.CoverageToolchain} {
		dimensionReport := coverageDimension(t, result, dimension)
		if dimensionReport.Status != curriculum.CoverageMissing || !reflect.DeepEqual(dimensionReport.Reasons, []string{"no_requirements_declared"}) {
			t.Fatalf("undeclared %s coverage = %+v", dimension, dimensionReport)
		}
	}

	reordered := request
	reordered.Concepts = append([]curriculum.Concept(nil), request.Concepts...)
	reordered.Concepts[0], reordered.Concepts[1] = reordered.Concepts[1], reordered.Concepts[0]
	reordered.Requirements = append([]curriculum.CoverageRequirement(nil), request.Requirements...)
	for left, right := 0, len(reordered.Requirements)-1; left < right; left, right = left+1, right-1 {
		reordered.Requirements[left], reordered.Requirements[right] = reordered.Requirements[right], reordered.Requirements[left]
	}
	repeated, err := NewCoverageEngineV1().Analyze(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered coverage differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestCoverageEngineV1RejectsUnavailableSupportEvidence(t *testing.T) {
	t.Parallel()
	request := coverageFixture(t)
	request.Supports[0].EvidenceRefs[0].ClaimID = curriculumID(t, "claim.missing")
	_, err := NewCoverageEngineV1().Analyze(context.Background(), request)
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "unavailable evidence") {
		t.Fatalf("Analyze() error = %v", err)
	}
}

func coverageFixture(t *testing.T) CoverageAnalysisRequest {
	t.Helper()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, false)
	goal.Outcomes = append(goal.Outcomes, curriculum.GoalOutcome{
		ID: curriculumID(t, "outcome.build"), Statement: "Build an HTTP service.",
		Category: curriculum.OutcomeApplication, Capability: curriculum.OutcomeCapabilityBuild,
		EvidenceRefs: []curriculum.EvidenceRef{reference},
	})
	present := graphConcept(t, "concept.request", true)
	present.EvidenceRefs = []curriculum.EvidenceRef{reference}
	unmapped := graphConcept(t, "concept.response", false)
	unmapped.EvidenceRefs = []curriculum.EvidenceRef{reference}
	missingID, err := curriculum.NewConceptID("concept.missing")
	if err != nil {
		t.Fatal(err)
	}
	competency := curriculum.Competency{
		ID: curriculumID(t, "competency.http"), AreaID: curriculumID(t, "area.http"), Area: "HTTP",
		OutcomeID: goal.Outcomes[0].ID, ExpectedLevel: curriculum.CompetencyApply,
		EvidenceRefs: []curriculum.EvidenceRef{reference}, ConceptRefs: []curriculum.ConceptID{present.ID, missingID},
	}
	matrix := curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: goal.ID, Competencies: []curriculum.Competency{competency}}
	curriculumIDValue, err := curriculum.NewCurriculumID("curriculum.http")
	if err != nil {
		t.Fatal(err)
	}
	requirements := []curriculum.CoverageRequirement{
		coverageRequirement(t, "coverage.competency", curriculum.CoverageCompetency, curriculum.CoverageTargetGoal, goal.ID, reference),
		coverageRequirement(t, "coverage.concept", curriculum.CoverageConcept, curriculum.CoverageTargetCompetency, competency.ID, reference),
		coverageRequirement(t, "coverage.evidence", curriculum.CoverageEvidence, curriculum.CoverageTargetGoal, goal.ID, reference),
		coverageRequirement(t, "coverage.theory", curriculum.CoverageTheory, curriculum.CoverageTargetCompetency, competency.ID, reference),
		coverageRequirement(t, "coverage.practice", curriculum.CoveragePractice, curriculum.CoverageTargetCompetency, competency.ID, reference),
	}
	return CoverageAnalysisRequest{
		CurriculumID: curriculumIDValue, Goal: goal, Competencies: matrix,
		Concepts: []curriculum.Concept{present, unmapped}, Requirements: requirements,
		Supports: []curriculum.CoverageSupport{{
			ID: curriculumID(t, "support.theory"), RequirementID: requirements[3].ID,
			ConceptIDs: []curriculum.ConceptID{present.ID}, EvidenceRefs: []curriculum.EvidenceRef{reference},
			Reason: "The concept explicitly supports the theory requirement.",
		}},
		EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	}
}

func coverageRequirement(t *testing.T, rawID string, dimension curriculum.CoverageDimension, targetKind curriculum.CoverageTargetKind, targetID curriculum.ID, evidence curriculum.EvidenceRef) curriculum.CoverageRequirement {
	t.Helper()
	return curriculum.CoverageRequirement{
		ID: curriculumID(t, rawID), Dimension: dimension, TargetKind: targetKind, TargetID: targetID,
		Description: "A deterministic coverage requirement.", EvidenceRefs: []curriculum.EvidenceRef{evidence},
	}
}

func coverageDimension(t *testing.T, report curriculum.CoverageReport, dimension curriculum.CoverageDimension) curriculum.CoverageDimensionReport {
	t.Helper()
	for _, candidate := range report.Dimensions {
		if candidate.Dimension == dimension {
			return candidate
		}
	}
	t.Fatalf("coverage dimension %q is absent", dimension)
	return curriculum.CoverageDimensionReport{}
}
