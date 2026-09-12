package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestGapScannerV1EmitsEveryActionableGapKind(t *testing.T) {
	t.Parallel()
	coverageRequest := allMissingCoverageFixture(t)
	coverageReport, err := NewCoverageEngineV1().Analyze(context.Background(), coverageRequest)
	if err != nil {
		t.Fatal(err)
	}
	missingRequired, err := curriculum.NewConceptID("concept.foundation")
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := curriculum.NewConceptID("concept.target")
	if err != nil {
		t.Fatal(err)
	}
	kind := curriculum.PrerequisiteHard
	reference := coverageRequest.Requirements[0].EvidenceRefs[0]
	request := GapScanRequest{
		GoalID: coverageRequest.Goal.ID, Coverage: coverageReport, Requirements: coverageRequest.Requirements,
		PrerequisiteGaps: []curriculum.PrerequisiteExpansionGap{{
			ConceptID: dependent, RequiredConceptID: &missingRequired, Kind: &kind,
			Code: curriculum.PrerequisiteGapConceptUnavailable, Reason: "Required foundation is unavailable.", EvidenceRefs: []curriculum.EvidenceRef{reference},
		}},
		CurrentGuidanceFindings: []curriculum.CurrentGuidanceFinding{{
			TargetID: coverageRequest.Goal.ID, Reason: "No verified current recommendation covers the target.", EvidenceRefs: []curriculum.EvidenceRef{reference},
		}},
	}
	result, err := NewGapScannerV1().Scan(context.Background(), request)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result.AlgorithmVersion != curriculum.GapScannerVersionV1 || len(result.Gaps) != 10 {
		t.Fatalf("gap report = %+v", result)
	}
	wantKinds := map[curriculum.GapKind]curriculum.GapSeverity{
		curriculum.GapMissingCompetency:      curriculum.GapBlocking,
		curriculum.GapMissingConcept:         curriculum.GapBlocking,
		curriculum.GapMissingPrerequisite:    curriculum.GapBlocking,
		curriculum.GapMissingEvidence:        curriculum.GapBlocking,
		curriculum.GapMissingTheory:          curriculum.GapImportant,
		curriculum.GapMissingPractice:        curriculum.GapImportant,
		curriculum.GapMissingProduction:      curriculum.GapImportant,
		curriculum.GapMissingToolchain:       curriculum.GapImportant,
		curriculum.GapMissingSecurity:        curriculum.GapBlocking,
		curriculum.GapMissingCurrentGuidance: curriculum.GapImportant,
	}
	for _, gap := range result.Gaps {
		severity, exists := wantKinds[gap.Kind]
		if !exists {
			t.Fatalf("unexpected gap kind %q", gap.Kind)
		}
		if gap.Severity != severity || gap.Reason == "" {
			t.Fatalf("gap = %+v, want severity %q", gap, severity)
		}
		delete(wantKinds, gap.Kind)
	}
	if len(wantKinds) != 0 {
		t.Fatalf("missing gap kinds = %v", wantKinds)
	}

	reordered := request
	reordered.Requirements = append([]curriculum.CoverageRequirement(nil), request.Requirements...)
	for left, right := 0, len(reordered.Requirements)-1; left < right; left, right = left+1, right-1 {
		reordered.Requirements[left], reordered.Requirements[right] = reordered.Requirements[right], reordered.Requirements[left]
	}
	repeated, err := NewGapScannerV1().Scan(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered scan differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestGapScannerV1DowngradesPartialCoverageSeverity(t *testing.T) {
	t.Parallel()
	coverageRequest := coverageFixture(t)
	report, err := NewCoverageEngineV1().Analyze(context.Background(), coverageRequest)
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewGapScannerV1().Scan(context.Background(), GapScanRequest{
		GoalID: coverageRequest.Goal.ID, Coverage: report, Requirements: coverageRequest.Requirements,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, gap := range result.Gaps {
		switch gap.Kind {
		case curriculum.GapMissingCompetency, curriculum.GapMissingConcept:
			if gap.Severity != curriculum.GapImportant || !strings.Contains(gap.Reason, "partial coverage") {
				t.Fatalf("partial structural gap = %+v", gap)
			}
		case curriculum.GapMissingPractice:
			if strings.Contains(gap.Reason, "partial coverage") && gap.Severity != curriculum.GapRecommended {
				t.Fatalf("partial practice gap = %+v", gap)
			}
		}
	}
}

func TestGapScannerV1RejectsCoverageRequirementMismatch(t *testing.T) {
	t.Parallel()
	coverageRequest := coverageFixture(t)
	report, err := NewCoverageEngineV1().Analyze(context.Background(), coverageRequest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewGapScannerV1().Scan(context.Background(), GapScanRequest{
		GoalID: coverageRequest.Goal.ID, Coverage: report, Requirements: coverageRequest.Requirements[:len(coverageRequest.Requirements)-1],
	})
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "missing requirement") {
		t.Fatalf("Scan() error = %v", err)
	}
}

func allMissingCoverageFixture(t *testing.T) CoverageAnalysisRequest {
	t.Helper()
	request := coverageFixture(t)
	reference := request.Requirements[0].EvidenceRefs[0]
	missingTarget := curriculumID(t, "competency.absent")
	request.Requirements = nil
	for _, dimension := range curriculum.AllCoverageDimensions() {
		request.Requirements = append(request.Requirements, coverageRequirement(
			t, "coverage.missing."+string(dimension), dimension,
			curriculum.CoverageTargetCompetency, missingTarget, reference,
		))
	}
	request.Supports = nil
	return request
}
