package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestSecurityCoverageV1CoversDomainDeclaredCategoriesWithVerifiedEvidence(t *testing.T) {
	t.Parallel()
	request, concept, reference := securityCoverageFixture(t, true)
	second := curriculum.SecurityRequirement{
		ID: curriculumID(t, "security.secure-defaults"), Category: curriculum.SecuritySecureDefaults,
		TargetKind: curriculum.CoverageTargetCompetency, TargetID: request.Competencies.Competencies[0].ID,
		Description: "Use secure defaults for the service.", EvidenceRefs: []curriculum.EvidenceRef{reference},
	}
	request.Requirements = append(request.Requirements, second)
	request.Supports = append(request.Supports, curriculum.SecuritySupport{
		ID: curriculumID(t, "security.support.defaults"), RequirementID: second.ID,
		ConceptIDs: []curriculum.ConceptID{concept.ID}, EvidenceRefs: []curriculum.EvidenceRef{reference},
		Reason: "The concept covers verified secure-default guidance.",
	})

	report, err := NewSecurityCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(report.Results) != 2 || len(report.CoverageRequirements) != 2 || len(report.CoverageSupports) != 2 {
		t.Fatalf("security report = %+v", report)
	}
	for _, result := range report.Results {
		if result.Status != curriculum.CoverageCovered || len(result.VerifiedSupportIDs) != 1 {
			t.Fatalf("security result = %+v", result)
		}
	}
	if report.EvidencePolicyVersion != curriculum.SecurityEvidencePolicyVersionV1 || report.AlgorithmVersion != curriculum.SecurityCoverageVersionV1 {
		t.Fatalf("security versions = %+v", report)
	}

	general, err := NewCoverageEngineV1().Analyze(context.Background(), CoverageAnalysisRequest{
		CurriculumID: request.CurriculumID, Goal: request.Goal, Competencies: request.Competencies,
		Concepts: request.Concepts, Requirements: report.CoverageRequirements,
		Supports: report.CoverageSupports, EvidenceSets: request.EvidenceSets,
	})
	if err != nil || coverageDimension(t, general, curriculum.CoverageSecurity).Status != curriculum.CoverageCovered {
		t.Fatalf("general coverage bridge = %+v / %v", general, err)
	}

	reordered := request
	reordered.Requirements = []curriculum.SecurityRequirement{request.Requirements[1], request.Requirements[0]}
	reordered.Supports = []curriculum.SecuritySupport{request.Supports[1], request.Supports[0]}
	repeated, err := NewSecurityCoverageV1().Analyze(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(report, repeated) {
		t.Fatalf("reordered security coverage differs: %+v / %+v / %v", report, repeated, err)
	}
}

func TestSecurityCoverageV1RejectsSingleSourceAsSensitiveVerification(t *testing.T) {
	t.Parallel()
	request, _, _ := securityCoverageFixture(t, false)
	report, err := NewSecurityCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.Results[0].Status != curriculum.CoverageMissing || len(report.Results[0].RejectedSupportIDs) != 1 || report.Results[0].Reasons[0] != "security_requirement_evidence_unverified" || len(report.CoverageSupports) != 0 {
		t.Fatalf("security report = %+v", report)
	}
}

func TestSecurityCoverageV1RejectsCaveatedBundle(t *testing.T) {
	t.Parallel()
	request, _, _ := securityCoverageFixture(t, true)
	request.EvidenceSets[0].Eligibility = curriculum.EvidenceReadyWithCaveats
	request.EvidenceSets[0].Caveats = []string{"security verification incomplete"}
	report, err := NewSecurityCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.Results[0].Status != curriculum.CoverageMissing || len(report.CoverageSupports) != 0 {
		t.Fatalf("security report = %+v", report)
	}
}

func securityCoverageFixture(t *testing.T, multiSource bool) (SecurityCoverageRequest, curriculum.Concept, curriculum.EvidenceRef) {
	t.Helper()
	evidence, reference := decompositionEvidence(t)
	evidence.Claims[0].Kind = curriculum.EvidenceClaimSecurity
	if multiSource {
		secondSource := curriculumID(t, "source.security.supporting")
		evidence.SourceAuthority = append(evidence.SourceAuthority, curriculum.EvidenceSourceAuthority{
			SourceID: secondSource, Role: "supporting", TemporalScope: "current",
		})
		evidence.Claims[0].SourceIDs = append(evidence.Claims[0].SourceIDs, secondSource)
	}
	if err := evidence.Validate(); err != nil {
		t.Fatal(err)
	}
	goal := decompositionGoal(t, reference, false)
	concept := graphConcept(t, "concept.input-validation", false)
	concept.EvidenceRefs = []curriculum.EvidenceRef{reference}
	competency := curriculum.Competency{
		ID: curriculumID(t, "competency.secure-input"), AreaID: curriculumID(t, "area.security"), Area: "Security",
		OutcomeID: goal.Outcomes[0].ID, ExpectedLevel: curriculum.CompetencyApply,
		EvidenceRefs: []curriculum.EvidenceRef{reference}, ConceptRefs: []curriculum.ConceptID{concept.ID},
	}
	curriculumIDValue, err := curriculum.NewCurriculumID("curriculum.security")
	if err != nil {
		t.Fatal(err)
	}
	requirement := curriculum.SecurityRequirement{
		ID: curriculumID(t, "security.input-validation"), Category: curriculum.SecurityInputValidation,
		TargetKind: curriculum.CoverageTargetCompetency, TargetID: competency.ID,
		Description: "Validate untrusted input.", EvidenceRefs: []curriculum.EvidenceRef{reference},
	}
	support := curriculum.SecuritySupport{
		ID: curriculumID(t, "security.support.input"), RequirementID: requirement.ID,
		ConceptIDs: []curriculum.ConceptID{concept.ID}, EvidenceRefs: []curriculum.EvidenceRef{reference},
		Reason: "The concept covers verified input-validation guidance.",
	}
	return SecurityCoverageRequest{
		CurriculumID: curriculumIDValue, Goal: goal,
		Competencies: curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: goal.ID, Competencies: []curriculum.Competency{competency}},
		Concepts:     []curriculum.Concept{concept}, Requirements: []curriculum.SecurityRequirement{requirement},
		Supports: []curriculum.SecuritySupport{support}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	}, concept, reference
}
