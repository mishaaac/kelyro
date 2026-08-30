package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type researchCostRepository struct {
	executor executor
	timeout  time.Duration
}

func insertResearchCostControl(ctx context.Context, target executor, runID research.ID, metadata research.ResearchCostMetadata) error {
	budget := metadata.Budget
	var daily [5]any
	if budget.Daily != nil {
		daily = [5]any{budget.Daily.SearchRequests, budget.Daily.FetchRequests, budget.Daily.Bytes, budget.Daily.ProviderAPICalls, budget.Daily.ModelCalls}
	}
	_, err := target.ExecContext(ctx, `INSERT INTO research_cost_controls (
run_id,run_search_limit,run_fetch_limit,run_bytes_limit,run_provider_limit,run_model_limit,
topic_search_limit,topic_fetch_limit,topic_bytes_limit,topic_provider_limit,topic_model_limit,
daily_search_limit,daily_fetch_limit,daily_bytes_limit,daily_provider_limit,daily_model_limit,algorithm_version)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		runID.String(), budget.PerRun.SearchRequests, budget.PerRun.FetchRequests, budget.PerRun.Bytes, budget.PerRun.ProviderAPICalls, budget.PerRun.ModelCalls,
		budget.PerTopic.SearchRequests, budget.PerTopic.FetchRequests, budget.PerTopic.Bytes, budget.PerTopic.ProviderAPICalls, budget.PerTopic.ModelCalls,
		daily[0], daily[1], daily[2], daily[3], daily[4], research.ResearchCostControlAlgorithmV1)
	return err
}

func (repository *researchCostRepository) Reserve(ctx context.Context, reservation application.CostReservation) (application.CostReservationResult, error) {
	const operation = "reserve SQLite research cost"
	if err := reservation.Validate(); err != nil {
		return application.CostReservationResult{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return application.CostReservationResult{}, err
	}
	defer cancel()
	usage := reservation.Usage
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO research_cost_events (run_id,occurred_at,search_requests,fetch_requests,bytes,provider_api_calls,model_calls) VALUES (?,?,?,?,?,?,?)`,
		reservation.RunID.String(), timestampText(reservation.At), usage.SearchRequests, usage.FetchRequests, usage.Bytes, usage.ProviderAPICalls, usage.ModelCalls)
	if err == nil {
		_, _ = repository.executor.ExecContext(opCtx, `UPDATE research_cost_controls SET stopped_by_budget=0,stop_scope='',stop_reason='' WHERE run_id=?`, reservation.RunID.String())
		metadata, metadataErr := repository.metadataWithContext(opCtx, reservation.RunID)
		return application.CostReservationResult{Allowed: true, Metadata: metadata}, metadataErr
	}
	if !strings.Contains(err.Error(), "research cost budget exceeded") {
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			return application.CostReservationResult{}, researchNotFound(operation)
		}
		return application.CostReservationResult{}, researchPersistence(operation, err)
	}
	scope, err := repository.exceededScope(opCtx, reservation)
	if err != nil {
		return application.CostReservationResult{}, err
	}
	reason := fmt.Sprintf("Research stopped because the %s budget would be exceeded.", scope)
	if _, err := repository.executor.ExecContext(opCtx, `UPDATE research_cost_controls SET stopped_by_budget=1,stop_scope=?,stop_reason=? WHERE run_id=?`, string(scope), reason, reservation.RunID.String()); err != nil {
		return application.CostReservationResult{}, researchPersistence(operation, err)
	}
	metadata, err := repository.metadataWithContext(opCtx, reservation.RunID)
	return application.CostReservationResult{Scope: scope, Reason: reason, Metadata: metadata}, err
}

