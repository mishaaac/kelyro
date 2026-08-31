package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/research/trigger"
)

func TestResearchQueueConsumerClaimsExecutesAcknowledgesAndIsIdempotent(t *testing.T) {
	t.Parallel()
	fixture := newQueueConsumerFixture(t)
	orchestrator := &queueConsumerOrchestrator{research: fixture.research, at: fixture.at}
	consumer := fixture.consumer(t, orchestrator)
	request := application.ResearchQueueConsumeRequest{QueueItemID: fixture.item.ID, RunID: fixture.run.ID, Mode: application.ResearchModeAuto}

	first, err := consumer.Consume(context.Background(), request)
	if err != nil || first.Disposition != application.ResearchQueueConsumeCompleted || first.Execution.Attempts != 1 ||
		first.Execution.Status != application.ResearchQueueExecutionCompleted || first.QueueItem.Status != research.ResearchQueueDispatched || orchestrator.calls != 1 {
		t.Fatalf("first consume = (%+v,%v), calls=%d", first, err, orchestrator.calls)
	}
	second, err := consumer.Consume(context.Background(), request)
	if err != nil || second.Disposition != application.ResearchQueueConsumeCompleted || !second.Idempotent || orchestrator.calls != 1 {
		t.Fatalf("idempotent consume = (%+v,%v), calls=%d", second, err, orchestrator.calls)
	}
}

func TestResearchQueueConsumerRetriesTransientFailureWithSameLogicalQueue(t *testing.T) {
	t.Parallel()
	fixture := newQueueConsumerFixture(t)
	orchestrator := &queueConsumerOrchestrator{research: fixture.research, at: fixture.at, fail: application.ErrorExternalFailure}
	consumer := fixture.consumer(t, orchestrator)
	first, err := consumer.Consume(context.Background(), application.ResearchQueueConsumeRequest{
		QueueItemID: fixture.item.ID, RunID: fixture.run.ID, Mode: application.ResearchModeOnline,
	})
	if !errors.Is(err, application.ErrExternalFailure) || first.Disposition != application.ResearchQueueConsumeRetry ||
		first.Execution.Status != application.ResearchQueueExecutionRetry || first.QueueItem.Status != research.ResearchQueueQueued {
		t.Fatalf("transient consume = (%+v,%v)", first, err)
	}

	retryRun := fixture.startRun(t, "run.queue-consumer-retry")
	orchestrator.fail = ""
	second, err := consumer.Consume(context.Background(), application.ResearchQueueConsumeRequest{
		QueueItemID: fixture.item.ID, RunID: retryRun.ID, Mode: application.ResearchModeOnline,
	})
	if err != nil || second.Disposition != application.ResearchQueueConsumeCompleted || second.Execution.Attempts != 2 ||
		second.Execution.RunID != retryRun.ID || second.QueueItem.ID != fixture.item.ID {
		t.Fatalf("retry consume = (%+v,%v)", second, err)
	}
}

func TestResearchQueueConsumerFailsPermanentWorkAndCancelsStoppedWork(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name        string
		failure     application.ErrorKind
		cause       error
		disposition application.ResearchQueueConsumeDisposition
		execution   application.ResearchQueueExecutionStatus
		queue       research.ResearchQueueStatus
	}{
		{name: "permanent", failure: application.ErrorNetworkResearchBlocked, disposition: application.ResearchQueueConsumeFailed, execution: application.ResearchQueueExecutionFailed, queue: research.ResearchQueueDispatched},
		{name: "cancelled", cause: context.Canceled, disposition: application.ResearchQueueConsumeCancelled, execution: application.ResearchQueueExecutionCancelled, queue: research.ResearchQueueCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newQueueConsumerFixture(t)
			orchestrator := &queueConsumerOrchestrator{research: fixture.research, at: fixture.at, fail: test.failure, cause: test.cause}
			result, err := fixture.consumer(t, orchestrator).Consume(context.Background(), application.ResearchQueueConsumeRequest{
				QueueItemID: fixture.item.ID, RunID: fixture.run.ID, Mode: application.ResearchModeAuto,
			})
			if err == nil || result.Disposition != test.disposition || result.Execution.Status != test.execution || result.QueueItem.Status != test.queue {
				t.Fatalf("consume = (%+v,%v)", result, err)
			}
		})
	}
}

