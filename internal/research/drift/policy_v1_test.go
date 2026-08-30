package drift

import (
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestDetectV1IgnoresNonSemanticWordingChanges(t *testing.T) {
	t.Parallel()
	fixture := driftFixture(t)
	updated := fixture.claim
	updated.Statement = "USE   the Fixture API!"
	updated.EvidenceIDs = []research.ID{driftID(t, "evidence.new")}
	updated.CreatedAt = driftTime(t, 12)
	newBundle := fixture.bundle(t, "bundle.new", updated, 13)

	result, err := DetectV1(Input{
		OldBundle: fixture.oldBundle, NewBundle: &newBundle,
		OldClaims: []research.Claim{fixture.claim}, NewClaims: []research.Claim{updated},
		DetectedAt: driftTime(t, 14),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 || len(result.UnresolvedClaims) != 0 {
		t.Fatalf("wording-only result = %+v", result)
	}
}

func TestDetectV1FindsVersionDeprecationRecommendationAndScopeChanges(t *testing.T) {
	t.Parallel()
	fixture := driftFixture(t)
	updated := fixture.claim
	version := driftVersion(t, "2.0.0")
	updated.VersionScope = &version
	updated.Type = research.ClaimDeprecation
	updated.Statement = "The Fixture API is deprecated; use Current API."
	updated.Scope = "Fixture API legacy surface"
	updated.EvidenceIDs = []research.ID{driftID(t, "evidence.new")}
	updated.CreatedAt = driftTime(t, 12)
	newBundle := fixture.bundle(t, "bundle.new", updated, 13)

	result, err := DetectV1(Input{
		OldBundle: fixture.oldBundle, NewBundle: &newBundle,
		OldClaims: []research.Claim{fixture.claim}, NewClaims: []research.Claim{updated},
		DetectedAt: driftTime(t, 14),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[research.DriftType]research.Severity{
		research.DriftClaimInvalidated:      research.SeverityImportant,
		research.DriftVersionSuperseded:     research.SeverityImportant,
		research.DriftDeprecationIntroduced: research.SeverityCritical,
		research.DriftScopeChanged:          research.SeverityImportant,
	}
	for _, finding := range result.Findings {
		if severity, exists := want[finding.Type]; !exists || severity != finding.Severity {
			t.Fatalf("unexpected finding = %+v", finding)
		} else {
			delete(want, finding.Type)
		}
		if len(finding.OldEvidence) != 1 || len(finding.NewEvidence) != 1 || finding.Confidence.Value() == 0 {
			t.Fatalf("finding evidence/confidence = %+v", finding)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing findings = %+v; got %+v", want, result.Findings)
	}
}

func TestDetectV1ClassifiesRecommendationChange(t *testing.T) {
	t.Parallel()
	fixture := driftFixture(t)
	fixture.claim.Type = research.ClaimRecommendation
	fixture.claim.Statement = "Prefer the stable API."
	fixture.oldBundle = fixture.bundle(t, "bundle.old-recommendation", fixture.claim, 11)
	updated := fixture.claim
	updated.Statement = "Prefer the preview API."
	updated.EvidenceIDs = []research.ID{driftID(t, "evidence.new-recommendation")}
	updated.CreatedAt = driftTime(t, 12)
	newBundle := fixture.bundle(t, "bundle.new-recommendation", updated, 13)
	result, err := DetectV1(Input{
		OldBundle: fixture.oldBundle, NewBundle: &newBundle,
		OldClaims: []research.Claim{fixture.claim}, NewClaims: []research.Claim{updated}, DetectedAt: driftTime(t, 14),
	})
	if err != nil || len(result.Findings) != 1 || result.Findings[0].Type != research.DriftRecommendationChanged {
		t.Fatalf("recommendation drift = (%+v, %v)", result, err)
	}
}

func TestDetectV1ReportsGoneSourceAndReleaseSupersession(t *testing.T) {
	t.Parallel()
	fixture := driftFixture(t)
	oldSnapshot := fixture.snapshot(t, "snapshot.old", 9, 200, "sha256:old")
	newSnapshot := fixture.snapshot(t, "snapshot.gone", 12, 410, "")
	release := research.ReleaseRecord{
		ID: driftID(t, "release.current"), TechnologyID: driftID(t, "technology.fixture"),
		Version: driftVersion(t, "2.0.0"), Channel: research.ReleaseStable, Status: research.ReleaseCurrent,
		SourceIDs: []research.SourceID{fixture.sourceID}, VerifiedAt: driftTime(t, 12),
	}
	result, err := DetectV1(Input{
		OldBundle: fixture.oldBundle, OldClaims: []research.Claim{fixture.claim}, DetectedAt: driftTime(t, 14),
		SnapshotObservations: []SnapshotObservation{{
			SourceID: fixture.sourceID, OldSnapshot: oldSnapshot, NewSnapshot: &newSnapshot,
			AffectedClaims: []research.ClaimID{fixture.claim.ID},
		}},
		ReleaseObservations: []ReleaseObservation{{Release: release, AffectedClaims: []research.ClaimID{fixture.claim.ID}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[research.DriftType]Finding)
	for _, finding := range result.Findings {
		seen[finding.Type] = finding
	}
	if seen[research.DriftSourceChanged].Severity != research.SeverityImportant || len(seen[research.DriftSourceChanged].NewEvidence) != 0 {
		t.Fatalf("gone source finding = %+v", seen[research.DriftSourceChanged])
	}
	if seen[research.DriftVersionSuperseded].Confidence.Value() != .95 {
		t.Fatalf("release finding = %+v", seen[research.DriftVersionSuperseded])
	}
}

func TestDetectV1LeavesMissingCurrentEvidenceUnresolved(t *testing.T) {
	t.Parallel()
	fixture := driftFixture(t)
	result, err := DetectV1(Input{
		OldBundle: fixture.oldBundle, OldClaims: []research.Claim{fixture.claim}, DetectedAt: driftTime(t, 14),
		SnapshotObservations: []SnapshotObservation{{
			SourceID: fixture.sourceID, OldSnapshot: fixture.snapshot(t, "snapshot.old", 9, 200, "sha256:old"),
			AffectedClaims: []research.ClaimID{fixture.claim.ID},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 || len(result.UnresolvedClaims) != 1 || result.UnresolvedClaims[0] != fixture.claim.ID {
		t.Fatalf("unresolved result = %+v", result)
	}
}

type policyFixture struct {
	topic     research.ResearchTopic
	sourceID  research.SourceID
	claim     research.Claim
	oldBundle research.SourceBundle
}

func driftFixture(t *testing.T) policyFixture {
	t.Helper()
	topic, err := research.NewResearchTopic("Fixture API", "software", "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := driftVersion(t, "1.0.0")
	confidence, _ := research.NewClaimConfidence(.9)
	fixture := policyFixture{topic: topic, sourceID: driftSourceID(t, "source.fixture")}
	fixture.claim = research.Claim{
		ID: driftClaimID(t, "claim.fixture"), Topic: topic, Statement: "Use the Fixture API.",
		Type: research.ClaimBehavior, Scope: "Fixture API", VersionScope: &version,
		StatusScope: research.ClaimStatusStable, Confidence: confidence,
		SourceIDs: []research.SourceID{fixture.sourceID}, EvidenceIDs: []research.ID{driftID(t, "evidence.old")},
		CreatedAt: driftTime(t, 10),
	}
	fixture.oldBundle = fixture.bundle(t, "bundle.old", fixture.claim, 11)
	return fixture
}

func (fixture policyFixture) bundle(t *testing.T, id string, claim research.Claim, hour int) research.SourceBundle {
	t.Helper()
	verifiedAt := driftTime(t, hour)
	score, _ := research.NewFreshnessScore(1)
	bundle, err := research.SealSourceBundleV1(research.SourceBundle{
		ID: driftID(t, id), RunID: driftID(t, "run."+id), Topic: fixture.topic,
		Purpose: research.PurposeCurrentUsage, ClaimIDs: []research.ClaimID{claim.ID},
		Sources: []research.SourceBundleSource{{
			SourceID: fixture.sourceID, Role: research.BundleSourcePrimary, TemporalScope: research.SourceTemporalCurrent,
		}},
		Freshness: research.SourceBundleFreshness{
			State: research.FreshnessFresh, Score: score, LastVerifiedAt: &verifiedAt,
			SourceAlgorithms: []string{research.FreshnessAlgorithmV1}, AlgorithmVersion: research.SourceBundleFreshnessV1,
		},
		State: research.BundleReady, Summary: "Verified fixture evidence.", VerifiedAt: verifiedAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func (fixture policyFixture) snapshot(t *testing.T, id string, hour, status int, hash string) research.SourceSnapshot {
	t.Helper()
	locator, _ := research.NewSourceLocator("https://example.test/fixture")
	return research.SourceSnapshot{
		ID: driftID(t, id), SourceID: fixture.sourceID, Locator: locator, FetchedAt: driftTime(t, hour),
		Fetch: research.FetchMetadata{StatusCode: status, ContentType: "text/html", ContentHash: hash, FetchVersion: "fetch-v1"},
	}
}

func driftID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func driftSourceID(t *testing.T, value string) research.SourceID {
	t.Helper()
	id, err := research.NewSourceID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func driftClaimID(t *testing.T, value string) research.ClaimID {
	t.Helper()
	id, err := research.NewClaimID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func driftVersion(t *testing.T, value string) research.SourceVersion {
	t.Helper()
	version, err := research.NewSourceVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func driftTime(t *testing.T, hour int) research.Timestamp {
	t.Helper()
	at, err := research.NewTimestamp(time.Date(2026, 8, 28, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return at
}
