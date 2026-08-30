package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	conflictpolicy "github.com/mishaaac/kelyro/internal/research/conflict"
)

func TestUpdateScanV1ReportsAllStoredSignalsOffline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()

	source := testSource(t, "update-scan")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(ctx, testSnapshot(t, source, "scan-old", 9)); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Snapshots.Append(ctx, testSnapshot(t, source, "scan-new", 10)); err != nil {
		t.Fatal(err)
	}

	technologyID := testID(t, "technology.update-scan")
	oldRelease := research.ReleaseRecord{
		ID: testID(t, "release.update-scan-old"), TechnologyID: technologyID,
		Version: testVersion(t, "1.0.0"), Channel: research.ReleaseStable,
		Status: research.ReleaseSuperseded, SourceIDs: []research.SourceID{source.ID}, VerifiedAt: testTimestamp(t, 11),
	}
	currentRelease := oldRelease
	currentRelease.ID = testID(t, "release.update-scan-current")
	currentRelease.Version = testVersion(t, "2.0.0")
	currentRelease.Status = research.ReleaseCurrent
	currentRelease.VerifiedAt = testTimestamp(t, 12)
	if err := repositories.Releases.Create(ctx, oldRelease); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Releases.Create(ctx, currentRelease); err != nil {
		t.Fatal(err)
	}

	score, _ := research.NewFreshnessScore(.25)
	dueAt := testTimestamp(t, 15)
	if err := repositories.Freshness.Save(ctx, application.FreshnessRecord{
		SubjectID: testID(t, "claim.update-scan-stale"), State: research.FreshnessStale, Score: score,
		LastVerifiedAt: testTimestamp(t, 10), NextVerifyAt: &dueAt,
		VerificationReason: research.VerificationTTLExpired, Priority: research.VerificationPriorityHigh,
		AlgorithmVersion: research.FreshnessAlgorithmV1, SchedulingAlgorithmVersion: research.RefreshSchedulingAlgorithmV1,
	}); err != nil {
		t.Fatal(err)
	}

	deprecationSignal := appendDeprecationSignal(t, repositories, "update-scan", application.DeprecationSignalExplicitStatement, .9)
	deprecationService := application.NewDeprecationIntelligenceService(
		repositories.Deprecations, repositories.Claims, repositories.Evidence, fixedClock{now: testTimestamp(t, 16)},
	)
	if _, err := deprecationService.Assess(ctx, application.DeprecationAssessmentRequest{
		Subject: "Legacy API", Signals: []application.DeprecationSignal{deprecationSignal},
	}); err != nil {
		t.Fatal(err)
	}

	leftClaim, leftSource := appendConflictFixture(t, repositories, "scan-left", research.SourceSpecification, research.AuthorityTierA)
	rightClaim, rightSource := appendConflictFixture(t, repositories, "scan-right", research.SourceSpecification, research.AuthorityTierA)
	conflictService := application.NewConflictResolutionService(
		repositories.Conflicts, repositories.Claims, repositories.Sources, repositories.TrustRegistry,
		fixedClock{now: testTimestamp(t, 18)},
	)
	conflict, err := conflictService.Assess(ctx, application.ConflictAssessmentRequest{
		Relation: conflictpolicy.RelationContradiction,
		Observations: []application.ConflictObservationRef{
			{ClaimID: leftClaim.ID, SourceID: leftSource.ID},
			{ClaimID: rightClaim.ID, SourceID: rightSource.ID},
		},
	})
	if err != nil || !conflict.Unresolved {
		t.Fatalf("unresolved conflict = (%+v, %v)", conflict, err)
	}

	service := application.NewUpdateScanService(
		repositories.Sources, repositories.Snapshots, repositories.Releases,
		repositories.Deprecations, repositories.Freshness, repositories.Conflicts, nil,
	)
	scan, err := service.Scan(ctx, application.ResearchModeOffline, application.NetworkResearchAccess{}, testTimestamp(t, 23))
	if err != nil {
		t.Fatal(err)
	}
	if scan.Complete() || len(scan.IncompleteReasons) != 1 || scan.IncompleteReasons[0] != research.UpdateScanNetworkDisabled {
		t.Fatalf("offline completeness = %+v", scan.IncompleteReasons)
	}
	if scan.Inventory.KnownTechnologies != 1 || scan.Inventory.KnownReleases != 2 || scan.Inventory.TrackedSources != 4 || scan.Inventory.FreshnessDue != 1 {
		t.Fatalf("inventory = %+v", scan.Inventory)
	}
	wantTypes := []research.UpdateSignalType{
		research.UpdateSignalNewRelease, research.UpdateSignalChangedSource, research.UpdateSignalStaleEvidence,
		research.UpdateSignalDeprecatedSubject, research.UpdateSignalUnresolvedConflict,
	}
	seen := make(map[research.UpdateSignalType]bool)
	for _, signal := range scan.Signals {
		seen[signal.Type] = true
	}
	for _, signalType := range wantTypes {
		if !seen[signalType] {
			t.Errorf("scan signals = %+v, missing %s", scan.Signals, signalType)
		}
	}
	if err := scan.Validate(); err != nil {
		t.Fatalf("scan validation: %v", err)
	}
}

