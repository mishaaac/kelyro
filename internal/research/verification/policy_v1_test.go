package verification_test

import (
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	verificationpolicy "github.com/mishaaac/kelyro/internal/research/verification"
)

func TestNormativePrimarySourceMayVerifyAlone(t *testing.T) {
	claim, observations := verificationFixture(t, research.ClaimDefinition,
		sourceFixture{"spec", research.SourceSpecification, research.AuthorityTierA, "Standards Org"})
	result := verify(t, claim, observations, nil)
	if result.Status != research.VerificationVerified ||
		result.Requirement != research.VerificationRequirementNormativePrimary ||
		result.Metrics.SourceCount != 1 || result.Metrics.IndependentOrganizationCount != 1 ||
		result.Metrics.AuthorityDistribution.TierA != 1 || !result.Metrics.ScopeConsistent ||
		!hasReason(result, research.VerificationReasonPrimarySourceSufficient) {
		t.Fatalf("normative verification = %+v", result)
	}
}

func TestProductionRecommendationRequiresIndependentStrongSourcesForFullVerification(t *testing.T) {
	claim, independent := verificationFixture(t, research.ClaimRecommendation,
		sourceFixture{"docs", research.SourceOfficialDocumentation, research.AuthorityTierA, "Runtime Org"},
		sourceFixture{"operations", research.SourcePaper, research.AuthorityTierB, "Operations Institute"})
	result := verify(t, claim, independent, nil)
	if result.Status != research.VerificationVerified || result.Metrics.IndependentOrganizationCount != 2 ||
		!hasReason(result, research.VerificationReasonIndependentSupport) {
		t.Fatalf("independent recommendation = %+v", result)
	}

	claim, mirrors := verificationFixture(t, research.ClaimRecommendation,
		sourceFixture{"docs-mirror", research.SourceOfficialDocumentation, research.AuthorityTierA, "Runtime Org"},
		sourceFixture{"blog-mirror", research.SourceOfficialBlog, research.AuthorityTierB, "Runtime Org"})
	result = verify(t, claim, mirrors, nil)
	if result.Status != research.VerificationVerifiedCaveat || result.Metrics.IndependentOrganizationCount != 1 ||
		!hasReason(result, research.VerificationReasonSameOrganization) {
		t.Fatalf("same-organization recommendation = %+v", result)
	}
}

func TestSecurityClaimRequiresTierAAuthoritativeSecurityKind(t *testing.T) {
	claim, community := verificationFixture(t, research.ClaimSecurity,
		sourceFixture{"security-blog", research.SourceCommunityArticle, research.AuthorityTierA, "Security Writers"})
	result := verify(t, claim, community, nil)
	if result.Status != research.VerificationInsufficient ||
		!hasReason(result, research.VerificationReasonSecurityAuthorityAbsent) {
		t.Fatalf("community security result = %+v", result)
	}

	claim, authority := verificationFixture(t, research.ClaimSecurity,
		sourceFixture{"security-spec", research.SourceStandard, research.AuthorityTierA, "Security Standards"})
	result = verify(t, claim, authority, nil)
	if result.Status != research.VerificationVerified ||
		!hasReason(result, research.VerificationReasonSecurityAuthority) {
		t.Fatalf("authoritative security result = %+v", result)
	}
}

func TestCommunityTechniqueRequiresDifferentOrganizations(t *testing.T) {
	claim, sameOrganization := verificationFixture(t, research.ClaimExample,
		sourceFixture{"forum", research.SourceCommunityForum, research.AuthorityTierC, "Community Org"},
		sourceFixture{"article", research.SourceCommunityArticle, research.AuthorityTierC, "Community Org"})
	result := verify(t, claim, sameOrganization, nil)
	if result.Status != research.VerificationInsufficient || result.Metrics.IndependentOrganizationCount != 1 {
		t.Fatalf("same-organization technique = %+v", result)
	}

	claim, independent := verificationFixture(t, research.ClaimExample,
		sourceFixture{"forum-independent", research.SourceCommunityForum, research.AuthorityTierC, "Community Org"},
		sourceFixture{"article-independent", research.SourceCommunityArticle, research.AuthorityTierC, "Practitioners Guild"})
	result = verify(t, claim, independent, nil)
	if result.Status != research.VerificationVerified || result.Metrics.IndependentOrganizationCount != 2 {
		t.Fatalf("independent technique = %+v", result)
	}
}

