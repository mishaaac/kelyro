package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestGuidanceClassifierV1DistinguishesAllGuidanceTypesWithEvidence(t *testing.T) {
	t.Parallel()
	request, concepts := guidanceClassificationFixture(t)
	temporal, err := NewTemporalClassificationV1().Classify(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	report, err := NewGuidanceClassifierV1().Classify(context.Background(), GuidanceClassificationRequest{
		TemporalResult: temporal, EvidenceSets: request.EvidenceSets,
	})
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	want := map[string]curriculum.GuidanceType{
		concepts[0].ID.String(): curriculum.GuidanceRecommendedCurrent,
		concepts[1].ID.String(): curriculum.GuidanceExperimental,
		concepts[2].ID.String(): curriculum.GuidanceHistoricalContext,
		concepts[3].ID.String(): curriculum.GuidanceLegacyMaintenance,
		concepts[4].ID.String(): curriculum.GuidanceAvoid,
		concepts[5].ID.String(): curriculum.GuidanceExperimental,
		concepts[6].ID.String(): curriculum.GuidanceAcceptableCurrent,
	}
	seenTypes := make(map[curriculum.GuidanceType]struct{})
	for _, classification := range report.Classifications {
		if len(classification.EvidenceRefs) == 0 {
			t.Fatalf("classification without evidence = %+v", classification)
		}
		if expected, exists := want[classification.TargetID.String()]; exists {
			if classification.Guidance != expected {
				t.Fatalf("guidance for %s = %s, want %s", classification.TargetID, classification.Guidance, expected)
			}
			seenTypes[classification.Guidance] = struct{}{}
		}
	}
	if len(seenTypes) != 6 || report.AlgorithmVersion != curriculum.GuidanceClassifierVersionV1 {
		t.Fatalf("guidance types = %+v / report = %+v", seenTypes, report)
	}
	if len(report.CurrentGuidanceFindings) != 1 || report.CurrentGuidanceFindings[0].TargetID.String() != concepts[4].ID.String() {
		t.Fatalf("current-guidance findings = %+v", report.CurrentGuidanceFindings)
	}

	reorderedTemporal := temporal
	reorderedTemporal.Classifications = append([]curriculum.TemporalClassification(nil), temporal.Classifications...)
	for left, right := 0, len(reorderedTemporal.Classifications)-1; left < right; left, right = left+1, right-1 {
		reorderedTemporal.Classifications[left], reorderedTemporal.Classifications[right] = reorderedTemporal.Classifications[right], reorderedTemporal.Classifications[left]
	}
	repeated, err := NewGuidanceClassifierV1().Classify(context.Background(), GuidanceClassificationRequest{
		TemporalResult: reorderedTemporal, EvidenceSets: request.EvidenceSets,
	})
	if err != nil || !reflect.DeepEqual(report, repeated) {
		t.Fatalf("reordered guidance differs: %+v / %+v / %v", report, repeated, err)
	}
}

func TestGuidanceClassifierV1ReportsMissingCurrentAlternativeForUnscopedHistoricalAndLegacy(t *testing.T) {
	t.Parallel()
	request, concepts := guidanceClassificationFixture(t)
	request.ContextualConceptIDs = nil
	temporal, err := NewTemporalClassificationV1().Classify(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	report, err := NewGuidanceClassifierV1().Classify(context.Background(), GuidanceClassificationRequest{
		TemporalResult: temporal, EvidenceSets: request.EvidenceSets,
	})
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	want := map[string]struct{}{concepts[2].ID.String(): {}, concepts[3].ID.String(): {}, concepts[4].ID.String(): {}}
	if len(report.CurrentGuidanceFindings) != len(want) {
		t.Fatalf("current-guidance findings = %+v", report.CurrentGuidanceFindings)
	}
	for _, finding := range report.CurrentGuidanceFindings {
		if _, exists := want[finding.TargetID.String()]; !exists || len(finding.EvidenceRefs) == 0 {
			t.Fatalf("unexpected finding = %+v", finding)
		}
	}
}

func guidanceClassificationFixture(t *testing.T) (TemporalClassificationRequest, []curriculum.Concept) {
	t.Helper()
	request, concepts := temporalClassificationFixture(t)
	request.EvidenceSets[0].Claims[0].Kind = curriculum.EvidenceClaimRecommendation
	currentSource := request.EvidenceSets[0].SourceAuthority[0].SourceID
	claim := curriculum.CurriculumEvidenceClaim{
		ID: curriculumID(t, "claim.temporal.acceptable"), Statement: "The behavior is currently supported.",
		Kind: curriculum.EvidenceClaimBehavior, Scope: "acceptable current behavior", Status: curriculum.ConceptCurrent,
		Confidence: .9, SourceIDs: []curriculum.ID{currentSource},
	}
	request.EvidenceSets[0].Claims = append(request.EvidenceSets[0].Claims, claim)
	concept := graphConcept(t, "concept.temporal.acceptable", false)
	concept.Status = curriculum.ConceptCurrent
	concept.EvidenceRefs = []curriculum.EvidenceRef{{BundleID: request.EvidenceSets[0].Bundle.ID, ClaimID: claim.ID}}
	request.Concepts = append(request.Concepts, concept)
	concepts = append(concepts, concept)
	if err := request.EvidenceSets[0].Validate(); err != nil {
		t.Fatal(err)
	}
	return request, concepts
}
