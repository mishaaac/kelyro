package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestLiveSourceBundleAssemblesDurableBundleForRunningResearchRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	claim := appendVerificationClaim(t, repositories, "live-bundle", "Fixture Foundation", "Fixture Institute")
	verificationService := newVerificationService(t, repositories, 18)
	verification, err := verificationService.Verify(ctx, claim.ID)
	if err != nil {
		t.Fatal(err)
	}
	unsupportedSource := testSource(t, "live-bundle-unsupported")
	if err := repositories.Sources.Create(ctx, unsupportedSource); err != nil {
		t.Fatal(err)
	}
	unsupportedClaim := verificationClaimFixture(t, "live-bundle-unsupported", claim.Topic, unsupportedSource.ID)
	unsupportedClaim.Type = research.ClaimExample
	persistVerificationEvidence(t, ctx, repositories, unsupportedSource, unsupportedClaim.EvidenceIDs[0])
	if err := repositories.Claims.Append(ctx, unsupportedClaim); err != nil {
		t.Fatal(err)
	}
	unsupportedVerification, err := verificationService.Verify(ctx, unsupportedClaim.ID)
	if err != nil || unsupportedVerification.Status != research.VerificationInsufficient {
		t.Fatalf("unsupported verification = (%+v, %v)", unsupportedVerification, err)
	}
	freshnessSubjectID, _ := research.NewID(claim.ID.String())
	if err := repositories.Freshness.Save(ctx, application.FreshnessRecord{
		SubjectID: freshnessSubjectID, State: research.FreshnessFresh, Score: testFreshnessScore(t, .95),
		LastVerifiedAt: testTimestamp(t, 18), AlgorithmVersion: research.FreshnessAlgorithmV1,
	}); err != nil {
		t.Fatal(err)
	}
	request := research.ResearchRequest{
		ID: testID(t, "request.live-bundle"), Topic: claim.Topic,
		Purpose: research.PurposeProductionPractice, RequestedAt: testTimestamp(t, 7),
	}
	run := research.ResearchRun{
		ID: testID(t, "run.live-bundle"), RequestID: request.ID,
		Status: research.ResearchRunRunning, StartedAt: testTimestamp(t, 8),
	}
	if err := repositories.Runs.Create(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	bundles := application.NewSourceBundleService(
		repositories.Bundles, repositories.Runs, repositories.Claims, repositories.Sources,
		repositories.Evidence, repositories.TrustRegistry, repositories.Verification,
		repositories.Conflicts, repositories.Freshness, fixedClock{now: testTimestamp(t, 20)},
	)
	stage, err := application.NewLiveSourceBundleService(bundles)
	if err != nil {
		t.Fatal(err)
	}
	input := application.LiveResearchStageInput{
		Request: request, Run: run,
		Artifacts: application.LiveResearchArtifacts{
			Claims:        []research.Claim{unsupportedClaim, claim},
			Verifications: []research.VerificationResult{unsupportedVerification, verification},
		},
	}
	artifacts, err := stage.Execute(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if artifacts.Bundle == nil || artifacts.Bundle.RunID != run.ID || artifacts.Bundle.State != research.BundleReady ||
		artifacts.Bundle.AlgorithmVersion != research.SourceBundleAlgorithmV1 || len(artifacts.Bundle.ClaimIDs) != 1 ||
		artifacts.Bundle.ClaimIDs[0] != claim.ID {
		t.Fatalf("live bundle = %+v", artifacts.Bundle)
	}
	if len(artifacts.Claims) != 2 || len(artifacts.Verifications) != 2 {
		t.Fatalf("bundle selection discarded auditable candidates: %+v", artifacts)
	}
	stored, err := bundles.Get(ctx, artifacts.Bundle.ID)
	if err != nil || stored.ContentHash != artifacts.Bundle.ContentHash {
		t.Fatalf("stored live bundle = (%+v, %v)", stored, err)
	}
	finalizationStage, err := application.NewLiveResearchFinalizationStage(bundles)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := finalizationStage.Execute(ctx, application.LiveResearchStageInput{
		Run: run, Artifacts: application.LiveResearchArtifacts{Bundle: artifacts.Bundle},
	})
	if err != nil || prepared.Bundle == nil || prepared.Bundle.ContentHash != stored.ContentHash {
		t.Fatalf("prepared finalization bundle = (%+v, %v)", prepared.Bundle, err)
	}
	artifacts.Bundle.ClaimIDs[0] = testClaimID(t, "mutated-live-bundle")
	listed, err := bundles.ListForRun(ctx, run.ID)
	if err != nil || len(listed) != 1 || listed[0].ClaimIDs[0] != claim.ID {
		t.Fatalf("durable bundle changed through artifacts = (%+v, %v)", listed, err)
	}
	if input.Artifacts.Bundle != nil {
		t.Fatal("bundle stage mutated input artifacts")
	}
	incomplete := stored
	incomplete.ID = testID(t, "bundle.live-bundle.incomplete")
	incomplete.Issues = append(incomplete.Issues, research.BundleIssueInsufficientEvidence)
	incomplete.ContentHash = ""
	incomplete, err = research.SealSourceBundleV1(incomplete)
	if err != nil || incomplete.State != research.BundleIncomplete {
		t.Fatalf("incomplete bundle fixture = (%+v, %v)", incomplete, err)
	}
	if err := repositories.Bundles.Append(ctx, incomplete); err != nil {
		t.Fatal(err)
	}
	if _, err := finalizationStage.Execute(ctx, application.LiveResearchStageInput{
		Run: run, Artifacts: application.LiveResearchArtifacts{Bundle: &incomplete},
	}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("incomplete bundle finalization error = %v", err)
	}
}

func TestLiveSourceBundleRejectsEmptyClaimSet(t *testing.T) {
	t.Parallel()
	stage, err := application.NewLiveSourceBundleService(application.NewSourceBundleService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	runID := testID(t, "run.live-bundle-empty")
	if _, err := stage.Execute(context.Background(), application.LiveResearchStageInput{Run: research.ResearchRun{ID: runID}}); err == nil {
		t.Fatal("empty live bundle Claims succeeded")
	}
}
