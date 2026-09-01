package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/trigger"
)

func TestSQLiteResearchTriggerQueuePersistsDeduplicatesAndOrdersMetadata(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	service := application.NewResearchTriggerService(database.Repositories().Research.TriggerQueue)
	critical := sqliteTriggerInput(t, "request.sqlite-trigger-critical", "queue.sqlite-trigger-critical", "topic critical", 0)
	critical.Signals = trigger.Signals{SecuritySensitiveRefresh: true, EvidenceCount: 1}
	normal := sqliteTriggerInput(t, "request.sqlite-trigger-normal", "queue.sqlite-trigger-normal", "topic normal", time.Minute)
	normal.Signals = trigger.Signals{EvidenceCount: 0}
	first, err := service.Evaluate(ctx, normal)
	if err != nil || first.QueueItem == nil {
		t.Fatalf("normal trigger = (%+v,%v)", first, err)
	}
	second, err := service.Evaluate(ctx, critical)
	if err != nil || second.QueueItem == nil {
		t.Fatalf("critical trigger = (%+v,%v)", second, err)
	}
	queued, err := service.Queued(ctx)
	if err != nil || len(queued) != 2 || queued[0].Priority != research.VerificationPriorityCritical || queued[1].Priority != research.VerificationPriorityNormal {
		t.Fatalf("ordered queue = (%+v,%v)", queued, err)
	}
	duplicate := normal
	duplicate.QueueID, _ = research.NewID("queue.sqlite-trigger-duplicate")
	duplicate.Request.ID, _ = research.NewID("request.sqlite-trigger-duplicate")
	duplicate.Signals = trigger.Signals{Manual: true, EvidenceCount: 3}
	deduplicated, err := service.Evaluate(ctx, duplicate)
	if err != nil || deduplicated.QueueItem.ID != first.QueueItem.ID || deduplicated.QueueItem.Triggers[0] != research.ResearchTriggerMissingEvidence {
		t.Fatalf("deduplicated queue = (%+v,%v)", deduplicated, err)
	}
	cancelledAt, _ := research.NewTimestamp(normal.AsOf.Time().Add(time.Hour))
	cancelled, err := service.Cancel(ctx, first.QueueItem.ID, cancelledAt)
	if err != nil || cancelled.Status != research.ResearchQueueCancelled {
		t.Fatalf("cancelled = (%+v,%v)", cancelled, err)
	}
	loaded, err := service.Get(ctx, cancelled.ID)
	if err != nil || loaded.StatusChangedAt == nil || loaded.Status != research.ResearchQueueCancelled {
		t.Fatalf("loaded cancelled = (%+v,%v)", loaded, err)
	}
}

