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
	provenanceStore := memory.New()
	provenance := researchapp.NewProvenanceService(provenanceStore.Repositories().Provenance)
	entry := appRegistryEntry(t)
	if err := registry.Save(ctx, entry); err != nil {
		t.Fatal(err)
	}
	graph := appProvenanceGraph(t)
	if err := provenance.Record(ctx, graph); err != nil {
		t.Fatal(err)
	}
	factory := &fakeSourceRegistryStoreFactory{registry: registry, provenance: provenance}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithResearchStores(factory)
	listed, err := service.Execute(ctx, Command{Action: ActionSources, Workspace: root, SourceRegistryOperation: "list"})
	if err != nil || len(listed.SourceRegistryEntries) != 1 || listed.SourceRegistryEntries[0].ID != entry.ID {
		t.Fatalf("registry list = (%+v, %v)", listed, err)
	}
	shown, err := service.Execute(ctx, Command{Action: ActionSources, Workspace: root, SourceRegistryOperation: "show", SourceRegistryID: entry.ID})
	if err != nil || shown.SourceRegistryEntry == nil || shown.SourceRegistryEntry.Status != research.RegistryBlocked {
		t.Fatalf("registry show = (%+v, %v)", shown, err)
	}
	traced, err := service.Execute(ctx, Command{Action: ActionSources, Workspace: root, SourceRegistryOperation: "trace", ProvenanceClaimID: graph.ClaimID})
	if err != nil || traced.ProvenanceGraph == nil || traced.ProvenanceGraph.ID != graph.ID {
		t.Fatalf("provenance trace = (%+v, %v)", traced, err)
	}
	if factory.openRoot != root || factory.closed != 3 {
		t.Fatalf("registry factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}

type fakeSourceRegistryStoreFactory struct {
	registry   researchapp.SourceRegistryService
	provenance researchapp.ProvenanceService
	openRoot   string
	closed     int
}

func (factory *fakeSourceRegistryStoreFactory) Open(_ context.Context, root string) (researchapp.SourceRegistryStore, error) {
	factory.openRoot = root
	return &fakeSourceRegistryStore{registry: factory.registry, provenance: factory.provenance, close: func() { factory.closed++ }}, nil
}

type fakeSourceRegistryStore struct {
	registry   researchapp.SourceRegistryService
	provenance researchapp.ProvenanceService
	close      func()
}

func (store *fakeSourceRegistryStore) Registry() researchapp.SourceRegistryService {
	return store.registry
}
func (store *fakeSourceRegistryStore) Provenance() researchapp.ProvenanceService {
	return store.provenance
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

func appProvenanceGraph(t *testing.T) research.ProvenanceGraph {
	t.Helper()
	id := func(value string) research.ID {
		result, err := research.NewID(value)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	claimID, _ := research.NewClaimID("claim.traceable")
	claimNode := id(claimID.String())
	at, _ := research.NewTimestamp(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	recorded, _ := research.NewTimestamp(at.Time().Add(time.Hour))
	request, run, source := id("request.trace"), id("run.trace"), id("source.trace")
	snapshot, evidence := id("snapshot.trace"), id("evidence.trace")
	return research.ProvenanceGraph{
		ID: id("graph.trace"), ClaimID: claimID, RecordedAt: recorded, AlgorithmVersion: research.ProvenanceGraphAlgorithmV1,
		Nodes: []research.ProvenanceNode{
			{ID: request, Kind: research.ProvenanceRequest, Label: "trace request", OccurredAt: at},
			{ID: run, Kind: research.ProvenanceRun, Label: "trace run", OccurredAt: at},
			{ID: source, Kind: research.ProvenanceSource, Label: "manual source", OccurredAt: at},
			{ID: snapshot, Kind: research.ProvenanceSnapshot, Label: "historical snapshot", OccurredAt: at, ToolVersion: "fetch/v1"},
			{ID: evidence, Kind: research.ProvenanceEvidence, Label: "section 1", OccurredAt: at, ToolVersion: "extract/v1"},
			{ID: claimNode, Kind: research.ProvenanceClaim, Label: "traceable claim", OccurredAt: at},
		},
		Edges: []research.ProvenanceEdge{
			{From: request, To: run}, {From: run, To: source}, {From: source, To: snapshot},
			{From: snapshot, To: evidence}, {From: evidence, To: claimNode},
		},
	}
}
