package sqlite

import (
	"context"
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