func TestVerificationPreservesUnknownOrganizationsScopeAndConflicts(t *testing.T) {
	claim, observations := verificationFixture(t, research.ClaimDefinition,
		sourceFixture{"unknown-org", research.SourceSpecification, research.AuthorityTierA, ""})
	result := verify(t, claim, observations, nil)
	if result.Metrics.IndependentOrganizationCount != 0 ||
		!hasReason(result, research.VerificationReasonOrganizationUnknown) {
		t.Fatalf("unknown organization result = %+v", result)
	}
	unknownClaim, unknownObservations := verificationFixture(t, research.ClaimRecommendation,
		sourceFixture{"unknown-org-one", research.SourceOfficialDocumentation, research.AuthorityTierA, ""},
		sourceFixture{"unknown-org-two", research.SourcePaper, research.AuthorityTierB, ""})
	unknownResult := verify(t, unknownClaim, unknownObservations, nil)
	if hasReason(unknownResult, research.VerificationReasonSameOrganization) ||
		!hasReason(unknownResult, research.VerificationReasonOrganizationUnknown) {
		t.Fatalf("unknown organizations must not be treated as same organization: %+v", unknownResult)
	}
	observations[0].TrustDecision = nil
	result = verify(t, claim, observations, nil)
	if result.Status != research.VerificationInsufficient ||
		result.Metrics.AuthorityDistribution.Unknown != 1 {
		t.Fatalf("unknown authority result = %+v", result)
	}

	historicalClaim, historical := verificationFixture(t, research.ClaimDefinition,
		sourceFixture{"historical", research.SourceSpecification, research.AuthorityTierA, "Standards Org"})
	historical[0].Source.TemporalScope = research.SourceTemporalHistorical
	result = verify(t, historicalClaim, historical, nil)
	if result.Status != research.VerificationInsufficient || result.Metrics.ScopeConsistent ||
		!hasReason(result, research.VerificationReasonScopeInconsistent) {
		t.Fatalf("scope-inconsistent result = %+v", result)
	}

	otherClaimID, _ := research.NewClaimID("claim.other")
	conflictID, _ := research.NewID("conflict.unresolved")
	conflictConfidence, _ := research.NewClaimConfidence(0.5)
	conflict := research.Conflict{
		ID: conflictID, Type: research.ConflictDirectContradiction,
		ClaimIDs: []research.ClaimID{claim.ID, otherClaimID}, Confidence: conflictConfidence,
		Reason: "Equivalent sources disagree.", Unresolved: true,
		DetectedAt: timestamp(t, 14), AlgorithmVersion: research.ConflictResolverAlgorithmV1,
	}
	result = verify(t, claim, observations, []research.Conflict{conflict})
	if result.Status != research.VerificationConflicted ||
		!hasReason(result, research.VerificationReasonUnresolvedConflict) {
		t.Fatalf("conflicted verification = %+v", result)
	}
	winnerClaimID, winnerSourceID := otherClaimID, observations[0].Source.ID
	conflict.ID, _ = research.NewID("conflict.resolved")
	conflict.Unresolved = false
	conflict.Resolution = "The other Claim has the applicable authority."
	conflict.WinningClaimID = &winnerClaimID
	conflict.WinningSourceID = &winnerSourceID
	conflict.WinningScope = claim.Scope
	conflict.DetectedAt = timestamp(t, 15)
	result = verify(t, claim, observations, []research.Conflict{conflict})
	if result.Status != research.VerificationRejected ||
		!hasReason(result, research.VerificationReasonLosesResolvedConflict) {
		t.Fatalf("losing verification = %+v", result)
	}
}

type sourceFixture struct {
	suffix       string
	kind         research.SourceKind
	tier         research.AuthorityTier
	organization string
}

func verificationFixture(
	t *testing.T,
	claimType research.ClaimType,
	fixtures ...sourceFixture,
) (research.Claim, []verificationpolicy.Observation) {
	t.Helper()
	topic, _ := research.NewResearchTopic("Verification fixture", "software", "Fixture")
	confidence, _ := research.NewClaimConfidence(0.9)
	claimID, _ := research.NewClaimID("claim.verification." + string(claimType))
	claim := research.Claim{
		ID: claimID, Topic: topic, Statement: "Structured claim " + string(claimType),
		Type: claimType, Scope: "fixture API", StatusScope: research.ClaimStatusStable,
		Confidence: confidence, CreatedAt: timestamp(t, 12),
	}
	observations := make([]verificationpolicy.Observation, 0, len(fixtures))
	for _, fixture := range fixtures {
		sourceID, _ := research.NewSourceID("source." + fixture.suffix)
		locator, _ := research.NewSourceLocator("https://" + fixture.suffix + ".example.test/docs")
		source := research.Source{
			ID: sourceID, Kind: fixture.kind, Locator: locator,
			TemporalScope: research.SourceTemporalCurrent,
			Metadata:      research.SourceMetadata{Title: "Source " + fixture.suffix}, CreatedAt: timestamp(t, 10),
		}
		decision := research.TrustDecision{
			SourceID: sourceID, State: research.TrustAccepted, Tier: fixture.tier,
			Reasons: []research.TrustReason{{Code: "fixture.accepted", Detail: "Accepted fixture."}},
			Policy:  "trust-policy-v1", EvaluatedAt: timestamp(t, 11),
		}
		observation := verificationpolicy.Observation{Source: source, TrustDecision: &decision}
		if fixture.organization != "" {
			status := research.RegistryTrusted
			observation.RegistryOrganization = fixture.organization
			observation.RegistryStatus = &status
		}
		claim.SourceIDs = append(claim.SourceIDs, sourceID)
		evidenceID, _ := research.NewID("evidence." + fixture.suffix)
		claim.EvidenceIDs = append(claim.EvidenceIDs, evidenceID)
		observations = append(observations, observation)
	}
	return claim, observations
}

func verify(
	t *testing.T,
	claim research.Claim,
	observations []verificationpolicy.Observation,
	conflicts []research.Conflict,
) research.VerificationResult {
	t.Helper()
	id, _ := research.NewID("verification.fixture")
	result, err := verificationpolicy.Verify(verificationpolicy.Input{
		ID: id, Claim: claim, Observations: observations, Conflicts: conflicts, VerifiedAt: timestamp(t, 20),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func hasReason(result research.VerificationResult, want research.ClaimVerificationReason) bool {
	for _, reason := range result.ReasonCodes {
		if reason == want {
			return true
		}
	}
	return false
}

func timestamp(t *testing.T, hour int) research.Timestamp {
	t.Helper()
	value, err := research.NewTimestamp(time.Date(2026, time.August, 27, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
