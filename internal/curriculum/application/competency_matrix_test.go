package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCompetencyMatrixBuilderV1BuildsCompleteEvidenceBackedMatrix(t *testing.T) {
	t.Parallel()
	request := competencyMatrixFixture(t)
	service := NewCompetencyMatrixBuilderV1()
	result, err := service.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.Version != curriculum.CompetencyMatrixVersionV1 || result.GoalID != request.Decomposition.GoalID || len(result.Competencies) != 2 || len(result.Competencies[0].Dimensions) != 2 {
		t.Fatalf("matrix = %+v", result)
	}
	repeated, err := service.Build(context.Background(), request)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("repeated matrix differs: %+v / %+v / %v", result, repeated, err)
	}
	request.Competencies[0].Dimensions[0].ExpectedLevel = curriculum.CompetencyAwareness
	if result.Competencies[0].Dimensions[0].ExpectedLevel != curriculum.CompetencyUnderstand {
		t.Fatal("builder output aliases input dimensions")
	}
}

func TestCompetencyMatrixBuilderV1RejectsIncompleteDuplicateAndUnsupportedSpecs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(*CompetencyMatrixRequest)
		want string
	}{
		{"incomplete", func(request *CompetencyMatrixRequest) { request.Competencies = request.Competencies[:1] }, "not covered by a competency"},
		{"duplicate", func(request *CompetencyMatrixRequest) {
			request.Competencies = append(request.Competencies, request.Competencies[0])
		}, "duplicate competency"},
		{"unsupported competency", func(request *CompetencyMatrixRequest) {
			request.Competencies[0].AreaID = curriculumID(t, "area.unknown")
		}, "unsupported area"},
		{"no evidence", func(request *CompetencyMatrixRequest) { request.Competencies[0].EvidenceRefs = nil }, "has no evidence"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request := competencyMatrixFixture(t)
			test.edit(&request)
			_, err := NewCompetencyMatrixBuilderV1().Build(context.Background(), request)
			if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Build() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCompetencyMatrixBuilderV1ValidatesOptionalHierarchy(t *testing.T) {
	t.Parallel()
	request := competencyMatrixFixture(t)
	parent := request.Competencies[0].ID
	child := request.Competencies[0]
	child.ID = curriculumID(t, "competency.http.details")
	child.ParentID = &parent
	request.Competencies = append(request.Competencies, child)
	if _, err := NewCompetencyMatrixBuilderV1().Build(context.Background(), request); err != nil {
		t.Fatalf("valid competency hierarchy rejected: %v", err)
	}

	cycleParent := child.ID
	request.Competencies[0].ParentID = &cycleParent
	_, err := NewCompetencyMatrixBuilderV1().Build(context.Background(), request)
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("cycle error = %v", err)
	}
}

func competencyMatrixFixture(t *testing.T) CompetencyMatrixRequest {
	t.Helper()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, false)
	secondOutcome := curriculum.GoalOutcome{
		ID: curriculumID(t, "outcome.apply"), Statement: "Apply HTTP semantics.",
		Category: curriculum.OutcomeApplication, Capability: curriculum.OutcomeCapabilityBuild,
		EvidenceRefs: []curriculum.EvidenceRef{reference},
	}
	goal.Outcomes = append(goal.Outcomes, secondOutcome)
	profile := decompositionProfile(t, reference, goal.Outcomes)
	profile.Areas = append(profile.Areas, curriculum.CompetencyAreaSpec{
		ID: curriculumID(t, "area.http-application"), Name: "HTTP application", Description: "Apply HTTP semantics.",
		Scopes: []string{"http"}, OutcomeIDs: []curriculum.ID{secondOutcome.ID}, EvidenceRefs: []curriculum.EvidenceRef{reference},
	})
	decomposition, err := NewGoalDecomposerV1().Decompose(context.Background(), GoalDecompositionRequest{Goal: goal, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, Profile: profile})
	if err != nil {
		t.Fatal(err)
	}
	return CompetencyMatrixRequest{
		Decomposition: decomposition,
		EvidenceSets:  []curriculum.CurriculumEvidenceSet{evidence},
		Competencies: []curriculum.Competency{
			{
				ID: curriculumID(t, "competency.http.explain"), AreaID: profile.Areas[0].ID, Area: profile.Areas[0].Name,
				OutcomeID: goal.Outcomes[0].ID, ExpectedLevel: curriculum.CompetencyUnderstand,
				Dimensions: []curriculum.CompetencyDimension{
					{ID: curriculumID(t, "dimension.theory"), ExpectedLevel: curriculum.CompetencyUnderstand},
					{ID: curriculumID(t, "dimension.communication"), ExpectedLevel: curriculum.CompetencyExplain},
				},
				EvidenceRefs: []curriculum.EvidenceRef{reference},
			},
			{
				ID: curriculumID(t, "competency.http.apply"), AreaID: profile.Areas[1].ID, Area: profile.Areas[1].Name,
				OutcomeID: secondOutcome.ID, ExpectedLevel: curriculum.CompetencyApply,
				EvidenceRefs: []curriculum.EvidenceRef{reference},
			},
		},
	}
}
