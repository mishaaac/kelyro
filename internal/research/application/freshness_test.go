package application_test

import (
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/freshness"
)

func TestFreshnessRecordFromAssessmentPreservesKnownV1Output(t *testing.T) {
	t.Parallel()
	lastVerified := testTimestamp(t, 10)
	evaluated := testTimestamp(t, 12)
	score := testFreshnessScore(t, .8)
	assessment := freshness.Assessment{
		State: research.FreshnessFresh, Score: score, EvaluatedAt: evaluated,
		LastVerifiedAt:   &lastVerified,
		EffectiveTTLDays: 90, AlgorithmVersion: research.FreshnessAlgorithmV1,
		Reasons: []freshness.Reason{{Code: freshness.ReasonAgeFresh, Detail: "Fixture age is fresh."}},
	}
	record, err := application.FreshnessRecordFromAssessment(testID(t, "claim.fresh"), assessment)
	if err != nil {
		t.Fatalf("FreshnessRecordFromAssessment() error = %v", err)
	}
	if record.State != assessment.State || record.Score.Value() != assessment.Score.Value() ||
		record.AlgorithmVersion != research.FreshnessAlgorithmV1 || record.NextVerifyAt != nil {
		t.Fatalf("record = %+v", record)
	}
}

func TestFreshnessRecordFromAssessmentRejectsUnknownWithoutInventingTimestamp(t *testing.T) {
	t.Parallel()
	assessment := freshness.Assessment{
		State: research.FreshnessUnknown, Score: testFreshnessScore(t, 0),
		EvaluatedAt: testTimestamp(t, 12), AlgorithmVersion: research.FreshnessAlgorithmV1,
		Reasons: []freshness.Reason{{Code: freshness.ReasonMissingLastVerified, Detail: "Missing verification."}},
	}
	_, err := application.FreshnessRecordFromAssessment(testID(t, "claim.unknown"), assessment)
	if err == nil || !strings.Contains(err.Error(), "no persistable last verification") {
		t.Fatalf("unknown conversion error = %v", err)
	}
}
