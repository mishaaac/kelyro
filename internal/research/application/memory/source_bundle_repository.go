package memory

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
)

type sourceBundleRepository struct{ store *Store }

func (repository sourceBundleRepository) Append(ctx context.Context, bundle research.SourceBundle) error {
	const operation = "append memory source bundle"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := bundle.Validate(); err != nil {
		return invalid(operation, err)
	}
	if bundle.AlgorithmVersion != research.SourceBundleAlgorithmV1 {
		return invalid(operation, fmt.Errorf("new source bundles require %q", research.SourceBundleAlgorithmV1))
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.bundles[bundle.ID]; exists {
		return conflict(operation)
	}
	run, exists := repository.store.runs[bundle.RunID]
	if !exists {
		return notFound(operation)
	}
	request, exists := repository.store.requests[run.RequestID]
	if !exists {
		return notFound(operation)
	}
	if run.Status != research.ResearchRunCompleted || run.CompletedAt == nil || bundle.VerifiedAt.Before(*run.CompletedAt) ||
		request.Topic != bundle.Topic || request.Purpose != bundle.Purpose || !sameBundleVersion(request.TargetVersion, bundle.TargetVersion) {
		return invalid(operation, fmt.Errorf("bundle research run/request relationship does not match"))
	}
	claims := make(map[research.ClaimID]research.Claim, len(bundle.ClaimIDs))
	declaredSources := make(map[research.SourceID]struct{})
	for _, claimID := range bundle.ClaimIDs {
		claim, exists := repository.store.claims[claimID]
		if !exists {
			return notFound(operation)
		}
		claims[claimID] = claim
		for _, sourceID := range claim.SourceIDs {
			declaredSources[sourceID] = struct{}{}
		}
	}
	if len(declaredSources) != len(bundle.Sources) {
		return invalid(operation, fmt.Errorf("bundle sources do not match bundle claim source union"))
	}
	for _, source := range bundle.Sources {
		if _, exists := repository.store.sources[source.SourceID]; !exists {
			return notFound(operation)
		}
		if _, declared := declaredSources[source.SourceID]; !declared {
			return invalid(operation, fmt.Errorf("bundle source is not declared by a bundle claim"))
		}
	}
	for _, conflictID := range bundle.ConflictIDs {
		item, exists := repository.store.conflicts[conflictID]
		if !exists {
			return notFound(operation)
		}
		relevant := false
		for _, claimID := range item.ClaimIDs {
			if _, exists := claims[claimID]; exists {
				relevant = true
				break
			}
		}
		if !relevant {
			return invalid(operation, fmt.Errorf("bundle conflict is unrelated to bundle claims"))
		}
	}
	repository.store.bundles[bundle.ID] = cloneSourceBundle(bundle)
	return nil
}

func sameBundleVersion(left, right *research.SourceVersion) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (repository sourceBundleRepository) Get(ctx context.Context, id research.ID) (research.SourceBundle, error) {
	const operation = "get memory source bundle"
	if err := contextError(operation, ctx); err != nil {
		return research.SourceBundle{}, err
	}
	if err := id.Validate(); err != nil {
		return research.SourceBundle{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	bundle, exists := repository.store.bundles[id]
	if !exists {
		return research.SourceBundle{}, notFound(operation)
	}
	return cloneSourceBundle(bundle), nil
}

func (repository sourceBundleRepository) ListByRun(ctx context.Context, runID research.ID) ([]research.SourceBundle, error) {
	const operation = "list memory source bundles by run"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if err := runID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result := make([]research.SourceBundle, 0)
	for _, bundle := range repository.store.bundles {
		if bundle.RunID == runID {
			result = append(result, cloneSourceBundle(bundle))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].VerifiedAt.Time().Equal(result[j].VerifiedAt.Time()) {
			return result[i].ID.String() < result[j].ID.String()
		}
		return result[i].VerifiedAt.Before(result[j].VerifiedAt)
	})
	return result, nil
}
