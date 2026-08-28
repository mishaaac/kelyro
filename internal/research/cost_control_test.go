package research_test

import (
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestResearchCostBudgetV1IsBoundedAndProviderNeutral(t *testing.T) {
	t.Parallel()
	budget := research.DefaultResearchCostBudgetV1()
	if err := budget.Validate(); err != nil {
		t.Fatal(err)
	}
	if budget.PerRun.SearchRequests != 6 || budget.PerRun.FetchRequests != 12 || budget.PerRun.Bytes != 8<<20 || budget.PerRun.ModelCalls != 0 {
		t.Fatalf("default per-run budget = %+v", budget.PerRun)
	}
	invalid := budget
	invalid.PerRun.SearchRequests = invalid.PerTopic.SearchRequests + 1
	if err := invalid.Validate(); err == nil {
		t.Fatal("budget accepted a per-run limit above its per-topic limit")
	}
}

func TestResearchCostMetadataRequiresVisibleBudgetStopReason(t *testing.T) {
	t.Parallel()
	metadata := research.ResearchCostMetadata{
		Budget: research.DefaultResearchCostBudgetV1(), StoppedByBudget: true,
		StopScope: research.ResearchBudgetRun, AlgorithmVersion: research.ResearchCostControlAlgorithmV1,
	}
	if err := metadata.Validate(); err == nil {
		t.Fatal("metadata accepted a hidden budget stop")
	}
	metadata.StopReason = "Research stopped because the run budget would be exceeded."
	if err := metadata.Validate(); err != nil {
		t.Fatal(err)
	}
}
