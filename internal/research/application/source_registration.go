package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

const SourceCandidateRegistrationV1 = "source-candidate-registration-v1"

type SourceCandidateRegistrationRequest struct {
	RequestID  research.ID
	Candidates []DeduplicatedSourceCandidate
}

func (request SourceCandidateRegistrationRequest) Validate() error {
	if err := request.RequestID.Validate(); err != nil {
		return fmt.Errorf("source registration request: %w", err)
	}
	if len(request.Candidates) > MaximumDiscoveryCandidatesPerRun {
		return fmt.Errorf("source registration candidates exceed %d", MaximumDiscoveryCandidatesPerRun)
	}
	for index, candidate := range request.Candidates {
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("source registration candidate %d: %w", index, err)
		}
		for _, discovery := range candidate.Candidate.Discoveries {
			if discovery.Query.RequestID != request.RequestID {
				return fmt.Errorf("source registration candidate %d belongs to another request", index)
			}
		}
	}
	return nil
}

type SourceCandidateRegistrationResult struct {
	Sources          []research.Source
	Discoveries      []research.DiscoveredSource
	CreatedCount     int
	ExistingCount    int
	AlgorithmVersion string
}

func (result SourceCandidateRegistrationResult) Validate() error {
	if result.CreatedCount < 0 || result.ExistingCount < 0 || result.CreatedCount+result.ExistingCount != len(result.Sources) {
		return errors.New("source registration counts are inconsistent")
	}
	if len(result.Sources) > MaximumDiscoveryCandidatesPerRun || len(result.Discoveries) > MaximumDiscoveryCandidatesPerRun {
		return errors.New("source registration result exceeds run bounds")
	}
	seenSources := make(map[research.SourceID]struct{}, len(result.Sources))
	for index, source := range result.Sources {
		if err := source.Validate(); err != nil {
			return fmt.Errorf("registered source %d: %w", index, err)
		}
		if _, duplicate := seenSources[source.ID]; duplicate {
			return fmt.Errorf("registered source %q is repeated", source.ID)
		}
		seenSources[source.ID] = struct{}{}
	}
	for index, discovery := range result.Discoveries {
		if err := discovery.Validate(); err != nil {
			return fmt.Errorf("registered discovery %d: %w", index, err)
		}
		if _, exists := seenSources[discovery.SourceID]; !exists {
			return fmt.Errorf("registered discovery %q references an absent source", discovery.ID)
		}
	}
	if result.AlgorithmVersion != SourceCandidateRegistrationV1 {
		return fmt.Errorf("source registration algorithm must be %q", SourceCandidateRegistrationV1)
	}
	return nil
}

type sourceCandidateRegistrationService struct {
	sources     SourceRepository
	discoveries SourceDiscoveryRepository
}

func NewSourceCandidateRegistrationService(sources SourceRepository, discoveries SourceDiscoveryRepository) (SourceCandidateRegistrationService, error) {
	const operation = "configure source candidate registration"
	if err := requireDependency(operation, "source repository", sources); err != nil {
		return nil, err
	}
	if err := requireDependency(operation, "source discovery repository", discoveries); err != nil {
		return nil, err
	}
	return &sourceCandidateRegistrationService{sources: sources, discoveries: discoveries}, nil
}

func (service *sourceCandidateRegistrationService) Register(ctx context.Context, request SourceCandidateRegistrationRequest) (SourceCandidateRegistrationResult, error) {
	const operation = "register source candidates"
	if ctx == nil {
		return SourceCandidateRegistrationResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return SourceCandidateRegistrationResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.Validate(); err != nil {
		return SourceCandidateRegistrationResult{}, invalid(operation, err)
	}
	result := SourceCandidateRegistrationResult{AlgorithmVersion: SourceCandidateRegistrationV1}
	for index, candidate := range request.Candidates {
		if err := ctx.Err(); err != nil {
			return SourceCandidateRegistrationResult{}, Classify(ErrorUnavailable, operation, err)
		}
		source, created, err := service.resolveSource(ctx, candidate)
		if err != nil {
			return SourceCandidateRegistrationResult{}, boundaryError(ErrorPersistenceFailure, operation, fmt.Errorf("candidate %d: %w", index, err))
		}
		if created {
			result.CreatedCount++
		} else {
			result.ExistingCount++
		}
		result.Sources = append(result.Sources, source)
		for _, observation := range candidate.Candidate.Discoveries {
			discovery := discoveredSourceV1(source, observation)
			if err := service.discoveries.Append(ctx, discovery); err != nil {
				if !errors.Is(err, ErrConflict) {
					return SourceCandidateRegistrationResult{}, boundaryError(ErrorPersistenceFailure, operation, err)
				}
				persisted, getErr := service.discoveries.Get(ctx, discovery.ID)
				if getErr != nil || !sameDiscoveredSource(persisted, discovery) {
					return SourceCandidateRegistrationResult{}, boundaryError(ErrorPersistenceFailure, operation, errors.Join(err, getErr))
				}
			}
			result.Discoveries = append(result.Discoveries, cloneDiscoveredSource(discovery))
		}
	}
	if err := result.Validate(); err != nil {
		return SourceCandidateRegistrationResult{}, invalid(operation, err)
	}
	return cloneSourceCandidateRegistrationResult(result), nil
}

func (service *sourceCandidateRegistrationService) resolveSource(ctx context.Context, candidate DeduplicatedSourceCandidate) (research.Source, bool, error) {
	if candidate.ExistingSourceID != nil {
		source, err := service.sources.Get(ctx, *candidate.ExistingSourceID)
		if err != nil {
			return research.Source{}, false, err
		}
		if source.Locator != candidate.Candidate.Locator {
			return research.Source{}, false, errors.New("existing source locator changed after deduplication")
		}
		return source, false, nil
	}
	if existing, err := service.sources.FindByLocator(ctx, candidate.Candidate.Locator); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, ErrNotFound) {
		return research.Source{}, false, err
	}
	first := candidate.Candidate.Discoveries[0]
	source := research.Source{
		ID: stableSourceIDV1(candidate.Candidate.Locator), Kind: research.SourceOther,
		Locator: candidate.Candidate.Locator, TemporalScope: research.SourceTemporalCurrent,
		Metadata: research.SourceMetadata{Title: first.Title}, CreatedAt: earliestDiscovery(candidate.Candidate.Discoveries),
	}
	if err := service.sources.Create(ctx, source); err != nil {
		if !errors.Is(err, ErrConflict) {
			return research.Source{}, false, err
		}
		existing, findErr := service.sources.FindByLocator(ctx, candidate.Candidate.Locator)
		if findErr != nil {
			return research.Source{}, false, errors.Join(err, findErr)
		}
		return existing, false, nil
	}
	return source, true, nil
}

