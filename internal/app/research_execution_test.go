package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/research/queryplanner"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestResearchTopicSearchStageExecutesBoundedPlanAndRecordsAdapterMetadata(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	requestID, _ := research.NewID("request.topic-search-stage")
	runID, _ := research.NewID("run.topic-search-stage")
	first, _ := research.NewSourceLocator("https://docs.example.test/interfaces")
	second, _ := research.NewSourceLocator("https://spec.example.test/interfaces")
	discovery := &topicSearchDiscovery{results: [][]researchapp.SearchResult{
		{{Title: "Go interfaces documentation", Locator: first, Provider: "fixture", Rank: 0}},
		{{Title: "Go interfaces specification", Locator: second, Provider: "fixture", Rank: 0}},
	}}
	costs := &fakeResearchCostService{metadata: research.ResearchCostMetadata{
		Used:   research.ResearchCostUsage{SearchRequests: 2, ProviderAPICalls: 2},
		Budget: research.DefaultResearchCostBudgetV1(), AlgorithmVersion: research.ResearchCostControlAlgorithmV1,
	}}
	store := &fakeSourceRegistryStore{costs: costs, close: func() {}}
	searchFactory := &topicSearchFactory{result: researchapp.LiveSearchBuildResult{
		Discovery: discovery, ProviderID: "fixture", AdapterVersion: "fixture-search-v1",
	}}
	service := NewService(nil, nil).
		WithConfig(&recordingConfigStore{project: config.Settings{
			config.KeyAllowNetwork: config.BoolValue(true), config.KeyResearchSearchProvider: config.StringValue("fixture"),
			config.KeyResearchSearchMaxResultsPerQuery: config.NumberValue(2), config.KeyResearchSearchMaxQueriesPerRun: config.NumberValue(2),
		}}).
		WithSecrets(&recordingSecretStore{}).
		WithResearchSearch(searchFactory).
		WithResearchClock(func() time.Time { return at })
	plan := queryplanner.ResearchQueryPlan{AlgorithmVersion: queryplanner.AlgorithmVersion, Queries: []queryplanner.ResearchQuery{
		{Query: "Go interfaces documentation", DesiredSourceKind: research.SourceOfficialDocumentation, RequiredAuthority: research.AuthorityTierC, Priority: 1},
		{Query: "Go interfaces specification", DesiredSourceKind: research.SourceSpecification, RequiredAuthority: research.AuthorityTierC, Priority: 2},
	}}
	stage := &researchTopicSearchStage{service: service, request: ResearchTopicExecutionRequest{
		Store: store, Workspace: "/workspace", RunID: runID, Plan: plan, MaxResultsPerQuery: 2,
	}}
	topic, _ := research.NewResearchTopic("Go interfaces", "software", "Go")
	requestedAt, _ := research.NewTimestamp(at)
	artifacts, err := stage.Execute(context.Background(), researchapp.LiveResearchStageInput{
		Request: research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: requestedAt},
		Mode:    researchapp.ResearchModeAuto,
	})
	if err != nil {
		t.Fatal(err)
	}
	if discovery.calls != 2 || len(artifacts.SearchResults) != 2 || len(artifacts.Candidates) != 2 || artifacts.SearchExecution == nil {
		t.Fatalf("search artifacts = %+v, calls=%d", artifacts, discovery.calls)
	}
	metadata := artifacts.SearchExecution
	if metadata.ProviderID != "fixture" || metadata.AdapterVersion != "fixture-search-v1" || metadata.QueryCount != 2 || metadata.ResultCount != 2 || metadata.ProviderAPICalls != 2 {
		t.Fatalf("search execution metadata = %+v", metadata)
	}
	if searchFactory.calls != 1 || searchFactory.request.RunID != runID {
		t.Fatalf("search factory calls=%d request=%+v", searchFactory.calls, searchFactory.request)
	}
}

