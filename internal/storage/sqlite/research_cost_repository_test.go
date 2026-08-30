package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestSQLiteResearchCostReservationIsAtomicAcrossRunAndTopicBudgets(t *testing.T) {
	database, _ := openTestDatabase(t)
	ctx := context.Background()
	repositories := database.Repositories().Research
	at, _ := research.NewTimestamp(time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	topic, _ := research.NewResearchTopic("range-over-func", "software", "go")
	budget := research.DefaultResearchCostBudgetV1()
	budget.PerRun.SearchRequests = 2
	budget.PerTopic.SearchRequests = 3

	createCostRun := func(requestValue, runValue string) research.ID {
		requestID, _ := research.NewID(requestValue)
		runID, _ := research.NewID(runValue)
		metadata := research.ResearchCostMetadata{Budget: budget, AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
		request := research.ResearchRequest{ID: requestID, Topic: topic, Purpose: research.PurposeCurrentUsage, RequestedAt: at}
		run := research.ResearchRun{ID: runID, RequestID: requestID, Status: research.ResearchRunRunning, StartedAt: at, Cost: &metadata}
		if err := repositories.Runs.Create(ctx, request, run); err != nil {
			t.Fatal(err)
		}
		return runID
	}
	firstID := createCostRun("request.cost-sqlite-1", "run.cost-sqlite-1")
	secondID := createCostRun("request.cost-sqlite-2", "run.cost-sqlite-2")
	usage := research.ResearchCostUsage{SearchRequests: 2, ProviderAPICalls: 1}
	first, err := repositories.Costs.Reserve(ctx, application.CostReservation{RunID: firstID, Usage: usage, At: at})
	if err != nil || !first.Allowed {
		t.Fatalf("first reservation = (%+v,%v)", first, err)
	}
	second, err := repositories.Costs.Reserve(ctx, application.CostReservation{RunID: secondID, Usage: usage, At: at})
	if err != nil || second.Allowed || second.Scope != research.ResearchBudgetTopic || second.Metadata.Used.SearchRequests != 0 {
		t.Fatalf("topic-stopped reservation = (%+v,%v)", second, err)
	}
	loaded, err := repositories.Runs.GetRun(ctx, secondID)
	if err != nil || loaded.Cost == nil || !loaded.Cost.StoppedByBudget || loaded.Cost.StopReason == "" {
		t.Fatalf("loaded cost metadata = (%+v,%v)", loaded.Cost, err)
	}
	if err := repositories.Costs.RecordCacheSavings(ctx, application.CostReservation{RunID: secondID, Usage: research.ResearchCostUsage{FetchRequests: 1, Bytes: 1024}, At: at}); err != nil {
		t.Fatal(err)
	}
	stats, err := repositories.Costs.Stats(ctx, at)
	if err != nil || stats.Runs != 2 || stats.BudgetStoppedRuns != 1 || stats.Used.SearchRequests != 2 || stats.CacheSavings.Bytes != 1024 {
		t.Fatalf("SQLite research cost stats = (%+v,%v)", stats, err)
	}
}
