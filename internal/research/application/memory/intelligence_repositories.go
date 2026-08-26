package memory

import (
	"context"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type freshnessRepository struct{ store *Store }

func (repository freshnessRepository) Save(ctx context.Context, record application.FreshnessRecord) error {
	const operation = "save memory freshness"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.freshness[record.SubjectID] = cloneFreshness(record)
	return nil
}

func (repository freshnessRepository) Get(ctx context.Context, subjectID research.ID) (application.FreshnessRecord, error) {
	const operation = "get memory freshness"
	if err := contextError(operation, ctx); err != nil {
		return application.FreshnessRecord{}, err
	}
	if err := subjectID.Validate(); err != nil {
		return application.FreshnessRecord{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	record, exists := repository.store.freshness[subjectID]
	if !exists {
		return application.FreshnessRecord{}, notFound(operation)
	}
	return cloneFreshness(record), nil
}

func (repository freshnessRepository) ListDue(ctx context.Context, asOf research.Timestamp) ([]application.FreshnessRecord, error) {
	const operation = "list memory freshness due"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if err := asOf.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result := make([]application.FreshnessRecord, 0)
	for _, record := range repository.store.freshness {
		if record.NextVerifyAt != nil && !record.NextVerifyAt.After(asOf) {
			result = append(result, cloneFreshness(record))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority.Rank() != result[j].Priority.Rank() {
			return result[i].Priority.Rank() < result[j].Priority.Rank()
		}
		if result[i].NextVerifyAt.Time().Equal(result[j].NextVerifyAt.Time()) {
			return result[i].SubjectID.String() < result[j].SubjectID.String()
		}
		return result[i].NextVerifyAt.Before(*result[j].NextVerifyAt)
	})
	return result, nil
}

type verificationRepository struct{ store *Store }

func (repository verificationRepository) Append(ctx context.Context, result research.VerificationResult) error {
	const operation = "append memory verification"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := result.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.verification[result.ID]; exists {
		return conflict(operation)
	}
	for _, sourceID := range result.SourceIDs {
		if _, exists := repository.store.sources[sourceID]; !exists {
			return notFound(operation)
		}
	}
	repository.store.verification[result.ID] = cloneVerification(result)
	return nil
}

func (repository verificationRepository) Get(ctx context.Context, id research.ID) (research.VerificationResult, error) {
	const operation = "get memory verification"
	if err := contextError(operation, ctx); err != nil {
		return research.VerificationResult{}, err
	}
	if err := id.Validate(); err != nil {
		return research.VerificationResult{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result, exists := repository.store.verification[id]
	if !exists {
		return research.VerificationResult{}, notFound(operation)
	}
	return cloneVerification(result), nil
}

func (repository verificationRepository) LatestByClaim(ctx context.Context, claimID research.ClaimID) (research.VerificationResult, error) {
	const operation = "get latest memory verification"
	if err := contextError(operation, ctx); err != nil {
		return research.VerificationResult{}, err
	}
	if err := claimID.Validate(); err != nil {
		return research.VerificationResult{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	var latest research.VerificationResult
	found := false
	for _, result := range repository.store.verification {
		if result.ClaimID != claimID {
			continue
		}
		if !found || result.VerifiedAt.After(latest.VerifiedAt) ||
			(result.VerifiedAt.Time().Equal(latest.VerifiedAt.Time()) && result.ID.String() > latest.ID.String()) {
			latest = result
			found = true
		}
	}
	if !found {
		return research.VerificationResult{}, notFound(operation)
	}
	return cloneVerification(latest), nil
}

type driftRepository struct{ store *Store }

func (repository driftRepository) Append(ctx context.Context, report research.DriftReport) error {
	const operation = "append memory drift"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := report.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.drift[report.ID]; exists {
		return conflict(operation)
	}
	repository.store.drift[report.ID] = cloneDrift(report)
	return nil
}

func (repository driftRepository) Get(ctx context.Context, id research.ID) (research.DriftReport, error) {
	const operation = "get memory drift"
	if err := contextError(operation, ctx); err != nil {
		return research.DriftReport{}, err
	}
	if err := id.Validate(); err != nil {
		return research.DriftReport{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	report, exists := repository.store.drift[id]
	if !exists {
		return research.DriftReport{}, notFound(operation)
	}
	return cloneDrift(report), nil
}

type impactRepository struct{ store *Store }

func (repository impactRepository) Append(ctx context.Context, report research.ImpactReport) error {
	const operation = "append memory impact"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := report.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.impact[report.ID]; exists {
		return conflict(operation)
	}
	if _, exists := repository.store.drift[report.DriftReportID]; !exists {
		return notFound(operation)
	}
	repository.store.impact[report.ID] = cloneImpact(report)
	return nil
}

func (repository impactRepository) Get(ctx context.Context, id research.ID) (research.ImpactReport, error) {
	const operation = "get memory impact"
	if err := contextError(operation, ctx); err != nil {
		return research.ImpactReport{}, err
	}
	if err := id.Validate(); err != nil {
		return research.ImpactReport{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	report, exists := repository.store.impact[id]
	if !exists {
		return research.ImpactReport{}, notFound(operation)
	}
	return cloneImpact(report), nil
}

type cacheRepository struct{ store *Store }

func (repository cacheRepository) Put(ctx context.Context, entry application.CacheEntry) error {
	const operation = "put memory cache entry"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := entry.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.cache[entry.Key] = cloneCache(entry)
	return nil
}

func (repository cacheRepository) Get(ctx context.Context, key string) (application.CacheEntry, error) {
	const operation = "get memory cache entry"
	if err := contextError(operation, ctx); err != nil {
		return application.CacheEntry{}, err
	}
	if err := validateKey(operation, key); err != nil {
		return application.CacheEntry{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	entry, exists := repository.store.cache[key]
	if !exists {
		return application.CacheEntry{}, notFound(operation)
	}
	return cloneCache(entry), nil
}

func (repository cacheRepository) Delete(ctx context.Context, key string) error {
	const operation = "delete memory cache entry"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := validateKey(operation, key); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.cache[key]; !exists {
		return notFound(operation)
	}
	delete(repository.store.cache, key)
	return nil
}
