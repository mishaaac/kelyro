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
		WithConfig(&recordingConfigStore{project: config.Settings{config.KeyAllowNetwork: config.BoolValue(false)}}).
		WithResearchStores(factory).WithResearchClock(func() time.Time { return at })
	planned, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchOperation: "topic", ResearchTopic: "Go range over func"})
	if err != nil || planned.ResearchView == nil || planned.ResearchView.Plan == nil || len(planned.ResearchView.Plan.Queries) == 0 || planned.ResearchView.QueueItem == nil || !planned.ResearchView.DiscoveryPending || planned.ResearchView.NetworkAllowed {
		t.Fatalf("research topic = (%+v, %v)", planned.ResearchView, err)
	}
	repeated, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchOperation: "topic", ResearchTopic: "Go range over func"})
	if err != nil || repeated.ResearchView.Request.ID != planned.ResearchView.Request.ID || repeated.ResearchView.Run.ID == planned.ResearchView.Run.ID {
		t.Fatalf("repeated research topic = (%+v, %v)", repeated.ResearchView, err)
	}
	status, err := service.Execute(context.Background(), Command{Action: ActionResearch, Workspace: root, ResearchOperation: "status", ResearchRunID: planned.ResearchView.Run.ID})
	if err != nil || status.ResearchView == nil || status.ResearchView.Request.Topic.Subject != "Go range over func" || status.ResearchView.Run.Status != research.ResearchRunPlanned {
		t.Fatalf("research status = (%+v, %v)", status.ResearchView, err)
	}
}

type fakeResearchCostService struct {
	stats      researchapp.ResearchCostStats
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
func (*fakeResearchCostService) Metadata(context.Context, research.ID) (research.ResearchCostMetadata, error) {
	return research.ResearchCostMetadata{}, nil
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
