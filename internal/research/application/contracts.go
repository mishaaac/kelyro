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

// ProvenanceRepository appends immutable claim graphs and returns the latest
// recorded graph for a stable claim identity.
type ProvenanceRepository interface {
	Append(context.Context, research.ProvenanceGraph) error
	LatestByClaim(context.Context, research.ClaimID) (research.ProvenanceGraph, error)
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
	Provenance     ProvenanceRepository
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

// SearchQuery, SearchOptions, and SearchResult keep provider-specific
// request/response types outside the application and domain packages.
type SearchQuery struct {
	RequestID research.ID
	Text      string
}

func (query SearchQuery) Validate() error {
	if err := query.RequestID.Validate(); err != nil {
		return fmt.Errorf("search request: %w", err)
	}
	return requireText("search query", query.Text)
}

type SearchOptions struct {
	DesiredKind   *research.SourceKind
	TargetVersion *research.SourceVersion
	Limit         int
}

func (options SearchOptions) Validate() error {
	if options.DesiredKind != nil {
		if err := options.DesiredKind.Validate(); err != nil {
			return err
		}
	}
	if options.TargetVersion != nil {
		if err := options.TargetVersion.Validate(); err != nil {
			return err
		}
	}
	if options.Limit <= 0 {
		return fmt.Errorf("search result limit must be positive")
	}
	if options.Limit > MaximumSearchResults {
		return fmt.Errorf("search result limit exceeds %d", MaximumSearchResults)
	}
	return nil
}

type SearchResult struct {
	Title         string
	Locator       research.SourceLocator
	Snippet       string
	Provider      string
	Rank          int
	PublishedHint *research.Timestamp
}

const MaximumSearchResults = 100

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
	if result.PublishedHint != nil {
		if err := result.PublishedHint.Validate(); err != nil {
			return fmt.Errorf("search result published hint: %w", err)
		}
	}
	return nil
}

type SearchProvider interface {
	Search(context.Context, SearchQuery, SearchOptions) ([]SearchResult, error)
}

// SearchCache reads previously cached discovery output without network access.
// It is deliberately distinct from SearchProvider so offline fallback cannot
// accidentally invoke a live adapter.
type SearchCache interface {
	SearchCached(context.Context, SearchQuery, SearchOptions) ([]SearchResult, error)
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
	Origin    FetchOrigin
}

type FetchOrigin string

const (
	FetchOriginLive  FetchOrigin = "live"
	FetchOriginCache FetchOrigin = "cache"
)

