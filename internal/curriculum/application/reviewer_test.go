package application

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCurriculumReviewerV1ApprovesCompleteCompilation(t *testing.T) {
	t.Parallel()
	request := completeCompilerFixture(t)
	compilation, err := NewCurriculumCompilerV1().Compile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	reviewer := NewCurriculumReviewerV1(nil)
	review, err := reviewer.Review(context.Background(), CurriculumReviewRequest{Compilation: compilation, EvidenceSets: request.EvidenceSets})
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if review.Decision != curriculum.ReviewApproved || len(review.Dimensions) != 13 || len(review.AdvisorNotes) != 0 {
		t.Fatalf("review = %+v", review)
	}
	for _, dimension := range review.Dimensions {
		if !dimension.Passed || len(dimension.Findings) != 0 {
			t.Fatalf("dimension = %+v", dimension)
		}
	}
	repeated, err := reviewer.Review(context.Background(), CurriculumReviewRequest{Compilation: compilation, EvidenceSets: request.EvidenceSets})
	if err != nil || !reflect.DeepEqual(review, repeated) {
		t.Fatalf("repeated review differs: %+v / %+v / %v", review, repeated, err)
	}
}

func TestCurriculumReviewerV1WarnsWithoutChangingAdvisorAuthority(t *testing.T) {
	t.Parallel()
	request := completeCompilerFixture(t)
	compilation, err := NewCurriculumCompilerV1().Compile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	evidence := append([]curriculum.CurriculumEvidenceSet(nil), request.EvidenceSets...)
	evidence[0].Eligibility = curriculum.EvidenceReadyWithCaveats
	evidence[0].Caveats = []string{"One non-blocking source caveat."}
	evidence[0].Freshness.State = "aging"
	evidence[0].Freshness.Score = .7
	reviewer := NewCurriculumReviewerV1(reviewAdvisorStub{notes: []string{"Consider an expert spot check."}})

	review, err := reviewer.Review(context.Background(), CurriculumReviewRequest{Compilation: compilation, EvidenceSets: evidence})
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if review.Decision != curriculum.ReviewApprovedWithWarnings || len(review.AdvisorNotes) != 1 {
		t.Fatalf("review = %+v", review)
	}
	if !reviewDimension(t, review, curriculum.ReviewSourceReadiness).Passed || !reviewDimension(t, review, curriculum.ReviewFreshness).Passed {
		t.Fatalf("warning-only dimensions should pass")
	}
}

func TestCurriculumReviewerV1RejectsBlockingCoverageAndIgnoresAdvisorFailure(t *testing.T) {
	t.Parallel()
	request := compilerFixture(t)
	compilation, err := NewCurriculumCompilerV1().Compile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	reviewer := NewCurriculumReviewerV1(reviewAdvisorStub{err: errors.New("offline")})

	review, err := reviewer.Review(context.Background(), CurriculumReviewRequest{Compilation: compilation, EvidenceSets: request.EvidenceSets})
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if review.Decision != curriculum.ReviewRejected || reviewDimension(t, review, curriculum.ReviewCoverage).Passed || reviewDimension(t, review, curriculum.ReviewSecurity).Passed || len(review.AdvisorNotes) != 1 {
		t.Fatalf("review = %+v", review)
	}
}

type reviewAdvisorStub struct {
	notes []string
	err   error
}

func (stub reviewAdvisorStub) Advise(context.Context, CurriculumReviewRequest, curriculum.CurriculumReviewResult) ([]string, error) {
	return stub.notes, stub.err
}

func completeCompilerFixture(t *testing.T) CurriculumCompileRequest {
	t.Helper()
	request := compilerFixture(t)
	reference := request.EvidenceSets[0].Bundle
	evidence := curriculum.EvidenceRef{BundleID: reference.ID, ClaimID: request.EvidenceSets[0].Claims[0].ID}
	competencyID := request.Competencies[0].ID
	conceptID := request.Competencies[0].ConceptRefs[0]
	conceptTarget, _ := curriculum.NewID(conceptID.String())
	targets := []struct {
		dimension curriculum.CoverageDimension
		kind      curriculum.CoverageTargetKind
		target    curriculum.ID
	}{
		{curriculum.CoverageCompetency, curriculum.CoverageTargetGoal, request.Input.Goal.ID},
		{curriculum.CoverageConcept, curriculum.CoverageTargetCompetency, competencyID},
		{curriculum.CoverageEvidence, curriculum.CoverageTargetConcept, conceptTarget},
		{curriculum.CoverageTheory, curriculum.CoverageTargetCompetency, competencyID},
		{curriculum.CoveragePractice, curriculum.CoverageTargetCompetency, competencyID},
		{curriculum.CoverageProduction, curriculum.CoverageTargetCompetency, competencyID},
		{curriculum.CoverageSecurity, curriculum.CoverageTargetCompetency, competencyID},
		{curriculum.CoverageToolchain, curriculum.CoverageTargetCompetency, competencyID},
	}
	for index, target := range targets {
		requirementID := curriculumID(t, "review.coverage."+string(target.dimension))
		request.CoverageRequirements = append(request.CoverageRequirements, curriculum.CoverageRequirement{ID: requirementID, Dimension: target.dimension, TargetKind: target.kind, TargetID: target.target, Description: "Publication coverage requirement.", EvidenceRefs: []curriculum.EvidenceRef{evidence}})
		request.CoverageSupports = append(request.CoverageSupports, curriculum.CoverageSupport{ID: curriculumID(t, "review.support."+string(rune('a'+index))), RequirementID: requirementID, ConceptIDs: []curriculum.ConceptID{conceptID}, EvidenceRefs: []curriculum.EvidenceRef{evidence}, Reason: "Verified support for publication review."})
	}
	return request
}

func reviewDimension(t *testing.T, result curriculum.CurriculumReviewResult, target curriculum.CurriculumReviewDimension) curriculum.CurriculumReviewDimensionResult {
	t.Helper()
	for _, dimension := range result.Dimensions {
		if dimension.Dimension == target {
			return dimension
		}
	}
	t.Fatalf("review dimension %q not found", target)
	return curriculum.CurriculumReviewDimensionResult{}
}
