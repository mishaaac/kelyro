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

func TestServiceCoordinatesResearchCacheStatusAndClear(t *testing.T) {
	t.Parallel()
	root := "/workspaces/research-cache"
	cache := &fakeResearchCacheService{
		status: researchapp.ResearchCacheStatus{
			AlgorithmVersion: researchapp.ResearchCacheAlgorithmV1,
			TotalEntries:     2, TotalPayloadBytes: 120,
			Layers: []researchapp.CacheLayerStatus{{Layer: researchapp.CacheLayerDiscovery, Entries: 2, PayloadBytes: 120}},
		},
		cleared: researchapp.ResearchCacheClearResult{RemovedEntries: 2, RemovedBytes: 220},
	}
	factory := &fakeResearchCacheFactory{service: cache}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).
		WithResearchCaches(factory)
	status, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchCacheOperation: "status"})
	if err != nil || status.ResearchCacheStatus == nil || status.ResearchCacheStatus.TotalEntries != 2 {
		t.Fatalf("cache status = (%+v,%v)", status, err)
	}
	cleared, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchCacheOperation: "clear"})
	if err != nil || cleared.ResearchCacheCleared == nil || cleared.ResearchCacheCleared.RemovedEntries != 2 {
		t.Fatalf("cache clear = (%+v,%v)", cleared, err)
	}
	if factory.openRoots[0] != root || factory.openRoots[1] != root || cache.statusCalls != 1 || cache.clearCalls != 1 {
		t.Fatalf("cache coordination roots=%v status=%d clear=%d", factory.openRoots, cache.statusCalls, cache.clearCalls)
	}
}

func TestServiceCoordinatesResearchCostStats(t *testing.T) {
	t.Parallel()
	root := "/workspaces/research-stats"
	at, _ := research.NewTimestamp(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	costs := &fakeResearchCostService{stats: researchapp.ResearchCostStats{
		Used: research.ResearchCostUsage{SearchRequests: 2}, TodayUsed: research.ResearchCostUsage{SearchRequests: 1},
		Runs: 2, BudgetStoppedRuns: 1, AsOf: at, AlgorithmVersion: research.ResearchCostControlAlgorithmV1,
	}}
	factory := &fakeSourceRegistryStoreFactory{costs: costs}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).
		WithResearchStores(factory).WithResearchClock(func() time.Time { return at.Time() })
	result, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchOperation: "stats"})
	if err != nil || result.ResearchCostStats == nil || result.ResearchCostStats.BudgetStoppedRuns != 1 || costs.statsCalls != 1 {
		t.Fatalf("research stats = (%+v,%v), calls=%d", result, err, costs.statsCalls)
	}
}

func TestServiceCoordinatesOfflineResearchUpdateScan(t *testing.T) {
	t.Parallel()
	root := "/workspaces/research-update-scan"
	at, _ := research.NewTimestamp(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	scanner := &fakeUpdateScanService{scan: research.UpdateScan{
		ScannedAt: at, IncompleteReasons: []research.UpdateScanIncompleteReason{research.UpdateScanNetworkDisabled},
		AlgorithmVersion: research.UpdateScanAlgorithmV1,
	}}
	factory := &fakeSourceRegistryStoreFactory{updateScan: scanner}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).
		WithConfig(&recordingConfigStore{project: config.Settings{config.KeyAllowNetwork: config.BoolValue(false)}}).
		WithResearchStores(factory).WithResearchClock(func() time.Time { return at.Time() })
	result, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchOperation: "update-scan"})
	if err != nil || result.UpdateScan == nil || result.UpdateScan.Complete() || scanner.calls != 1 {
		t.Fatalf("update scan = (%+v,%v), calls=%d", result.UpdateScan, err, scanner.calls)
	}
}

