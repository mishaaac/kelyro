package research

import "fmt"

const ResearchCostControlAlgorithmV1 = "research-cost-control-v1"

// ResearchCostUsage uses provider-neutral units. It deliberately does not
// attach money or vendor pricing to Research domain state.
type ResearchCostUsage struct {
	SearchRequests   int64
	FetchRequests    int64
	Bytes            int64
	ProviderAPICalls int64
	ModelCalls       int64
}

func (usage ResearchCostUsage) Validate() error {
	for name, value := range map[string]int64{
		"search requests":    usage.SearchRequests,
		"fetch requests":     usage.FetchRequests,
		"bytes":              usage.Bytes,
		"provider API calls": usage.ProviderAPICalls,
		"model calls":        usage.ModelCalls,
	} {
		if value < 0 {
			return fmt.Errorf("research cost %s is negative", name)
		}
	}
	return nil
}

func (usage ResearchCostUsage) IsZero() bool {
	return usage == (ResearchCostUsage{})
}

func (usage ResearchCostUsage) Add(other ResearchCostUsage) ResearchCostUsage {
	return ResearchCostUsage{
		SearchRequests:   usage.SearchRequests + other.SearchRequests,
		FetchRequests:    usage.FetchRequests + other.FetchRequests,
		Bytes:            usage.Bytes + other.Bytes,
		ProviderAPICalls: usage.ProviderAPICalls + other.ProviderAPICalls,
		ModelCalls:       usage.ModelCalls + other.ModelCalls,
	}
}

type ResearchCostBudget struct {
	PerRun           ResearchCostUsage
	PerTopic         ResearchCostUsage
	Daily            *ResearchCostUsage
	AlgorithmVersion string
}

func (budget ResearchCostBudget) Validate() error {
	if budget.AlgorithmVersion != ResearchCostControlAlgorithmV1 {
		return fmt.Errorf("research cost budget algorithm must be %q", ResearchCostControlAlgorithmV1)
	}
	if err := budget.PerRun.Validate(); err != nil {
		return fmt.Errorf("research per-run budget: %w", err)
	}
	if err := budget.PerTopic.Validate(); err != nil {
		return fmt.Errorf("research per-topic budget: %w", err)
	}
	if budget.PerRun.IsZero() || budget.PerTopic.IsZero() {
		return fmt.Errorf("research run and topic budgets must not be empty")
	}
	if !usageWithin(budget.PerRun, budget.PerTopic) {
		return fmt.Errorf("research per-run budget exceeds per-topic budget")
	}
	if budget.Daily != nil {
		if err := budget.Daily.Validate(); err != nil {
			return fmt.Errorf("research daily budget: %w", err)
		}
		if budget.Daily.IsZero() {
			return fmt.Errorf("research daily budget must not be empty")
		}
	}
	return nil
}

func usageWithin(usage, limit ResearchCostUsage) bool {
	return usage.SearchRequests <= limit.SearchRequests &&
		usage.FetchRequests <= limit.FetchRequests &&
		usage.Bytes <= limit.Bytes &&
		usage.ProviderAPICalls <= limit.ProviderAPICalls &&
		usage.ModelCalls <= limit.ModelCalls
}

func DefaultResearchCostBudgetV1() ResearchCostBudget {
	return ResearchCostBudget{
		PerRun: ResearchCostUsage{
			SearchRequests: 6, FetchRequests: 12, Bytes: 8 << 20,
			ProviderAPICalls: 18, ModelCalls: 0,
		},
		PerTopic: ResearchCostUsage{
			SearchRequests: 12, FetchRequests: 24, Bytes: 16 << 20,
			ProviderAPICalls: 36, ModelCalls: 0,
		},
		AlgorithmVersion: ResearchCostControlAlgorithmV1,
	}
}

type ResearchBudgetScope string

const (
	ResearchBudgetRun   ResearchBudgetScope = "run"
	ResearchBudgetTopic ResearchBudgetScope = "topic"
	ResearchBudgetDaily ResearchBudgetScope = "daily"
)

func (scope ResearchBudgetScope) Validate() error {
	switch scope {
	case ResearchBudgetRun, ResearchBudgetTopic, ResearchBudgetDaily:
		return nil
	default:
		return fmt.Errorf("invalid research budget scope %q", scope)
	}
}

type ResearchCostMetadata struct {
	Budget           ResearchCostBudget
	Used             ResearchCostUsage
	CacheSavings     ResearchCostUsage
	StoppedByBudget  bool
	StopScope        ResearchBudgetScope
	StopReason       string
	AlgorithmVersion string
}

func (metadata ResearchCostMetadata) Validate() error {
	if metadata.AlgorithmVersion != ResearchCostControlAlgorithmV1 {
		return fmt.Errorf("research cost metadata algorithm must be %q", ResearchCostControlAlgorithmV1)
	}
	if err := metadata.Budget.Validate(); err != nil {
		return err
	}
	if err := metadata.Used.Validate(); err != nil {
		return err
	}
	if err := metadata.CacheSavings.Validate(); err != nil {
		return err
	}
	if metadata.StoppedByBudget {
		if err := metadata.StopScope.Validate(); err != nil {
			return err
		}
		if err := requireText("research budget stop reason", metadata.StopReason); err != nil {
			return err
		}
	} else if metadata.StopScope != "" || metadata.StopReason != "" {
		return fmt.Errorf("research cost metadata has stop details without a budget stop")
	}
	return nil
}
