package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestGoalDecomposerV1SelectsNarrowEvidenceBackedArea(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, false)
	goal.Scope = []string{"http"}
	profile := decompositionProfile(t, reference, goal.Outcomes)
	profile.Areas = append(profile.Areas, curriculum.CompetencyAreaSpec{
		ID: curriculumID(t, "area.databases"), Name: "Databases", Description: "Database foundations.",
		Scopes: []string{"databases"}, OutcomeIDs: []curriculum.ID{goal.Outcomes[0].ID}, EvidenceRefs: []curriculum.EvidenceRef{reference},
	})

	service := NewGoalDecomposerV1()
	result, err := service.Decompose(context.Background(), GoalDecompositionRequest{Goal: goal, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, Profile: profile})
	if err != nil {
		t.Fatalf("Decompose() error = %v", err)
	}
	if len(result.CompetencyAreas) != 1 || result.CompetencyAreas[0].ID.String() != "area.http" || result.AlgorithmVersion != curriculum.GoalDecomposerVersionV1 {
		t.Fatalf("decomposition = %+v", result)
	}
	repeated, err := service.Decompose(context.Background(), GoalDecompositionRequest{Goal: goal, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, Profile: profile})
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("repeated decomposition differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestGoalDecomposerV1IncludesPackDeclaredProfessionalArea(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, true)
	operateID := curriculumID(t, "outcome.operate")
	goal.Outcomes = append(goal.Outcomes, curriculum.GoalOutcome{ID: operateID, Statement: "Operate the service.", EvidenceRefs: []curriculum.EvidenceRef{reference}})
	profile := decompositionProfile(t, reference, goal.Outcomes)
	profile.Areas = append(profile.Areas, curriculum.CompetencyAreaSpec{
		ID: curriculumID(t, "area.operations"), Name: "Operations", Description: "Operate the target system.",
		Scopes: []string{"operations"}, OutcomeIDs: []curriculum.ID{operateID}, RequiredForProfessional: true,
		EvidenceRefs: []curriculum.EvidenceRef{reference},
	})

	result, err := NewGoalDecomposerV1().Decompose(context.Background(), GoalDecompositionRequest{Goal: goal, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, Profile: profile})
	if err != nil {
		t.Fatalf("Decompose() error = %v", err)
	}
	if len(result.CompetencyAreas) != 2 || result.CompetencyAreas[1].Name != "Operations" {
		t.Fatalf("professional decomposition = %+v", result)
	}
}

func TestGoalDecomposerV1RejectsMissingEvidenceAndUnsupportedScope(t *testing.T) {
	t.Parallel()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, false)
	profile := decompositionProfile(t, reference, goal.Outcomes)

	_, err := NewGoalDecomposerV1().Decompose(context.Background(), GoalDecompositionRequest{Goal: goal, Profile: profile})
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "requires evidence") {
		t.Fatalf("no evidence error = %v", err)
	}

	goal.Scope = []string{"unsupported"}
	_, err = NewGoalDecomposerV1().Decompose(context.Background(), GoalDecompositionRequest{Goal: goal, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, Profile: profile})
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "unsupported goal scope") {
		t.Fatalf("unsupported scope error = %v", err)
	}
}

func decompositionEvidence(t *testing.T) (curriculum.CurriculumEvidenceSet, curriculum.EvidenceRef) {
	t.Helper()
	bundleID := curriculumID(t, "bundle.goal")
	claimID := curriculumID(t, "claim.goal")
	sourceID := curriculumID(t, "source.goal")
	verifiedAt, _ := curriculum.NewTimestamp(time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC))
	set := curriculum.CurriculumEvidenceSet{
		Bundle:           curriculum.SourceBundleRef{ID: bundleID, ContentHash: "sha256:" + strings.Repeat("a", 64), AlgorithmVersion: "source-bundle-v1", VerifiedAt: verifiedAt},
		Eligibility:      curriculum.EvidenceReadyForCompile,
		Claims:           []curriculum.CurriculumEvidenceClaim{{ID: claimID, Statement: "The target requires HTTP knowledge.", Scope: "http", Status: curriculum.ConceptCurrent, Confidence: .95, SourceIDs: []curriculum.ID{sourceID}}},
		SourceAuthority:  []curriculum.EvidenceSourceAuthority{{SourceID: sourceID, Role: "primary", TemporalScope: "current"}},
		Freshness:        curriculum.EvidenceFreshness{State: "fresh", Score: 1, LastVerifiedAt: &verifiedAt, Algorithm: "source-bundle-freshness-v1"},
		AlgorithmVersion: curriculum.EvidenceIngestionAlgorithmV1,
	}
	if err := set.Validate(); err != nil {
		t.Fatal(err)
	}
	return set, curriculum.EvidenceRef{BundleID: bundleID, ClaimID: claimID}
}

func decompositionGoal(t *testing.T, evidence curriculum.EvidenceRef, professional bool) curriculum.LearningGoalSpec {
	t.Helper()
	goal := curriculum.LearningGoalSpec{
		ID: curriculumID(t, "goal.http"), Title: "HTTP service", Description: "Understand an HTTP service.", Domain: "custom-domain",
		Outcomes: []curriculum.GoalOutcome{{ID: curriculumID(t, "outcome.explain"), Statement: "Explain the HTTP service.", EvidenceRefs: []curriculum.EvidenceRef{evidence}}},
		Scope:    []string{"http"}, Exclusions: []string{"browser UI"},
	}
	if professional {
		goal.Role = &curriculum.ProfessionalRole{ID: curriculumID(t, "role.operator"), Name: "Service operator", Description: "Operates the service professionally."}
	}
	return goal
}

func decompositionProfile(t *testing.T, evidence curriculum.EvidenceRef, outcomes []curriculum.GoalOutcome) curriculum.DomainProfile {
	t.Helper()
	return curriculum.DomainProfile{
		ID: curriculumID(t, "profile.custom"), Version: "custom-profile-v1", Domain: "custom-domain",
		SupportedScopes: []string{"http", "databases", "operations"},
		Areas: []curriculum.CompetencyAreaSpec{{
			ID: curriculumID(t, "area.http"), Name: "HTTP semantics", Description: "HTTP semantics from the pack.",
			Scopes: []string{"http"}, OutcomeIDs: []curriculum.ID{outcomes[0].ID}, EvidenceRefs: []curriculum.EvidenceRef{evidence},
		}},
	}
}

func curriculumID(t *testing.T, value string) curriculum.ID {
	t.Helper()
	id, err := curriculum.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
