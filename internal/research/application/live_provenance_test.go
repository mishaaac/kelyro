package application_test

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestLiveResearchProvenancePreservesQueryToVerifiedBundleChain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fixture := liveProvenanceFixture(t)
	store := memory.New()
	provenance := application.NewProvenanceService(store.Repositories().Provenance)
	stage, err := application.NewLiveResearchProvenanceService(provenance, fixedClock{now: testTimestamp(t, 17)})
	if err != nil {
		t.Fatal(err)
	}

	artifacts, err := stage.Execute(ctx, fixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts.ProvenanceGraphs) != 1 {
		t.Fatalf("provenance graphs = %d, want 1", len(artifacts.ProvenanceGraphs))
	}
	graph := artifacts.ProvenanceGraphs[0]
	wantKinds := map[research.ProvenanceNodeKind]bool{
		research.ProvenanceRequest: true, research.ProvenanceRun: true, research.ProvenanceQuery: true,
		research.ProvenanceDiscoveredSource: true, research.ProvenanceSource: true,
		research.ProvenanceSnapshot: true, research.ProvenanceEvidence: true,
		research.ProvenanceClaim: true, research.ProvenanceSourceBundle: true,
	}
	for _, node := range graph.Nodes {
		delete(wantKinds, node.Kind)
	}
	if len(wantKinds) != 0 || graph.ClaimID != fixture.Artifacts.Claims[0].ID || graph.AlgorithmVersion != research.ProvenanceGraphAlgorithmV1 {
		t.Fatalf("provenance graph = %+v; missing kinds %v", graph, wantKinds)
	}
	stored, err := provenance.Trace(ctx, graph.ClaimID)
	if err != nil || stored.ID != graph.ID {
		t.Fatalf("stored provenance = (%+v, %v)", stored, err)
	}

	// A worker replay resolves the semantic graph instead of appending a second
	// explanation or changing the recorded identity.
	replayed, err := stage.Execute(ctx, fixture)
	if err != nil || len(replayed.ProvenanceGraphs) != 1 || replayed.ProvenanceGraphs[0].ID != graph.ID {
		t.Fatalf("replayed provenance = (%+v, %v)", replayed.ProvenanceGraphs, err)
	}
	artifacts.ProvenanceGraphs[0].Nodes[0].Label = "mutated"
	again, err := provenance.Trace(ctx, graph.ClaimID)
	if err != nil || again.Nodes[0].Label == "mutated" {
		t.Fatalf("provenance ownership leaked = (%+v, %v)", again, err)
	}
}

func TestLiveResearchProvenanceRejectsMissingDiscoveryOrVerification(t *testing.T) {
	t.Parallel()
	provenance := application.NewProvenanceService(memory.New().Repositories().Provenance)
	stage, err := application.NewLiveResearchProvenanceService(provenance, fixedClock{now: testTimestamp(t, 17)})
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*application.LiveResearchStageInput){
		"discovery":    func(input *application.LiveResearchStageInput) { input.Artifacts.Discoveries = nil },
		"verification": func(input *application.LiveResearchStageInput) { input.Artifacts.Verifications = nil },
	} {
		t.Run(name, func(t *testing.T) {
			input := liveProvenanceFixture(t)
			mutate(&input)
			if _, err := stage.Execute(context.Background(), input); err == nil {
				t.Fatalf("missing %s succeeded", name)
			}
		})
	}
}

func liveProvenanceFixture(t *testing.T) application.LiveResearchStageInput {
	t.Helper()
	request, run := testRequestRun(t)
	source := testSource(t, "live-provenance")
	discovery := research.DiscoveredSource{
		ID: testID(t, "discovery.live-provenance"), RequestID: request.ID, SourceID: source.ID,
		Locator: source.Locator, Query: "Go interfaces official documentation", Title: "Fixture result",
		Provider: "fixture-search-v1", Rank: 1, DiscoveredAt: testTimestamp(t, 11),
	}
	snapshot := testSnapshot(t, source, "live-provenance", 12)
	evidence := research.Evidence{
		ID: testID(t, "evidence.live-provenance"), SourceID: source.ID, SnapshotID: snapshot.ID,
		Location: "section 1", Excerpt: "Interfaces define behavior.",
		ExcerptHash: research.CanonicalEvidenceExcerptHashV1("Interfaces define behavior."),
		ExtractedAt: testTimestamp(t, 13), ExtractorVersion: application.EvidenceExtractorV1,
	}
	claim := research.Claim{
		ID: testClaimID(t, "live-provenance"), Topic: request.Topic,
		Statement: "Interfaces define behavior.", Type: research.ClaimDefinition, Scope: "current usage",
		StatusScope: research.ClaimStatusAll, Confidence: testConfidence(t, .8),
		SourceIDs: []research.SourceID{source.ID}, EvidenceIDs: []research.ID{evidence.ID}, CreatedAt: testTimestamp(t, 14),
	}
	verification := research.VerificationResult{
		ID: testID(t, "verification.live-provenance"), ClaimID: claim.ID,
		Status: research.VerificationVerified, Requirement: research.VerificationRequirementGeneral,
		SourceIDs: []research.SourceID{source.ID},
		Metrics: research.VerificationMetrics{SourceCount: 1, IndependentOrganizationCount: 1,
			AuthorityDistribution: research.VerificationAuthorityDistribution{TierA: 1}, ScopeConsistent: true},
		ReasonCodes: []research.ClaimVerificationReason{research.VerificationReasonSingleStrongSource},
		Confidence:  claim.Confidence, VerifiedAt: testTimestamp(t, 15), AlgorithmVersion: research.MultiSourceVerificationAlgorithmV1,
	}
	bundleSource, err := research.NewSourceBundleSource(source)
	if err != nil {
		t.Fatal(err)
	}
	lastVerified := testTimestamp(t, 15)
	bundle, err := research.SealSourceBundleV1(research.SourceBundle{
		ID: testID(t, "bundle.live-provenance"), RunID: run.ID, Topic: request.Topic, Purpose: request.Purpose,
		ClaimIDs: []research.ClaimID{claim.ID}, Sources: []research.SourceBundleSource{bundleSource},
		Freshness: research.SourceBundleFreshness{State: research.FreshnessFresh, Score: testFreshnessScore(t, .9),
			LastVerifiedAt: &lastVerified, SourceAlgorithms: []string{research.FreshnessAlgorithmV1},
			AlgorithmVersion: research.SourceBundleFreshnessV1},
		VerifiedAt: testTimestamp(t, 16),
	})
	if err != nil {
		t.Fatal(err)
	}
	return application.LiveResearchStageInput{
		Request: request, Run: run,
		Artifacts: application.LiveResearchArtifacts{
			Discoveries: []research.DiscoveredSource{discovery}, Sources: []research.Source{source},
			Snapshots: []research.SourceSnapshot{snapshot}, Evidence: []research.Evidence{evidence},
			Claims: []research.Claim{claim}, Verifications: []research.VerificationResult{verification}, Bundle: &bundle,
		},
	}
}
