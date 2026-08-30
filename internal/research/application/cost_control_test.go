package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestResearchCostControlUsesCacheStopsSatisfiedWorkAndEnforcesRunBudget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	runID, _ := research.NewID("run.cost-control")
	requestID, _ := research.NewID("request.cost-control")
	topic, _ := research.NewResearchTopic("range-over-func", "software", "go")
	at, _ := research.NewTimestamp(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	budget := research.DefaultResearchCostBudgetV1()
	budget.PerRun.SearchRequests = 1
	metadata := research.ResearchCostMetadata{Budget: budget, AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	run := research.ResearchRun{ID: runID, RequestID: requestID, Status: research.ResearchRunRunning, StartedAt: at, Cost: &metadata}
	request := research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: at}
	if err := application.NewResearchService(store.Repositories().Runs).Start(ctx, request, run); err != nil {
		t.Fatal(err)
	}
	service := application.NewResearchCostService(store.Repositories().Costs)
	usage := research.ResearchCostUsage{SearchRequests: 1, ProviderAPICalls: 1}
	cached, err := service.Evaluate(ctx, application.CostControlRequest{RunID: runID, ProposedUsage: usage, At: at, ValidCacheAvailable: true})
	if err != nil || cached.NetworkAllowed || cached.Reason != application.CostControlUseValidCache || cached.Metadata.CacheSavings.SearchRequests != 1 {
		t.Fatalf("cached decision = (%+v,%v)", cached, err)
	}
	satisfied, err := service.Evaluate(ctx, application.CostControlRequest{RunID: runID, ProposedUsage: usage, At: at, VerificationSatisfied: true})
	if err != nil || satisfied.NetworkAllowed || satisfied.Reason != application.CostControlVerificationSatisfied || satisfied.Metadata.CacheSavings.SearchRequests != 1 {
		t.Fatalf("satisfied decision = (%+v,%v)", satisfied, err)
	}
	allowed, err := service.Evaluate(ctx, application.CostControlRequest{RunID: runID, ProposedUsage: usage, At: at})
	if err != nil || !allowed.NetworkAllowed || allowed.Metadata.Used.SearchRequests != 1 {
		t.Fatalf("allowed decision = (%+v,%v)", allowed, err)
	}
	stopped, err := service.Evaluate(ctx, application.CostControlRequest{RunID: runID, ProposedUsage: usage, At: at})
	if err != nil || !stopped.BudgetStopped || stopped.Scope != research.ResearchBudgetRun || stopped.UserExplanation == "" {
		t.Fatalf("stopped decision = (%+v,%v)", stopped, err)
	}
	stats, err := service.Stats(ctx, at)
	if err != nil || stats.Runs != 1 || stats.BudgetStoppedRuns != 1 || stats.TodayUsed.SearchRequests != 1 || stats.CacheSavings.SearchRequests != 1 {
		t.Fatalf("stats = (%+v,%v)", stats, err)
	}
}
