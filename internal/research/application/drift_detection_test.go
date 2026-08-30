package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestDriftServiceDetectsVersionedReportsBeforeExplicitPersistence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	service := application.NewDriftService(repositories.Drift)
	oldClaim, oldBundle := driftDetectionFixture(t, "old", "Use stable API.", "1.0.0", 10)
	newClaim := oldClaim
	newClaim.Statement = "Use current API."
	newVersion := testVersion(t, "2.0.0")
	newClaim.VersionScope = &newVersion
	newClaim.EvidenceIDs = []research.ID{testID(t, "evidence.drift-new")}
	newClaim.CreatedAt = testTimestamp(t, 12)
	newBundle := testDriftDetectionBundle(t, "new", newClaim, 13)
	request := application.DriftDetectionRequest{
		OldBundle: oldBundle, NewBundle: &newBundle,
		OldClaims: []research.Claim{oldClaim}, NewClaims: []research.Claim{newClaim},
		DetectedAt: testTimestamp(t, 14),
	}
	result, err := service.Detect(ctx, request)
	if err != nil || len(result.Reports) != 2 {
		t.Fatalf("Detect() = (%+v, %v)", result, err)
	}
	for _, report := range result.Reports {
		if report.AlgorithmVersion != research.DriftAlgorithmV1 || report.Confidence.Value() == 0 || report.NewBundleID == nil || *report.NewBundleID != newBundle.ID {
			t.Fatalf("detected report = %+v", report)
		}
		if _, err := service.Get(ctx, report.ID); !errors.Is(err, application.ErrNotFound) {
			t.Fatalf("Detect persisted report %s: %v", report.ID, err)
		}
		if err := service.Record(ctx, report); err != nil {
			t.Fatal(err)
		}
		stored, err := service.Get(ctx, report.ID)
		if err != nil || stored.AlgorithmVersion != research.DriftAlgorithmV1 || stored.Confidence.Value() != report.Confidence.Value() {
			t.Fatalf("stored report = (%+v, %v)", stored, err)
		}
	}
	again, err := service.Detect(ctx, request)
	if err != nil || again.Reports[0].ID != result.Reports[0].ID || again.Reports[1].ID != result.Reports[1].ID {
		t.Fatalf("deterministic Detect() = (%+v, %v)", again, err)
	}
}

func TestDriftServicePreservesUnresolvedComparisonWithoutInventingReport(t *testing.T) {
	t.Parallel()
	service := application.NewDriftService(memory.New().Repositories().Drift)
	claim, bundle := driftDetectionFixture(t, "unresolved", "Use stable API.", "1.0.0", 10)
	result, err := service.Detect(context.Background(), application.DriftDetectionRequest{
		OldBundle: bundle, OldClaims: []research.Claim{claim}, DetectedAt: testTimestamp(t, 14),
	})
	if err != nil || len(result.Reports) != 0 || len(result.UnresolvedClaims) != 1 || result.UnresolvedClaims[0] != claim.ID {
		t.Fatalf("unresolved Detect() = (%+v, %v)", result, err)
	}
	legacy := testDrift(t)
	legacy.AlgorithmVersion = research.DriftLegacyAlgorithm
	legacy.Confidence = research.ClaimConfidence{}
	if err := service.Record(context.Background(), legacy); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("Record(legacy) error = %v, want invalid_state", err)
	}
}

func driftDetectionFixture(t *testing.T, suffix, statement, versionValue string, hour int) (research.Claim, research.SourceBundle) {
	t.Helper()
	topic, err := research.NewResearchTopic("Drift fixture", "software", "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := testVersion(t, versionValue)
	sourceID := testSourceID(t, "drift-"+suffix)
	claim := research.Claim{
		ID: testClaimID(t, "drift-stable"), Topic: topic, Statement: statement,
		Type: research.ClaimBehavior, Scope: "Fixture API", VersionScope: &version,
		StatusScope: research.ClaimStatusStable, Confidence: testConfidence(t, .9),
		SourceIDs: []research.SourceID{sourceID}, EvidenceIDs: []research.ID{testID(t, "evidence.drift-"+suffix)},
		CreatedAt: testTimestamp(t, hour),
	}
	return claim, testDriftDetectionBundle(t, suffix, claim, hour+1)
}

func testDriftDetectionBundle(t *testing.T, suffix string, claim research.Claim, hour int) research.SourceBundle {
	t.Helper()
	verifiedAt := testTimestamp(t, hour)
	score := testFreshnessScore(t, 1)
	bundle, err := research.SealSourceBundleV1(research.SourceBundle{
		ID: testID(t, "bundle.drift-"+suffix), RunID: testID(t, "run.drift-"+suffix), Topic: claim.Topic,
		Purpose: research.PurposeCurrentUsage, ClaimIDs: []research.ClaimID{claim.ID},
		Sources: []research.SourceBundleSource{{
			SourceID: claim.SourceIDs[0], Role: research.BundleSourcePrimary, TemporalScope: research.SourceTemporalCurrent,
		}},
		Freshness: research.SourceBundleFreshness{
			State: research.FreshnessFresh, Score: score, LastVerifiedAt: &verifiedAt,
			SourceAlgorithms: []string{research.FreshnessAlgorithmV1}, AlgorithmVersion: research.SourceBundleFreshnessV1,
		},
		State: research.BundleReady, Summary: "Verified drift fixture.", VerifiedAt: verifiedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}
