package application_test

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestVerificationServiceUsesRegistryOrganizationsAndPersistsPolicyOutput(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	claim := appendVerificationClaim(t, repositories, "independent", "Runtime Org", "Operations Institute")
	service := newVerificationService(t, repositories, 20)
	result, err := service.Verify(ctx, claim.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != research.VerificationVerified ||
		result.Requirement != research.VerificationRequirementProduction ||
		result.Metrics.SourceCount != 2 || result.Metrics.IndependentOrganizationCount != 2 ||
		result.Metrics.AuthorityDistribution.TierA != 1 || result.Metrics.AuthorityDistribution.TierB != 1 ||
		result.AlgorithmVersion != research.MultiSourceVerificationAlgorithmV1 {
		t.Fatalf("verification result = %+v", result)
	}
	stored, err := service.Get(ctx, result.ID)
	if err != nil || stored.ID != result.ID {
		t.Fatalf("stored verification = (%+v, %v)", stored, err)
	}
	latest, err := service.Latest(ctx, claim.ID)
	if err != nil || latest.ID != result.ID {
		t.Fatalf("latest verification = (%+v, %v)", latest, err)
	}
	result.ReasonCodes[0] = research.VerificationReasonWeakSupport
	again, err := service.Get(ctx, stored.ID)
	if err != nil || again.ReasonCodes[0] == result.ReasonCodes[0] {
		t.Fatalf("verification repository did not retain a defensive copy: (%+v, %v)", again, err)
	}
}

func TestVerificationServiceDoesNotCountMirrorsAndConsumesUnresolvedConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	claim := appendVerificationClaim(t, repositories, "mirrors", "Same Publisher", "Same Publisher")
	service := newVerificationService(t, repositories, 20)
	result, err := service.Verify(ctx, claim.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != research.VerificationVerifiedCaveat ||
		result.Metrics.IndependentOrganizationCount != 1 ||
		!containsVerificationReason(result.ReasonCodes, research.VerificationReasonSameOrganization) {
		t.Fatalf("mirror verification = %+v", result)
	}

	other := claim
	other.ID = testClaimID(t, "verification.mirrors.other")
	other.Statement = "A conflicting production recommendation."
	if err := repositories.Claims.Append(ctx, other); err != nil {
		t.Fatal(err)
	}
	conflictConfidence := testConfidence(t, 0.5)
	conflict := research.Conflict{
		ID: testID(t, "conflict.verification.mirrors"), Type: research.ConflictRecommendationDisagreement,
		ClaimIDs: []research.ClaimID{claim.ID, other.ID}, Confidence: conflictConfidence,
		Reason: "Comparable recommendations remain unresolved.", Unresolved: true,
		DetectedAt: testTimestamp(t, 19), AlgorithmVersion: research.ConflictResolverAlgorithmV1,
	}
	if err := repositories.Conflicts.Append(ctx, conflict); err != nil {
		t.Fatal(err)
	}
	service = newVerificationService(t, repositories, 21)
	result, err = service.Verify(ctx, claim.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != research.VerificationConflicted ||
		!containsVerificationReason(result.ReasonCodes, research.VerificationReasonUnresolvedConflict) {
		t.Fatalf("conflict-aware verification = %+v", result)
	}
}

func newVerificationService(
	t *testing.T,
	repositories application.Repositories,
	hour int,
) application.VerificationService {
	t.Helper()
	return application.NewVerificationService(
		repositories.Verification, repositories.Claims, repositories.Sources,
		repositories.TrustRegistry, repositories.SourceRegistry, repositories.Conflicts,
		fixedClock{now: testTimestamp(t, hour)},
	)
}

func appendVerificationClaim(
	t *testing.T,
	repositories application.Repositories,
	suffix, firstOrganization, secondOrganization string,
) research.Claim {
	t.Helper()
	ctx := context.Background()
	topic, err := research.NewResearchTopic("Production verification", "software", "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	claim := research.Claim{
		ID: testClaimID(t, "verification."+suffix), Topic: topic,
		Statement: "Use the production-safe fixture technique.", Type: research.ClaimRecommendation,
		Scope: "fixture production", StatusScope: research.ClaimStatusStable,
		Confidence: testConfidence(t, 0.9), CreatedAt: testTimestamp(t, 12),
	}
	fixtures := []struct {
		name         string
		organization string
		kind         research.SourceKind
		tier         research.AuthorityTier
	}{
		{"one", firstOrganization, research.SourceOfficialDocumentation, research.AuthorityTierA},
		{"two", secondOrganization, research.SourcePaper, research.AuthorityTierB},
	}
	for _, fixture := range fixtures {
		host := suffix + "-" + fixture.name + ".example.test"
		sourceID := testSourceID(t, "verification."+suffix+"."+fixture.name)
		locator, locatorErr := research.NewSourceLocator("https://" + host + "/guide")
		if locatorErr != nil {
			t.Fatal(locatorErr)
		}
		source := research.Source{
			ID: sourceID, Kind: fixture.kind, Locator: locator,
			TemporalScope: research.SourceTemporalCurrent,
			Metadata:      research.SourceMetadata{Title: "Verification " + fixture.name},
			CreatedAt:     testTimestamp(t, 9),
		}
		if err := repositories.Sources.Create(ctx, source); err != nil {
			t.Fatal(err)
		}
		snapshot := testSnapshot(t, source, "verification."+suffix+"."+fixture.name, 10)
		if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
		excerpt := "Production evidence " + fixture.name
		evidence := research.Evidence{
			ID:       testID(t, "evidence.verification."+suffix+"."+fixture.name),
			SourceID: source.ID, SnapshotID: snapshot.ID, Location: "production",
			Excerpt: excerpt, ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt),
			ExtractedAt: testTimestamp(t, 11), ExtractorVersion: "verification-fixture-v1",
		}
		if err := repositories.Evidence.Append(ctx, evidence); err != nil {
			t.Fatal(err)
		}
		domain, domainErr := research.NewCanonicalDomain(host)
		if domainErr != nil {
			t.Fatal(domainErr)
		}
		entry := research.SourceRegistryEntry{
			ID:           testID(t, "registry.verification."+suffix+"."+fixture.name),
			Organization: fixture.organization, CanonicalDomains: []research.CanonicalDomain{domain},
			SourceKinds:     []research.SourceKind{fixture.kind},
			AuthorityHints:  []research.RegistryAuthorityHint{{SourceKind: fixture.kind, Tier: fixture.tier, Reason: "Verification fixture authority."}},
			ResearchDomains: []string{"software"}, TopicPatterns: []string{"*"},
			Status: research.RegistryTrusted, AddedAt: testTimestamp(t, 8), LastReviewedAt: testTimestamp(t, 9),
		}
		if err := repositories.SourceRegistry.Save(ctx, entry); err != nil {
			t.Fatal(err)
		}
		decision := research.TrustDecision{
			SourceID: source.ID, State: research.TrustAccepted, Tier: fixture.tier,
			Reasons: []research.TrustReason{{Code: "fixture.accepted", Detail: "Accepted for verification."}},
			Policy:  "trust-policy-v1", EvaluatedAt: testTimestamp(t, 11),
		}
		if err := repositories.TrustRegistry.SaveDecision(ctx, decision); err != nil {
			t.Fatal(err)
		}
		claim.SourceIDs = append(claim.SourceIDs, source.ID)
		claim.EvidenceIDs = append(claim.EvidenceIDs, evidence.ID)
	}
	if err := repositories.Claims.Append(ctx, claim); err != nil {
		t.Fatal(err)
	}
	return claim
}

func containsVerificationReason(values []research.ClaimVerificationReason, want research.ClaimVerificationReason) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