func TestResearchRunProgressUsesDurableAuditAndBundleState(t *testing.T) {
	t.Parallel()
	started, _ := research.NewTimestamp(time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC))
	completed, _ := research.NewTimestamp(time.Date(2026, 9, 1, 9, 1, 0, 0, time.UTC))
	runID, _ := research.NewID("run.progress.failed")
	query := "Go interfaces official documentation"
	planned, err := research.SealResearchRunAuditV1(research.ResearchRunAudit{
		ID: mustAppResearchID(t, "audit.progress.planned"), RunID: runID, RecordedAt: started, StartedAt: started,
		Outcome: research.ResearchRunPlanned, QueryPlannerVersion: "query-planner-v1", TrustPolicyVersion: "trust-policy-v1",
		FreshnessVersion: research.FreshnessAlgorithmV1, ConflictResolverVersion: research.ConflictResolverAlgorithmV1,
		NetworkMode: research.ResearchAuditNetworkAuto, NetworkAllowed: true, Queries: []string{query},
	})
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := research.SealResearchRunAuditV1(research.ResearchRunAudit{
		ID: mustAppResearchID(t, "audit.progress.failed"), RunID: runID, RecordedAt: completed, StartedAt: started, CompletedAt: &completed,
		Outcome: research.ResearchRunFailed, QueryPlannerVersion: "query-planner-v1", TrustPolicyVersion: "trust-policy-v1",
		FreshnessVersion: research.FreshnessAlgorithmV1, ConflictResolverVersion: research.ConflictResolverAlgorithmV1,
		ProvidersUsed: []string{"brave"}, NetworkMode: research.ResearchAuditNetworkAuto, NetworkAllowed: true, Queries: []string{query},
		Execution: &research.ResearchAuditExecution{
			Providers:  []research.ResearchAuditProvider{{ProviderID: "brave", AdapterVersion: "brave-web-search-v1", APICalls: 1}},
			QueryCount: 1, ResultCount: 2, FailureKind: string(researchapp.ErrorExternalFailure),
			CostUsed: research.ResearchCostUsage{SearchRequests: 1, ProviderAPICalls: 1}, AlgorithmVersion: research.LiveResearchExecutionAuditV1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	progress, err := researchRunProgress(research.ResearchRun{
		ID: runID, RequestID: mustAppResearchID(t, "request.progress.failed"), Status: research.ResearchRunFailed,
		StartedAt: started, CompletedAt: &completed,
	}, []research.ResearchRunAudit{planned, terminal}, nil)
	if err != nil || progress.Phase != "failed" || len(progress.Queries) != 1 || progress.Queries[0] != query ||
		len(progress.Providers) != 1 || progress.Providers[0].AdapterVersion != "brave-web-search-v1" ||
		progress.Results != 2 || progress.FailureReason != string(researchapp.ErrorExternalFailure) {
		t.Fatalf("failed progress = (%+v, %v)", progress, err)
	}

	bundleID := mustAppResearchID(t, "bundle.progress.completed")
	completedRunID := mustAppResearchID(t, "run.progress.completed")
	completedProgress, err := researchRunProgress(research.ResearchRun{
		ID: completedRunID, RequestID: mustAppResearchID(t, "request.progress.completed"), Status: research.ResearchRunCompleted,
		StartedAt: started, CompletedAt: &completed,
	}, nil, &research.SourceBundle{
		ID: bundleID, RunID: completedRunID, State: research.BundleReadyWithCaveats,
		Issues: []research.SourceBundleIssue{research.BundleIssueVerificationCaveat},
	})
	if err != nil || completedProgress.BundleID == nil || *completedProgress.BundleID != bundleID ||
		completedProgress.BundleState != research.BundleReadyWithCaveats || len(completedProgress.Warnings) != 1 ||
		completedProgress.Warnings[0] != string(research.BundleIssueVerificationCaveat) {
		t.Fatalf("completed progress = (%+v, %v)", completedProgress, err)
	}
}

func TestServicePlansAndInspectsManualResearchTopic(t *testing.T) {
	t.Parallel()
	root := "/workspaces/research-topic"
	at := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	memoryStore := memory.New()
	repositories := memoryStore.Repositories()
	factory := &fakeSourceRegistryStoreFactory{
		research: researchapp.NewResearchService(repositories.Runs),
		bundles:  researchapp.NewSourceBundleService(repositories.Bundles, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		triggers: researchapp.NewResearchTriggerService(repositories.TriggerQueue),
	}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).
		WithConfig(&recordingConfigStore{project: config.Settings{
			config.KeyAllowNetwork: config.BoolValue(false), config.KeyResearchSearchMaxQueriesPerRun: config.NumberValue(2),
		}}).
		WithResearchStores(factory).WithResearchClock(func() time.Time { return at })
	planned, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchOperation: "topic", ResearchTopic: "Go range over func"})
	if err != nil || planned.ResearchView == nil || planned.ResearchView.Plan == nil || len(planned.ResearchView.Plan.Queries) == 0 || planned.ResearchView.QueueItem == nil || !planned.ResearchView.DiscoveryPending || planned.ResearchView.NetworkAllowed {
		t.Fatalf("research topic = (%+v, %v)", planned.ResearchView, err)
	}
	if len(planned.ResearchView.Plan.Queries) != 2 || planned.ResearchView.Run.Cost == nil || planned.ResearchView.Run.Cost.Budget.PerRun.SearchRequests != 2 {
		t.Fatalf("configured query/search budget = plan:%d cost:%+v", len(planned.ResearchView.Plan.Queries), planned.ResearchView.Run.Cost)
	}
	repeated, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchOperation: "topic", ResearchTopic: "Go range over func"})
	if err != nil || repeated.ResearchView.Request.ID != planned.ResearchView.Request.ID || repeated.ResearchView.Run.ID == planned.ResearchView.Run.ID {
		t.Fatalf("repeated research topic = (%+v, %v)", repeated.ResearchView, err)
	}
	status, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchOperation: "status", ResearchRunID: planned.ResearchView.Run.ID})
	if err != nil || status.ResearchView == nil || status.ResearchView.Request.Topic.Subject != "Go range over func" ||
		status.ResearchView.Run.Status != research.ResearchRunPlanned || status.ResearchView.Progress == nil ||
		status.ResearchView.Progress.Phase != "queued" || len(status.ResearchView.Progress.Queries) != 2 {
		t.Fatalf("research status = (%+v, %v)", status.ResearchView, err)
	}
	audit, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchOperation: "show", ResearchRunID: planned.ResearchView.Run.ID})
	if err != nil || audit.ResearchAuditView == nil || len(audit.ResearchAuditView.Records) != 1 ||
		audit.ResearchAuditView.Records[0].QueryPlannerVersion != "query-planner-v1" ||
		audit.ResearchAuditView.Records[0].Outcome != research.ResearchRunPlanned ||
		audit.ResearchAuditView.Records[0].NetworkAllowed || audit.ResearchAuditView.Progress.Phase != "queued" ||
		len(audit.ResearchAuditView.Progress.Queries) != 2 {
		t.Fatalf("research audit = (%+v, %v)", audit.ResearchAuditView, err)
	}
}

