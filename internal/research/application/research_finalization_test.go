package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	triggerpolicy "github.com/mishaaac/kelyro/internal/research/trigger"
)

func TestResearchFinalizationCommitsRunQueueAndBundleReferenceTogether(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	claim := appendVerificationClaim(t, repositories, "finalization", "Fixture Foundation", "Fixture Institute")
	verification := newVerificationService(t, repositories, 18)
	if _, err := verification.Verify(ctx, claim.ID); err != nil {
		t.Fatal(err)
	}
	subjectID, _ := research.NewID(claim.ID.String())
	if err := repositories.Freshness.Save(ctx, application.FreshnessRecord{
		SubjectID: subjectID, State: research.FreshnessFresh, Score: testFreshnessScore(t, .95),
		LastVerifiedAt: testTimestamp(t, 18), AlgorithmVersion: research.FreshnessAlgorithmV1,
	}); err != nil {
		t.Fatal(err)
	}
	request := research.ResearchRequest{
		ID: testID(t, "request.finalization"), Topic: claim.Topic,
		Purpose: research.PurposeProductionPractice, RequestedAt: testTimestamp(t, 7),
	}
	queue := application.NewResearchTriggerService(repositories.TriggerQueue)
	decision, err := queue.Evaluate(ctx, triggerpolicy.Input{
		QueueID: testID(t, "queue.finalization"), Request: request,
		Signals: triggerpolicy.Signals{Manual: true}, AsOf: testTimestamp(t, 7),
	})
	if err != nil || decision.QueueItem == nil {
		t.Fatalf("queue = (%+v, %v)", decision, err)
	}
	run := research.ResearchRun{
		ID: testID(t, "run.finalization"), RequestID: request.ID,
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
	bundle, err := bundles.Assemble(ctx, application.AssembleSourceBundleRequest{RunID: run.ID, ClaimIDs: []research.ClaimID{claim.ID}})
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := queue.ClaimExecution(ctx, application.ResearchQueueExecutionClaim{
		QueueItemID: decision.QueueItem.ID, RunID: run.ID, At: testTimestamp(t, 19),
	})
	if err != nil || !claimed.Acquired {
		t.Fatalf("claim = (%+v, %v)", claimed, err)
	}
	service := application.NewResearchFinalizationService(repositories.Finalization)
	result, err := service.Finalize(ctx, application.ResearchFinalization{
		QueueItemID: decision.QueueItem.ID, RunID: run.ID, RunStatus: research.ResearchRunCompleted,
		ExecutionStatus: application.ResearchQueueExecutionCompleted, Attempts: claimed.Execution.Attempts,
		BundleID: &bundle.ID, FinalizedAt: testTimestamp(t, 21), AlgorithmVersion: application.ResearchFinalizationV1,
	})
	if err != nil || result.Run.Status != research.ResearchRunCompleted || result.Execution.BundleID == nil || *result.Execution.BundleID != bundle.ID {
		t.Fatalf("finalization = (%+v, %v)", result, err)
	}
	item, itemErr := queue.Get(ctx, decision.QueueItem.ID)
	storedRun, runErr := repositories.Runs.GetRun(ctx, run.ID)
	storedExecution, executionErr := queue.Execution(ctx, decision.QueueItem.ID)
	if itemErr != nil || runErr != nil || executionErr != nil || item.Status != research.ResearchQueueDispatched ||
		storedRun.Status != research.ResearchRunCompleted || storedExecution.BundleID == nil || *storedExecution.BundleID != bundle.ID {
		t.Fatalf("durable finalization = item(%+v,%v) run(%+v,%v) execution(%+v,%v)", item, itemErr, storedRun, runErr, storedExecution, executionErr)
	}
}

func TestResearchFinalizationFailureStoresOnlySafeKind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	queue, run, claim := finalizationFailureFixture(t, ctx, repositories)
	service := application.NewResearchFinalizationService(repositories.Finalization)
	result, err := service.Finalize(ctx, application.ResearchFinalization{
		QueueItemID: claim.Execution.QueueItemID, RunID: run.ID, RunStatus: research.ResearchRunFailed,
		ExecutionStatus: application.ResearchQueueExecutionFailed, Attempts: claim.Execution.Attempts,
		FailureKind: application.ErrorNetworkResearchBlocked, FinalizedAt: testTimestamp(t, 12), AlgorithmVersion: application.ResearchFinalizationV1,
	})
	if err != nil || result.Execution.FailureKind != application.ErrorNetworkResearchBlocked || result.Execution.BundleID != nil {
		t.Fatalf("failed finalization = (%+v, %v)", result, err)
	}
	if _, err := service.Finalize(ctx, application.ResearchFinalization{
		QueueItemID: claim.Execution.QueueItemID, RunID: run.ID, RunStatus: research.ResearchRunFailed,
		ExecutionStatus: application.ResearchQueueExecutionFailed, Attempts: claim.Execution.Attempts,
		FailureKind: application.ErrorExternalFailure, FinalizedAt: testTimestamp(t, 13), AlgorithmVersion: application.ResearchFinalizationV1,
	}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("repeated finalization error = %v", err)
	}
	item, _ := queue.Get(ctx, claim.Execution.QueueItemID)
	if item.Status != research.ResearchQueueDispatched {
		t.Fatalf("failed queue = %+v", item)
	}
}

func finalizationFailureFixture(t *testing.T, ctx context.Context, repositories application.Repositories) (application.ResearchTriggerService, research.ResearchRun, application.ResearchQueueExecutionClaimResult) {
	t.Helper()
	topic, _ := research.NewResearchTopic("Go interfaces", "software", "Go")
	request := research.ResearchRequest{
		ID: testID(t, "request.finalization-failure"), Topic: topic,
		Purpose: research.PurposeCurrentUsage, RequestedAt: testTimestamp(t, 8),
	}
	queue := application.NewResearchTriggerService(repositories.TriggerQueue)
	decision, err := queue.Evaluate(ctx, triggerpolicy.Input{
		QueueID: testID(t, "queue.finalization-failure"), Request: request,
		Signals: triggerpolicy.Signals{Manual: true}, AsOf: testTimestamp(t, 8),
	})
	if err != nil || decision.QueueItem == nil {
		t.Fatal(err)
	}
	run := research.ResearchRun{ID: testID(t, "run.finalization-failure"), RequestID: request.ID, Status: research.ResearchRunRunning, StartedAt: testTimestamp(t, 9)}
	if err := repositories.Runs.Create(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	claim, err := queue.ClaimExecution(ctx, application.ResearchQueueExecutionClaim{QueueItemID: decision.QueueItem.ID, RunID: run.ID, At: testTimestamp(t, 10)})
	if err != nil || !claim.Acquired {
		t.Fatal(err)
	}
	return queue, run, claim
}
