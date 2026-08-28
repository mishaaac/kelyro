package memory

import (
	"context"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
)

type sourceRepository struct{ store *Store }

func (repository sourceRepository) Create(ctx context.Context, source research.Source) error {
	const operation = "create memory source"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := source.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.sources[source.ID]; exists {
		return conflict(operation)
	}
	if _, exists := repository.store.sourceLocators[source.Locator.String()]; exists {
		return conflict(operation)
	}
	repository.store.sources[source.ID] = cloneSource(source)
	repository.store.sourceLocators[source.Locator.String()] = source.ID
	return nil
}

func (repository sourceRepository) Get(ctx context.Context, id research.SourceID) (research.Source, error) {
	const operation = "get memory source"
	if err := contextError(operation, ctx); err != nil {
		return research.Source{}, err
	}
	if err := id.Validate(); err != nil {
		return research.Source{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	source, exists := repository.store.sources[id]
	if !exists {
		return research.Source{}, notFound(operation)
	}
	return cloneSource(source), nil
}

func (repository sourceRepository) FindByLocator(ctx context.Context, locator research.SourceLocator) (research.Source, error) {
	const operation = "find memory source by locator"
	if err := contextError(operation, ctx); err != nil {
		return research.Source{}, err
	}
	if err := locator.Validate(); err != nil {
		return research.Source{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	id, exists := repository.store.sourceLocators[locator.String()]
	if !exists {
		return research.Source{}, notFound(operation)
	}
	return cloneSource(repository.store.sources[id]), nil
}

func (repository sourceRepository) List(ctx context.Context) ([]research.Source, error) {
	const operation = "list memory sources"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	ids := make([]research.SourceID, 0, len(repository.store.sources))
	for id := range repository.store.sources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	result := make([]research.Source, 0, len(ids))
	for _, id := range ids {
		result = append(result, cloneSource(repository.store.sources[id]))
	}
	return result, nil
}

func (repository sourceRepository) SetTemporalScope(ctx context.Context, id research.SourceID, scope research.SourceTemporalScope) error {
	const operation = "set memory source temporal scope"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := id.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := scope.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	source, exists := repository.store.sources[id]
	if !exists {
		return notFound(operation)
	}
	source.TemporalScope = scope
	if err := source.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.sources[id] = cloneSource(source)
	return nil
}

type snapshotRepository struct{ store *Store }

func (repository snapshotRepository) Append(ctx context.Context, snapshot research.SourceSnapshot) error {
	const operation = "append memory snapshot"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := snapshot.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.snapshots[snapshot.ID]; exists {
		return conflict(operation)
	}
	if _, exists := repository.store.sources[snapshot.SourceID]; !exists {
		return notFound(operation)
	}
	repository.store.snapshots[snapshot.ID] = snapshot
	return nil
}

func (repository snapshotRepository) Get(ctx context.Context, id research.ID) (research.SourceSnapshot, error) {
	const operation = "get memory snapshot"
	if err := contextError(operation, ctx); err != nil {
		return research.SourceSnapshot{}, err
	}
	if err := id.Validate(); err != nil {
		return research.SourceSnapshot{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	snapshot, exists := repository.store.snapshots[id]
	if !exists {
		return research.SourceSnapshot{}, notFound(operation)
	}
	return snapshot, nil
}

func (repository snapshotRepository) LatestBySource(ctx context.Context, sourceID research.SourceID) (research.SourceSnapshot, error) {
	const operation = "get latest memory snapshot"
	if err := contextError(operation, ctx); err != nil {
		return research.SourceSnapshot{}, err
	}
	if err := sourceID.Validate(); err != nil {
		return research.SourceSnapshot{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	var latest research.SourceSnapshot
	found := false
	for _, snapshot := range repository.store.snapshots {
		if snapshot.SourceID != sourceID {
			continue
		}
		if !found || snapshot.FetchedAt.After(latest.FetchedAt) ||
			(snapshot.FetchedAt.Time().Equal(latest.FetchedAt.Time()) && snapshot.ID.String() > latest.ID.String()) {
			latest = snapshot
			found = true
		}
	}
	if !found {
		return research.SourceSnapshot{}, notFound(operation)
	}
	return latest, nil
}

func (repository snapshotRepository) ListBySource(ctx context.Context, sourceID research.SourceID) ([]research.SourceSnapshot, error) {
	const operation = "list memory snapshots"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if err := sourceID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result := make([]research.SourceSnapshot, 0)
	for _, snapshot := range repository.store.snapshots {
		if snapshot.SourceID == sourceID {
			result = append(result, snapshot)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].FetchedAt.Time().Equal(result[j].FetchedAt.Time()) {
			return result[i].ID.String() < result[j].ID.String()
		}
		return result[i].FetchedAt.Before(result[j].FetchedAt)
	})
	return result, nil
}

type evidenceRepository struct{ store *Store }

func (repository evidenceRepository) Append(ctx context.Context, evidence research.Evidence) error {
	const operation = "append memory evidence"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := evidence.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.evidence[evidence.ID]; exists {
		return conflict(operation)
	}
	snapshot, exists := repository.store.snapshots[evidence.SnapshotID]
	if !exists {
		return notFound(operation)
	}
	if snapshot.SourceID != evidence.SourceID {
		return invalid(operation, errRelationship("evidence source does not match snapshot"))
	}
	source := repository.store.sources[evidence.SourceID]
	if err := research.ValidateSourceCodeEvidenceRelationship(source, evidence); err != nil {
		return invalid(operation, err)
	}
	repository.store.evidence[evidence.ID] = cloneEvidence(evidence)
	return nil
}

func (repository evidenceRepository) Get(ctx context.Context, id research.ID) (research.Evidence, error) {
	const operation = "get memory evidence"
	if err := contextError(operation, ctx); err != nil {
		return research.Evidence{}, err
	}
	if err := id.Validate(); err != nil {
		return research.Evidence{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	evidence, exists := repository.store.evidence[id]
	if !exists {
		return research.Evidence{}, notFound(operation)
	}
	return cloneEvidence(evidence), nil
}

func (repository evidenceRepository) ListBySource(ctx context.Context, sourceID research.SourceID) ([]research.Evidence, error) {
	const operation = "list memory evidence by source"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if err := sourceID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	return repository.list(ctx, func(evidence research.Evidence) bool { return evidence.SourceID == sourceID })
}

func (repository evidenceRepository) ListBySnapshot(ctx context.Context, snapshotID research.ID) ([]research.Evidence, error) {
	const operation = "list memory evidence by snapshot"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if err := snapshotID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	return repository.list(ctx, func(evidence research.Evidence) bool { return evidence.SnapshotID == snapshotID })
}

func (repository evidenceRepository) list(_ context.Context, include func(research.Evidence) bool) ([]research.Evidence, error) {
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result := make([]research.Evidence, 0)
	for _, evidence := range repository.store.evidence {
		if include(evidence) {
			result = append(result, cloneEvidence(evidence))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID.String() < result[j].ID.String() })
	return result, nil
}

type relationshipError string

func (err relationshipError) Error() string { return string(err) }
func errRelationship(message string) error  { return relationshipError(message) }

type claimRepository struct{ store *Store }

func (repository claimRepository) Append(ctx context.Context, claim research.Claim) error {
	const operation = "append memory claim"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := claim.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.claims[claim.ID]; exists {
		return conflict(operation)
	}
	seenSources := make(map[research.SourceID]struct{}, len(claim.SourceIDs))
	for _, sourceID := range claim.SourceIDs {
		if _, exists := repository.store.sources[sourceID]; !exists {
			return notFound(operation)
		}
		seenSources[sourceID] = struct{}{}
	}
	usedSources := make(map[research.SourceID]struct{}, len(claim.SourceIDs))
	for _, evidenceID := range claim.EvidenceIDs {
		evidence, exists := repository.store.evidence[evidenceID]
		if !exists {
			return notFound(operation)
		}
		if _, exists := seenSources[evidence.SourceID]; !exists {
			return invalid(operation, errRelationship("claim evidence source is not declared"))
		}
		usedSources[evidence.SourceID] = struct{}{}
	}
	if len(usedSources) != len(seenSources) {
		return invalid(operation, errRelationship("claim source has no supporting evidence"))
	}
	repository.store.claims[claim.ID] = cloneClaim(claim)
	return nil
}

func (repository claimRepository) Get(ctx context.Context, id research.ClaimID) (research.Claim, error) {
	const operation = "get memory claim"
	if err := contextError(operation, ctx); err != nil {
		return research.Claim{}, err
	}
	if err := id.Validate(); err != nil {
		return research.Claim{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	claim, exists := repository.store.claims[id]
	if !exists {
		return research.Claim{}, notFound(operation)
	}
	return cloneClaim(claim), nil
}
