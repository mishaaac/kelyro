package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestProductionCoverageV1CoversOnlyDomainDeclaredCategoriesWithAdequateEvidence(t *testing.T) {
	t.Parallel()
	request, concept, reference := productionCoverageFixture(t, curriculum.EvidenceClaimRequirement, "primary", "current")
	second := curriculum.ProductionRequirement{
		ID: curriculumID(t, "production.performance"), Category: curriculum.ProductionPerformance,
		TargetKind: curriculum.CoverageTargetCompetency, TargetID: request.Competencies.Competencies[0].ID,
		Description: "Explain performance behavior under load.", EvidenceRefs: []curriculum.EvidenceRef{reference},
	}
	request.Requirements = append(request.Requirements, second)
	request.Supports = append(request.Supports, curriculum.ProductionSupport{
		ID: curriculumID(t, "production.support.performance"), RequirementID: second.ID,
		ConceptIDs: []curriculum.ConceptID{concept.ID}, EvidenceRefs: []curriculum.EvidenceRef{reference},
		Reason: "The concept covers evidence-backed performance behavior.",
	})

	report, err := NewProductionCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(report.Results) != 2 || len(report.CoverageRequirements) != 2 || len(report.CoverageSupports) != 2 {
		t.Fatalf("production report = %+v", report)
	}
	for _, result := range report.Results {
		if result.Status != curriculum.CoverageCovered || len(result.AdequateSupportIDs) != 1 {
			t.Fatalf("production result = %+v", result)
		}
	}
	if report.EvidencePolicyVersion != curriculum.ProductionEvidencePolicyVersionV1 || report.AlgorithmVersion != curriculum.ProductionCoverageVersionV1 {
		t.Fatalf("production versions = %+v", report)
	}

	general, err := NewCoverageEngineV1().Analyze(context.Background(), CoverageAnalysisRequest{
		CurriculumID: request.CurriculumID, Goal: request.Goal, Competencies: request.Competencies,
		Concepts: request.Concepts, Requirements: report.CoverageRequirements,
		Supports: report.CoverageSupports, EvidenceSets: request.EvidenceSets,
	})
	if err != nil || coverageDimension(t, general, curriculum.CoverageProduction).Status != curriculum.CoverageCovered {
		t.Fatalf("general coverage bridge = %+v / %v", general, err)
	}

	reordered := request
	reordered.Requirements = []curriculum.ProductionRequirement{request.Requirements[1], request.Requirements[0]}
	reordered.Supports = []curriculum.ProductionSupport{request.Supports[1], request.Supports[0]}
	repeated, err := NewProductionCoverageV1().Analyze(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(report, repeated) {
		t.Fatalf("reordered production coverage differs: %+v / %+v / %v", report, repeated, err)
	}
}

func TestProductionCoverageV1RejectsTutorialOnlyEvidenceAsCoverage(t *testing.T) {
	t.Parallel()
	request, _, _ := productionCoverageFixture(t, curriculum.EvidenceClaimDefinition, "primary", "current")
	report, err := NewProductionCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.Results[0].Status != curriculum.CoverageMissing || len(report.Results[0].RejectedSupportIDs) != 1 || report.Results[0].Reasons[0] != "production_requirement_evidence_inadequate" || len(report.CoverageSupports) != 0 {
		t.Fatalf("production report = %+v", report)
	}
}

func TestProductionCoverageV1RejectsHistoricalAuthorityAsCurrentProductionSupport(t *testing.T) {
	t.Parallel()
	request, _, _ := productionCoverageFixture(t, curriculum.EvidenceClaimRequirement, "historical", "historical")
	report, err := NewProductionCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.Results[0].Status != curriculum.CoverageMissing || len(report.CoverageSupports) != 0 {
		t.Fatalf("production report = %+v", report)
	}
}

func productionCoverageFixture(t *testing.T, kind curriculum.EvidenceClaimKind, role, temporal string) (ProductionCoverageRequest, curriculum.Concept, curriculum.EvidenceRef) {
	t.Helper()
	evidence, reference := decompositionEvidence(t)
	evidence.Claims[0].Kind = kind
	evidence.SourceAuthority[0].Role = role
	evidence.SourceAuthority[0].TemporalScope = temporal
	if err := evidence.Validate(); err != nil {
		t.Fatal(err)
	}
	goal := decompositionGoal(t, reference, false)
	concept := graphConcept(t, "concept.production-failures", false)
	concept.EvidenceRefs = []curriculum.EvidenceRef{reference}
	competency := curriculum.Competency{
		ID: curriculumID(t, "competency.production"), AreaID: curriculumID(t, "area.operations"), Area: "Operations",
		OutcomeID: goal.Outcomes[0].ID, ExpectedLevel: curriculum.CompetencyOperate,
		EvidenceRefs: []curriculum.EvidenceRef{reference}, ConceptRefs: []curriculum.ConceptID{concept.ID},
	}
	curriculumIDValue, err := curriculum.NewCurriculumID("curriculum.production")
	if err != nil {
		t.Fatal(err)
	}
	requirement := curriculum.ProductionRequirement{
		ID: curriculumID(t, "production.failure-modes"), Category: curriculum.ProductionFailureModes,
		TargetKind: curriculum.CoverageTargetCompetency, TargetID: competency.ID,
		Description: "Diagnose production failure modes.", EvidenceRefs: []curriculum.EvidenceRef{reference},
	}
	support := curriculum.ProductionSupport{
		ID: curriculumID(t, "production.support.failures"), RequirementID: requirement.ID,
		ConceptIDs: []curriculum.ConceptID{concept.ID}, EvidenceRefs: []curriculum.EvidenceRef{reference},
		Reason: "The concept covers verified operational failure modes.",
	}
	return ProductionCoverageRequest{
		CurriculumID: curriculumIDValue, Goal: goal,
		Competencies: curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: goal.ID, Competencies: []curriculum.Competency{competency}},
		Concepts:     []curriculum.Concept{concept}, Requirements: []curriculum.ProductionRequirement{requirement},
		Supports: []curriculum.ProductionSupport{support}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	}, concept, reference
}
