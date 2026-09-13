package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestTheoryCoverageV1ReportsMissingDefinitionAndMentalModel(t *testing.T) {
	t.Parallel()
	request, competency, concept, reference := theoryCoverageFixture(t)
	request.Contracts[0].RequiredFacets = []curriculum.TheoryFacet{
		curriculum.TheoryFailureModes, curriculum.TheoryMentalModel, curriculum.TheoryDefinition,
		curriculum.TheoryTradeoffs, curriculum.TheoryMechanism,
	}
	for index, facet := range []curriculum.TheoryFacet{curriculum.TheoryMechanism, curriculum.TheoryTradeoffs, curriculum.TheoryFailureModes} {
		request.Supports = append(request.Supports, curriculum.TheoryFacetSupport{
			ID: curriculumID(t, "theory.support."+string(rune('a'+index))), ContractID: request.Contracts[0].ID,
			Facet: facet, ConceptIDs: []curriculum.ConceptID{concept.ID}, EvidenceRefs: []curriculum.EvidenceRef{reference},
			Reason: "The concept explicitly covers the required theory facet.",
		})
	}

	report, err := NewTheoryCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.AlgorithmVersion != curriculum.TheoryCoverageVersionV1 || len(report.Competencies) != 1 || len(report.CoverageRequirements) != 5 || len(report.CoverageSupports) != 3 {
		t.Fatalf("theory report = %+v", report)
	}
	coverage := report.Competencies[0]
	if coverage.CompetencyID != competency.ID || coverage.Status != curriculum.CoveragePartial {
		t.Fatalf("competency coverage = %+v", coverage)
	}
	if theoryFacetStatus(t, coverage, curriculum.TheoryDefinition) != curriculum.CoverageMissing || theoryFacetStatus(t, coverage, curriculum.TheoryMentalModel) != curriculum.CoverageMissing {
		t.Fatalf("expected missing definition and mental model: %+v", coverage.Facets)
	}
	for _, facet := range []curriculum.TheoryFacet{curriculum.TheoryMechanism, curriculum.TheoryTradeoffs, curriculum.TheoryFailureModes} {
		if theoryFacetStatus(t, coverage, facet) != curriculum.CoverageCovered {
			t.Fatalf("facet %s not covered: %+v", facet, coverage.Facets)
		}
	}
	goal := decompositionGoal(t, reference, false)
	curriculumIDValue, err := curriculum.NewCurriculumID("curriculum.theory")
	if err != nil {
		t.Fatal(err)
	}
	general, err := NewCoverageEngineV1().Analyze(context.Background(), CoverageAnalysisRequest{
		CurriculumID: curriculumIDValue, Goal: goal, Competencies: request.Competencies,
		Concepts: request.Concepts, Requirements: report.CoverageRequirements,
		Supports: report.CoverageSupports, EvidenceSets: request.EvidenceSets,
	})
	if err != nil || coverageDimension(t, general, curriculum.CoverageTheory).Status != curriculum.CoveragePartial {
		t.Fatalf("general coverage bridge = %+v / %v", general, err)
	}

	reordered := request
	for left, right := 0, len(reordered.Supports)-1; left < right; left, right = left+1, right-1 {
		reordered.Supports[left], reordered.Supports[right] = reordered.Supports[right], reordered.Supports[left]
	}
	for left, right := 0, len(reordered.Contracts[0].RequiredFacets)-1; left < right; left, right = left+1, right-1 {
		reordered.Contracts[0].RequiredFacets[left], reordered.Contracts[0].RequiredFacets[right] = reordered.Contracts[0].RequiredFacets[right], reordered.Contracts[0].RequiredFacets[left]
	}
	repeated, err := NewTheoryCoverageV1().Analyze(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(report, repeated) {
		t.Fatalf("reordered theory coverage differs: %+v / %+v / %v", report, repeated, err)
	}
}

func TestTheoryCoverageV1ReportsMissingContractForImportantCompetency(t *testing.T) {
	t.Parallel()
	request, competency, _, _ := theoryCoverageFixture(t)
	request.Contracts = nil

	report, err := NewTheoryCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(report.Competencies) != 1 || report.Competencies[0].ContractID != nil || report.Competencies[0].Status != curriculum.CoverageMissing {
		t.Fatalf("theory report = %+v", report)
	}
	if len(report.CoverageRequirements) != 1 || report.CoverageRequirements[0].TargetID != competency.ID || len(report.CoverageSupports) != 0 {
		t.Fatalf("coverage bridge = %+v / %+v", report.CoverageRequirements, report.CoverageSupports)
	}
}

func TestTheoryCoverageV1CoversCompleteContract(t *testing.T) {
	t.Parallel()
	request, _, concept, reference := theoryCoverageFixture(t)
	for index, facet := range request.Contracts[0].RequiredFacets {
		request.Supports = append(request.Supports, curriculum.TheoryFacetSupport{
			ID: curriculumID(t, "theory.complete."+string(rune('a'+index))), ContractID: request.Contracts[0].ID,
			Facet: facet, ConceptIDs: []curriculum.ConceptID{concept.ID}, EvidenceRefs: []curriculum.EvidenceRef{reference},
			Reason: "Explicit theory support.",
		})
	}
	report, err := NewTheoryCoverageV1().Analyze(context.Background(), request)
	if err != nil || report.Competencies[0].Status != curriculum.CoverageCovered {
		t.Fatalf("complete theory report = %+v / %v", report, err)
	}
}

func TestTheoryCoverageV1RejectsSupportOutsideCompetency(t *testing.T) {
	t.Parallel()
	request, _, _, reference := theoryCoverageFixture(t)
	outside := graphConcept(t, "concept.outside", false)
	outside.EvidenceRefs = []curriculum.EvidenceRef{reference}
	request.Concepts = append(request.Concepts, outside)
	request.Supports = []curriculum.TheoryFacetSupport{{
		ID: curriculumID(t, "theory.outside"), ContractID: request.Contracts[0].ID,
		Facet: curriculum.TheoryDefinition, ConceptIDs: []curriculum.ConceptID{outside.ID},
		EvidenceRefs: []curriculum.EvidenceRef{reference}, Reason: "Invalid cross-competency support.",
	}}
	_, err := NewTheoryCoverageV1().Analyze(context.Background(), request)
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "outside competency") {
		t.Fatalf("Analyze() error = %v", err)
	}
}

