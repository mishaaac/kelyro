package bundle

import (
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestAssembleV1MarksMissingRequiredEvidenceIncomplete(t *testing.T) {
	t.Parallel()
	topic, _ := research.NewResearchTopic("Bundle evidence", "software", "Fixture")
	requestID := bundleTestID(t, "request.bundle")
	runID := bundleTestID(t, "run.bundle")
	claimID := bundleTestClaimID(t, "claim.bundle")
	sourceID := bundleTestSourceID(t, "source.bundle")
	evidenceID := bundleTestID(t, "evidence.bundle")
	missingID := bundleTestID(t, "evidence.missing")
	completedAt := bundleTestTime(t, 10)
	request := research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeConceptDefinition, RequestedAt: bundleTestTime(t, 8)}
	run := research.ResearchRun{ID: runID, RequestID: requestID, Status: research.ResearchRunCompleted, StartedAt: bundleTestTime(t, 9), CompletedAt: &completedAt}
	confidence, _ := research.NewClaimConfidence(0.9)
	claim := research.Claim{
		ID: claimID, Topic: topic, Statement: "The fixture has deterministic behavior.",
		Type: research.ClaimDefinition, Scope: "fixture", StatusScope: research.ClaimStatusStable,
		Confidence: confidence, SourceIDs: []research.SourceID{sourceID},
		EvidenceIDs: []research.ID{evidenceID, missingID}, CreatedAt: bundleTestTime(t, 10),
	}
	verification := research.VerificationResult{
		ID: bundleTestID(t, "verification.bundle"), ClaimID: claimID,
		Status: research.VerificationVerified, Requirement: research.VerificationRequirementNormativePrimary,
		SourceIDs: []research.SourceID{sourceID}, Metrics: research.VerificationMetrics{
			SourceCount: 1, IndependentOrganizationCount: 1,
			AuthorityDistribution: research.VerificationAuthorityDistribution{TierA: 1}, ScopeConsistent: true,
		},
		ReasonCodes: []research.ClaimVerificationReason{research.VerificationReasonPrimarySourceSufficient},
		Confidence:  confidence, VerifiedAt: bundleTestTime(t, 11), AlgorithmVersion: research.MultiSourceVerificationAlgorithmV1,
	}
	locator, _ := research.NewSourceLocator("https://fixture.example.test/reference")
	source := research.Source{
		ID: sourceID, Kind: research.SourceOfficialDocumentation, Locator: locator,
		TemporalScope: research.SourceTemporalCurrent, Metadata: research.SourceMetadata{Title: "Fixture reference"}, CreatedAt: bundleTestTime(t, 8),
	}
	decision := research.TrustDecision{
		SourceID: sourceID, State: research.TrustAccepted, Tier: research.AuthorityTierA,
		Reasons: []research.TrustReason{{Code: "fixture.accepted"}}, Policy: "trust-policy-v1", EvaluatedAt: bundleTestTime(t, 10),
	}
	freshnessScore, _ := research.NewFreshnessScore(1)
	bundle, err := AssembleV1(Input{
		ID: bundleTestID(t, "bundle.missing"), Request: request, Run: run,
		Claims:     []ClaimObservation{{Claim: claim, Verification: &verification, MissingEvidenceIDs: []research.ID{missingID}}},
		Sources:    []SourceObservation{{Source: source, TrustDecision: &decision}},
		Freshness:  []FreshnessObservation{{ClaimID: claimID, State: research.FreshnessFresh, Score: freshnessScore, LastVerifiedAt: bundleTestTime(t, 11), AlgorithmVersion: research.FreshnessAlgorithmV1}},
		VerifiedAt: bundleTestTime(t, 12),
	})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.State != research.BundleIncomplete || !containsIssue(bundle.Issues, research.BundleIssueMissingEvidence) {
		t.Fatalf("bundle state/issues = %s/%v", bundle.State, bundle.Issues)
	}
	if bundle.Sources[0].Role != research.BundleSourcePrimary || bundle.ContentHash == "" {
		t.Fatalf("bundle source/hash = %+v/%q", bundle.Sources, bundle.ContentHash)
	}
}

func bundleTestID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func bundleTestClaimID(t *testing.T, value string) research.ClaimID {
	t.Helper()
	id, err := research.NewClaimID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func bundleTestSourceID(t *testing.T, value string) research.SourceID {
	t.Helper()
	id, err := research.NewSourceID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func bundleTestTime(t *testing.T, hour int) research.Timestamp {
	t.Helper()
	value, err := research.NewTimestamp(time.Date(2026, time.August, 27, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func containsIssue(values []research.SourceBundleIssue, want research.SourceBundleIssue) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
