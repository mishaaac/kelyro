package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestServiceAssemblesLiveVerificationFromWorkspaceService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	topic, _ := research.NewResearchTopic("Go module", "software", "Go")
	source := appSource(t)
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	claim := appVerificationClaim(t, topic, source.ID)
	snapshotID := mustAppResearchID(t, "snapshot.app-live-verification")
	snapshot := research.SourceSnapshot{
		ID: snapshotID, SourceID: source.ID, Locator: source.Locator, FetchedAt: claim.CreatedAt,
		Fetch: research.FetchMetadata{StatusCode: 200, ContentType: "text/plain", ContentHash: "sha256:" + strings.Repeat("a", 64), ContentLength: 37, FetchVersion: "fixture-v1"},
	}
	if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	excerpt := "A Go module must declare its path."
	evidence := research.Evidence{
		ID: claim.EvidenceIDs[0], SourceID: source.ID, SnapshotID: snapshot.ID, Location: "text[0000]",
		Excerpt: excerpt, ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt),
		ExtractedAt: claim.CreatedAt, ExtractorVersion: researchapp.EvidenceExtractorV1,
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	if err := repositories.Claims.Append(ctx, claim); err != nil {
		t.Fatal(err)
	}
	decision := research.TrustDecision{
		SourceID: source.ID, State: research.TrustAccepted, Tier: research.AuthorityTierA,
		Reasons: []research.TrustReason{{Code: "fixture", Detail: "Reviewed authoritative fixture."}},
		Policy:  "trust-policy-v1", EvaluatedAt: claim.CreatedAt,
	}
	if err := repositories.TrustRegistry.SaveDecision(ctx, decision); err != nil {
		t.Fatal(err)
	}
	verification := researchapp.NewVerificationService(
		repositories.Verification, repositories.Claims, repositories.Sources, repositories.TrustRegistry,
		repositories.SourceRegistry, repositories.Conflicts, researchSearchClock{now: func() time.Time { return claim.CreatedAt.Time().Add(time.Hour) }},
	)
	diversityService := researchapp.NewSourceDiversityService(repositories.Claims, repositories.Sources, repositories.TrustRegistry, repositories.SourceRegistry)
	store := &fakeSourceRegistryStore{verifications: verification, diversity: diversityService, close: func() {}}
	stage, err := NewService(nil, nil).researchVerificationForRun(store)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := stage.Execute(ctx, researchapp.LiveResearchStageInput{Artifacts: researchapp.LiveResearchArtifacts{Claims: []research.Claim{claim}}})
	if err != nil || len(artifacts.Verifications) != 1 || artifacts.Verifications[0].Status != research.VerificationVerified {
		t.Fatalf("verification stage = (%+v, %v)", artifacts.Verifications, err)
	}
}

func appVerificationClaim(t *testing.T, topic research.ResearchTopic, sourceID research.SourceID) research.Claim {
	t.Helper()
	confidence, _ := research.NewClaimConfidence(.9)
	createdAt, _ := research.NewTimestamp(time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC))
	return research.Claim{
		ID: mustAppClaimID(t, "claim.app-live-verification"), Topic: topic,
		Statement: "A Go module must declare its path.", Type: research.ClaimRequirement,
		Scope: topic.Subject, StatusScope: research.ClaimStatusStable, Confidence: confidence,
		SourceIDs: []research.SourceID{sourceID}, EvidenceIDs: []research.ID{mustAppResearchID(t, "evidence.app-live-verification")},
		CreatedAt: createdAt,
	}
}

func mustAppClaimID(t *testing.T, value string) research.ClaimID {
	t.Helper()
	id, err := research.NewClaimID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
