package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

const SourceCandidateMapperV1 = "source-candidate-mapper-v1"

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