func (repository *researchCostRepository) RecordCacheSavings(ctx context.Context, reservation application.CostReservation) error {
	const operation = "record SQLite research cache savings"
	if err := reservation.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	u := reservation.Usage
	result, err := repository.executor.ExecContext(opCtx, `UPDATE research_cost_controls SET cache_search_saved=cache_search_saved+?,cache_fetch_saved=cache_fetch_saved+?,cache_bytes_saved=cache_bytes_saved+?,cache_provider_saved=cache_provider_saved+?,cache_model_saved=cache_model_saved+? WHERE run_id=?`,
		u.SearchRequests, u.FetchRequests, u.Bytes, u.ProviderAPICalls, u.ModelCalls, reservation.RunID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if err := requireAffected(result); errors.Is(err, sql.ErrNoRows) {
		return researchNotFound(operation)
	} else if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchCostRepository) Metadata(ctx context.Context, runID research.ID) (research.ResearchCostMetadata, error) {
	const operation = "get SQLite research cost metadata"
	if err := runID.Validate(); err != nil {
		return research.ResearchCostMetadata{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.ResearchCostMetadata{}, err
	}
	defer cancel()
	return repository.metadataWithContext(opCtx, runID)
}

func (repository *researchCostRepository) metadataWithContext(ctx context.Context, runID research.ID) (research.ResearchCostMetadata, error) {
	const operation = "get SQLite research cost metadata"
	row := repository.executor.QueryRowContext(ctx, `SELECT
c.run_search_limit,c.run_fetch_limit,c.run_bytes_limit,c.run_provider_limit,c.run_model_limit,
c.topic_search_limit,c.topic_fetch_limit,c.topic_bytes_limit,c.topic_provider_limit,c.topic_model_limit,
c.daily_search_limit,c.daily_fetch_limit,c.daily_bytes_limit,c.daily_provider_limit,c.daily_model_limit,
COALESCE(SUM(e.search_requests),0),COALESCE(SUM(e.fetch_requests),0),COALESCE(SUM(e.bytes),0),COALESCE(SUM(e.provider_api_calls),0),COALESCE(SUM(e.model_calls),0),
c.cache_search_saved,c.cache_fetch_saved,c.cache_bytes_saved,c.cache_provider_saved,c.cache_model_saved,
c.stopped_by_budget,c.stop_scope,c.stop_reason,c.algorithm_version
FROM research_cost_controls c LEFT JOIN research_cost_events e ON e.run_id=c.run_id WHERE c.run_id=? GROUP BY c.run_id`, runID.String())
	var metadata research.ResearchCostMetadata
	var dailySearch, dailyFetch, dailyBytes, dailyProvider, dailyModel sql.NullInt64
	var stopped int
	err := row.Scan(
		&metadata.Budget.PerRun.SearchRequests, &metadata.Budget.PerRun.FetchRequests, &metadata.Budget.PerRun.Bytes, &metadata.Budget.PerRun.ProviderAPICalls, &metadata.Budget.PerRun.ModelCalls,
		&metadata.Budget.PerTopic.SearchRequests, &metadata.Budget.PerTopic.FetchRequests, &metadata.Budget.PerTopic.Bytes, &metadata.Budget.PerTopic.ProviderAPICalls, &metadata.Budget.PerTopic.ModelCalls,
		&dailySearch, &dailyFetch, &dailyBytes, &dailyProvider, &dailyModel,
		&metadata.Used.SearchRequests, &metadata.Used.FetchRequests, &metadata.Used.Bytes, &metadata.Used.ProviderAPICalls, &metadata.Used.ModelCalls,
		&metadata.CacheSavings.SearchRequests, &metadata.CacheSavings.FetchRequests, &metadata.CacheSavings.Bytes, &metadata.CacheSavings.ProviderAPICalls, &metadata.CacheSavings.ModelCalls,
		&stopped, &metadata.StopScope, &metadata.StopReason, &metadata.AlgorithmVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return research.ResearchCostMetadata{}, researchNotFound(operation)
	}
	if err != nil {
		return research.ResearchCostMetadata{}, researchPersistence(operation, err)
	}
	metadata.Budget.AlgorithmVersion = metadata.AlgorithmVersion
	if dailySearch.Valid {
		metadata.Budget.Daily = &research.ResearchCostUsage{SearchRequests: dailySearch.Int64, FetchRequests: dailyFetch.Int64, Bytes: dailyBytes.Int64, ProviderAPICalls: dailyProvider.Int64, ModelCalls: dailyModel.Int64}
	}
	metadata.StoppedByBudget = stopped == 1
	if err := metadata.Validate(); err != nil {
		return research.ResearchCostMetadata{}, researchPersistence(operation, err)
	}
	return metadata, nil
}

func (repository *researchCostRepository) exceededScope(ctx context.Context, reservation application.CostReservation) (research.ResearchBudgetScope, error) {
	metadata, err := repository.metadataWithContext(ctx, reservation.RunID)
	if err != nil {
		return "", err
	}
	if !researchCostWithin(metadata.Used.Add(reservation.Usage), metadata.Budget.PerRun) {
		return research.ResearchBudgetRun, nil
	}
	topic, err := repository.topicUsage(ctx, reservation.RunID)
	if err != nil {
		return "", err
	}
	if !researchCostWithin(topic.Add(reservation.Usage), metadata.Budget.PerTopic) {
		return research.ResearchBudgetTopic, nil
	}
	if metadata.Budget.Daily != nil {
		daily, dailyErr := repository.dailyUsage(ctx, reservation.At)
		if dailyErr != nil {
			return "", dailyErr
		}
		if !researchCostWithin(daily.Add(reservation.Usage), *metadata.Budget.Daily) {
			return research.ResearchBudgetDaily, nil
		}
	}
	return "", researchPersistence("determine SQLite research cost scope", errors.New("budget trigger fired without an exceeded scope"))
}

func (repository *researchCostRepository) topicUsage(ctx context.Context, runID research.ID) (research.ResearchCostUsage, error) {
	var usage research.ResearchCostUsage
	err := repository.executor.QueryRowContext(ctx, `SELECT COALESCE(SUM(e.search_requests),0),COALESCE(SUM(e.fetch_requests),0),COALESCE(SUM(e.bytes),0),COALESCE(SUM(e.provider_api_calls),0),COALESCE(SUM(e.model_calls),0)
FROM research_runs current_run JOIN research_topics current_topic ON current_topic.request_id=current_run.request_id
LEFT JOIN research_topics t ON t.subject=current_topic.subject AND t.domain=current_topic.domain AND t.technology=current_topic.technology
LEFT JOIN research_runs r ON r.request_id=t.request_id LEFT JOIN research_cost_events e ON e.run_id=r.id WHERE current_run.id=?`, runID.String()).Scan(
		&usage.SearchRequests, &usage.FetchRequests, &usage.Bytes, &usage.ProviderAPICalls, &usage.ModelCalls)
	if err != nil {
		return research.ResearchCostUsage{}, researchPersistence("get SQLite topic research cost", err)
	}
	return usage, nil
}

func (repository *researchCostRepository) dailyUsage(ctx context.Context, at research.Timestamp) (research.ResearchCostUsage, error) {
	var usage research.ResearchCostUsage
	err := repository.executor.QueryRowContext(ctx, `SELECT COALESCE(SUM(search_requests),0),COALESCE(SUM(fetch_requests),0),COALESCE(SUM(bytes),0),COALESCE(SUM(provider_api_calls),0),COALESCE(SUM(model_calls),0) FROM research_cost_events WHERE substr(occurred_at,1,10)=?`, at.Time().UTC().Format("2006-01-02")).Scan(
		&usage.SearchRequests, &usage.FetchRequests, &usage.Bytes, &usage.ProviderAPICalls, &usage.ModelCalls)
	if err != nil {
		return research.ResearchCostUsage{}, researchPersistence("get SQLite daily research cost", err)
	}
	return usage, nil
}

func (repository *researchCostRepository) Stats(ctx context.Context, asOf research.Timestamp) (application.ResearchCostStats, error) {
	const operation = "get SQLite research cost stats"
	if err := asOf.Validate(); err != nil {
		return application.ResearchCostStats{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return application.ResearchCostStats{}, err
	}
	defer cancel()
	stats := application.ResearchCostStats{AsOf: asOf, AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	err = repository.executor.QueryRowContext(opCtx, `SELECT COUNT(*),COALESCE(SUM(stopped_by_budget),0),COALESCE(SUM(cache_search_saved),0),COALESCE(SUM(cache_fetch_saved),0),COALESCE(SUM(cache_bytes_saved),0),COALESCE(SUM(cache_provider_saved),0),COALESCE(SUM(cache_model_saved),0) FROM research_cost_controls`).Scan(
		&stats.Runs, &stats.BudgetStoppedRuns, &stats.CacheSavings.SearchRequests, &stats.CacheSavings.FetchRequests, &stats.CacheSavings.Bytes, &stats.CacheSavings.ProviderAPICalls, &stats.CacheSavings.ModelCalls)
	if err != nil {
		return application.ResearchCostStats{}, researchPersistence(operation, err)
	}
	err = repository.executor.QueryRowContext(opCtx, `SELECT COALESCE(SUM(search_requests),0),COALESCE(SUM(fetch_requests),0),COALESCE(SUM(bytes),0),COALESCE(SUM(provider_api_calls),0),COALESCE(SUM(model_calls),0),COALESCE(SUM(CASE WHEN substr(occurred_at,1,10)=? THEN search_requests ELSE 0 END),0),COALESCE(SUM(CASE WHEN substr(occurred_at,1,10)=? THEN fetch_requests ELSE 0 END),0),COALESCE(SUM(CASE WHEN substr(occurred_at,1,10)=? THEN bytes ELSE 0 END),0),COALESCE(SUM(CASE WHEN substr(occurred_at,1,10)=? THEN provider_api_calls ELSE 0 END),0),COALESCE(SUM(CASE WHEN substr(occurred_at,1,10)=? THEN model_calls ELSE 0 END),0) FROM research_cost_events`,
		asOf.Time().UTC().Format("2006-01-02"), asOf.Time().UTC().Format("2006-01-02"), asOf.Time().UTC().Format("2006-01-02"), asOf.Time().UTC().Format("2006-01-02"), asOf.Time().UTC().Format("2006-01-02")).Scan(
		&stats.Used.SearchRequests, &stats.Used.FetchRequests, &stats.Used.Bytes, &stats.Used.ProviderAPICalls, &stats.Used.ModelCalls,
		&stats.TodayUsed.SearchRequests, &stats.TodayUsed.FetchRequests, &stats.TodayUsed.Bytes, &stats.TodayUsed.ProviderAPICalls, &stats.TodayUsed.ModelCalls)
	if err != nil {
		return application.ResearchCostStats{}, researchPersistence(operation, err)
	}
	return stats, nil
}

func researchCostWithin(usage, limit research.ResearchCostUsage) bool {
	return usage.SearchRequests <= limit.SearchRequests && usage.FetchRequests <= limit.FetchRequests && usage.Bytes <= limit.Bytes && usage.ProviderAPICalls <= limit.ProviderAPICalls && usage.ModelCalls <= limit.ModelCalls
}
