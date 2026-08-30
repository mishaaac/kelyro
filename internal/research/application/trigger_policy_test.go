package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/research/trigger"
)

func TestResearchTriggerServiceDeduplicatesQueuedWorkAndTransitionsExplicitly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	service := application.NewResearchTriggerService(store.Repositories().TriggerQueue)
	input := applicationTriggerInput(t, "request.trigger-service-one", "queue.trigger-service-one")
	input.Signals = trigger.Signals{Manual: true, EvidenceCount: 2}
	first, err := service.Evaluate(ctx, input)
	if err != nil || first.QueueItem == nil {
		t.Fatalf("first trigger = (%+v,%v)", first, err)
	}
	duplicate := applicationTriggerInput(t, "request.trigger-service-two", "queue.trigger-service-two")
	duplicate.Signals = trigger.Signals{NewTechnologyRelease: true, EvidenceCount: 2}
	second, err := service.Evaluate(ctx, duplicate)
	if err != nil || second.QueueItem == nil || second.QueueItem.ID != first.QueueItem.ID || second.Priority != research.VerificationPriorityCritical {
		t.Fatalf("deduplicated trigger = (%+v,%v)", second, err)
	}
	queued, err := service.Queued(ctx)
	if err != nil || len(queued) != 1 {
		t.Fatalf("queued = (%+v,%v)", queued, err)
	}
	dispatchedAt, _ := research.NewTimestamp(input.AsOf.Time().Add(time.Minute))
	dispatched, err := service.MarkDispatched(ctx, first.QueueItem.ID, dispatchedAt)
	if err != nil || dispatched.Status != research.ResearchQueueDispatched || dispatched.StatusChangedAt == nil {
		t.Fatalf("dispatched = (%+v,%v)", dispatched, err)
	}
	queued, err = service.Queued(ctx)
	if err != nil || len(queued) != 0 {
		t.Fatalf("post-dispatch queue = (%+v,%v)", queued, err)
	}
	third := applicationTriggerInput(t, "request.trigger-service-three", "queue.trigger-service-three")
	third.Signals = trigger.Signals{NewTechnologyRelease: true, EvidenceCount: 2}
	requeued, err := service.Evaluate(ctx, third)
	if err != nil || requeued.QueueItem == nil || requeued.QueueItem.ID == first.QueueItem.ID {
		t.Fatalf("requeued trigger = (%+v,%v)", requeued, err)
	}
}

func applicationTriggerInput(t *testing.T, requestValue, queueValue string) trigger.Input {
	t.Helper()
	at, _ := research.NewTimestamp(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	topic, _ := research.NewResearchTopic("range-over-func", "software", "go")
	requestID, _ := research.NewID(requestValue)
	queueID, _ := research.NewID(queueValue)
	return trigger.Input{QueueID: queueID, AsOf: at, Request: research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: at}}
}
