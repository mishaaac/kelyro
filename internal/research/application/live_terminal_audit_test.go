package application_test

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestLiveResearchTerminalAuditRecordsAdapterCountsBundlePoliciesAndFinalCost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	researchService := application.NewResearchService(repositories.Runs)
	costs := application.NewResearchCostService(repositories.Costs)
	input := liveProvenanceFixture(t)
	request := input.Request
	budget := research.DefaultResearchCostBudgetV1()
	cost := research.ResearchCostMetadata{Budget: budget, AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	run := input.Run
	run.Status = research.ResearchRunPlanned
	run.Cost = &cost
	if err := researchService.Start(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	plannedID := testID(t, "audit.live-terminal.planned")
	planned, err := research.SealResearchRunAuditV1(research.ResearchRunAudit{
		ID: plannedID, RunID: run.ID, RecordedAt: run.StartedAt, StartedAt: run.StartedAt, Outcome: research.ResearchRunPlanned,
		QueryPlannerVersion: "query-planner-v1", TrustPolicyVersion: "trust-policy-v1",
		FreshnessVersion: research.FreshnessAlgorithmV1, ConflictResolverVersion: research.ConflictResolverAlgorithmV1,
		NetworkMode: research.ResearchAuditNetworkAuto, NetworkAllowed: true,
		Queries: []string{"Go interfaces official documentation"}, TargetTechnology: request.Topic.Technology,
		AdditionalAlgorithms: []research.ResearchAuditAlgorithm{{Stage: "cost_control", Version: research.ResearchCostControlAlgorithmV1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := researchService.RecordAudit(ctx, planned); err != nil {
		t.Fatal(err)
	}
	if _, err := researchService.TransitionRun(ctx, run.ID, research.ResearchRunRunning, run.StartedAt); err != nil {
		t.Fatal(err)
	}
	for _, usage := range []research.ResearchCostUsage{{SearchRequests: 1}, {ProviderAPICalls: 1}, {FetchRequests: 1, Bytes: 1024}} {
		decision, reserveErr := costs.Evaluate(ctx, application.CostControlRequest{RunID: run.ID, ProposedUsage: usage, At: testTimestamp(t, 11)})
		if reserveErr != nil || !decision.NetworkAllowed {
			t.Fatalf("cost reservation = (%+v, %v)", decision, reserveErr)
		}
	}

	input.Run.Status = research.ResearchRunRunning
	input.Artifacts.Snapshots[0].Fetch.ContentHash = research.CanonicalContentHashV1([]byte("snapshot body"))
	input.Artifacts.SearchResults = []application.SearchResult{{
		Title: "Fixture result", Locator: input.Artifacts.Sources[0].Locator, Provider: "fixture-search", Rank: 1,
	}}
	input.Artifacts.SearchExecution = &application.LiveSearchExecutionMetadata{
		ProviderID: "fixture-search", AdapterVersion: "fixture-search-adapter-v1", QueryCount: 1, ResultCount: 1,
		ProviderAPICalls: 1, AlgorithmVersion: application.LiveSearchExecutionMetadataV1,
	}
	body := []byte("snapshot body")
	input.Artifacts.FetchedSources = []application.FetchedSource{{
		SourceID: input.Artifacts.Sources[0].ID, Locator: input.Artifacts.Sources[0].Locator,
		FetchedAt: testTimestamp(t, 12), Body: body, Origin: application.FetchOriginLive,
		Metadata: research.FetchMetadata{StatusCode: 200, ContentType: "text/html", ContentHash: research.CanonicalContentHashV1(body), ContentLength: int64(len(body)), FetchVersion: "fixture-fetch-v1"},
	}}
	service, err := application.NewLiveResearchTerminalAuditService(researchService, costs)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Prepare(ctx, application.LiveResearchTerminalAuditRequest{
		Run: input.Run, Outcome: research.ResearchRunCompleted, FinalizedAt: testTimestamp(t, 18),
		Mode: application.ResearchModeAuto, Artifacts: input.Artifacts,
	})
	if err != nil {
		t.Fatal(err)
	}
	audit := result.Audit
	if audit.Execution == nil || audit.Execution.BundleID == nil || *audit.Execution.BundleID != input.Artifacts.Bundle.ID ||
		audit.Execution.QueryCount != 1 || audit.Execution.ResultCount != 1 || audit.Execution.FetchCount != 1 ||
		audit.BytesFetched != int64(len(body)) || len(audit.ProvidersUsed) != 1 || audit.ProvidersUsed[0] != "fixture-search" ||
		len(audit.Execution.Providers) != 1 || audit.Execution.Providers[0].AdapterVersion != "fixture-search-adapter-v1" ||
		audit.Execution.CostUsed.SearchRequests != 1 || audit.Execution.CostUsed.ProviderAPICalls != 1 ||
		audit.Execution.CostUsed.FetchRequests != 1 || audit.Execution.CostUsed.Bytes != 1024 ||
		audit.QueryPlannerVersion != planned.QueryPlannerVersion || audit.TrustPolicyVersion != planned.TrustPolicyVersion ||
		result.AlgorithmVersion != application.LiveResearchTerminalAuditV1 {
		t.Fatalf("terminal audit = %+v; result=%+v", audit, result)
	}
	encoded, err := audit.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := research.ParseResearchRunAuditJSON(encoded)
	if err != nil || parsed.Execution == nil || parsed.Execution.Providers[0].AdapterVersion != "fixture-search-adapter-v1" {
		t.Fatalf("terminal audit roundtrip = (%+v, %v)", parsed, err)
	}
}

func TestLiveResearchTerminalAuditRejectsUnattributedProviderCost(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	researchService := application.NewResearchService(repositories.Runs)
	costs := application.NewResearchCostService(repositories.Costs)
	request, run := testRequestRun(t)
	run.Status = research.ResearchRunPlanned
	metadata := research.ResearchCostMetadata{Budget: research.DefaultResearchCostBudgetV1(), AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	run.Cost = &metadata
	if err := researchService.Start(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	planned, _ := research.SealResearchRunAuditV1(research.ResearchRunAudit{
		ID: testID(t, "audit.live-terminal-missing.planned"), RunID: run.ID, RecordedAt: run.StartedAt, StartedAt: run.StartedAt,
		Outcome: research.ResearchRunPlanned, QueryPlannerVersion: "query-planner-v1", TrustPolicyVersion: "trust-policy-v1",
		FreshnessVersion: research.FreshnessAlgorithmV1, ConflictResolverVersion: research.ConflictResolverAlgorithmV1,
		NetworkMode: research.ResearchAuditNetworkAuto, Queries: []string{"Go interfaces official documentation"}, TargetTechnology: request.Topic.Technology,
	})
	if err := researchService.RecordAudit(ctx, planned); err != nil {
		t.Fatal(err)
	}
	running, _ := researchService.TransitionRun(ctx, run.ID, research.ResearchRunRunning, run.StartedAt)
	if _, err := costs.Evaluate(ctx, application.CostControlRequest{RunID: run.ID, ProposedUsage: research.ResearchCostUsage{ProviderAPICalls: 1}, At: testTimestamp(t, 11)}); err != nil {
		t.Fatal(err)
	}
	service, _ := application.NewLiveResearchTerminalAuditService(researchService, costs)
	_, err := service.Prepare(ctx, application.LiveResearchTerminalAuditRequest{
		Run: running, Outcome: research.ResearchRunFailed, FinalizedAt: testTimestamp(t, 12), Mode: application.ResearchModeOnline,
		FailureKind: application.ErrorExternalFailure,
	})
	if err == nil {
		t.Fatal("unattributed provider cost succeeded")
	}
}
