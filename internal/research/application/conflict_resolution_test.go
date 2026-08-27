package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	conflictpolicy "github.com/mishaaac/kelyro/internal/research/conflict"
)

func TestConflictResolutionServicePersistsExplainableAuthorityDecision(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	specClaim, specSource := appendConflictFixture(t, repositories, "spec", research.SourceSpecification, research.AuthorityTierA)
	blogClaim, blogSource := appendConflictFixture(t, repositories, "blog", research.SourceCommunityArticle, research.AuthorityTierC)
	service := application.NewConflictResolutionService(
		repositories.Conflicts, repositories.Claims, repositories.Sources, repositories.TrustRegistry,
		fixedClock{now: testTimestamp(t, 20)},
	)
	result, err := service.Assess(ctx, application.ConflictAssessmentRequest{
		Relation: conflictpolicy.RelationContradiction,
		Observations: []application.ConflictObservationRef{
			{ClaimID: blogClaim.ID, SourceID: blogSource.ID},
			{ClaimID: specClaim.ID, SourceID: specSource.ID},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != research.ConflictAuthorityMismatch || result.Unresolved ||
		result.WinningClaimID == nil || *result.WinningClaimID != specClaim.ID ||
		result.WinningSourceID == nil || *result.WinningSourceID != specSource.ID ||
		result.AlgorithmVersion != research.ConflictResolverAlgorithmV1 {
		t.Fatalf("assessment = %+v", result)
	}
	stored, err := service.Get(ctx, result.ID)
	if err != nil || stored.Reason != result.Reason {
		t.Fatalf("stored conflict = (%+v, %v)", stored, err)
	}
	list, err := service.ListForClaim(ctx, blogClaim.ID)
	if err != nil || len(list) != 1 || list[0].ID != result.ID {
		t.Fatalf("claim conflicts = (%+v, %v)", list, err)
	}
	result.ClaimIDs[0] = testClaimID(t, "mutated")
	again, err := service.Get(ctx, stored.ID)
	if err != nil || again.ClaimIDs[0] == result.ClaimIDs[0] {
		t.Fatalf("conflict repository did not retain a defensive copy: (%+v, %v)", again, err)
	}
}

func TestConflictResolutionServiceRequiresAcceptedTrustAndClaimSourceRelationship(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	leftClaim, leftSource := appendConflictFixture(t, repositories, "left", research.SourceOfficialDocumentation, research.AuthorityTierA)
	rightClaim, rightSource := appendConflictFixture(t, repositories, "right", research.SourceOfficialDocumentation, research.AuthorityTierA)
	rejected := research.TrustDecision{
		SourceID: rightSource.ID, State: research.TrustRejected, Tier: research.AuthorityTierE,
		Reasons: []research.TrustReason{{Code: "fixture.rejected", Detail: "Rejected for test."}},
		Policy:  "trust-policy-v1", EvaluatedAt: testTimestamp(t, 14),
	}
	if err := repositories.TrustRegistry.SaveDecision(ctx, rejected); err != nil {
		t.Fatal(err)
	}
	service := application.NewConflictResolutionService(
		repositories.Conflicts, repositories.Claims, repositories.Sources, repositories.TrustRegistry,
		fixedClock{now: testTimestamp(t, 20)},
	)
	request := application.ConflictAssessmentRequest{
		Relation: conflictpolicy.RelationContradiction,
		Observations: []application.ConflictObservationRef{
			{ClaimID: leftClaim.ID, SourceID: leftSource.ID},
			{ClaimID: rightClaim.ID, SourceID: rightSource.ID},
		},
	}
	if _, err := service.Assess(ctx, request); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("rejected source error = %v, want invalid_state", err)
	}

	accepted := rejected
	accepted.State = research.TrustAccepted
	accepted.Tier = research.AuthorityTierA
	accepted.EvaluatedAt = testTimestamp(t, 15)
	if err := repositories.TrustRegistry.SaveDecision(ctx, accepted); err != nil {
		t.Fatal(err)
	}
	request.Observations[1].SourceID = leftSource.ID
	if _, err := service.Assess(ctx, request); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("unrelated source error = %v, want invalid_state", err)
	}
}

func appendConflictFixture(
	t *testing.T,
	repositories application.Repositories,
	suffix string,
	kind research.SourceKind,
	tier research.AuthorityTier,
) (research.Claim, research.Source) {
	t.Helper()
	ctx := context.Background()
	source := testSource(t, "conflict-"+suffix)
	source.Kind = kind
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	snapshot := testSnapshot(t, source, "conflict-"+suffix, 10)
	if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	excerpt := "Conflict evidence " + suffix
	evidence := research.Evidence{
		ID: testID(t, "evidence.conflict."+suffix), SourceID: source.ID, SnapshotID: snapshot.ID,
		Location: "section", Excerpt: excerpt, ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt),
		ExtractedAt: testTimestamp(t, 11), ExtractorVersion: "conflict-fixture-v1",
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	topic, err := research.NewResearchTopic("Conflict fixture", "software", "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	claim := research.Claim{
		ID: testClaimID(t, "conflict."+suffix), Topic: topic,
		Statement: "Conflicting statement " + suffix, Type: research.ClaimRequirement,
		Scope: "fixture API", StatusScope: research.ClaimStatusStable,
		Confidence: testConfidence(t, 0.9), SourceIDs: []research.SourceID{source.ID},
		EvidenceIDs: []research.ID{evidence.ID}, CreatedAt: testTimestamp(t, 12),
	}
	if err := repositories.Claims.Append(ctx, claim); err != nil {
		t.Fatal(err)
	}
	decision := research.TrustDecision{
		SourceID: source.ID, State: research.TrustAccepted, Tier: tier,
		Reasons: []research.TrustReason{{Code: "fixture.accepted", Detail: "Accepted for conflict fixture."}},
		Policy:  "trust-policy-v1", EvaluatedAt: testTimestamp(t, 13),
	}
	if err := repositories.TrustRegistry.SaveDecision(ctx, decision); err != nil {
		t.Fatal(err)
	}
	return claim, source
}
