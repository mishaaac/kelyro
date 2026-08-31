package application

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

const SourceCandidateMapperV1 = "source-candidate-mapper-v1"

const SourceCandidateDeduplicationV1 = "source-candidate-deduplication-v1"

// SourceCandidateDiscovery is one provider observation that led to a URL.
// Provider snippets, ranks, and publication hints remain untrusted metadata;
// none of these fields are Evidence or an authority decision.
type SourceCandidateDiscovery struct {
	Query         SearchQuery
	Title         string
	Snippet       string
	Provider      string
	Rank          int
	DiscoveredAt  research.Timestamp
	PublishedHint *research.Timestamp
	CacheHit      bool
	CacheStale    bool
	CacheWarning  CacheWarning
}

func (discovery SourceCandidateDiscovery) Validate() error {
	if err := discovery.Query.Validate(); err != nil {
		return err
	}
	if normalizeSearchText(discovery.Query.Text) != discovery.Query.Text {
		return errors.New("source candidate query is not normalized")
	}
	if err := requireText("source candidate title", discovery.Title); err != nil {
		return err
	}
	if err := validateOptionalText("source candidate snippet", discovery.Snippet); err != nil {
		return err
	}
	if err := requireText("source candidate provider", discovery.Provider); err != nil {
		return err
	}
	if normalizeSearchText(discovery.Title) != discovery.Title || normalizeSearchText(discovery.Snippet) != discovery.Snippet ||
		normalizeSearchText(discovery.Provider) != discovery.Provider {
		return errors.New("source candidate provider metadata is not normalized")
	}
	if discovery.Rank < 0 {
		return errors.New("source candidate rank is negative")
	}
	if discovery.PublishedHint != nil {
		if err := discovery.PublishedHint.Validate(); err != nil {
			return fmt.Errorf("source candidate published hint: %w", err)
		}
	}
	if discovery.CacheStale && (!discovery.CacheHit || discovery.CacheWarning != CacheWarningStaleOffline) {
		return errors.New("stale source candidate requires explicit offline warning")
	}
	if !discovery.CacheStale && discovery.CacheWarning != "" {
		return errors.New("fresh source candidate cannot contain a cache warning")
	}
	if err := discovery.DiscoveredAt.Validate(); err != nil {
		return fmt.Errorf("source candidate discovered at: %w", err)
	}
	return nil
}

// SourceCandidate is transient discovery data grouped by a normalized URL.
// It deliberately has no SourceID, SourceKind, trust tier, Evidence, or Claim.
type SourceCandidate struct {
	Locator          research.SourceLocator
	Discoveries      []SourceCandidateDiscovery
	AlgorithmVersion string
}

func (candidate SourceCandidate) Validate() error {
	if err := candidate.Locator.Validate(); err != nil {
		return fmt.Errorf("source candidate locator: %w", err)
	}
	normalized, err := normalizeDiscoveryLocator(candidate.Locator)
	if err != nil {
		return err
	}
	if normalized != candidate.Locator {
		return errors.New("source candidate locator is not normalized")
	}
	if len(candidate.Discoveries) == 0 || len(candidate.Discoveries) > MaximumDiscoveryCandidatesPerRun {
		return fmt.Errorf("source candidate discoveries must contain between 1 and %d entries", MaximumDiscoveryCandidatesPerRun)
	}
	for index, discovery := range candidate.Discoveries {
		if err := discovery.Validate(); err != nil {
			return fmt.Errorf("source candidate discovery %d: %w", index, err)
		}
	}
	if candidate.AlgorithmVersion != SourceCandidateMapperV1 {
		return fmt.Errorf("source candidate algorithm must be %q", SourceCandidateMapperV1)
	}
	return nil
}

type SourceCandidateMappingInput struct {
	Query        SearchQuery
	Results      []SearchResult
	DiscoveredAt research.Timestamp
}

