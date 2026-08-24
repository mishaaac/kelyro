package app

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesWorkspaceSourceRegistryQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	root := "/workspaces/source-registry"
	registry := researchapp.NewSourceRegistryService(memory.New().Repositories().SourceRegistry)
	entry := appRegistryEntry(t)
	if err := registry.Save(ctx, entry); err != nil {
		t.Fatal(err)
	}
	factory := &fakeSourceRegistryStoreFactory{registry: registry}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithResearchStores(factory)
	listed, err := service.Execute(ctx, Command{Action: ActionSources, Workspace: root, SourceRegistryOperation: "list"})
	if err != nil || len(listed.SourceRegistryEntries) != 1 || listed.SourceRegistryEntries[0].ID != entry.ID {
		t.Fatalf("registry list = (%+v, %v)", listed, err)
	}
	shown, err := service.Execute(ctx, Command{Action: ActionSources, Workspace: root, SourceRegistryOperation: "show", SourceRegistryID: entry.ID})
	if err != nil || shown.SourceRegistryEntry == nil || shown.SourceRegistryEntry.Status != research.RegistryBlocked {
		t.Fatalf("registry show = (%+v, %v)", shown, err)
	}
	if factory.openRoot != root || factory.closed != 2 {
		t.Fatalf("registry factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}

type fakeSourceRegistryStoreFactory struct {
	registry researchapp.SourceRegistryService
	openRoot string
	closed   int
}

func (factory *fakeSourceRegistryStoreFactory) Open(_ context.Context, root string) (researchapp.SourceRegistryStore, error) {
	factory.openRoot = root
	return &fakeSourceRegistryStore{registry: factory.registry, close: func() { factory.closed++ }}, nil
}

type fakeSourceRegistryStore struct {
	registry researchapp.SourceRegistryService
	close    func()
}

func (store *fakeSourceRegistryStore) Registry() researchapp.SourceRegistryService {
	return store.registry
}
func (store *fakeSourceRegistryStore) Close() error {
	store.close()
	return nil
}

func appRegistryEntry(t *testing.T) research.SourceRegistryEntry {
	t.Helper()
	id, _ := research.NewID("registry.blocked-example")
	domain, _ := research.NewCanonicalDomain("blocked.example")
	at, _ := research.NewTimestamp(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	return research.SourceRegistryEntry{
		ID: id, Organization: "Blocked Example", CanonicalDomains: []research.CanonicalDomain{domain},
		SourceKinds:     []research.SourceKind{research.SourceOther},
		AuthorityHints:  []research.RegistryAuthorityHint{{SourceKind: research.SourceOther, Tier: research.AuthorityTierE, Reason: "Explicitly blocked fixture."}},
		ResearchDomains: []string{"*"}, TopicPatterns: []string{"*"}, Notes: "Do not use.",
		Status: research.RegistryBlocked, AddedAt: at, LastReviewedAt: at,
	}
}
