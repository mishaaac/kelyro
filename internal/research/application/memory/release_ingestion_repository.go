package memory

import (
	"context"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type releaseIngestionRepository struct{ store *Store }

func (repository releaseIngestionRepository) Commit(ctx context.Context, batch application.ReleaseIngestionBatch) error {
	const operation = "commit memory release ingestion"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := batch.ValidateBounds(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()

	evidence := cloneMap(repository.store.evidence)
	claims := cloneMap(repository.store.claims)
	releases := cloneMap(repository.store.releases)
	for _, item := range batch.Evidence {
		if err := item.Validate(); err != nil {
			return invalid(operation, err)
		}
		if _, exists := evidence[item.ID]; exists {
			return conflict(operation)
		}
		snapshot, exists := repository.store.snapshots[item.SnapshotID]
		if !exists {
			return notFound(operation)
		}
		if snapshot.SourceID != item.SourceID {
			return invalid(operation, errRelationship("evidence source does not match snapshot"))
		}
		evidence[item.ID] = item
	}
	for _, claim := range batch.Claims {
		if err := claim.Validate(); err != nil {
			return invalid(operation, err)
		}
		if _, exists := claims[claim.ID]; exists {
			return conflict(operation)
		}
		declared := make(map[research.SourceID]struct{}, len(claim.SourceIDs))
		for _, sourceID := range claim.SourceIDs {
			if _, exists := repository.store.sources[sourceID]; !exists {
				return notFound(operation)
			}
			declared[sourceID] = struct{}{}
		}
		used := make(map[research.SourceID]struct{}, len(declared))
		for _, evidenceID := range claim.EvidenceIDs {
			item, exists := evidence[evidenceID]
			if !exists {
				return notFound(operation)
			}
			if _, exists := declared[item.SourceID]; !exists {
				return invalid(operation, errRelationship("claim evidence source is not declared"))
			}
			used[item.SourceID] = struct{}{}
		}
		if len(used) != len(declared) {
			return invalid(operation, errRelationship("claim source has no supporting evidence"))
		}
		claims[claim.ID] = cloneClaim(claim)
	}
	for _, record := range batch.Releases {
		if err := record.Validate(); err != nil {
			return invalid(operation, err)
		}
		if _, exists := releases[record.ID]; exists {
			return conflict(operation)
		}
		for _, sourceID := range record.SourceIDs {
			if _, exists := repository.store.sources[sourceID]; !exists {
				return notFound(operation)
			}
		}
		releases[record.ID] = cloneRelease(record)
	}
	for _, record := range batch.StatusUpdates {
		if err := record.Validate(); err != nil {
			return invalid(operation, err)
		}
		stored, exists := releases[record.ID]
		if !exists {
			return notFound(operation)
		}
		if stored.TechnologyID != record.TechnologyID || stored.Version != record.Version || stored.Channel != record.Channel ||
			!sameOptionalTimestamp(stored.ReleasedAt, record.ReleasedAt) || !stored.VerifiedAt.Time().Equal(record.VerifiedAt.Time()) ||
			!sameSourceIDs(stored.SourceIDs, record.SourceIDs) {
			return invalid(operation, errRelationship("release update may only change lifecycle status"))
		}
		releases[record.ID] = cloneRelease(record)
	}
	repository.store.evidence, repository.store.claims, repository.store.releases = evidence, claims, releases
	return nil
}

func cloneMap[K comparable, V any](source map[K]V) map[K]V {
	result := make(map[K]V, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
