package application_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestLiveMultiSourceVerificationReusesPersistedPolicyInClaimOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	firstSource := testSource(t, "live-verify-first")
	secondSource := testSource(t, "live-verify-second")
	for _, source := range []research.Source{firstSource, secondSource} {
		if err := repositories.Sources.Create(ctx, source); err != nil {
			t.Fatal(err)
		}
		if err := repositories.TrustRegistry.SaveDecision(ctx, research.TrustDecision{
			SourceID: source.ID, State: research.TrustAccepted, Tier: research.AuthorityTierA,
			Reasons: []research.TrustReason{{Code: "fixture", Detail: "Reviewed authoritative fixture."}},
			Policy:  "trust-policy-v1", EvaluatedAt: testTimestamp(t, 10),
		}); err != nil {
			t.Fatal(err)
		}
	}
	firstClaim := verificationClaimFixture(t, "live-verify-first", topic, firstSource.ID)
	secondClaim := verificationClaimFixture(t, "live-verify-second", topic, secondSource.ID)
	for index, claim := range []research.Claim{firstClaim, secondClaim} {
		source := []research.Source{firstSource, secondSource}[index]
		persistVerificationEvidence(t, ctx, repositories, source, claim.EvidenceIDs[0])
		if err := repositories.Claims.Append(ctx, claim); err != nil {
			t.Fatal(err)
		}
	}
	verification := application.NewVerificationService(
		repositories.Verification, repositories.Claims, repositories.Sources, repositories.TrustRegistry,
		repositories.SourceRegistry, repositories.Conflicts, fixedClock{now: testTimestamp(t, 12)},
	)
	diversityService := application.NewSourceDiversityService(repositories.Claims, repositories.Sources, repositories.TrustRegistry, repositories.SourceRegistry)
	service, err := application.NewLiveMultiSourceVerificationService(verification, diversityService)
	if err != nil {
		t.Fatal(err)
	}
	input := application.LiveResearchStageInput{Artifacts: application.LiveResearchArtifacts{
		Claims: []research.Claim{secondClaim, firstClaim},
	}}
	artifacts, err := service.Execute(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts.Verifications) != 2 || artifacts.Verifications[0].ClaimID != firstClaim.ID ||
		artifacts.Verifications[1].ClaimID != secondClaim.ID {
		t.Fatalf("verification order = %+v", artifacts.Verifications)
	}
	if len(artifacts.DiversityAssessments) != 2 || artifacts.DiversityAssessments[0].ClaimID != firstClaim.ID ||
		artifacts.DiversityAssessments[0].Assessment.AlgorithmVersion != "source-diversity-v1" {
		t.Fatalf("diversity assessments = %+v", artifacts.DiversityAssessments)
	}
	for _, result := range artifacts.Verifications {
		if result.Status != research.VerificationVerified || result.AlgorithmVersion != research.MultiSourceVerificationAlgorithmV1 {
			t.Fatalf("verification = %+v", result)
		}
		persisted, getErr := repositories.Verification.LatestByClaim(ctx, result.ClaimID)
		if getErr != nil || !reflect.DeepEqual(persisted, result) {
			t.Fatalf("persisted verification = (%+v, %v)", persisted, getErr)
		}
	}
	if input.Artifacts.Verifications != nil {
		t.Fatal("verification stage mutated input artifacts")
	}
}

func TestLiveMultiSourceVerificationPreservesInsufficientEvidence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	source := testSource(t, "live-verify-insufficient")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	claim := verificationClaimFixture(t, "live-verify-insufficient", topic, source.ID)
	claim.Type = research.ClaimExample
	persistVerificationEvidence(t, ctx, repositories, source, claim.EvidenceIDs[0])
	if err := repositories.Claims.Append(ctx, claim); err != nil {
		t.Fatal(err)
	}
	verification := application.NewVerificationService(
		repositories.Verification, repositories.Claims, repositories.Sources, repositories.TrustRegistry,
		repositories.SourceRegistry, repositories.Conflicts, fixedClock{now: testTimestamp(t, 12)},
	)
	diversityService := application.NewSourceDiversityService(repositories.Claims, repositories.Sources, repositories.TrustRegistry, repositories.SourceRegistry)
	service, err := application.NewLiveMultiSourceVerificationService(verification, diversityService)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.VerifyClaims(ctx, application.LiveMultiSourceVerificationRequest{Claims: []research.Claim{claim}})
	if err != nil || len(result.Verifications) != 1 || result.Verifications[0].Status != research.VerificationInsufficient {
		t.Fatalf("verification = (%+v, %v)", result, err)
	}
}

func persistVerificationEvidence(t *testing.T, ctx context.Context, repositories application.Repositories, source research.Source, evidenceID research.ID) {
	t.Helper()
	snapshot := testSnapshot(t, source, evidenceID.String(), 10)
	if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	excerpt := "A Go module must declare its path."
	evidence := research.Evidence{
		ID: evidenceID, SourceID: source.ID, SnapshotID: snapshot.ID, Location: "text[0000]",
		Excerpt: excerpt, ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt),
		ExtractedAt: testTimestamp(t, 11), ExtractorVersion: application.EvidenceExtractorV1,
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
}

func verificationClaimFixture(t *testing.T, suffix string, topic research.ResearchTopic, sourceID research.SourceID) research.Claim {
	t.Helper()
	confidence, _ := research.NewClaimConfidence(.9)
	return research.Claim{
		ID: testClaimID(t, suffix), Topic: topic, Statement: "A Go module must declare its path.",
		Type: research.ClaimRequirement, Scope: topic.Subject, StatusScope: research.ClaimStatusStable,
		Confidence: confidence, SourceIDs: []research.SourceID{sourceID},
		EvidenceIDs: []research.ID{testID(t, "evidence."+suffix)}, CreatedAt: testTimestamp(t, 11),
	}
}