func TestUpdateScanV1ChecksPrivacyBeforeOptionalProvider(t *testing.T) {
	t.Parallel()
	repositories := memory.New().Repositories()
	provider := &recordingUpdateSignalProvider{}
	service := application.NewUpdateScanService(
		repositories.Sources, repositories.Snapshots, repositories.Releases,
		repositories.Deprecations, repositories.Freshness, repositories.Conflicts, provider,
	)
	denied := &recordingNetworkGate{err: privacy.ErrNetworkBlocked}
	scan, err := service.Scan(context.Background(), application.ResearchModeAuto, application.NetworkResearchAccess{Gate: denied}, testTimestamp(t, 20))
	if err != nil || provider.calls != 0 || len(scan.IncompleteReasons) != 1 || scan.IncompleteReasons[0] != research.UpdateScanNetworkDisabled {
		t.Fatalf("denied scan = (%+v, %v), provider calls=%d", scan, err, provider.calls)
	}
	if len(denied.requests) != 1 || denied.requests[0].Operation != "research.update_scan" {
		t.Fatalf("privacy requests = %+v", denied.requests)
	}

	provider.signals = []research.UpdateSignal{{
		Type: research.UpdateSignalNewRelease, Reference: "release.live",
		Detail: "live adapter observed a newer stable release", Origin: research.UpdateSignalCurrentLookup,
		ObservedAt: testTimestamp(t, 20),
	}}
	allowed := &recordingNetworkGate{}
	scan, err = service.Scan(context.Background(), application.ResearchModeAuto, application.NetworkResearchAccess{Gate: allowed}, testTimestamp(t, 20))
	if err != nil || provider.calls != 1 || !scan.Complete() || len(scan.Signals) != 1 {
		t.Fatalf("allowed scan = (%+v, %v), provider calls=%d", scan, err, provider.calls)
	}

	provider.err = errors.New("provider unavailable")
	scan, err = service.Scan(context.Background(), application.ResearchModeAuto, application.NetworkResearchAccess{Gate: allowed}, testTimestamp(t, 20))
	if err != nil || len(scan.IncompleteReasons) != 1 || scan.IncompleteReasons[0] != research.UpdateScanProviderFailed {
		t.Fatalf("failed provider scan = (%+v, %v)", scan, err)
	}
}

type recordingUpdateSignalProvider struct {
	signals []research.UpdateSignal
	err     error
	calls   int
}

func (provider *recordingUpdateSignalProvider) Scan(context.Context, application.UpdateSignalLookup) ([]research.UpdateSignal, error) {
	provider.calls++
	return append([]research.UpdateSignal(nil), provider.signals...), provider.err
}
