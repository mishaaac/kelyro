package memory

import (
	"context"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
)

type citationRepository struct{ store *Store }

func (repository citationRepository) Append(ctx context.Context, citation research.Citation) error {
	const operation = "append memory citation"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := citation.Validate(); err != nil {
		return invalid(operation, err)
	}
	if citation.TemporalAlgorithmVersion != research.SourceTemporalPolicyV1 {
		return invalid(operation, errRelationship("new citations must use source-temporal-policy-v1"))
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.citations[citation.ID]; exists {
		return conflict(operation)
	}
	source, sourceExists := repository.store.sources[citation.SourceID]
	snapshot, snapshotExists := repository.store.snapshots[citation.SnapshotID]
	evidence, evidenceExists := repository.store.evidence[citation.EvidenceID]
	if !sourceExists || !snapshotExists || !evidenceExists {
		return notFound(operation)
	}
	if err := research.ValidateCitationRelationships(citation, source, snapshot, evidence); err != nil {
		return invalid(operation, err)
	}
	repository.store.citations[citation.ID] = cloneCitation(citation)
	return nil
}

func (repository citationRepository) Get(ctx context.Context, id research.ID) (research.Citation, error) {
	const operation = "get memory citation"
	if err := contextError(operation, ctx); err != nil {
		return research.Citation{}, err
	}
	if err := id.Validate(); err != nil {
		return research.Citation{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result, exists := repository.store.citations[id]
	if !exists {
		return research.Citation{}, notFound(operation)
	}
	return cloneCitation(result), nil
}

func (repository citationRepository) ListByEvidence(ctx context.Context, evidenceID research.ID) ([]research.Citation, error) {
	const operation = "list memory citations by evidence"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if err := evidenceID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result := make([]research.Citation, 0)
	for _, item := range repository.store.citations {
		if item.EvidenceID == evidenceID {
			result = append(result, cloneCitation(item))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID.String() < result[j].ID.String() })
	return result, nil
}
