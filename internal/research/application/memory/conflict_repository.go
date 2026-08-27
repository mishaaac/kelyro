package memory

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
)

type conflictRepository struct{ store *Store }

func (repository conflictRepository) Append(ctx context.Context, result research.Conflict) error {
	const operation = "append memory source conflict"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := result.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.conflicts[result.ID]; exists {
		return conflict(operation)
	}
	for _, claimID := range result.ClaimIDs {
		if _, exists := repository.store.claims[claimID]; !exists {
			return notFound(operation)
		}
	}
	if result.WinningClaimID != nil {
		claim := repository.store.claims[*result.WinningClaimID]
		if !claimContainsSource(claim, *result.WinningSourceID) {
			return invalid(operation, fmt.Errorf("winning source is not declared by winning claim"))
		}
	}
	repository.store.conflicts[result.ID] = cloneConflict(result)
	return nil
}

func (repository conflictRepository) Get(ctx context.Context, id research.ID) (research.Conflict, error) {
	const operation = "get memory source conflict"
	if err := contextError(operation, ctx); err != nil {
		return research.Conflict{}, err
	}
	if err := id.Validate(); err != nil {
		return research.Conflict{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result, exists := repository.store.conflicts[id]
	if !exists {
		return research.Conflict{}, notFound(operation)
	}
	return cloneConflict(result), nil
}

func (repository conflictRepository) ListByClaim(ctx context.Context, claimID research.ClaimID) ([]research.Conflict, error) {
	const operation = "list memory source conflicts by claim"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if err := claimID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	results := make([]research.Conflict, 0)
	for _, result := range repository.store.conflicts {
		for _, candidate := range result.ClaimIDs {
			if candidate == claimID {
				results = append(results, cloneConflict(result))
				break
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].DetectedAt.Time().Equal(results[j].DetectedAt.Time()) {
			return results[i].ID.String() < results[j].ID.String()
		}
		return results[i].DetectedAt.Before(results[j].DetectedAt)
	})
	return results, nil
}

func claimContainsSource(claim research.Claim, sourceID research.SourceID) bool {
	for _, candidate := range claim.SourceIDs {
		if candidate == sourceID {
			return true
		}
	}
	return false
}