func discoveredSourceV1(source research.Source, observation SourceCandidateDiscovery) research.DiscoveredSource {
	return research.DiscoveredSource{
		ID: stableDiscoveryIDV1(source.ID, observation), RequestID: observation.Query.RequestID, SourceID: source.ID,
		Locator: source.Locator, Query: observation.Query.Text, Title: observation.Title, Snippet: observation.Snippet,
		Provider: observation.Provider, Rank: observation.Rank, DiscoveredAt: observation.DiscoveredAt,
		PublishedHint: cloneTimestamp(observation.PublishedHint), CacheHit: observation.CacheHit, CacheStale: observation.CacheStale,
	}
}

func stableSourceIDV1(locator research.SourceLocator) research.SourceID {
	hash := sha256.Sum256([]byte(locator.String()))
	id, err := research.NewSourceID("source.live." + hex.EncodeToString(hash[:16]))
	if err != nil {
		panic(err)
	}
	return id
}

func stableDiscoveryIDV1(sourceID research.SourceID, observation SourceCandidateDiscovery) research.ID {
	parts := []string{sourceID.String(), observation.Query.RequestID.String(), observation.Query.Text, observation.Title,
		observation.Snippet, observation.Provider, fmt.Sprint(observation.Rank), observation.DiscoveredAt.Time().Format("2006-01-02T15:04:05.999999999Z07:00")}
	if observation.PublishedHint != nil {
		parts = append(parts, observation.PublishedHint.Time().Format("2006-01-02T15:04:05.999999999Z07:00"))
	}
	parts = append(parts, fmt.Sprint(observation.CacheHit), fmt.Sprint(observation.CacheStale))
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	id, err := research.NewID("discovery.live." + hex.EncodeToString(hash[:16]))
	if err != nil {
		panic(err)
	}
	return id
}

func earliestDiscovery(discoveries []SourceCandidateDiscovery) research.Timestamp {
	earliest := discoveries[0].DiscoveredAt
	for _, discovery := range discoveries[1:] {
		if discovery.DiscoveredAt.Before(earliest) {
			earliest = discovery.DiscoveredAt
		}
	}
	return earliest
}

func sameDiscoveredSource(left, right research.DiscoveredSource) bool {
	if left.ID != right.ID || left.RequestID != right.RequestID || left.SourceID != right.SourceID || left.Locator != right.Locator ||
		left.Query != right.Query || left.Title != right.Title || left.Snippet != right.Snippet || left.Provider != right.Provider ||
		left.Rank != right.Rank || !left.DiscoveredAt.Time().Equal(right.DiscoveredAt.Time()) || left.CacheHit != right.CacheHit || left.CacheStale != right.CacheStale {
		return false
	}
	if left.PublishedHint == nil || right.PublishedHint == nil {
		return left.PublishedHint == nil && right.PublishedHint == nil
	}
	return left.PublishedHint.Time().Equal(right.PublishedHint.Time())
}

func cloneDiscoveredSource(discovery research.DiscoveredSource) research.DiscoveredSource {
	clone := discovery
	clone.PublishedHint = cloneTimestamp(discovery.PublishedHint)
	return clone
}

func cloneSourceCandidateRegistrationResult(result SourceCandidateRegistrationResult) SourceCandidateRegistrationResult {
	clone := result
	clone.Sources = append([]research.Source(nil), result.Sources...)
	clone.Discoveries = make([]research.DiscoveredSource, len(result.Discoveries))
	for index, discovery := range result.Discoveries {
		clone.Discoveries[index] = cloneDiscoveredSource(discovery)
	}
	return clone
}

// Execute adapts registration to the fixed live-research stage contract.
func (service *sourceCandidateRegistrationService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.Register(ctx, SourceCandidateRegistrationRequest{RequestID: input.Request.ID, Candidates: input.Artifacts.DeduplicatedCandidates})
	if err != nil {
		return LiveResearchArtifacts{}, err
	}
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.Sources = append([]research.Source(nil), result.Sources...)
	artifacts.Discoveries = make([]research.DiscoveredSource, len(result.Discoveries))
	for index, discovery := range result.Discoveries {
		artifacts.Discoveries[index] = cloneDiscoveredSource(discovery)
	}
	return artifacts, nil
}

var (
	_ SourceCandidateRegistrationService = (*sourceCandidateRegistrationService)(nil)
	_ LiveResearchStageService           = (*sourceCandidateRegistrationService)(nil)
)
