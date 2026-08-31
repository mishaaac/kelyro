package memory

import (
	"context"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
)

type sourceDiscoveryRepository struct{ store *Store }

func (repository sourceDiscoveryRepository) Append(ctx context.Context, discovery research.DiscoveredSource) error {
	const operation = "append memory source discovery"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := discovery.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.discoveries[discovery.ID]; exists {
		return conflict(operation)
	}
	if _, exists := repository.store.requests[discovery.RequestID]; !exists {
		return notFound(operation)
	}
	source, exists := repository.store.sources[discovery.SourceID]
	if !exists {
		return notFound(operation)
	}
	if source.Locator != discovery.Locator {
		return invalid(operation, errDiscoveryLocatorMismatch)
	}
	repository.store.discoveries[discovery.ID] = cloneDiscoveredSource(discovery)
	return nil
}

func (repository sourceDiscoveryRepository) Get(ctx context.Context, id research.ID) (research.DiscoveredSource, error) {
	const operation = "get memory source discovery"
	if err := contextError(operation, ctx); err != nil {
		return research.DiscoveredSource{}, err
	}
	if err := id.Validate(); err != nil {
		return research.DiscoveredSource{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	discovery, exists := repository.store.discoveries[id]
	if !exists {
		return research.DiscoveredSource{}, notFound(operation)
	}
	return cloneDiscoveredSource(discovery), nil
}

func (repository sourceDiscoveryRepository) ListBySource(ctx context.Context, sourceID research.SourceID) ([]research.DiscoveredSource, error) {
	const operation = "list memory source discoveries"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if err := sourceID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	if _, exists := repository.store.sources[sourceID]; !exists {
		return nil, notFound(operation)
	}
	result := make([]research.DiscoveredSource, 0)
	for _, discovery := range repository.store.discoveries {
		if discovery.SourceID == sourceID {
			result = append(result, cloneDiscoveredSource(discovery))
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DiscoveredAt.Time().Equal(result[j].DiscoveredAt.Time()) {
			return result[i].ID.String() < result[j].ID.String()
		}
		return result[i].DiscoveredAt.Before(result[j].DiscoveredAt)
	})
	return result, nil
}
