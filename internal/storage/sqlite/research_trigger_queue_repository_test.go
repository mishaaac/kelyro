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

func sqliteTriggerInput(t *testing.T, requestValue, queueValue, subject string, offset time.Duration) trigger.Input {
	t.Helper()
	requested, _ := research.NewTimestamp(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC).Add(offset))
	topic, _ := research.NewResearchTopic(subject, "software", "go")
	requestID, _ := research.NewID(requestValue)
	queueID, _ := research.NewID(queueValue)
	return trigger.Input{QueueID: queueID, AsOf: requested, Request: research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: requested}}
}