func TestResearchQueueConsumerDoesNotExecuteAnExistingClaim(t *testing.T) {
	t.Parallel()
	fixture := newQueueConsumerFixture(t)
	claim, err := fixture.queue.ClaimExecution(context.Background(), application.ResearchQueueExecutionClaim{
		QueueItemID: fixture.item.ID, RunID: fixture.run.ID, At: fixture.at,
	})
	if err != nil || !claim.Acquired {
		t.Fatalf("claim = (%+v,%v)", claim, err)
	}
	orchestrator := &queueConsumerOrchestrator{research: fixture.research, at: fixture.at}
	result, err := fixture.consumer(t, orchestrator).Consume(context.Background(), application.ResearchQueueConsumeRequest{
		QueueItemID: fixture.item.ID, RunID: fixture.run.ID, Mode: application.ResearchModeAuto,
	})
	if err != nil || result.Disposition != application.ResearchQueueConsumeInFlight || !result.Idempotent || orchestrator.calls != 0 {
		t.Fatalf("consume claimed = (%+v,%v), calls=%d", result, err, orchestrator.calls)
	}
}

type queueConsumerFixture struct {
	queue    application.ResearchTriggerService
	research application.ResearchService
	item     research.ResearchQueueItem
	run      research.ResearchRun
	at       research.Timestamp
}

func newQueueConsumerFixture(t *testing.T) queueConsumerFixture {
	t.Helper()
	at, _ := research.NewTimestamp(time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC))
	store := memory.New()
	repositories := store.Repositories()
	queue := application.NewResearchTriggerService(repositories.TriggerQueue)
	researchService := application.NewResearchService(repositories.Runs)
	topic, _ := research.NewResearchTopic("Go interfaces", "software", "go")
	requestID, _ := research.NewID("request.queue-consumer")
	queueID, _ := research.NewID("queue.queue-consumer")
	request := research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: at}
	decision, err := queue.Evaluate(context.Background(), trigger.Input{
		QueueID: queueID, Request: request, Signals: trigger.Signals{Manual: true, EvidenceCount: 0}, AsOf: at,
	})
	if err != nil || decision.QueueItem == nil {
		t.Fatalf("queue fixture = (%+v,%v)", decision, err)
	}
	runID, _ := research.NewID("run.queue-consumer")
	run := research.ResearchRun{ID: runID, RequestID: requestID, Status: research.ResearchRunPlanned, StartedAt: at}
	if err := researchService.Start(context.Background(), request, run); err != nil {
		t.Fatal(err)
	}
	return queueConsumerFixture{queue: queue, research: researchService, item: *decision.QueueItem, run: run, at: at}
}

func (fixture queueConsumerFixture) consumer(t *testing.T, orchestrator application.LiveResearchOrchestrator) application.ResearchQueueConsumer {
	t.Helper()
	consumer, err := application.NewResearchQueueConsumer(application.ResearchQueueConsumerDependencies{
		Queue: fixture.queue, Research: fixture.research, Orchestrator: orchestrator, Clock: fixedQueueConsumerClock{fixture.at},
	})
	if err != nil {
		t.Fatal(err)
	}
	return consumer
}

func (fixture queueConsumerFixture) startRun(t *testing.T, value string) research.ResearchRun {
	t.Helper()
	id, _ := research.NewID(value)
	run := research.ResearchRun{ID: id, RequestID: fixture.item.Request.ID, Status: research.ResearchRunPlanned, StartedAt: fixture.at}
	if err := fixture.research.Start(context.Background(), fixture.item.Request, run); err != nil {
		t.Fatal(err)
	}
	return run
}

type fixedQueueConsumerClock struct{ at research.Timestamp }

func (clock fixedQueueConsumerClock) Now() research.Timestamp { return clock.at }

type queueConsumerOrchestrator struct {
	research application.ResearchService
	at       research.Timestamp
	fail     application.ErrorKind
	cause    error
	calls    int
}

func (orchestrator *queueConsumerOrchestrator) Execute(ctx context.Context, request application.LiveResearchOrchestrationRequest) (application.LiveResearchOrchestrationResult, error) {
	orchestrator.calls++
	running, err := orchestrator.research.TransitionRun(ctx, request.RunID, research.ResearchRunRunning, orchestrator.at)
	if err != nil {
		return application.LiveResearchOrchestrationResult{}, err
	}
	if orchestrator.cause != nil {
		cancelled, _ := orchestrator.research.TransitionRun(context.WithoutCancel(ctx), request.RunID, research.ResearchRunCancelled, orchestrator.at)
		return application.LiveResearchOrchestrationResult{Run: cancelled}, orchestrator.cause
	}
	if orchestrator.fail != "" {
		failed, _ := orchestrator.research.TransitionRun(ctx, request.RunID, research.ResearchRunFailed, orchestrator.at)
		return application.LiveResearchOrchestrationResult{Run: failed}, application.Classify(orchestrator.fail, "fixture orchestration", errors.New("fixture failure"))
	}
	completed, err := orchestrator.research.TransitionRun(ctx, request.RunID, research.ResearchRunCompleted, orchestrator.at)
	if err != nil {
		return application.LiveResearchOrchestrationResult{}, err
	}
	return application.LiveResearchOrchestrationResult{Run: completed, Request: research.ResearchRequest{ID: running.RequestID}}, nil
}
