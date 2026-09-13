package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCurriculumCompilerV1RunsDeterministicPipeline(t *testing.T) {
	t.Parallel()
	request := compilerFixture(t)
	compiler := NewCurriculumCompilerV1()

	result, err := compiler.Compile(context.Background(), request)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	wantPasses := []string{
		"validate-input", "goal-decomposition", "competency-matrix", "concept-candidates",
		"atomize", "granularity", "prerequisite-extraction", "prerequisite-expansion",
		"vocabulary-graph", "knowledge-graph", "hierarchy", "coverage",
		"definition-before-use", "zero-assumption", "temporal-classification",
		"guidance-classification", "gap-scan", "final-review", "compiled-artifact",
	}
	if len(result.Passes) != len(wantPasses) || result.Diagnostics == nil {
		t.Fatalf("passes=%d diagnostics=%v", len(result.Passes), result.Diagnostics != nil)
	}
	for index, name := range wantPasses {
		pass := result.Passes[index]
		if pass.Name != name || !strings.HasPrefix(pass.InputHash, "sha256:") || !strings.HasPrefix(pass.OutputHash, "sha256:") || len(pass.Errors) != 0 {
			t.Fatalf("pass %d = %+v", index, pass)
		}
	}
	if result.Curriculum.ID != request.Metadata.ID || len(result.Curriculum.Concepts) != 1 || len(result.Curriculum.Topics) != 1 {
		t.Fatalf("compiled curriculum = %+v", result.Curriculum)
	}
	if result.Diagnostics.Review == nil || result.Diagnostics.Review.Decision != curriculum.ReviewRejected {
		t.Fatalf("pipeline final review = %+v", result.Diagnostics.Review)
	}

	repeated, err := compiler.Compile(context.Background(), request)
	if err != nil {
		t.Fatalf("repeated Compile() error = %v", err)
	}
	for index := range result.Passes {
		if result.Passes[index].InputHash != repeated.Passes[index].InputHash || result.Passes[index].OutputHash != repeated.Passes[index].OutputHash {
			t.Fatalf("pass %q hashes changed: %+v / %+v", result.Passes[index].Name, result.Passes[index], repeated.Passes[index])
		}
	}
	if !reflect.DeepEqual(result.Curriculum, repeated.Curriculum) || !reflect.DeepEqual(result.Diagnostics, repeated.Diagnostics) {
		t.Fatalf("same input/config produced different artifact")
	}
}

func TestCurriculumCompilerV1RecordsFailingPass(t *testing.T) {
	t.Parallel()
	request := compilerFixture(t)
	request.AtomizationPlans = nil

	result, err := NewCurriculumCompilerV1().Compile(context.Background(), request)
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "no atomization plan") {
		t.Fatalf("Compile() error = %v", err)
	}
	last := result.Passes[len(result.Passes)-1]
	if last.Name != "atomize" || len(last.Errors) != 1 || last.OutputHash == "" {
		t.Fatalf("failed pass = %+v", last)
	}
}

func compilerFixture(t *testing.T) CurriculumCompileRequest {
	t.Helper()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, false)
	profile := decompositionProfile(t, reference, goal.Outcomes)
	candidates, err := NewConceptCandidateExtractorV1().Extract(context.Background(), ConceptCandidateExtractionRequest{EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}})
	if err != nil {
		t.Fatal(err)
	}
	candidate := candidates.Candidates[0]
	conceptID, _ := curriculum.NewConceptID("concept.http")
	hint := curriculum.ConceptAtomizationHint{
		CandidateID: candidate.ID, ConceptID: conceptID, Title: "HTTP", Definition: "HTTP is the target protocol.", Version: "concept-v1",
		Difficulty: curriculum.DifficultyFoundational, Foundational: true, ClaimRefs: candidate.ClaimRefs, Criteria: atomicCriteria(),
	}
	competency := curriculum.Competency{
		ID: curriculumID(t, "competency.http"), AreaID: profile.Areas[0].ID, Area: profile.Areas[0].Name,
		OutcomeID: goal.Outcomes[0].ID, ExpectedLevel: curriculum.CompetencyUnderstand,
		EvidenceRefs: []curriculum.EvidenceRef{reference}, ConceptRefs: []curriculum.ConceptID{conceptID},
	}
	createdAt := evidence.Bundle.VerifiedAt
	curriculumIDValue, _ := curriculum.NewCurriculumID("curriculum.http")
	curriculumVersion, _ := curriculum.NewCurriculumVersion("2026.09.12")
	return CurriculumCompileRequest{
		Input:        curriculum.CompilationInput{Goal: goal, SourceBundles: []curriculum.SourceBundleRef{evidence.Bundle}, RequestedAt: createdAt},
		Config:       curriculum.CompilationConfig{CompilerVersion: curriculum.CurriculumCompilerVersionV1, SourcePolicy: curriculum.SourceReferencesRequired},
		Metadata:     CurriculumBuildMetadata{ID: curriculumIDValue, Version: curriculumVersion, Title: "HTTP curriculum", Description: "A deterministic HTTP curriculum.", CreatedAt: createdAt},
		EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, DomainProfile: profile,
		Competencies:     []curriculum.Competency{competency},
		AtomizationPlans: []AtomizationPlan{{CandidateID: candidate.ID, Criteria: atomicCriteria(), Hints: []curriculum.ConceptAtomizationHint{hint}}},
		AssumptionBaseline: curriculum.AssumptionBaseline{
			ID: curriculumID(t, "baseline.http"), Version: "baseline-v1", DomainProfileID: profile.ID,
			DomainProfileVersion: profile.Version, LearnerProfile: curriculum.LearnerProfileZero,
			Requirements: []curriculum.FoundationRequirement{{ID: curriculumID(t, "foundation.http"), ConceptID: conceptID, TargetCompetencyIDs: []curriculum.ID{competency.ID}, Reason: "HTTP is the declared root.", EvidenceRefs: []curriculum.EvidenceRef{reference}}},
		},
	}
}
