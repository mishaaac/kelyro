package memory

import (
	"context"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchRunRepository struct{ store *Store }

func (repository researchRunRepository) Create(ctx context.Context, request research.ResearchRequest, run research.ResearchRun) error {
	const operation = "create memory research run"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := run.Validate(); err != nil {
		return invalid(operation, err)
	}
	if run.RequestID != request.ID {
		return invalid(operation, errRelationship("run request does not match"))
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.runs[run.ID]; exists {
		return conflict(operation)
	}
	if stored, exists := repository.store.requests[request.ID]; exists {
		if !sameRequest(stored, request) {
			return conflict(operation)
		}
	} else {
		repository.store.requests[request.ID] = cloneRequest(request)
	}
	repository.store.runs[run.ID] = cloneRun(run)
	return nil
}

func (repository researchRunRepository) GetRequest(ctx context.Context, id research.ID) (research.ResearchRequest, error) {
	const operation = "get memory research request"
	if err := contextError(operation, ctx); err != nil {
		return research.ResearchRequest{}, err
	}
	if err := id.Validate(); err != nil {
		return research.ResearchRequest{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	request, exists := repository.store.requests[id]
	if !exists {
		return research.ResearchRequest{}, notFound(operation)
	}
	return cloneRequest(request), nil
}

func (repository researchRunRepository) GetRun(ctx context.Context, id research.ID) (research.ResearchRun, error) {
	const operation = "get memory research run"
	if err := contextError(operation, ctx); err != nil {
		return research.ResearchRun{}, err
	}
	if err := id.Validate(); err != nil {
		return research.ResearchRun{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	run, exists := repository.store.runs[id]
	if !exists {
		return research.ResearchRun{}, notFound(operation)
	}
	return cloneRun(run), nil
}

func (repository researchRunRepository) UpdateRun(ctx context.Context, run research.ResearchRun) error {
	const operation = "update memory research run"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := run.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	stored, exists := repository.store.runs[run.ID]
	if !exists {
		return notFound(operation)
	}
	if stored.RequestID != run.RequestID {
		return invalid(operation, errRelationship("run request identity cannot change"))
	}
	repository.store.runs[run.ID] = cloneRun(run)
	return nil
}

type trustRegistryRepository struct{ store *Store }

func (repository trustRegistryRepository) SaveProfile(ctx context.Context, profile research.AuthorityProfile) error {
	const operation = "save memory authority profile"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := profile.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.profiles[profile.ID] = cloneProfile(profile)
	return nil
}

func (repository trustRegistryRepository) GetProfile(ctx context.Context, id research.ID) (research.AuthorityProfile, error) {
	const operation = "get memory authority profile"
	if err := contextError(operation, ctx); err != nil {
		return research.AuthorityProfile{}, err
	}
	if err := id.Validate(); err != nil {
		return research.AuthorityProfile{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	profile, exists := repository.store.profiles[id]
	if !exists {
		return research.AuthorityProfile{}, notFound(operation)
	}
	return cloneProfile(profile), nil
}

func (repository trustRegistryRepository) ListProfiles(ctx context.Context) ([]research.AuthorityProfile, error) {
	const operation = "list memory authority profiles"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	ids := sortedIDs(repository.store.profiles)
	result := make([]research.AuthorityProfile, 0, len(ids))
	for _, id := range ids {
		result = append(result, cloneProfile(repository.store.profiles[id]))
	}
	return result, nil
}

func (repository trustRegistryRepository) SaveDecision(ctx context.Context, decision research.TrustDecision) error {
	const operation = "save memory trust decision"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := decision.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.sources[decision.SourceID]; !exists {
		return notFound(operation)
	}
	repository.store.decisions[decision.SourceID] = append(
		repository.store.decisions[decision.SourceID], cloneDecision(decision),
	)
	return nil
}

func (repository trustRegistryRepository) LatestDecision(ctx context.Context, sourceID research.SourceID) (research.TrustDecision, error) {
	const operation = "get latest memory trust decision"
	if err := contextError(operation, ctx); err != nil {
		return research.TrustDecision{}, err
	}
	if err := sourceID.Validate(); err != nil {
		return research.TrustDecision{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	decisions := repository.store.decisions[sourceID]
	if len(decisions) == 0 {
		return research.TrustDecision{}, notFound(operation)
	}
	latest := decisions[0]
	for _, decision := range decisions[1:] {
		if decision.EvaluatedAt.After(latest.EvaluatedAt) {
			latest = decision
		}
	}
	return cloneDecision(latest), nil
}

type sourceRegistryRepository struct{ store *Store }

func (repository sourceRegistryRepository) Save(ctx context.Context, entry research.SourceRegistryEntry) error {
	const operation = "save memory source registry entry"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := entry.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	for existingID, existing := range repository.store.registryEntries {
		if existingID == entry.ID {
			continue
		}
		for _, existingDomain := range existing.CanonicalDomains {
			for _, domain := range entry.CanonicalDomains {
				if existingDomain.String() == domain.String() {
					return conflict(operation)
				}
			}
		}
	}
	repository.store.registryEntries[entry.ID] = cloneRegistryEntry(entry)
	return nil
}

func (repository sourceRegistryRepository) Get(ctx context.Context, id research.ID) (research.SourceRegistryEntry, error) {
	const operation = "get memory source registry entry"
	if err := contextError(operation, ctx); err != nil {
		return research.SourceRegistryEntry{}, err
	}
	if err := id.Validate(); err != nil {
		return research.SourceRegistryEntry{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	entry, exists := repository.store.registryEntries[id]
	if !exists {
		return research.SourceRegistryEntry{}, notFound(operation)
	}
	return cloneRegistryEntry(entry), nil
}

func (repository sourceRegistryRepository) List(ctx context.Context) ([]research.SourceRegistryEntry, error) {
	const operation = "list memory source registry entries"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	ids := sortedIDs(repository.store.registryEntries)
	result := make([]research.SourceRegistryEntry, 0, len(ids))
	for _, id := range ids {
		result = append(result, cloneRegistryEntry(repository.store.registryEntries[id]))
	}
	return result, nil
}

type releaseRepository struct{ store *Store }

func (repository releaseRepository) Create(ctx context.Context, record research.ReleaseRecord) error {
	const operation = "create memory release"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return invalid(operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.releases[record.ID]; exists {
		return conflict(operation)
	}
	for _, sourceID := range record.SourceIDs {
		if _, exists := repository.store.sources[sourceID]; !exists {
			return notFound(operation)
		}
	}
	repository.store.releases[record.ID] = cloneRelease(record)
	return nil
}

func (repository releaseRepository) Get(ctx context.Context, id research.ID) (research.ReleaseRecord, error) {
	const operation = "get memory release"
	if err := contextError(operation, ctx); err != nil {
		return research.ReleaseRecord{}, err
	}
	if err := id.Validate(); err != nil {
		return research.ReleaseRecord{}, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	record, exists := repository.store.releases[id]
	if !exists {
		return research.ReleaseRecord{}, notFound(operation)
	}
	return cloneRelease(record), nil
}

func (repository releaseRepository) ListByTechnology(ctx context.Context, technologyID research.ID) ([]research.ReleaseRecord, error) {
	const operation = "list memory releases"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	if err := technologyID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	result := make([]research.ReleaseRecord, 0)
	for _, record := range repository.store.releases {
		if record.TechnologyID == technologyID {
			result = append(result, cloneRelease(record))
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

func sameRequest(left, right research.ResearchRequest) bool {
	if left.ID != right.ID || left.Topic != right.Topic || left.Purpose != right.Purpose ||
		!left.RequestedAt.Time().Equal(right.RequestedAt.Time()) {
		return false
	}
	if left.TargetVersion == nil || right.TargetVersion == nil {
		return left.TargetVersion == nil && right.TargetVersion == nil
	}
	return *left.TargetVersion == *right.TargetVersion
}
