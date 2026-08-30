package memory

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type costEvent struct {
	usage research.ResearchCostUsage
	at    research.Timestamp
}

type researchCostRepository struct{ store *Store }

func (repository researchCostRepository) Reserve(ctx context.Context, reservation application.CostReservation) (application.CostReservationResult, error) {
	const operation = "reserve memory research cost"
	if err := contextError(operation, ctx); err != nil {
		return application.CostReservationResult{}, err
	}
	if err := reservation.Validate(); err != nil {
		return application.CostReservationResult{}, invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	run, exists := repository.store.runs[reservation.RunID]
	if !exists || run.Cost == nil {
		return application.CostReservationResult{}, notFound(operation)
	}
	metadata := *run.Cost
	if !costWithin(metadata.Used.Add(reservation.Usage), metadata.Budget.PerRun) {
		return repository.stop(run, metadata, research.ResearchBudgetRun)
	}
	request := repository.store.requests[run.RequestID]
	topicUsed := research.ResearchCostUsage{}
	for _, other := range repository.store.runs {
		if other.Cost != nil && repository.store.requests[other.RequestID].Topic == request.Topic {
			topicUsed = topicUsed.Add(other.Cost.Used)
		}
	}
	if !costWithin(topicUsed.Add(reservation.Usage), metadata.Budget.PerTopic) {
		return repository.stop(run, metadata, research.ResearchBudgetTopic)
	}
	if metadata.Budget.Daily != nil {
		dailyUsed := research.ResearchCostUsage{}
		day := reservation.At.Time().UTC().Format("2006-01-02")
		for _, events := range repository.store.costEvents {
			for _, event := range events {
				if event.at.Time().UTC().Format("2006-01-02") == day {
					dailyUsed = dailyUsed.Add(event.usage)
				}
			}
		}
		if !costWithin(dailyUsed.Add(reservation.Usage), *metadata.Budget.Daily) {
			return repository.stop(run, metadata, research.ResearchBudgetDaily)
		}
	}
	metadata.Used = metadata.Used.Add(reservation.Usage)
	metadata.StoppedByBudget, metadata.StopScope, metadata.StopReason = false, "", ""
	run.Cost = &metadata
	repository.store.runs[run.ID] = cloneRun(run)
	repository.store.costEvents[run.ID] = append(repository.store.costEvents[run.ID], costEvent{usage: reservation.Usage, at: reservation.At})
	return application.CostReservationResult{Allowed: true, Metadata: metadata}, nil
}

func (repository researchCostRepository) stop(run research.ResearchRun, metadata research.ResearchCostMetadata, scope research.ResearchBudgetScope) (application.CostReservationResult, error) {
	metadata.StoppedByBudget, metadata.StopScope = true, scope
	metadata.StopReason = fmt.Sprintf("Research stopped because the %s budget would be exceeded.", scope)
	run.Cost = &metadata
	repository.store.runs[run.ID] = cloneRun(run)
	return application.CostReservationResult{Scope: scope, Reason: metadata.StopReason, Metadata: metadata}, nil
}

func (repository researchCostRepository) RecordCacheSavings(ctx context.Context, reservation application.CostReservation) error {
	const operation = "record memory research cache savings"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := reservation.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	run, exists := repository.store.runs[reservation.RunID]
	if !exists || run.Cost == nil {
		return notFound(operation)
	}
	metadata := *run.Cost
	metadata.CacheSavings = metadata.CacheSavings.Add(reservation.Usage)
	run.Cost = &metadata
	repository.store.runs[run.ID] = cloneRun(run)
	return nil
}

func (repository researchCostRepository) Metadata(ctx context.Context, runID research.ID) (research.ResearchCostMetadata, error) {
	const operation = "get memory research cost metadata"
	if err := contextError(operation, ctx); err != nil {
		return research.ResearchCostMetadata{}, err
	}
	if err := runID.Validate(); err != nil {
		return research.ResearchCostMetadata{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	run, exists := repository.store.runs[runID]
	if !exists || run.Cost == nil {
		return research.ResearchCostMetadata{}, notFound(operation)
	}
	return *cloneRun(run).Cost, nil
}

func (repository researchCostRepository) Stats(ctx context.Context, asOf research.Timestamp) (application.ResearchCostStats, error) {
	const operation = "get memory research cost stats"
	if err := contextError(operation, ctx); err != nil {
		return application.ResearchCostStats{}, err
	}
	if err := asOf.Validate(); err != nil {
		return application.ResearchCostStats{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	stats := application.ResearchCostStats{AsOf: asOf, AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	day := asOf.Time().UTC().Format("2006-01-02")
	for runID, run := range repository.store.runs {
		if run.Cost == nil {
			continue
		}
		stats.Runs++
		stats.Used = stats.Used.Add(run.Cost.Used)
		stats.CacheSavings = stats.CacheSavings.Add(run.Cost.CacheSavings)
		if run.Cost.StoppedByBudget {
			stats.BudgetStoppedRuns++
		}
		for _, event := range repository.store.costEvents[runID] {
			if event.at.Time().UTC().Format("2006-01-02") == day {
				stats.TodayUsed = stats.TodayUsed.Add(event.usage)
			}
		}
	}
	return stats, nil
}

func costWithin(usage, limit research.ResearchCostUsage) bool {
	return usage.SearchRequests <= limit.SearchRequests && usage.FetchRequests <= limit.FetchRequests &&
		usage.Bytes <= limit.Bytes && usage.ProviderAPICalls <= limit.ProviderAPICalls && usage.ModelCalls <= limit.ModelCalls
}
