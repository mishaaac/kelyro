package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestExpertCoverageReviewerV1ApprovesProfessionalDepth(t *testing.T) {
	t.Parallel()
	request := expertReviewFixture(t, true)
	reviewer := NewExpertCoverageReviewerV1(expertAdvisorStub{notes: []string{"Optional expert spot check."}})

	result, err := reviewer.Review(context.Background(), request)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if !result.Passed || !result.Professional || len(result.Findings) != 0 || len(result.AdvisorNotes) != 1 || result.AlgorithmVersion != curriculum.ExpertCoverageReviewVersionV1 {
		t.Fatalf("review = %+v", result)
	}

	reordered := request
	reordered.Concepts = append([]curriculum.Concept(nil), request.Concepts...)
	for left, right := 0, len(reordered.Concepts)-1; left < right; left, right = left+1, right-1 {
		reordered.Concepts[left], reordered.Concepts[right] = reordered.Concepts[right], reordered.Concepts[left]
	}
	reordered.Competencies.Competencies = append([]curriculum.Competency(nil), request.Competencies.Competencies...)
	for left, right := 0, len(reordered.Competencies.Competencies)-1; left < right; left, right = left+1, right-1 {
		reordered.Competencies.Competencies[left], reordered.Competencies.Competencies[right] = reordered.Competencies.Competencies[right], reordered.Competencies.Competencies[left]
	}
	repeated, err := reviewer.Review(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered review differs: %v\nfirst=%+v\nsecond=%+v", err, result, repeated)
	}
}

func TestExpertCoverageReviewerV1FindsAdvancedDepthAndProductionGaps(t *testing.T) {
	t.Parallel()
	request := expertReviewFixture(t, false)

	result, err := NewExpertCoverageReviewerV1(nil).Review(context.Background(), request)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if result.Passed || len(result.Findings) != 8 {
		t.Fatalf("review = %+v", result)
	}
	counts := map[curriculum.ExpertCoverageFindingKind]int{}
	for _, finding := range result.Findings {
		counts[finding.Kind]++
	}
	if counts[curriculum.ExpertMissingAdvancedCompetency] != 4 || counts[curriculum.ExpertInsufficientDepth] != 1 || counts[curriculum.ExpertMissingProductionCapability] != 3 {
		t.Fatalf("finding counts = %v", counts)
	}
}

type expertAdvisorStub struct{ notes []string }

func (stub expertAdvisorStub) Advise(context.Context, ExpertCoverageReviewRequest, curriculum.ExpertCoverageReviewResult) ([]string, error) {
	return stub.notes, nil
}

func expertReviewFixture(t *testing.T, complete bool) ExpertCoverageReviewRequest {
	t.Helper()
	_, evidence := decompositionEvidence(t)
	goal := decompositionGoal(t, evidence, true)
	goal.Outcomes = professionalOutcomes(t, evidence)
	levels := []curriculum.CompetencyLevel{
		curriculum.CompetencyUnderstand,
		curriculum.CompetencyApply,
		curriculum.CompetencyAnalyze,
		curriculum.CompetencyOperate,
		curriculum.CompetencyAnalyze,
	}
	difficulties := []curriculum.Difficulty{
		curriculum.DifficultyFoundational,
		curriculum.DifficultyIntermediate,
		curriculum.DifficultyAdvanced,
		curriculum.DifficultyAdvanced,
		curriculum.DifficultyAdvanced,
	}
	concepts := make([]curriculum.Concept, len(goal.Outcomes))
	competencies := make([]curriculum.Competency, len(goal.Outcomes))
	for index, outcome := range goal.Outcomes {
		concepts[index] = graphConcept(t, "concept.expert."+string(rune('a'+index)), index == 0)
		concepts[index].Difficulty = difficulties[index]
		competencies[index] = curriculum.Competency{
			ID:     curriculumID(t, "competency.expert."+string(rune('a'+index))),
			AreaID: curriculumID(t, "area.expert"), Area: "Expert", OutcomeID: outcome.ID,
			ExpectedLevel: levels[index], ConceptRefs: []curriculum.ConceptID{concepts[index].ID},
		}
	}

	coverageRequest := completeCompilerFixture(t)
	coverageCompilation, err := NewCurriculumCompilerV1().Compile(context.Background(), coverageRequest)
	if err != nil {
		t.Fatal(err)
	}
	coverage := coverageCompilation.Diagnostics.Coverage
	if !complete {
		for index := range competencies {
			competencies[index].ExpectedLevel = curriculum.CompetencyAwareness
			concepts[index].Difficulty = curriculum.DifficultyIntroductory
		}
		competencies[3].ExpectedLevel = curriculum.CompetencyOperate
		missingRequest := compilerFixture(t)
		missingCompilation, compileErr := NewCurriculumCompilerV1().Compile(context.Background(), missingRequest)
		if compileErr != nil {
			t.Fatal(compileErr)
		}
		coverage = missingCompilation.Diagnostics.Coverage
	}
	return ExpertCoverageReviewRequest{
		Goal:         goal,
		Competencies: curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: goal.ID, Competencies: competencies},
		Concepts:     concepts,
		Coverage:     coverage,
	}
}
