package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

// Persistence ports are intentionally split by aggregate/use case.
type SourceRepository interface {
	Create(context.Context, research.Source) error
	Get(context.Context, research.SourceID) (research.Source, error)
	FindByLocator(context.Context, research.SourceLocator) (research.Source, error)
	List(context.Context) ([]research.Source, error)
}

type SnapshotRepository interface {
	Append(context.Context, research.SourceSnapshot) error
	Get(context.Context, research.ID) (research.SourceSnapshot, error)
	LatestBySource(context.Context, research.SourceID) (research.SourceSnapshot, error)
	ListBySource(context.Context, research.SourceID) ([]research.SourceSnapshot, error)
}

type EvidenceRepository interface {
	Append(context.Context, research.Evidence) error
	Get(context.Context, research.ID) (research.Evidence, error)
	ListBySource(context.Context, research.SourceID) ([]research.Evidence, error)
	ListBySnapshot(context.Context, research.ID) ([]research.Evidence, error)
}

// ResearchRunRepository owns a request and its runs as one persistence
// aggregate; it does not perform discovery or network calls.
type ResearchRunRepository interface {
	Create(context.Context, research.ResearchRequest, research.ResearchRun) error
	GetRequest(context.Context, research.ID) (research.ResearchRequest, error)
	GetRun(context.Context, research.ID) (research.ResearchRun, error)
	UpdateRun(context.Context, research.ResearchRun) error
}

type TrustRegistryRepository interface {
	SaveProfile(context.Context, research.AuthorityProfile) error
	GetProfile(context.Context, research.ID) (research.AuthorityProfile, error)
	ListProfiles(context.Context) ([]research.AuthorityProfile, error)
	SaveDecision(context.Context, research.TrustDecision) error
	LatestDecision(context.Context, research.SourceID) (research.TrustDecision, error)
}

type SourceRegistryRepository interface {
	Save(context.Context, research.SourceRegistryEntry) error
	Get(context.Context, research.ID) (research.SourceRegistryEntry, error)
	List(context.Context) ([]research.SourceRegistryEntry, error)
}

type ReleaseRepository interface {
	Create(context.Context, research.ReleaseRecord) error
	Get(context.Context, research.ID) (research.ReleaseRecord, error)
	ListByTechnology(context.Context, research.ID) ([]research.ReleaseRecord, error)
}

// FreshnessRecord is stored policy output. The dedicated freshness step owns
// the future scoring formula; this boundary only preserves its versioned result.
type FreshnessRecord struct {
	SubjectID        research.ID
	State            research.FreshnessState
	Score            research.FreshnessScore
	LastVerifiedAt   research.Timestamp
	NextVerifyAt     *research.Timestamp
	AlgorithmVersion string
}

func (record FreshnessRecord) Validate() error {
	if err := record.SubjectID.Validate(); err != nil {
		return fmt.Errorf("freshness subject: %w", err)
	}
	if err := record.State.Validate(); err != nil {
		return err
	}
	if err := record.Score.Validate(); err != nil {
		return fmt.Errorf("freshness score: %w", err)
	}
	if err := record.LastVerifiedAt.Validate(); err != nil {
		return fmt.Errorf("freshness last verified: %w", err)
	}
	if record.NextVerifyAt != nil {
		if err := record.NextVerifyAt.Validate(); err != nil {
			return fmt.Errorf("freshness next verify: %w", err)
		}
		if record.NextVerifyAt.Before(record.LastVerifiedAt) {
			return fmt.Errorf("freshness next verification precedes last verification")
		}
	}
	return requireText("freshness algorithm version", record.AlgorithmVersion)
}

type FreshnessRepository interface {
	Save(context.Context, FreshnessRecord) error
	Get(context.Context, research.ID) (FreshnessRecord, error)
	ListDue(context.Context, research.Timestamp) ([]FreshnessRecord, error)
}

type VerificationRepository interface {
	Append(context.Context, research.VerificationResult) error
	Get(context.Context, research.ID) (research.VerificationResult, error)
	LatestByClaim(context.Context, research.ClaimID) (research.VerificationResult, error)
}

type DriftRepository interface {
	Append(context.Context, research.DriftReport) error
	Get(context.Context, research.ID) (research.DriftReport, error)
}

type ImpactRepository interface {
	Append(context.Context, research.ImpactReport) error
	Get(context.Context, research.ID) (research.ImpactReport, error)
}