// MapSearchResultsToSourceCandidatesV1 performs a one-to-one, stable-order
// conversion. It intentionally does not deduplicate candidates; Step 18 owns
// merging across queries and the existing source registry.
func MapSearchResultsToSourceCandidatesV1(ctx context.Context, input SourceCandidateMappingInput) ([]SourceCandidate, error) {
	if ctx == nil {
		return nil, errors.New("map source candidates: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	input.Query.Text = normalizeSearchText(input.Query.Text)
	if err := input.Query.Validate(); err != nil {
		return nil, fmt.Errorf("map source candidates: %w", err)
	}
	if err := input.DiscoveredAt.Validate(); err != nil {
		return nil, fmt.Errorf("map source candidates discovered at: %w", err)
	}
	if len(input.Results) > MaximumSearchResults {
		return nil, fmt.Errorf("map source candidates: results exceed %d", MaximumSearchResults)
	}
	candidates := make([]SourceCandidate, 0, len(input.Results))
	for index, result := range input.Results {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result.Title = normalizeSearchText(result.Title)
		result.Snippet = normalizeSearchText(result.Snippet)
		result.Provider = normalizeSearchText(result.Provider)
		locator, err := normalizeDiscoveryLocator(result.Locator)
		if err != nil {
			return nil, fmt.Errorf("map source candidate result %d: %w", index, err)
		}
		result.Locator = locator
		if err := result.Validate(); err != nil {
			return nil, fmt.Errorf("map source candidate result %d: %w", index, err)
		}
		discovery := SourceCandidateDiscovery{
			Query: input.Query, Title: result.Title, Snippet: result.Snippet, Provider: result.Provider,
			Rank: result.Rank, DiscoveredAt: input.DiscoveredAt, PublishedHint: cloneTimestamp(result.PublishedHint),
			CacheHit: result.CacheHit, CacheStale: result.CacheStale, CacheWarning: result.CacheWarning,
		}
		candidate := SourceCandidate{
			Locator: result.Locator, Discoveries: []SourceCandidateDiscovery{discovery}, AlgorithmVersion: SourceCandidateMapperV1,
		}
		if err := candidate.Validate(); err != nil {
			return nil, fmt.Errorf("map source candidate result %d: %w", index, err)
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func cloneSourceCandidate(candidate SourceCandidate) SourceCandidate {
	clone := candidate
	clone.Discoveries = make([]SourceCandidateDiscovery, len(candidate.Discoveries))
	for index, discovery := range candidate.Discoveries {
		clone.Discoveries[index] = discovery
		clone.Discoveries[index].PublishedHint = cloneTimestamp(discovery.PublishedHint)
	}
	return clone
}

func cloneTimestamp(timestamp *research.Timestamp) *research.Timestamp {
	if timestamp == nil {
		return nil
	}
	clone := *timestamp
	return &clone
}

type DeduplicatedSourceCandidate struct {
	Candidate        SourceCandidate
	ExistingSourceID *research.SourceID
}

func (candidate DeduplicatedSourceCandidate) Validate() error {
	if err := candidate.Candidate.Validate(); err != nil {
		return err
	}
	canonical, err := canonicalSourceCandidateLocatorV1(candidate.Candidate.Locator)
	if err != nil {
		return err
	}
	if canonical != candidate.Candidate.Locator {
		return errors.New("deduplicated source candidate locator is not canonical")
	}
	if candidate.ExistingSourceID != nil {
		if err := candidate.ExistingSourceID.Validate(); err != nil {
			return fmt.Errorf("deduplicated candidate existing source: %w", err)
		}
	}
	return nil
}

type SourceCandidateDeduplicationResult struct {
	Candidates       []DeduplicatedSourceCandidate
	InputCount       int
	DuplicateCount   int
	ExistingCount    int
	DiscoveryCount   int
	AlgorithmVersion string
}

func (result SourceCandidateDeduplicationResult) Validate() error {
	if result.InputCount < 0 || result.DuplicateCount < 0 || result.ExistingCount < 0 || result.DiscoveryCount < 0 {
		return errors.New("source candidate deduplication counts are negative")
	}
	if result.InputCount > MaximumDiscoveryCandidatesPerRun || result.DuplicateCount != result.InputCount-len(result.Candidates) ||
		result.ExistingCount > len(result.Candidates) || result.DiscoveryCount > MaximumDiscoveryCandidatesPerRun {
		return errors.New("source candidate deduplication counts are inconsistent")
	}
	seen := make(map[string]struct{}, len(result.Candidates))
	existing := 0
	discoveries := 0
	for index, candidate := range result.Candidates {
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("deduplicated source candidate %d: %w", index, err)
		}
		key := candidate.Candidate.Locator.String()
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("deduplicated source candidates repeat locator %q", key)
		}
		seen[key] = struct{}{}
		discoveries += len(candidate.Candidate.Discoveries)
		observations := make(map[sourceCandidateDiscoveryIdentity]struct{}, len(candidate.Candidate.Discoveries))
		for _, discovery := range candidate.Candidate.Discoveries {
			identity := sourceCandidateDiscoveryKey(discovery)
			if _, duplicate := observations[identity]; duplicate {
				return fmt.Errorf("deduplicated source candidate %q repeats a discovery observation", key)
			}
			observations[identity] = struct{}{}
		}
		if candidate.ExistingSourceID != nil {
			existing++
		}
	}
	if existing != result.ExistingCount || discoveries != result.DiscoveryCount {
		return errors.New("source candidate deduplication summaries are inconsistent")
	}
	if result.AlgorithmVersion != SourceCandidateDeduplicationV1 {
		return fmt.Errorf("source candidate deduplication algorithm must be %q", SourceCandidateDeduplicationV1)
	}
	return nil
}

type sourceCandidateDeduplicationService struct{ sources SourceRepository }

func NewSourceCandidateDeduplicationService(sources SourceRepository) (SourceCandidateDeduplicationService, error) {
	if err := requireDependency("configure source candidate deduplication", "source repository", sources); err != nil {
		return nil, err
	}
	return &sourceCandidateDeduplicationService{sources: sources}, nil
}

func (service *sourceCandidateDeduplicationService) Deduplicate(ctx context.Context, candidates []SourceCandidate) (SourceCandidateDeduplicationResult, error) {
	const operation = "deduplicate source candidates"
	if ctx == nil {
		return SourceCandidateDeduplicationResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return SourceCandidateDeduplicationResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if len(candidates) > MaximumDiscoveryCandidatesPerRun {
		return SourceCandidateDeduplicationResult{}, invalid(operation, fmt.Errorf("candidate count exceeds %d", MaximumDiscoveryCandidatesPerRun))
	}
	if len(candidates) == 0 {
		result := SourceCandidateDeduplicationResult{AlgorithmVersion: SourceCandidateDeduplicationV1}
		return result, nil
	}
	totalDiscoveries := 0
	for index, candidate := range candidates {
		if err := candidate.Validate(); err != nil {
			return SourceCandidateDeduplicationResult{}, invalid(operation, fmt.Errorf("candidate %d: %w", index, err))
		}
		totalDiscoveries += len(candidate.Discoveries)
		if totalDiscoveries > MaximumDiscoveryCandidatesPerRun {
			return SourceCandidateDeduplicationResult{}, invalid(operation, fmt.Errorf("candidate discoveries exceed %d", MaximumDiscoveryCandidatesPerRun))
		}
	}
	existingSources, err := service.sources.List(ctx)
	if err != nil {
		return SourceCandidateDeduplicationResult{}, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	existingByLocator, err := canonicalExistingSourceLocators(existingSources)
	if err != nil {
		return SourceCandidateDeduplicationResult{}, invalid(operation, err)
	}

	result := SourceCandidateDeduplicationResult{InputCount: len(candidates), AlgorithmVersion: SourceCandidateDeduplicationV1}
	positions := make(map[string]int, len(candidates))
	discoveryKeys := make(map[string]map[sourceCandidateDiscoveryIdentity]struct{}, len(candidates))
	for index, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return SourceCandidateDeduplicationResult{}, Classify(ErrorUnavailable, operation, err)
		}
		canonical, err := canonicalSourceCandidateLocatorV1(candidate.Locator)
		if err != nil {
			return SourceCandidateDeduplicationResult{}, invalid(operation, fmt.Errorf("candidate %d: %w", index, err))
		}
		candidate.Locator = canonical
		key := canonical.String()
		position, duplicate := positions[key]
		if !duplicate {
			position = len(result.Candidates)
			positions[key] = position
			discoveryKeys[key] = make(map[sourceCandidateDiscoveryIdentity]struct{}, len(candidate.Discoveries))
			item := DeduplicatedSourceCandidate{Candidate: SourceCandidate{Locator: canonical, AlgorithmVersion: candidate.AlgorithmVersion}}
			if sourceID, exists := existingByLocator[key]; exists {
				id := sourceID
				item.ExistingSourceID = &id
				result.ExistingCount++
			}
			result.Candidates = append(result.Candidates, item)
		}
		item := &result.Candidates[position]
		for _, discovery := range candidate.Discoveries {
			identity := sourceCandidateDiscoveryKey(discovery)
			if _, exists := discoveryKeys[key][identity]; exists {
				continue
			}
			discoveryKeys[key][identity] = struct{}{}
			cloned := discovery
			cloned.PublishedHint = cloneTimestamp(discovery.PublishedHint)
			item.Candidate.Discoveries = append(item.Candidate.Discoveries, cloned)
			result.DiscoveryCount++
		}
	}
	result.DuplicateCount = result.InputCount - len(result.Candidates)
	if err := result.Validate(); err != nil {
		return SourceCandidateDeduplicationResult{}, invalid(operation, err)
	}
	return cloneSourceCandidateDeduplicationResult(result), nil
}

func canonicalExistingSourceLocators(sources []research.Source) (map[string]research.SourceID, error) {
	result := make(map[string]research.SourceID, len(sources))
	for index, source := range sources {
		if err := source.Validate(); err != nil {
			return nil, fmt.Errorf("existing source %d: %w", index, err)
		}
		locator, err := canonicalSourceCandidateLocatorV1(source.Locator)
		if err != nil {
			return nil, fmt.Errorf("existing source %q: %w", source.ID, err)
		}
		key := locator.String()
		if previous, collision := result[key]; collision && previous != source.ID {
			return nil, fmt.Errorf("existing sources %q and %q share canonical locator %q", previous, source.ID, key)
		}
		result[key] = source.ID
	}
	return result, nil
}

func canonicalSourceCandidateLocatorV1(locator research.SourceLocator) (research.SourceLocator, error) {
	normalized, err := normalizeDiscoveryLocator(locator)
	if err != nil {
		return research.SourceLocator{}, err
	}
	parsed, err := url.Parse(normalized.String())
	if err != nil {
		return research.SourceLocator{}, fmt.Errorf("parse candidate locator: %w", err)
	}
	if (parsed.Scheme == "https" && parsed.Port() == "443") || (parsed.Scheme == "http" && parsed.Port() == "80") {
		hostname := parsed.Hostname()
		if strings.Contains(hostname, ":") {
			hostname = "[" + hostname + "]"
		}
		parsed.Host = hostname
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	canonical, err := research.NewSourceLocator(parsed.String())
	if err != nil {
		return research.SourceLocator{}, fmt.Errorf("canonical candidate locator: %w", err)
	}
	return canonical, nil
}

type sourceCandidateDiscoveryIdentity struct {
	requestID, query, title, snippet, provider, discoveredAt, publishedHint string
	rank                                                                    int
	cacheHit, cacheStale                                                    bool
	cacheWarning                                                            CacheWarning
}

func sourceCandidateDiscoveryKey(discovery SourceCandidateDiscovery) sourceCandidateDiscoveryIdentity {
	published := ""
	if discovery.PublishedHint != nil {
		published = discovery.PublishedHint.Time().Format(time.RFC3339Nano)
	}
	return sourceCandidateDiscoveryIdentity{
		requestID: discovery.Query.RequestID.String(), query: discovery.Query.Text, title: discovery.Title,
		snippet: discovery.Snippet, provider: discovery.Provider, rank: discovery.Rank,
		discoveredAt: discovery.DiscoveredAt.Time().Format(time.RFC3339Nano), publishedHint: published,
		cacheHit: discovery.CacheHit, cacheStale: discovery.CacheStale, cacheWarning: discovery.CacheWarning,
	}
}

func cloneSourceCandidateDeduplicationResult(result SourceCandidateDeduplicationResult) SourceCandidateDeduplicationResult {
	clone := result
	clone.Candidates = make([]DeduplicatedSourceCandidate, len(result.Candidates))
	for index, candidate := range result.Candidates {
		clone.Candidates[index].Candidate = cloneSourceCandidate(candidate.Candidate)
		if candidate.ExistingSourceID != nil {
			id := *candidate.ExistingSourceID
			clone.Candidates[index].ExistingSourceID = &id
		}
	}
	return clone
}

var _ SourceCandidateDeduplicationService = (*sourceCandidateDeduplicationService)(nil)