func TestSQLiteResearchFinalizationCommitsAndRollsBackRunQueueBundleAtomically(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	repositories := database.Repositories().Research
	queue := application.NewResearchTriggerService(repositories.TriggerQueue)
	researchService := application.NewResearchService(repositories.Runs)
	input := sqliteTriggerInput(t, "request.sqlite-finalization", "queue.sqlite-finalization", "finalization topic", 0)
	input.Signals = trigger.Signals{Manual: true}
	decision, err := queue.Evaluate(ctx, input)
	if err != nil || decision.QueueItem == nil {
		t.Fatalf("queue = (%+v, %v)", decision, err)
	}
	runID, _ := research.NewID("run.sqlite-finalization")
	run := research.ResearchRun{ID: runID, RequestID: input.Request.ID, Status: research.ResearchRunRunning, StartedAt: input.AsOf}
	cost := research.ResearchCostMetadata{Budget: research.DefaultResearchCostBudgetV1(), AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	run.Cost = &cost
	if err := researchService.Start(ctx, input.Request, run); err != nil {
		t.Fatal(err)
	}
	bundleID, _ := research.NewID("bundle.sqlite-finalization")
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO source_bundles (id,run_id,topic_subject,topic_domain,topic_technology,purpose,state,verified_at) VALUES (?,?,?,?,?,?,?,?)`,
		bundleID.String(), run.ID.String(), input.Request.Topic.Subject, input.Request.Topic.Domain, input.Request.Topic.Technology,
		string(input.Request.Purpose), string(research.BundleIncomplete), timestampText(input.AsOf)); err != nil {
		t.Fatal(err)
	}
	claimed, err := queue.ClaimExecution(ctx, application.ResearchQueueExecutionClaim{QueueItemID: decision.QueueItem.ID, RunID: run.ID, At: input.AsOf})
	if err != nil || !claimed.Acquired {
		t.Fatalf("claim = (%+v, %v)", claimed, err)
	}
	finalizedAt, _ := research.NewTimestamp(input.AsOf.Time().Add(time.Hour))
	service := application.NewResearchFinalizationService(repositories.Finalization)
	auditID, _ := research.NewID("audit.sqlite-finalization.terminal")
	audit, err := research.SealResearchRunAuditV1(research.ResearchRunAudit{
		ID: auditID, RunID: run.ID, RecordedAt: finalizedAt, StartedAt: run.StartedAt,
		CompletedAt: &finalizedAt, Outcome: research.ResearchRunCompleted, QueryPlannerVersion: "query-planner-v1",
		TrustPolicyVersion: "trust-policy-v1", FreshnessVersion: research.FreshnessAlgorithmV1,
		ConflictResolverVersion: research.ConflictResolverAlgorithmV1, NetworkMode: research.ResearchAuditNetworkAuto,
		Queries: []string{"finalization topic official documentation"}, TargetTechnology: input.Request.Topic.Technology,
		Execution: &research.ResearchAuditExecution{BundleID: &bundleID, CostUsed: cost.Used, CacheSavings: cost.CacheSavings, AlgorithmVersion: research.LiveResearchExecutionAuditV1},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Finalize(ctx, application.ResearchFinalization{
		QueueItemID: decision.QueueItem.ID, RunID: run.ID, RunStatus: research.ResearchRunCompleted,
		ExecutionStatus: application.ResearchQueueExecutionCompleted, Attempts: claimed.Execution.Attempts,
		BundleID: &bundleID, Audit: &audit, Cost: &cost, FinalizedAt: finalizedAt, AlgorithmVersion: application.ResearchFinalizationV1,
	})
	if err != nil || result.Run.Status != research.ResearchRunCompleted || result.Execution.BundleID == nil || *result.Execution.BundleID != bundleID || result.Audit == nil {
		t.Fatalf("finalization = (%+v, %v)", result, err)
	}
	storedRun, _ := researchService.Run(ctx, run.ID)
	storedItem, _ := queue.Get(ctx, decision.QueueItem.ID)
	storedExecution, _ := queue.Execution(ctx, decision.QueueItem.ID)
	storedAudit, auditErr := researchService.AuditTrail(ctx, run.ID)
	if storedRun.Status != research.ResearchRunCompleted || storedItem.Status != research.ResearchQueueDispatched ||
		storedExecution.BundleID == nil || *storedExecution.BundleID != bundleID || auditErr != nil || len(storedAudit) != 1 || storedAudit[0].ID != audit.ID {
		t.Fatalf("durable finalization = run(%+v) item(%+v) execution(%+v)", storedRun, storedItem, storedExecution)
	}

	wrong := sqliteTriggerInput(t, "request.sqlite-finalization-wrong", "queue.sqlite-finalization-wrong", "wrong finalization topic", 2*time.Hour)
	wrong.Signals = trigger.Signals{Manual: true}
	wrongDecision, err := queue.Evaluate(ctx, wrong)
	if err != nil || wrongDecision.QueueItem == nil {
		t.Fatal(err)
	}
	wrongRunID, _ := research.NewID("run.sqlite-finalization-wrong")
	wrongRun := research.ResearchRun{ID: wrongRunID, RequestID: wrong.Request.ID, Status: research.ResearchRunRunning, StartedAt: wrong.AsOf}
	if err := researchService.Start(ctx, wrong.Request, wrongRun); err != nil {
		t.Fatal(err)
	}
	wrongClaim, err := queue.ClaimExecution(ctx, application.ResearchQueueExecutionClaim{QueueItemID: wrongDecision.QueueItem.ID, RunID: wrongRun.ID, At: wrong.AsOf})
	if err != nil || !wrongClaim.Acquired {
		t.Fatal(err)
	}
	wrongFinalizedAt, _ := research.NewTimestamp(wrong.AsOf.Time().Add(time.Hour))
	_, err = service.Finalize(ctx, application.ResearchFinalization{
		QueueItemID: wrongDecision.QueueItem.ID, RunID: wrongRun.ID, RunStatus: research.ResearchRunCompleted,
		ExecutionStatus: application.ResearchQueueExecutionCompleted, Attempts: wrongClaim.Execution.Attempts,
		BundleID: &bundleID, FinalizedAt: wrongFinalizedAt, AlgorithmVersion: application.ResearchFinalizationV1,
	})
	if !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("wrong bundle finalization error = %v", err)
	}
	rolledBackRun, _ := researchService.Run(ctx, wrongRun.ID)
	rolledBackItem, _ := queue.Get(ctx, wrongDecision.QueueItem.ID)
	rolledBackExecution, _ := queue.Execution(ctx, wrongDecision.QueueItem.ID)
	if rolledBackRun.Status != research.ResearchRunRunning || rolledBackItem.Status != research.ResearchQueueQueued ||
		rolledBackExecution.Status != application.ResearchQueueExecutionClaimed || rolledBackExecution.BundleID != nil {
		t.Fatalf("rollback state = run(%+v) item(%+v) execution(%+v)", rolledBackRun, rolledBackItem, rolledBackExecution)
	}

	wrongBundleID, _ := research.NewID("bundle.sqlite-finalization-wrong")
	if _, err := database.sql.ExecContext(ctx, `INSERT INTO source_bundles (id,run_id,topic_subject,topic_domain,topic_technology,purpose,state,verified_at) VALUES (?,?,?,?,?,?,?,?)`,
		wrongBundleID.String(), wrongRun.ID.String(), wrong.Request.Topic.Subject, wrong.Request.Topic.Domain, wrong.Request.Topic.Technology,
		string(wrong.Request.Purpose), string(research.BundleIncomplete), timestampText(wrong.AsOf)); err != nil {
		t.Fatal(err)
	}
	wrongCost := research.ResearchCostMetadata{Budget: research.DefaultResearchCostBudgetV1(), AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	badAuditID, _ := research.NewID("audit.sqlite-finalization-wrong.terminal")
	badAudit, err := research.SealResearchRunAuditV1(research.ResearchRunAudit{
		ID: badAuditID, RunID: wrongRun.ID, RecordedAt: wrongFinalizedAt, StartedAt: wrongRun.StartedAt,
		CompletedAt: &wrongFinalizedAt, Outcome: research.ResearchRunCompleted, QueryPlannerVersion: "query-planner-v1",
		TrustPolicyVersion: "trust-policy-v1", FreshnessVersion: research.FreshnessAlgorithmV1,
		ConflictResolverVersion: research.ConflictResolverAlgorithmV1, NetworkMode: research.ResearchAuditNetworkAuto,
		Queries: []string{"wrong finalization topic official documentation"}, TargetTechnology: "mismatched-target",
		Execution: &research.ResearchAuditExecution{BundleID: &wrongBundleID, CostUsed: wrongCost.Used, CacheSavings: wrongCost.CacheSavings, AlgorithmVersion: research.LiveResearchExecutionAuditV1},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Finalize(ctx, application.ResearchFinalization{
		QueueItemID: wrongDecision.QueueItem.ID, RunID: wrongRun.ID, RunStatus: research.ResearchRunCompleted,
		ExecutionStatus: application.ResearchQueueExecutionCompleted, Attempts: wrongClaim.Execution.Attempts,
		BundleID: &wrongBundleID, Audit: &badAudit, Cost: &wrongCost, FinalizedAt: wrongFinalizedAt, AlgorithmVersion: application.ResearchFinalizationV1,
	})
	if !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("bad audit finalization error = %v", err)
	}
	rolledBackRun, _ = researchService.Run(ctx, wrongRun.ID)
	rolledBackExecution, _ = queue.Execution(ctx, wrongDecision.QueueItem.ID)
	wrongTrail, trailErr := researchService.AuditTrail(ctx, wrongRun.ID)
	if rolledBackRun.Status != research.ResearchRunRunning || rolledBackExecution.Status != application.ResearchQueueExecutionClaimed || trailErr != nil || len(wrongTrail) != 0 {
		t.Fatalf("bad audit rollback = run(%+v) execution(%+v) audit(%+v,%v)", rolledBackRun, rolledBackExecution, wrongTrail, trailErr)
	}
}

func TestSQLiteResearchTriggerQueueClaimsRetriesAndAcknowledgesAtomically(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	service := application.NewResearchTriggerService(database.Repositories().Research.TriggerQueue)
	input := sqliteTriggerInput(t, "request.sqlite-worker", "queue.sqlite-worker", "worker topic", 0)
	input.Signals = trigger.Signals{Manual: true, EvidenceCount: 0}
	decision, err := service.Evaluate(ctx, input)
	if err != nil || decision.QueueItem == nil {
		t.Fatalf("queue = (%+v,%v)", decision, err)
	}
	runOne, _ := research.NewID("run.sqlite-worker-one")
	claim, err := service.ClaimExecution(ctx, application.ResearchQueueExecutionClaim{QueueItemID: decision.QueueItem.ID, RunID: runOne, At: input.AsOf})
	if err != nil || !claim.Acquired || claim.Execution.Attempts != 1 {
		t.Fatalf("first claim = (%+v,%v)", claim, err)
	}
	repeated, err := service.ClaimExecution(ctx, application.ResearchQueueExecutionClaim{QueueItemID: decision.QueueItem.ID, RunID: runOne, At: input.AsOf})
	if err != nil || repeated.Acquired || repeated.Execution.RunID != runOne {
		t.Fatalf("repeated claim = (%+v,%v)", repeated, err)
	}
	retry := claim.Execution
	retry.Status = application.ResearchQueueExecutionRetry
	retry.FailureKind = application.ErrorExternalFailure
	if _, err := service.SettleExecution(ctx, application.ResearchQueueExecutionClaimed, retry); err != nil {
		t.Fatal(err)
	}
	queued, err := service.Queued(ctx)
	if err != nil || len(queued) != 1 {
		t.Fatalf("retry queue = (%+v,%v)", queued, err)
	}
	runTwo, _ := research.NewID("run.sqlite-worker-two")
	second, err := service.ClaimExecution(ctx, application.ResearchQueueExecutionClaim{QueueItemID: decision.QueueItem.ID, RunID: runTwo, At: input.AsOf})
	if err != nil || !second.Acquired || second.Execution.Attempts != 2 || second.Execution.RunID != runTwo {
		t.Fatalf("second claim = (%+v,%v)", second, err)
	}
	completed := second.Execution
	completed.Status = application.ResearchQueueExecutionCompleted
	if _, err := service.SettleExecution(ctx, application.ResearchQueueExecutionClaimed, completed); err != nil {
		t.Fatal(err)
	}
	item, err := service.Get(ctx, decision.QueueItem.ID)
	stored, executionErr := service.Execution(ctx, decision.QueueItem.ID)
	if err != nil || executionErr != nil || item.Status != research.ResearchQueueDispatched || stored.Status != application.ResearchQueueExecutionCompleted || stored.Attempts != 2 {
		t.Fatalf("settled queue = item(%+v,%v), execution(%+v,%v)", item, err, stored, executionErr)
	}
}

func sqliteTriggerInput(t *testing.T, requestValue, queueValue, subject string, offset time.Duration) trigger.Input {
	t.Helper()
	requested, _ := research.NewTimestamp(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC).Add(offset))
	topic, _ := research.NewResearchTopic(subject, "software", "go")
	requestID, _ := research.NewID(requestValue)
	queueID, _ := research.NewID(queueValue)
	return trigger.Input{QueueID: queueID, AsOf: requested, Request: research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: requested}}
}
