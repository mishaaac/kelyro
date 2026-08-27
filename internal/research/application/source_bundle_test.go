package application_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestSourceBundleServiceAssemblesPersistsAndExportsReadyBundle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	claim := appendVerificationClaim(t, repositories, "bundle", "Fixture Foundation", "Fixture Institute")
	verification := newVerificationService(t, repositories, 18)
	if _, err := verification.Verify(ctx, claim.ID); err != nil {
		t.Fatal(err)
	}
	freshnessScore := testFreshnessScore(t, 0.95)
	freshnessSubjectID, err := research.NewID(claim.ID.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Freshness.Save(ctx, application.FreshnessRecord{
		SubjectID: freshnessSubjectID, State: research.FreshnessFresh,
		Score: freshnessScore, LastVerifiedAt: testTimestamp(t, 18), AlgorithmVersion: research.FreshnessAlgorithmV1,
	}); err != nil {
		t.Fatal(err)
	}
	request := research.ResearchRequest{
		ID: testID(t, "request.bundle.ready"), Topic: claim.Topic,
		Purpose: research.PurposeProductionPractice, RequestedAt: testTimestamp(t, 7),
	}
	completedAt := testTimestamp(t, 19)
	run := research.ResearchRun{
		ID: testID(t, "run.bundle.ready"), RequestID: request.ID,
		Status: research.ResearchRunCompleted, StartedAt: testTimestamp(t, 8), CompletedAt: &completedAt,
	}
	if err := repositories.Runs.Create(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	service := application.NewSourceBundleService(
		repositories.Bundles, repositories.Runs, repositories.Claims, repositories.Sources,
		repositories.Evidence, repositories.TrustRegistry, repositories.Verification,
		repositories.Conflicts, repositories.Freshness, fixedClock{now: testTimestamp(t, 20)},
	)
	bundle, err := service.Assemble(ctx, application.AssembleSourceBundleRequest{RunID: run.ID, ClaimIDs: []research.ClaimID{claim.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if bundle.State != research.BundleReady || bundle.AlgorithmVersion != research.SourceBundleAlgorithmV1 ||
		bundle.ContentHash == "" || len(bundle.Sources) != 2 || bundle.Sources[0].Role != research.BundleSourcePrimary {
		t.Fatalf("assembled bundle = %+v", bundle)
	}
	stored, err := service.Get(ctx, bundle.ID)
	if err != nil || stored.ContentHash != bundle.ContentHash {
		t.Fatalf("stored bundle = (%+v, %v)", stored, err)
	}
	exported, err := service.Export(ctx, bundle.ID)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := bundle.ExportJSON()
	if !bytes.Equal(exported, want) {
		t.Fatalf("export mismatch:\n%s\n%s", exported, want)
	}
	listed, err := service.ListForRun(ctx, run.ID)
	if err != nil || len(listed) != 1 || listed[0].ID != bundle.ID {
		t.Fatalf("listed bundles = (%+v, %v)", listed, err)
	}
	stored.Sources[0].Role = research.BundleSourceHistorical
	again, err := service.Get(ctx, bundle.ID)
	if err != nil || again.Sources[0].Role == stored.Sources[0].Role {
		t.Fatalf("bundle repository did not retain defensive copy: (%+v, %v)", again, err)
	}
}