// CacheEntry is opaque adapter data with bounded ownership at the port. The
// cache is never the source of truth for persisted evidence or snapshots.
type CacheEntry struct {
	Key         string
	Payload     []byte
	ContentHash string
	StoredAt    research.Timestamp
	ExpiresAt   *research.Timestamp
}

func (entry CacheEntry) Validate() error {
	if err := requireText("cache key", entry.Key); err != nil {
		return err
	}
	if len(entry.Payload) == 0 {
		return fmt.Errorf("cache payload is empty")
	}
	if err := requireText("cache content hash", entry.ContentHash); err != nil {
		return err
	}
	if err := entry.StoredAt.Validate(); err != nil {
		return fmt.Errorf("cache stored at: %w", err)
	}
	if entry.ExpiresAt != nil {
		if err := entry.ExpiresAt.Validate(); err != nil {
			return fmt.Errorf("cache expires at: %w", err)
		}
		if entry.ExpiresAt.Before(entry.StoredAt) {
			return fmt.Errorf("cache expiry precedes storage")
		}
	}
	return nil
}

type ResearchCacheRepository interface {
	Put(context.Context, CacheEntry) error
	Get(context.Context, string) (CacheEntry, error)
	Delete(context.Context, string) error
}

// Repositories is a wiring bundle, not a repository and not a transaction.
// Consumers continue to depend on the narrow ports relevant to each use case.
type Repositories struct {
	Sources        SourceRepository
	Snapshots      SnapshotRepository
	Evidence       EvidenceRepository
	Runs           ResearchRunRepository
	TrustRegistry  TrustRegistryRepository
	SourceRegistry SourceRegistryRepository
	Releases       ReleaseRepository
	Freshness      FreshnessRepository
	Verification   VerificationRepository
	Drift          DriftRepository
	Impact         ImpactRepository
	Cache          ResearchCacheRepository
}

// SearchQuery and SearchResult keep provider-specific request/response types
// outside the application and domain packages.
type SearchQuery struct {
	RequestID     research.ID
	Text          string
	DesiredKind   *research.SourceKind
	TargetVersion *research.SourceVersion
	Limit         int
}

func (query SearchQuery) Validate() error {
	if err := query.RequestID.Validate(); err != nil {
		return fmt.Errorf("search request: %w", err)
	}
	if err := requireText("search query", query.Text); err != nil {
		return err
	}
	if query.DesiredKind != nil {
		if err := query.DesiredKind.Validate(); err != nil {
			return err
		}
	}
	if query.TargetVersion != nil {
		if err := query.TargetVersion.Validate(); err != nil {
			return err
		}
	}
	if query.Limit <= 0 {
		return fmt.Errorf("search result limit must be positive")
	}
	return nil
}

type SearchResult struct {
	Title    string
	Locator  research.SourceLocator
	Snippet  string
	Provider string
	Rank     int
}

func (result SearchResult) Validate() error {
	if err := requireText("search result title", result.Title); err != nil {
		return err
	}
	if err := result.Locator.Validate(); err != nil {
		return err
	}
	if err := validateOptionalText("search result snippet", result.Snippet); err != nil {
		return err
	}
	if err := requireText("search result provider", result.Provider); err != nil {
		return err
	}
	if result.Rank < 0 {
		return fmt.Errorf("search result rank is negative")
	}
	return nil
}

type SearchProvider interface {
	Search(context.Context, SearchQuery) ([]SearchResult, error)
}

type FetchRequest struct {
	SourceID     research.SourceID
	Locator      research.SourceLocator
	ETag         string
	LastModified string
	MaximumBytes int64
}

func (request FetchRequest) Validate() error {
	if err := request.SourceID.Validate(); err != nil {
		return err
	}
	if err := request.Locator.Validate(); err != nil {
		return err
	}
	if err := validateOptionalText("fetch etag", request.ETag); err != nil {
		return err
	}
	if err := validateOptionalText("fetch last modified", request.LastModified); err != nil {
		return err
	}
	if request.MaximumBytes <= 0 {
		return fmt.Errorf("fetch maximum bytes must be positive")
	}
	return nil
}

type FetchedSource struct {
	SourceID  research.SourceID
	Locator   research.SourceLocator
	FetchedAt research.Timestamp
	Metadata  research.FetchMetadata
	Body      []byte
}

