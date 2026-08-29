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
	freshness := researchapp.NewFreshnessService(provenanceStore.Repositories().Freshness)
	trust := researchapp.NewTrustDecisionService(provenanceStore.Repositories().TrustRegistry)
	sources := researchapp.NewSourceService(provenanceStore.Repositories().Sources, provenanceStore.Repositories().Snapshots)
	source := appSource(t)
	if err := sources.Register(ctx, source); err != nil {
		t.Fatal(err)
	}
	entry := appRegistryEntry(t)
	if err := registry.Save(ctx, entry); err != nil {
		t.Fatal(err)
	}
	graph := appProvenanceGraph(t)
	if err := provenance.Record(ctx, graph); err != nil {
		t.Fatal(err)
	}
	lastVerified, _ := research.NewTimestamp(time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
	dueAt, _ := research.NewTimestamp(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	score, _ := research.NewFreshnessScore(.4)
	staleRecord := researchapp.FreshnessRecord{
		SubjectID: graph.Nodes[len(graph.Nodes)-1].ID, State: research.FreshnessStale, Score: score,
		LastVerifiedAt: lastVerified, NextVerifyAt: &dueAt, VerificationReason: research.VerificationManualRequest,
		Priority: research.VerificationPriorityCritical, AlgorithmVersion: research.FreshnessAlgorithmV1,
		SchedulingAlgorithmVersion: research.RefreshSchedulingAlgorithmV1,
	}
	if err := freshness.Save(ctx, staleRecord); err != nil {
		t.Fatal(err)
	}
	sourceSubjectID, _ := research.NewID(source.ID.String())
	sourceFreshness := researchapp.FreshnessRecord{SubjectID: sourceSubjectID, State: research.FreshnessAging, Score: score,
		LastVerifiedAt: lastVerified, AlgorithmVersion: research.FreshnessAlgorithmV1}
	if err := freshness.Save(ctx, sourceFreshness); err != nil {
		t.Fatal(err)
	}
	decision := research.TrustDecision{SourceID: source.ID, State: research.TrustAccepted, Tier: research.AuthorityTierA,
		Reasons: []research.TrustReason{{Code: "primary", Detail: "Normative fixture."}}, Policy: "trust-policy-v1", EvaluatedAt: lastVerified}
	if err := provenanceStore.Repositories().TrustRegistry.SaveDecision(ctx, decision); err != nil {
		t.Fatal(err)
	}
	factory := &fakeSourceRegistryStoreFactory{sources: sources, registry: registry, trust: trust, provenance: provenance, freshness: freshness}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).
		WithResearchStores(factory).
		WithResearchClock(func() time.Time { return time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC) })
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
	due, err := service.Execute(ctx, Command{Action: ActionSources, Workspace: root, SourceRegistryOperation: "stale"})
	if err != nil || len(due.StaleSources) != 1 || due.StaleSources[0].SubjectID != staleRecord.SubjectID {
		t.Fatalf("stale sources = (%+v, %v)", due.StaleSources, err)
	}
	listedSources, err := service.Execute(ctx, Command{Action: ActionSources, Workspace: root, SourceRegistryOperation: "sources-list"})
	if err != nil || len(listedSources.Sources) != 1 || listedSources.Sources[0].ID != source.ID {
		t.Fatalf("source list = (%+v, %v)", listedSources.Sources, err)
	}
	shownSource, err := service.Execute(ctx, Command{Action: ActionSources, Workspace: root, SourceRegistryOperation: "source-show", SourceID: source.ID})
	if err != nil || shownSource.Source == nil || shownSource.Source.Source.ID != source.ID || shownSource.Source.TrustDecision == nil || shownSource.Source.Freshness == nil {
		t.Fatalf("source show = (%+v, %v)", shownSource.Source, err)
	}
	transparent, err := service.Execute(ctx, Command{Action: ActionSources, Workspace: root, SourceRegistryOperation: "transparency"})
	if err != nil || len(transparent.SourceTransparency) != 1 || transparent.SourceTransparency[0].TrustDecision == nil || transparent.SourceTransparency[0].Freshness == nil {
		t.Fatalf("source transparency = (%+v, %v)", transparent.SourceTransparency, err)
	}
	if factory.openRoot != root || factory.closed != 7 {
		t.Fatalf("registry factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}

type fakeSourceRegistryStoreFactory struct {
	sources    researchapp.SourceService
	registry   researchapp.SourceRegistryService
	trust      researchapp.TrustDecisionService
	provenance researchapp.ProvenanceService
	freshness  researchapp.FreshnessService
	costs      researchapp.ResearchCostService
	research   researchapp.ResearchService
	bundles    researchapp.SourceBundleService
	conflicts  researchapp.ConflictResolutionService
	triggers   researchapp.ResearchTriggerService
	updateScan researchapp.UpdateScanService
	openRoot   string
	closed     int
}

func (factory *fakeSourceRegistryStoreFactory) Open(_ context.Context, root string) (researchapp.SourceRegistryStore, error) {
	factory.openRoot = root
	return &fakeSourceRegistryStore{sources: factory.sources, registry: factory.registry, trust: factory.trust, provenance: factory.provenance, freshness: factory.freshness, costs: factory.costs, research: factory.research, bundles: factory.bundles, conflicts: factory.conflicts, triggers: factory.triggers, updateScan: factory.updateScan, close: func() { factory.closed++ }}, nil
}

type fakeSourceRegistryStore struct {
	sources    researchapp.SourceService
	registry   researchapp.SourceRegistryService
	trust      researchapp.TrustDecisionService
	provenance researchapp.ProvenanceService
	freshness  researchapp.FreshnessService
	costs      researchapp.ResearchCostService
	research   researchapp.ResearchService
	bundles    researchapp.SourceBundleService
	conflicts  researchapp.ConflictResolutionService
	triggers   researchapp.ResearchTriggerService
	updateScan researchapp.UpdateScanService
	close      func()
}

func (store *fakeSourceRegistryStore) Registry() researchapp.SourceRegistryService {
	return store.registry
}
func (store *fakeSourceRegistryStore) Sources() researchapp.SourceService { return store.sources }
func (store *fakeSourceRegistryStore) TrustDecisions() researchapp.TrustDecisionService {
	return store.trust
}
func (store *fakeSourceRegistryStore) Provenance() researchapp.ProvenanceService {
	return store.provenance
}
func (store *fakeSourceRegistryStore) Freshness() researchapp.FreshnessService {
	return store.freshness
}
func (store *fakeSourceRegistryStore) Research() researchapp.ResearchService    { return store.research }
func (store *fakeSourceRegistryStore) Bundles() researchapp.SourceBundleService { return store.bundles }
func (store *fakeSourceRegistryStore) Conflicts() researchapp.ConflictResolutionService {
	return store.conflicts
}
func (store *fakeSourceRegistryStore) Costs() researchapp.ResearchCostService { return store.costs }
func (store *fakeSourceRegistryStore) Triggers() researchapp.ResearchTriggerService {
	return store.triggers
}
func (store *fakeSourceRegistryStore) UpdateScan() researchapp.UpdateScanService {
	return store.updateScan
}
func (store *fakeSourceRegistryStore) Close() error {
	store.close()
	return nil
}

func appSource(t *testing.T) research.Source {
	t.Helper()
	id, _ := research.NewSourceID("source.app-doc")
	locator, _ := research.NewSourceLocator("https://example.test/docs")
	at, _ := research.NewTimestamp(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	return research.Source{ID: id, Kind: research.SourceOfficialDocumentation, Locator: locator,
		TemporalScope: research.SourceTemporalCurrent, Metadata: research.SourceMetadata{Title: "Example docs", Publisher: "Example"}, CreatedAt: at}
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