func theoryCoverageFixture(t *testing.T) (TheoryCoverageRequest, curriculum.Competency, curriculum.Concept, curriculum.EvidenceRef) {
	t.Helper()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, false)
	concept := graphConcept(t, "concept.http-semantics", false)
	concept.EvidenceRefs = []curriculum.EvidenceRef{reference}
	competency := curriculum.Competency{
		ID: curriculumID(t, "competency.http-theory"), AreaID: curriculumID(t, "area.http"), Area: "HTTP",
		OutcomeID: goal.Outcomes[0].ID, ExpectedLevel: curriculum.CompetencyUnderstand,
		EvidenceRefs: []curriculum.EvidenceRef{reference}, ConceptRefs: []curriculum.ConceptID{concept.ID},
	}
	contract := curriculum.TheoryContract{
		ID: curriculumID(t, "theory.http"), CompetencyID: competency.ID,
		RequiredFacets: curriculum.AllTheoryFacets(), Reason: "Professional HTTP understanding requires the complete conceptual model.",
		EvidenceRefs: []curriculum.EvidenceRef{reference},
	}
	return TheoryCoverageRequest{
		Competencies: curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: goal.ID, Competencies: []curriculum.Competency{competency}},
		Concepts:     []curriculum.Concept{concept}, ImportantCompetencyIDs: []curriculum.ID{competency.ID},
		Contracts: []curriculum.TheoryContract{contract}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	}, competency, concept, reference
}

func theoryFacetStatus(t *testing.T, coverage curriculum.TheoryCompetencyCoverage, facet curriculum.TheoryFacet) curriculum.CoverageStatus {
	t.Helper()
	for _, result := range coverage.Facets {
		if result.Facet == facet {
			return result.Status
		}
	}
	t.Fatalf("missing facet %s", facet)
	return ""
}