func (source FetchedSource) Validate() error {
	if err := source.SourceID.Validate(); err != nil {
		return err
	}
	if err := source.Locator.Validate(); err != nil {
		return err
	}
	if err := source.FetchedAt.Validate(); err != nil {
		return fmt.Errorf("fetched source timestamp: %w", err)
	}
	if err := source.Metadata.Validate(); err != nil {
		return err
	}
	if source.Metadata.StatusCode != 204 && source.Metadata.StatusCode != 304 && len(source.Body) == 0 {
		return fmt.Errorf("fetched source body is empty")
	}
	return nil
}

type SourceFetcher interface {
	Fetch(context.Context, FetchRequest) (FetchedSource, error)
}

type NormalizedSource struct {
	SourceID     research.SourceID
	Locator      research.SourceLocator
	Title        string
	Language     string
	TextSegments []string
	VersionHints []string
}

func (source NormalizedSource) Validate() error {
	if err := source.SourceID.Validate(); err != nil {
		return err
	}
	if err := source.Locator.Validate(); err != nil {
		return err
	}
	if err := validateOptionalText("normalized source title", source.Title); err != nil {
		return err
	}
	if err := validateOptionalText("normalized source language", source.Language); err != nil {
		return err
	}
	if err := validateTexts("normalized source segments", source.TextSegments, true); err != nil {
		return err
	}
	return validateTexts("normalized source version hints", source.VersionHints, false)
}

type SourceNormalizer interface {
	Normalize(context.Context, FetchedSource) (NormalizedSource, error)
}

type MetadataExtractor interface {
	Extract(context.Context, NormalizedSource) (research.SourceMetadata, error)
}

type Clock interface {
	Now() research.Timestamp
}

// Application service contracts expose use cases without leaking repositories.
type ResearchService interface {
	Start(context.Context, research.ResearchRequest, research.ResearchRun) error
	Run(context.Context, research.ID) (research.ResearchRun, error)
	UpdateRun(context.Context, research.ResearchRun) error
}

type DiscoveryService interface {
	Search(context.Context, SearchQuery) ([]SearchResult, error)
}

type SourceService interface {
	Register(context.Context, research.Source) error
	Get(context.Context, research.SourceID) (research.Source, error)
	List(context.Context) ([]research.Source, error)
	RecordSnapshot(context.Context, research.SourceSnapshot) error
	LatestSnapshot(context.Context, research.SourceID) (research.SourceSnapshot, error)
}

type SourceRegistryService interface {
	Save(context.Context, research.SourceRegistryEntry) error
	Get(context.Context, research.ID) (research.SourceRegistryEntry, error)
	List(context.Context) ([]research.SourceRegistryEntry, error)
}

// SourceRegistryStore scopes registry queries and the underlying workspace
// database lifetime without exposing SQLite to application or presentation.
type SourceRegistryStore interface {
	Registry() SourceRegistryService
	Close() error
}

type SourceRegistryStoreFactory interface {
	Open(context.Context, string) (SourceRegistryStore, error)
}

type VerificationService interface {
	Record(context.Context, research.VerificationResult) error
	Latest(context.Context, research.ClaimID) (research.VerificationResult, error)
}

type FreshnessService interface {
	Save(context.Context, FreshnessRecord) error
	Get(context.Context, research.ID) (FreshnessRecord, error)
	Due(context.Context, research.Timestamp) ([]FreshnessRecord, error)
}

type ReleaseIntelligenceService interface {
	Record(context.Context, research.ReleaseRecord) error
	Get(context.Context, research.ID) (research.ReleaseRecord, error)
	List(context.Context, research.ID) ([]research.ReleaseRecord, error)
}

type DriftService interface {
	Record(context.Context, research.DriftReport) error
	Get(context.Context, research.ID) (research.DriftReport, error)
}

type ImpactService interface {
	Record(context.Context, research.ImpactReport) error
	Get(context.Context, research.ID) (research.ImpactReport, error)
}

func requireText(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is empty", name)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s has surrounding whitespace", name)
	}
	return nil
}

func validateOptionalText(name, value string) error {
	if value == "" {
		return nil
	}
	return requireText(name, value)
}

func validateTexts(name string, values []string, required bool) error {
	if required && len(values) == 0 {
		return fmt.Errorf("%s are empty", name)
	}
	for _, value := range values {
		if err := requireText(name, value); err != nil {
			return err
		}
	}
	return nil
}
