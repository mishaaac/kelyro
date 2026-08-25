package memory

import (
	"context"

	"github.com/mishaaac/kelyro/internal/research"
)

type provenanceRepository struct{ store *Store }

func (repository provenanceRepository) Append(ctx context.Context, graph research.ProvenanceGraph) error {
	const operation = "append memory provenance graph"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := graph.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	for _, stored := range repository.store.provenance[graph.ClaimID] {
		if stored.ID == graph.ID {
			return conflict(operation)
		}
	}
	repository.store.provenance[graph.ClaimID] = append(repository.store.provenance[graph.ClaimID], cloneProvenanceGraph(graph))
	return nil
}

func (repository provenanceRepository) LatestByClaim(ctx context.Context, claimID research.ClaimID) (research.ProvenanceGraph, error) {
	const operation = "get latest memory provenance graph"
	if err := contextError(operation, ctx); err != nil {
		return research.ProvenanceGraph{}, err
	}
	if err := claimID.Validate(); err != nil {
		return research.ProvenanceGraph{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	graphs := repository.store.provenance[claimID]
	if len(graphs) == 0 {
		return research.ProvenanceGraph{}, notFound(operation)
	}
	latest := graphs[0]
	for _, graph := range graphs[1:] {
		if graph.RecordedAt.After(latest.RecordedAt) || (graph.RecordedAt.Time().Equal(latest.RecordedAt.Time()) && graph.ID.String() > latest.ID.String()) {
			latest = graph
		}
	}
	return cloneProvenanceGraph(latest), nil
}
