package app

import (
	"context"
	"testing"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
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
