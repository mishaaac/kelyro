package memory

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

type deprecationRepository struct{ store *Store }

func (repository deprecationRepository) Append(ctx context.Context, record research.DeprecationRecord) error {
	const operation = "append memory deprecation"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return invalid(operation, err)
	}
	if record.AlgorithmVersion != research.DeprecationIntelligenceAlgorithmV1 {
		return invalid(operation, errRelationship("new deprecation records must use deprecation-intelligence-v1"))
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.deprecations[record.ID]; exists {
		return conflict(operation)
	}
	declared := make(map[research.SourceID]struct{}, len(record.SourceIDs))
	for _, sourceID := range record.SourceIDs {
		if _, exists := repository.store.sources[sourceID]; !exists {
			return notFound(operation)
		}
		declared[sourceID] = struct{}{}
	}
	used := make(map[research.SourceID]struct{}, len(declared))
	for _, evidenceID := range record.EvidenceIDs {
		evidence, exists := repository.store.evidence[evidenceID]
		if !exists {
			return notFound(operation)
		}
		if _, exists := declared[evidence.SourceID]; !exists {
			return invalid(operation, errRelationship("deprecation evidence source is not declared"))
		}
		used[evidence.SourceID] = struct{}{}
	}
	if len(used) != len(declared) {
		return invalid(operation, errRelationship("deprecation source has no supporting evidence"))
	}
	repository.store.deprecations[record.ID] = cloneDeprecation(record)
	return nil
}

func (repository deprecationRepository) Get(ctx context.Context, id research.ID) (research.DeprecationRecord, error) {
	const operation = "get memory deprecation"
	if err := contextError(operation, ctx); err != nil {
		return research.DeprecationRecord{}, err
	}
	if err := id.Validate(); err != nil {
		return research.DeprecationRecord{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	record, exists := repository.store.deprecations[id]
	if !exists {
		return research.DeprecationRecord{}, notFound(operation)
	}
	return cloneDeprecation(record), nil
}

func (repository deprecationRepository) List(ctx context.Context) ([]research.DeprecationRecord, error) {
	const operation = "list all memory deprecations"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result := make([]research.DeprecationRecord, 0, len(repository.store.deprecations))
	for _, record := range repository.store.deprecations {
		result = append(result, cloneDeprecation(record))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Subject != result[j].Subject {
			return result[i].Subject < result[j].Subject
		}
		if result[i].VerifiedAt.Time().Equal(result[j].VerifiedAt.Time()) {
			return result[i].ID.String() < result[j].ID.String()
		}
		return result[i].VerifiedAt.Before(result[j].VerifiedAt)
	})
	return result, nil
}

func (repository deprecationRepository) ListBySubject(ctx context.Context, subject string) ([]research.DeprecationRecord, error) {
	const operation = "list memory deprecation history"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(subject) == "" || subject != strings.TrimSpace(subject) {
		return nil, invalid(operation, errors.New("deprecation subject is invalid"))
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result := make([]research.DeprecationRecord, 0)
	for _, record := range repository.store.deprecations {
		if record.Subject == subject {
			result = append(result, cloneDeprecation(record))
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