func TestResearchTopicSearchStageStopsAtLiveSourceProcessingBound(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	requestID, _ := research.NewID("request.topic-search-bounds")
	runID, _ := research.NewID("run.topic-search-bounds")
	discovery := &saturatingTopicSearchDiscovery{}
	costs := &fakeResearchCostService{metadata: research.ResearchCostMetadata{
		Used:   research.ResearchCostUsage{SearchRequests: 2, ProviderAPICalls: 2},
		Budget: research.DefaultResearchCostBudgetV1(), AlgorithmVersion: research.ResearchCostControlAlgorithmV1,
	}}
	store := &fakeSourceRegistryStore{costs: costs, close: func() {}}
	searchFactory := &topicSearchFactory{result: researchapp.LiveSearchBuildResult{
		Discovery: discovery, ProviderID: "fixture", AdapterVersion: "fixture-search-v1",
	}}
	service := NewService(nil, nil).
		WithConfig(&recordingConfigStore{project: config.Settings{
			config.KeyAllowNetwork: config.BoolValue(true), config.KeyResearchSearchProvider: config.StringValue("fixture"),
			config.KeyResearchSearchMaxResultsPerQuery: config.NumberValue(100), config.KeyResearchSearchMaxQueriesPerRun: config.NumberValue(3),
		}}).
		WithSecrets(&recordingSecretStore{}).
		WithResearchSearch(searchFactory).
		WithResearchClock(func() time.Time { return at })
	plan := queryplanner.ResearchQueryPlan{AlgorithmVersion: queryplanner.AlgorithmVersion, Queries: []queryplanner.ResearchQuery{
		{Query: "bounded documentation", DesiredSourceKind: research.SourceOfficialDocumentation, RequiredAuthority: research.AuthorityTierC, Priority: 1},
		{Query: "bounded specification", DesiredSourceKind: research.SourceSpecification, RequiredAuthority: research.AuthorityTierC, Priority: 2},
		{Query: "bounded standard", DesiredSourceKind: research.SourceStandard, RequiredAuthority: research.AuthorityTierC, Priority: 3},
	}}
	stage := &researchTopicSearchStage{service: service, request: ResearchTopicExecutionRequest{
		Store: store, Workspace: "/workspace", RunID: runID, Plan: plan, MaxResultsPerQuery: researchapp.MaximumSearchResults,
	}}
	topic, _ := research.NewResearchTopic("bounded research", "software", "Go")
	requestedAt, _ := research.NewTimestamp(at)
	artifacts, err := stage.Execute(context.Background(), researchapp.LiveResearchStageInput{
		Request: research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: requestedAt},
		Mode:    researchapp.ResearchModeAuto,
	})
	if err != nil {
		t.Fatal(err)
	}
	if discovery.calls != 2 || len(discovery.limits) != 2 ||
		discovery.limits[0] != researchapp.MaximumSearchResults || discovery.limits[1] != researchapp.MaximumSearchResults {
		t.Fatalf("discovery calls/limits = %d/%v", discovery.calls, discovery.limits)
	}
	if len(artifacts.SearchResults) != researchapp.MaximumLiveResearchSourcesPerRun ||
		len(artifacts.Candidates) != researchapp.MaximumLiveResearchSourcesPerRun || artifacts.SearchExecution == nil ||
		artifacts.SearchExecution.QueryCount != 2 || artifacts.SearchExecution.ResultCount != researchapp.MaximumLiveResearchSourcesPerRun {
		t.Fatalf("bounded search artifacts = results:%d candidates:%d metadata:%+v",
			len(artifacts.SearchResults), len(artifacts.Candidates), artifacts.SearchExecution)
	}
}

type topicSearchDiscovery struct {
	results [][]researchapp.SearchResult
	calls   int
}

type saturatingTopicSearchDiscovery struct {
	calls  int
	limits []int
}