func (origin FetchOrigin) Validate() error {
	switch origin {
	case FetchOriginLive, FetchOriginCache:
		return nil
	default:
		return fmt.Errorf("invalid fetch origin %q", origin)
	}
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
	if err := source.Origin.Validate(); err != nil {
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

// SourceFetchCache reads a previously fetched source without network access.
type SourceFetchCache interface {
	FetchCached(context.Context, FetchRequest) (FetchedSource, error)
}

type ReleaseLookupQuery struct {
	TechnologyID research.ID
	Channel      research.ReleaseChannel
}

func (query ReleaseLookupQuery) Validate() error {
	if err := query.TechnologyID.Validate(); err != nil {
		return fmt.Errorf("release lookup technology: %w", err)
	}
	return query.Channel.Validate()
}

// ReleaseLookupProvider is the live boundary for release metadata. Results are
// candidates until later evidence and verification steps persist them.
type ReleaseLookupProvider interface {
	LookupReleases(context.Context, ReleaseLookupQuery) ([]research.ReleaseRecord, error)
}

// ReleaseLookupCache reads previously cached release lookup output offline.
type ReleaseLookupCache interface {
	LookupCachedReleases(context.Context, ReleaseLookupQuery) ([]research.ReleaseRecord, error)
}

type NormalizedSource struct {
	SourceID             research.SourceID
	Locator              research.SourceLocator
	ContentType          string
	Title                string
	CanonicalLocator     *research.SourceLocator
	Language             string
	Headings             []NormalizedHeading
	TextSegments         []string
	CodeBlocks           []NormalizedCodeBlock
	Links                []NormalizedLink
	PublishedAt          *research.Timestamp
	UpdatedAt            *research.Timestamp
	VersionHints         []string
	NormalizationVersion string
}

type NormalizedHeading struct {
	Level int
	Text  string
	Path  []string
}

type NormalizedCodeBlock struct {
	Language string
	Content  string
}

type NormalizedLink struct {
	Label   string
	Locator research.SourceLocator
}

const (
	MaximumNormalizedHeadings     = 512
	MaximumNormalizedSegments     = 4096
	MaximumNormalizedCodeBlocks   = 256
	MaximumNormalizedLinks        = 2048
	MaximumNormalizedVersionHints = 64
	MaximumNormalizedTextBytes    = 16 * 1024
	MaximumNormalizedCodeBytes    = 128 * 1024
)

func (source NormalizedSource) Validate() error {
	if err := source.SourceID.Validate(); err != nil {
		return err
	}
	if err := source.Locator.Validate(); err != nil {
		return err
	}
	if err := requireText("normalized source content type", source.ContentType); err != nil {
		return err
	}
	if err := validateOptionalText("normalized source title", source.Title); err != nil {
		return err
	}
	if source.CanonicalLocator != nil {
		if err := source.CanonicalLocator.Validate(); err != nil {
			return fmt.Errorf("normalized canonical locator: %w", err)
		}
	}
	if err := validateOptionalText("normalized source language", source.Language); err != nil {
		return err
	}
	if len(source.Headings) > MaximumNormalizedHeadings {
		return fmt.Errorf("normalized headings exceed %d", MaximumNormalizedHeadings)
	}
	for index, heading := range source.Headings {
		if heading.Level < 1 || heading.Level > 6 {
			return fmt.Errorf("normalized heading %d has invalid level", index)
		}
		if err := boundedNormalizedText("normalized heading", heading.Text, MaximumNormalizedTextBytes, true); err != nil {
			return fmt.Errorf("normalized heading %d: %w", index, err)
		}
		if err := validateTexts("normalized heading path", heading.Path, true); err != nil {
			return fmt.Errorf("normalized heading %d: %w", index, err)
		}
		if heading.Path[len(heading.Path)-1] != heading.Text {
			return fmt.Errorf("normalized heading %d path does not end with heading text", index)
		}
	}
	if len(source.TextSegments) > MaximumNormalizedSegments {
		return fmt.Errorf("normalized text segments exceed %d", MaximumNormalizedSegments)
	}
	if err := validateTexts("normalized source segments", source.TextSegments, true); err != nil {
		return err
	}
	for index, segment := range source.TextSegments {
		if err := boundedNormalizedText("normalized text segment", segment, MaximumNormalizedTextBytes, true); err != nil {
			return fmt.Errorf("normalized text segment %d: %w", index, err)
		}
	}
	if len(source.CodeBlocks) > MaximumNormalizedCodeBlocks {
		return fmt.Errorf("normalized code blocks exceed %d", MaximumNormalizedCodeBlocks)
	}
	for index, block := range source.CodeBlocks {
		if err := validateOptionalText("normalized code language", block.Language); err != nil {
			return fmt.Errorf("normalized code block %d: %w", index, err)
		}
		if err := boundedNormalizedCode(block.Content); err != nil {
			return fmt.Errorf("normalized code block %d: %w", index, err)
		}
	}
	if len(source.Links) > MaximumNormalizedLinks {
		return fmt.Errorf("normalized links exceed %d", MaximumNormalizedLinks)
	}
	for index, link := range source.Links {
		if err := validateOptionalText("normalized link label", link.Label); err != nil {
			return fmt.Errorf("normalized link %d: %w", index, err)
		}
		if err := link.Locator.Validate(); err != nil {
			return fmt.Errorf("normalized link %d: %w", index, err)
		}
	}
	if err := validateOptionalNormalizedTimestamp("normalized published at", source.PublishedAt); err != nil {
		return err
	}
	if err := validateOptionalNormalizedTimestamp("normalized updated at", source.UpdatedAt); err != nil {
		return err
	}
	if source.PublishedAt != nil && source.UpdatedAt != nil && source.UpdatedAt.Before(*source.PublishedAt) {
		return fmt.Errorf("normalized updated at precedes published at")
	}
	if len(source.VersionHints) > MaximumNormalizedVersionHints {
		return fmt.Errorf("normalized version hints exceed %d", MaximumNormalizedVersionHints)
	}
	if err := validateTexts("normalized source version hints", source.VersionHints, false); err != nil {
		return err
	}
	if len(source.Headings) == 0 && len(source.TextSegments) == 0 && len(source.CodeBlocks) == 0 {
		return fmt.Errorf("normalized source has no researchable content")
	}
	return requireText("normalization version", source.NormalizationVersion)
}

func boundedNormalizedText(name, value string, maximum int, required bool) error {
	var err error
	if required {
		err = requireText(name, value)
	} else {
		err = validateOptionalText(name, value)
	}
	if err != nil {
		return err
	}
	if len(value) > maximum {
		return fmt.Errorf("%s exceeds %d bytes", name, maximum)
	}
	return nil
}

func boundedNormalizedCode(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("normalized code content is empty")
	}
	if len(value) > MaximumNormalizedCodeBytes {
		return fmt.Errorf("normalized code content exceeds %d bytes", MaximumNormalizedCodeBytes)
	}
	return nil
}

func validateOptionalNormalizedTimestamp(name string, value *research.Timestamp) error {
	if value == nil {
		return nil
	}
	if err := value.Validate(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
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
	Search(context.Context, ResearchMode, SearchQuery, SearchOptions) ([]SearchResult, error)
}

type FetchService interface {
	Fetch(context.Context, ResearchMode, FetchRequest) (FetchedSource, error)
}

type SnapshotBodyPolicy string

const (
	SnapshotMetadataOnly      SnapshotBodyPolicy = "metadata_only"
	SnapshotNormalizedExcerpt SnapshotBodyPolicy = "normalized_excerpt"
	SnapshotBoundedCachedBody SnapshotBodyPolicy = "bounded_cached_body"
)

func (policy SnapshotBodyPolicy) Validate() error {
	switch policy {
	case SnapshotMetadataOnly, SnapshotNormalizedExcerpt, SnapshotBoundedCachedBody:
		return nil
	default:
		return fmt.Errorf("invalid snapshot body policy %q", policy)
	}
}

const MaximumCachedSourceBodyBytes = 1 << 20

type SnapshotCaptureRequest struct {
	SourceID     research.SourceID
	MaximumBytes int64
	BodyPolicy   SnapshotBodyPolicy
}

func (request SnapshotCaptureRequest) Validate() error {
	if err := request.SourceID.Validate(); err != nil {
		return err
	}
	if request.MaximumBytes <= 0 {
		return fmt.Errorf("snapshot maximum bytes must be positive")
	}
	if err := request.BodyPolicy.Validate(); err != nil {
		return err
	}
	if request.BodyPolicy == SnapshotBoundedCachedBody && request.MaximumBytes > MaximumCachedSourceBodyBytes {
		return fmt.Errorf("cached source body maximum exceeds %d bytes", MaximumCachedSourceBodyBytes)
	}
	return nil
}

// SnapshotCapture keeps raw content outside SourceSnapshot. NormalizationInput
// is transient input for Step 10; CacheCandidate is bounded input for the
// future cache layer. Neither is evidence or persisted snapshot history.
type SnapshotCapture struct {
	Snapshot              research.SourceSnapshot
	RevalidatedSnapshotID *research.ID
	NormalizationInput    *FetchedSource
	CacheCandidate        []byte
}

type SnapshotCaptureService interface {
	Capture(context.Context, ResearchMode, SnapshotCaptureRequest) (SnapshotCapture, error)
}

type ReleaseLookupService interface {
	Lookup(context.Context, ResearchMode, ReleaseLookupQuery) ([]research.ReleaseRecord, error)
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

type ProvenanceService interface {
	Record(context.Context, research.ProvenanceGraph) error
	Trace(context.Context, research.ClaimID) (research.ProvenanceGraph, error)
	Export(context.Context, research.ClaimID) ([]byte, error)
}

// SourceRegistryStore scopes read-only source registry/provenance commands and
// the underlying workspace database lifetime without exposing SQLite.
type SourceRegistryStore interface {
	Registry() SourceRegistryService
	Provenance() ProvenanceService
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
