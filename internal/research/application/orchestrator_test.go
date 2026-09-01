package application_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	triggerpolicy "github.com/mishaaac/kelyro/internal/research/trigger"
)

func TestLiveResearchOrchestratorLoadsDurableWorkAndExecutesFixedStageOrder(t *testing.T) {
	t.Parallel()
	queue, service, queueItem, run := liveResearchOrchestrationFixture(t)
	var calls []application.LiveResearchStage
	dependencies := liveResearchDependencies(queue, service, &calls, "")
	orchestrator, err := application.NewLiveResearchOrchestrator(dependencies)
	if err != nil {
		t.Fatal(err)
	}

	result, err := orchestrator.Execute(context.Background(), application.LiveResearchOrchestrationRequest{
		QueueItemID: queueItem.ID, RunID: run.ID, Mode: application.ResearchModeAuto,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := []application.LiveResearchStage{
		application.LiveResearchStageSearch,
		application.LiveResearchStageRegisterCandidates,
		application.LiveResearchStageFetch,
		application.LiveResearchStageSnapshot,
		application.LiveResearchStageNormalize,
		application.LiveResearchStageExtract,
		application.LiveResearchStageVerify,
		application.LiveResearchStageBundle,
		application.LiveResearchStageFinalize,
	}
	if !reflect.DeepEqual(calls, want) || !reflect.DeepEqual(result.CompletedStages, want) {
		t.Fatalf("stage order calls=%v result=%v, want %v", calls, result.CompletedStages, want)
	}
	if result.QueueItem.ID != queueItem.ID || result.Request.ID != queueItem.Request.ID || result.Run.ID != run.ID || result.AlgorithmVersion != application.LiveResearchOrchestratorV1 {
		t.Fatalf("orchestration identity/result = %+v", result)
	}
	if result.Run.Status != research.ResearchRunRunning || result.Run.CompletedAt != nil || result.Artifacts.Bundle == nil {
		t.Fatalf("prepared lifecycle result = %+v", result)
	}
}

func TestLiveResearchOrchestratorStopsAtFirstFailedStage(t *testing.T) {
	t.Parallel()
	queue, service, queueItem, run := liveResearchOrchestrationFixture(t)
	var calls []application.LiveResearchStage
	orchestrator, err := application.NewLiveResearchOrchestrator(
		liveResearchDependencies(queue, service, &calls, application.LiveResearchStageSnapshot),
	)
	if err != nil {
		t.Fatal(err)
	}

	result, err := orchestrator.Execute(context.Background(), application.LiveResearchOrchestrationRequest{
		QueueItemID: queueItem.ID, RunID: run.ID, Mode: application.ResearchModeOnline,
	})
	if err == nil || len(calls) != 4 || len(result.CompletedStages) != 3 {
		t.Fatalf("failed execution result=(%+v,%v), calls=%v", result, err, calls)
	}
	if calls[len(calls)-1] != application.LiveResearchStageSnapshot {
		t.Fatalf("last call = %q, want snapshot", calls[len(calls)-1])
	}
	if result.Run.Status != research.ResearchRunRunning || result.Run.CompletedAt != nil {
		t.Fatalf("unsettled failed lifecycle run = %+v", result.Run)
	}
}

func TestLiveResearchOrchestratorPreservesFailedStageArtifacts(t *testing.T) {
	t.Parallel()
	queue, service, queueItem, run := liveResearchOrchestrationFixture(t)
	var calls []application.LiveResearchStage
	dependencies := liveResearchDependencies(queue, service, &calls, "")
	source := testSource(t, "orchestrator-partial-fetch")
	dependencies.Fetch = partialFailureLiveResearchStage{source: source, calls: &calls}
	orchestrator, err := application.NewLiveResearchOrchestrator(dependencies)
	if err != nil {
		t.Fatal(err)
	}
	result, err := orchestrator.Execute(context.Background(), application.LiveResearchOrchestrationRequest{
		QueueItemID: queueItem.ID, RunID: run.ID, Mode: application.ResearchModeOnline,
	})
	if err == nil || result.Run.Status != research.ResearchRunRunning || len(result.Artifacts.FetchFailures) != 1 ||
		result.Artifacts.FetchFailures[0].SourceID != source.ID {
		t.Fatalf("failed stage artifacts = (%+v,%v)", result, err)
	}
}

func TestLiveResearchOrchestratorRejectsMismatchedOrCancelledWork(t *testing.T) {
	t.Parallel()
	queue, service, queueItem, run := liveResearchOrchestrationFixture(t)
	var calls []application.LiveResearchStage
	orchestrator, err := application.NewLiveResearchOrchestrator(liveResearchDependencies(queue, service, &calls, ""))
	if err != nil {
		t.Fatal(err)
	}

	otherRun := run
	otherRun.ID = testID(t, "run.orchestrator.other")
	otherRequest := queueItem.Request
	otherRequest.ID = testID(t, "request.orchestrator.other")
	otherRun.RequestID = otherRequest.ID
	if err := service.Start(context.Background(), otherRequest, otherRun); err != nil {
		t.Fatal(err)
	}
	_, err = orchestrator.Execute(context.Background(), application.LiveResearchOrchestrationRequest{
		QueueItemID: queueItem.ID, RunID: otherRun.ID, Mode: application.ResearchModeAuto,
	})
	if !errors.Is(err, application.ErrInvalidState) || len(calls) != 0 {
		t.Fatalf("mismatched work error=%v calls=%v", err, calls)
	}

	cancelledAt := testTimestamp(t, 12)
	if _, err := queue.Cancel(context.Background(), queueItem.ID, cancelledAt); err != nil {
		t.Fatal(err)
	}
	_, err = orchestrator.Execute(context.Background(), application.LiveResearchOrchestrationRequest{
		QueueItemID: queueItem.ID, RunID: run.ID, Mode: application.ResearchModeAuto,
	})
	if !errors.Is(err, application.ErrInvalidState) || len(calls) != 0 {
		t.Fatalf("cancelled work error=%v calls=%v", err, calls)
	}
}

func TestLiveResearchOrchestratorRequiresEveryStage(t *testing.T) {
	t.Parallel()
	queue, service, _, _ := liveResearchOrchestrationFixture(t)
	var calls []application.LiveResearchStage
	dependencies := liveResearchDependencies(queue, service, &calls, "")
	dependencies.Verify = nil
	if _, err := application.NewLiveResearchOrchestrator(dependencies); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("NewLiveResearchOrchestrator() error = %v, want unavailable", err)
	}
}

func TestLiveResearchOrchestratorRequiresBundleBeforeCompletion(t *testing.T) {
	t.Parallel()
	queue, service, queueItem, run := liveResearchOrchestrationFixture(t)
	var calls []application.LiveResearchStage
	dependencies := liveResearchDependencies(queue, service, &calls, "")
	dependencies.Bundle = passthroughLiveResearchStage{}
	orchestrator, err := application.NewLiveResearchOrchestrator(dependencies)
	if err != nil {
		t.Fatal(err)
	}
	result, err := orchestrator.Execute(context.Background(), application.LiveResearchOrchestrationRequest{
		QueueItemID: queueItem.ID, RunID: run.ID, Mode: application.ResearchModeAuto,
	})
	if !errors.Is(err, application.ErrInvalidState) || result.Run.Status != research.ResearchRunRunning {
		t.Fatalf("missing bundle result = (%+v,%v)", result, err)
	}
	for _, call := range calls {
		if call == application.LiveResearchStageFinalize {
			t.Fatal("finalize ran after bundle stage returned no bundle")
		}
	}
}

func TestLiveResearchOrchestratorReturnsCancellationForAtomicSettlement(t *testing.T) {
	t.Parallel()
	queue, service, queueItem, run := liveResearchOrchestrationFixture(t)
	var calls []application.LiveResearchStage
	dependencies := liveResearchDependencies(queue, service, &calls, "")
	dependencies.Fetch = cancellingLiveResearchStage{calls: &calls}
	orchestrator, err := application.NewLiveResearchOrchestrator(dependencies)
	if err != nil {
		t.Fatal(err)
	}
	result, err := orchestrator.Execute(context.Background(), application.LiveResearchOrchestrationRequest{
		QueueItemID: queueItem.ID, RunID: run.ID, Mode: application.ResearchModeAuto,
	})
	if !errors.Is(err, context.Canceled) || result.Run.Status != research.ResearchRunRunning || result.Run.CompletedAt != nil {
		t.Fatalf("cancelled result = (%+v,%v)", result, err)
	}
	if len(calls) != 3 || calls[2] != application.LiveResearchStageFetch {
		t.Fatalf("calls after cancellation = %v", calls)
	}
}

type recordingLiveResearchStage struct {
	stage application.LiveResearchStage
	calls *[]application.LiveResearchStage
	fail  application.LiveResearchStage
}

type passthroughLiveResearchStage struct{}

type partialFailureLiveResearchStage struct {
	source research.Source
	calls  *[]application.LiveResearchStage
}

func (stage partialFailureLiveResearchStage) Execute(_ context.Context, input application.LiveResearchStageInput) (application.LiveResearchArtifacts, error) {
	*stage.calls = append(*stage.calls, application.LiveResearchStageFetch)
	input.Artifacts.FetchFailures = []application.SourceFetchFailure{{
		SourceID: stage.source.ID, Locator: stage.source.Locator, Kind: application.ErrorExternalFailure,
	}}
	return input.Artifacts, errors.New("fixture fetch failure")
}

func (passthroughLiveResearchStage) Execute(_ context.Context, input application.LiveResearchStageInput) (application.LiveResearchArtifacts, error) {
	return input.Artifacts, nil
}

type cancellingLiveResearchStage struct {
	calls *[]application.LiveResearchStage
}

func (stage cancellingLiveResearchStage) Execute(context.Context, application.LiveResearchStageInput) (application.LiveResearchArtifacts, error) {
	*stage.calls = append(*stage.calls, application.LiveResearchStageFetch)
	return application.LiveResearchArtifacts{}, context.Canceled
}

func (stage recordingLiveResearchStage) Execute(_ context.Context, input application.LiveResearchStageInput) (application.LiveResearchArtifacts, error) {
	*stage.calls = append(*stage.calls, stage.stage)
	if stage.stage == stage.fail {
		return application.LiveResearchArtifacts{}, errors.New("fixture stage failure")
	}
	if stage.stage == application.LiveResearchStageBundle {
		bundleID, _ := research.NewID("bundle.orchestrator.fixture")
		input.Artifacts.Bundle = &research.SourceBundle{ID: bundleID, RunID: input.Run.ID}
	}
	return input.Artifacts, nil
}

func liveResearchDependencies(queue application.ResearchTriggerService, service application.ResearchService, calls *[]application.LiveResearchStage, fail application.LiveResearchStage) application.LiveResearchOrchestratorDependencies {
	stage := func(value application.LiveResearchStage) application.LiveResearchStageService {
		return recordingLiveResearchStage{stage: value, calls: calls, fail: fail}
	}
	return application.LiveResearchOrchestratorDependencies{
		Queue: queue, Research: service, Clock: fixedClock{now: testTimestampNoHelper(13)},
		Search:             stage(application.LiveResearchStageSearch),
		RegisterCandidates: stage(application.LiveResearchStageRegisterCandidates),
		Fetch:              stage(application.LiveResearchStageFetch), Snapshot: stage(application.LiveResearchStageSnapshot),
		Normalize: stage(application.LiveResearchStageNormalize), Extract: stage(application.LiveResearchStageExtract),
		Verify: stage(application.LiveResearchStageVerify), Bundle: stage(application.LiveResearchStageBundle),
		Finalize: stage(application.LiveResearchStageFinalize),
	}
}

func testTimestampNoHelper(hour int) research.Timestamp {
	value, _ := research.NewTimestamp(time.Date(2026, 8, 24, hour, 0, 0, 0, time.UTC))
	return value
}

func liveResearchOrchestrationFixture(t *testing.T) (application.ResearchTriggerService, application.ResearchService, research.ResearchQueueItem, research.ResearchRun) {
	t.Helper()
	store := memory.New()
	repositories := store.Repositories()
	queue := application.NewResearchTriggerService(repositories.TriggerQueue)
	service := application.NewResearchService(repositories.Runs)
	request, run := testRequestRun(t)
	run.Status = research.ResearchRunPlanned
	input := triggerpolicy.Input{
		QueueID: testID(t, "queue.orchestrator"), Request: request,
		AsOf: testTimestamp(t, 11), Signals: triggerpolicy.Signals{Manual: true},
	}
	decision, err := queue.Evaluate(context.Background(), input)
	if err != nil || decision.QueueItem == nil {
		t.Fatalf("queue fixture = (%+v,%v)", decision, err)
	}
	if err := service.Start(context.Background(), request, run); err != nil {
		t.Fatal(err)
	}
	return queue, service, *decision.QueueItem, run
}