func (discovery *saturatingTopicSearchDiscovery) Search(
	_ context.Context,
	_ researchapp.ResearchMode,
	_ researchapp.SearchQuery,
	options researchapp.SearchOptions,
) ([]researchapp.SearchResult, error) {
	call := discovery.calls
	discovery.calls++
	discovery.limits = append(discovery.limits, options.Limit)
	results := make([]researchapp.SearchResult, options.Limit)
	for index := range results {
		locator, _ := research.NewSourceLocator(fmt.Sprintf("https://source-%03d-%03d.example.test/docs", call, index))
		results[index] = researchapp.SearchResult{Title: "Bounded result", Locator: locator, Provider: "fixture", Rank: index}
	}
	return results, nil
}

func (discovery *topicSearchDiscovery) Search(_ context.Context, _ researchapp.ResearchMode, _ researchapp.SearchQuery, _ researchapp.SearchOptions) ([]researchapp.SearchResult, error) {
	result := discovery.results[discovery.calls]
	discovery.calls++
	return result, nil
}

type topicSearchFactory struct {
	result  researchapp.LiveSearchBuildResult
	request researchapp.LiveSearchBuildRequest
	calls   int
}

func (factory *topicSearchFactory) Build(_ context.Context, request researchapp.LiveSearchBuildRequest) (researchapp.LiveSearchBuildResult, error) {
	factory.calls++
	factory.request = request
	return factory.result, nil
}

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
		Finalization:  appResearchFinalizer{queue: request.Store.Triggers(), research: request.Store.Research()},
		TerminalAudit: appTerminalAudit{},
		Clock:         researchTopicExecutionClock{at: executor.at},
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

type appTerminalAudit struct{}

func (appTerminalAudit) Prepare(_ context.Context, request researchapp.LiveResearchTerminalAuditRequest) (researchapp.LiveResearchTerminalAuditResult, error) {
	cost := *request.Run.Cost
	bundleID := request.Artifacts.Bundle.ID
	completedAt := request.FinalizedAt
	recordedAt, _ := research.NewTimestamp(request.FinalizedAt.Time().Add(time.Nanosecond))
	id, _ := research.NewID("audit.research-execution.terminal")
	audit, err := research.SealResearchRunAuditV1(research.ResearchRunAudit{
		ID: id, RunID: request.Run.ID, RecordedAt: recordedAt, StartedAt: request.Run.StartedAt,
		CompletedAt: &completedAt, Outcome: request.Outcome, QueryPlannerVersion: "query-planner-v1",
		TrustPolicyVersion: "trust-policy-v1", FreshnessVersion: research.FreshnessAlgorithmV1,
		ConflictResolverVersion: research.ConflictResolverAlgorithmV1, NetworkMode: research.ResearchAuditNetworkAuto,
		Queries: []string{"Go interfaces official documentation"}, Execution: &research.ResearchAuditExecution{
			BundleID: &bundleID, CostUsed: cost.Used, CacheSavings: cost.CacheSavings,
			AlgorithmVersion: research.LiveResearchExecutionAuditV1,
		},
	})
	return researchapp.LiveResearchTerminalAuditResult{Audit: audit, Cost: cost, AlgorithmVersion: researchapp.LiveResearchTerminalAuditV1}, err
}

func (finalizer appResearchFinalizer) Finalize(ctx context.Context, finalization researchapp.ResearchFinalization) (researchapp.ResearchFinalizationResult, error) {
	run, err := finalizer.research.TransitionRun(ctx, finalization.RunID, finalization.RunStatus, finalization.FinalizedAt)
	if err != nil {
		return researchapp.ResearchFinalizationResult{}, err
	}
	if finalization.Cost != nil {
		cost := *finalization.Cost
		run.Cost = &cost
	}
	if finalization.Audit != nil {
		if err := finalizer.research.RecordAudit(ctx, *finalization.Audit); err != nil {
			return researchapp.ResearchFinalizationResult{}, err
		}
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
	return researchapp.ResearchFinalizationResult{Run: run, Execution: settled, Audit: finalization.Audit}, nil
}