type fakeResearchCostService struct {
	stats      researchapp.ResearchCostStats
	metadata   research.ResearchCostMetadata
	statsCalls int
}

type fakeUpdateScanService struct {
	scan  research.UpdateScan
	err   error
	calls int
}

func (service *fakeUpdateScanService) Scan(context.Context, researchapp.ResearchMode, researchapp.NetworkResearchAccess, research.Timestamp) (research.UpdateScan, error) {
	service.calls++
	return service.scan, service.err
}

func (*fakeResearchCostService) Evaluate(context.Context, researchapp.CostControlRequest) (researchapp.CostControlDecision, error) {
	return researchapp.CostControlDecision{}, nil
}
func (service *fakeResearchCostService) Metadata(context.Context, research.ID) (research.ResearchCostMetadata, error) {
	return service.metadata, nil
}
func (service *fakeResearchCostService) Stats(context.Context, research.Timestamp) (researchapp.ResearchCostStats, error) {
	service.statsCalls++
	return service.stats, nil
}

type fakeResearchCacheFactory struct {
	service   researchapp.ResearchCacheService
	openRoots []string
}

func (factory *fakeResearchCacheFactory) Open(_ context.Context, root string) (researchapp.ResearchCacheService, error) {
	factory.openRoots = append(factory.openRoots, root)
	return factory.service, nil
}

type fakeResearchCacheService struct {
	status      researchapp.ResearchCacheStatus
	cleared     researchapp.ResearchCacheClearResult
	statusCalls int
	clearCalls  int
}

func (*fakeResearchCacheService) Put(context.Context, researchapp.CacheLayer, string, []byte) error {
	return nil
}
func (*fakeResearchCacheService) Get(context.Context, researchapp.CacheLayer, string, researchapp.CacheReadMode) (researchapp.CacheLookup, error) {
	return researchapp.CacheLookup{}, nil
}
func (service *fakeResearchCacheService) Status(context.Context) (researchapp.ResearchCacheStatus, error) {
	service.statusCalls++
	return service.status, nil
}
func (service *fakeResearchCacheService) Clear(context.Context) (researchapp.ResearchCacheClearResult, error) {
	service.clearCalls++
	return service.cleared, nil
}

var (
	_ researchapp.ResearchCacheServiceFactory = (*fakeResearchCacheFactory)(nil)
	_ researchapp.ResearchCacheService        = (*fakeResearchCacheService)(nil)
)
