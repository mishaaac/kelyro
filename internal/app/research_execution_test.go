package app

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestResearchTopicExecutesQueuedWorkSynchronouslyWithBoundedContext(t *testing.T) {
	t.Parallel()
	root := "/workspaces/research-execution"
	at := time.Date(2026, 8, 31, 11, 0, 0, 0, time.UTC)
	timestamp, _ := research.NewTimestamp(at)
	memoryStore := memory.New()
	repositories := memoryStore.Repositories()
	factory := &fakeSourceRegistryStoreFactory{
		research: researchapp.NewResearchService(repositories.Runs),
		bundles:  researchapp.NewSourceBundleService(repositories.Bundles, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		triggers: researchapp.NewResearchTriggerService(repositories.TriggerQueue),
	}
	executor := &synchronousResearchTopicExecutor{at: timestamp}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).
		WithConfig(&recordingConfigStore{project: config.Settings{
			config.KeyAllowNetwork: config.BoolValue(false), config.KeyResearchSearchMaxQueriesPerRun: config.NumberValue(2),
		}}).
		WithResearchStores(factory).
		WithResearchClock(func() time.Time { return at }).
		WithResearchTopicExecutor(executor)

	result, err := service.Execute(context.Background(), Command{
		Action: ActionResearch, Workspace: root, ResearchOperation: "topic", ResearchTopic: "Go interfaces",
	})
	if err != nil || result.ResearchView == nil {
		t.Fatalf("research topic = (%+v,%v)", result.ResearchView, err)
	}
	view := result.ResearchView
	if executor.calls != 1 || !executor.hadDeadline || executor.deadlineRemaining <= 0 || executor.deadlineRemaining > researchTopicExecutionTimeoutV1 {
		t.Fatalf("executor calls=%d deadline=%v remaining=%s", executor.calls, executor.hadDeadline, executor.deadlineRemaining)
	}
	if executor.request.Store == nil || executor.request.Workspace != root || executor.request.QueueItemID != view.QueueItem.ID ||
		executor.request.RunID != view.Run.ID || executor.request.Mode != researchapp.ResearchModeAuto || executor.request.NetworkAllowed || len(executor.request.Plan.Queries) != 2 {
		t.Fatalf("execution request = %+v", executor.request)
	}
	if view.Run.Status != research.ResearchRunCompleted || view.QueueItem.Status != research.ResearchQueueDispatched ||
		view.DiscoveryPending || view.Execution == nil || view.Execution.Disposition != researchapp.ResearchQueueConsumeCompleted ||
		view.Execution.AlgorithmVersion != researchapp.ResearchQueueWorkerV1 {
		t.Fatalf("executed research view = %+v", view)
	}
	if factory.closed != 1 {
		t.Fatalf("store close count = %d", factory.closed)
	}
}

type synchronousResearchTopicExecutor struct {
	at                research.Timestamp
	request           ResearchTopicExecutionRequest
	calls             int
	hadDeadline       bool
	deadlineRemaining time.Duration
}

func (executor *synchronousResearchTopicExecutor) Execute(ctx context.Context, request ResearchTopicExecutionRequest) (researchapp.ResearchQueueConsumeResult, error) {
	executor.calls++
	executor.request = request
	deadline, ok := ctx.Deadline()
	executor.hadDeadline = ok
	if ok {
		executor.deadlineRemaining = time.Until(deadline)
	}
	orchestrator := &completingResearchTopicOrchestrator{research: request.Store.Research(), at: executor.at}
	consumer, err := researchapp.NewResearchQueueConsumer(researchapp.ResearchQueueConsumerDependencies{
		Queue: request.Store.Triggers(), Research: request.Store.Research(), Orchestrator: orchestrator,
		Finalization: appResearchFinalizer{queue: request.Store.Triggers(), research: request.Store.Research()},
		Clock:        researchTopicExecutionClock{at: executor.at},
	})
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	return consumer.Consume(ctx, researchapp.ResearchQueueConsumeRequest{
		QueueItemID: request.QueueItemID, RunID: request.RunID, Mode: request.Mode,
	})
}

type researchTopicExecutionClock struct{ at research.Timestamp }

func (clock researchTopicExecutionClock) Now() research.Timestamp { return clock.at }

type completingResearchTopicOrchestrator struct {
	research researchapp.ResearchService
	at       research.Timestamp
}

func (orchestrator *completingResearchTopicOrchestrator) Execute(ctx context.Context, request researchapp.LiveResearchOrchestrationRequest) (researchapp.LiveResearchOrchestrationResult, error) {
	if _, err := orchestrator.research.TransitionRun(ctx, request.RunID, research.ResearchRunRunning, orchestrator.at); err != nil {
		return researchapp.LiveResearchOrchestrationResult{}, err
	}
	running, err := orchestrator.research.Run(ctx, request.RunID)
	if err != nil {
		return researchapp.LiveResearchOrchestrationResult{}, err
	}
	bundleID, _ := research.NewID("bundle.research-execution")
	return researchapp.LiveResearchOrchestrationResult{Run: running, Artifacts: researchapp.LiveResearchArtifacts{Bundle: &research.SourceBundle{ID: bundleID, RunID: running.ID}}}, nil
}

type appResearchFinalizer struct {
	queue    researchapp.ResearchTriggerService
	research researchapp.ResearchService
}

func (finalizer appResearchFinalizer) Finalize(ctx context.Context, finalization researchapp.ResearchFinalization) (researchapp.ResearchFinalizationResult, error) {
	run, err := finalizer.research.TransitionRun(ctx, finalization.RunID, finalization.RunStatus, finalization.FinalizedAt)
	if err != nil {
		return researchapp.ResearchFinalizationResult{}, err
	}
	execution := researchapp.ResearchQueueExecution{
		QueueItemID: finalization.QueueItemID, RunID: finalization.RunID, Status: finalization.ExecutionStatus,
		Attempts: finalization.Attempts, ChangedAt: finalization.FinalizedAt, FailureKind: finalization.FailureKind,
		BundleID: finalization.BundleID, AlgorithmVersion: researchapp.ResearchQueueWorkerV1,
	}
	settled, err := finalizer.queue.SettleExecution(ctx, researchapp.ResearchQueueExecutionClaimed, execution)
	if err != nil {
		return researchapp.ResearchFinalizationResult{}, err
	}
	return researchapp.ResearchFinalizationResult{Run: run, Execution: settled}, nil
}
