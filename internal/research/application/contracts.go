package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/citation"
	conflictpolicy "github.com/mishaaac/kelyro/internal/research/conflict"
)

// Persistence ports are intentionally split by aggregate/use case.
type SourceRepository interface {
	Create(context.Context, research.Source) error
	Get(context.Context, research.SourceID) (research.Source, error)
	FindByLocator(context.Context, research.SourceLocator) (research.Source, error)
	List(context.Context) ([]research.Source, error)
	SetTemporalScope(context.Context, research.SourceID, research.SourceTemporalScope) error
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

// ClaimRepository persists structured assertions separately from the evidence
// excerpts that support them.
type ClaimRepository interface {
	Append(context.Context, research.Claim) error
	Get(context.Context, research.ClaimID) (research.Claim, error)
}

type CitationRepository interface {
	Append(context.Context, research.Citation) error
	Get(context.Context, research.ID) (research.Citation, error)
	ListByEvidence(context.Context, research.ID) ([]research.Citation, error)
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
	Update(context.Context, research.ReleaseRecord) error
	Get(context.Context, research.ID) (research.ReleaseRecord, error)
	ListByTechnology(context.Context, research.ID) ([]research.ReleaseRecord, error)
}

// ReleaseIngestionBatch is the durable portion of one fully validated release
// discovery. StatusUpdates may only alter lifecycle status of existing rows.
type ReleaseIngestionBatch struct {
	Evidence      []research.Evidence
	Claims        []research.Claim
	Releases      []research.TechnologyRelease
	StatusUpdates []research.TechnologyRelease
}

type ReleaseIngestionRepository interface {
	Commit(context.Context, ReleaseIngestionBatch) error
}

// DeprecationRepository is append-only so later removed/legacy conclusions do
// not erase the guidance that applied to earlier versions.
type DeprecationRepository interface {
	Append(context.Context, research.DeprecationRecord) error
	Get(context.Context, research.ID) (research.DeprecationRecord, error)
	ListBySubject(context.Context, string) ([]research.DeprecationRecord, error)
}

// FreshnessRecord persists freshness-v1 output and, when scheduled, the
// independently versioned refresh-scheduling-v1 metadata.
type FreshnessRecord struct {
	SubjectID                  research.ID
	State                      research.FreshnessState
	Score                      research.FreshnessScore
	LastVerifiedAt             research.Timestamp
	NextVerifyAt               *research.Timestamp
	VerificationReason         research.VerificationReason
	Priority                   research.VerificationPriority
	AlgorithmVersion           string
	SchedulingAlgorithmVersion string
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
		if err := record.VerificationReason.Validate(); err != nil {
			return err
		}
		if err := record.Priority.Validate(); err != nil {
			return err
		}
		if record.SchedulingAlgorithmVersion != research.RefreshSchedulingAlgorithmV1 {
			return fmt.Errorf("freshness scheduling algorithm version must be %q", research.RefreshSchedulingAlgorithmV1)
		}
	} else if record.VerificationReason != "" || record.Priority != "" || record.SchedulingAlgorithmVersion != "" {
		return fmt.Errorf("unscheduled freshness cannot contain scheduling metadata")
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

// ConflictRepository is append-only: a later reassessment must not erase the
// conflict decision that was visible to an earlier research run.
type ConflictRepository interface {
	Append(context.Context, research.Conflict) error
	Get(context.Context, research.ID) (research.Conflict, error)
	ListByClaim(context.Context, research.ClaimID) ([]research.Conflict, error)
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
	Sources          SourceRepository
	Snapshots        SnapshotRepository
	Evidence         EvidenceRepository
	Claims           ClaimRepository
	Citations        CitationRepository
	Provenance       ProvenanceRepository
	Runs             ResearchRunRepository
	TrustRegistry    TrustRegistryRepository
	SourceRegistry   SourceRegistryRepository
	Releases         ReleaseRepository
	ReleaseIngestion ReleaseIngestionRepository
	Deprecations     DeprecationRepository
	Freshness        FreshnessRepository
	Verification     VerificationRepository
	Conflicts        ConflictRepository
	Drift            DriftRepository
	Impact           ImpactRepository
	Cache            ResearchCacheRepository
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

const (
	ReleaseDiscoveryAlgorithmV1       = "release-discovery-v1"
	ReleaseNotesIngestionAlgorithmV1  = "release-notes-ingestion-v1"
	MaximumReleaseDiscoverySources    = 16
	MaximumReleaseCandidates          = 256
	MaximumReleaseChangesPerCandidate = 256
	MaximumReleaseFeedBytes           = 1 << 20
)

// ReleaseDiscoverySource binds an already registered and trust-evaluated
// source to the adapter that understands its feed representation.
type ReleaseDiscoverySource struct {
	SourceID research.SourceID
	Provider string
}

func (source ReleaseDiscoverySource) Validate() error {
	if err := source.SourceID.Validate(); err != nil {
		return err
	}
	return requireText("release provider", source.Provider)
}

// ReleaseChange is a bounded, literal observation extracted from release
// notes. Statement is never interpreted as an instruction.
type ReleaseChange struct {
	Location  string
	Statement string
	Excerpt   string
}

func (change ReleaseChange) Validate() error {
	if err := requireText("release change location", change.Location); err != nil {
		return err
	}
	if err := requireText("release change statement", change.Statement); err != nil {
		return err
	}
	if err := requireText("release change excerpt", change.Excerpt); err != nil {
		return err
	}
	if len(change.Excerpt) > research.MaximumEvidenceExcerptBytes {
		return fmt.Errorf("release change excerpt exceeds %d bytes", research.MaximumEvidenceExcerptBytes)
	}
	return nil
}

// ReleaseCandidate is unpersisted provider output. A provider reports the
// channel explicitly; the discovery policy never guesses stable from rank.
type ReleaseCandidate struct {
	Version    research.VersionIdentifier
	Channel    research.ReleaseChannel
	ReleasedAt *research.Timestamp
	Changes    []ReleaseChange
}

func (candidate ReleaseCandidate) Validate() error {
	if err := candidate.Version.Validate(); err != nil {
		return err
	}
	if err := candidate.Channel.Validate(); err != nil {
		return err
	}
	if candidate.ReleasedAt != nil {
		if err := candidate.ReleasedAt.Validate(); err != nil {
			return fmt.Errorf("release candidate date: %w", err)
		}
	}
	if len(candidate.Changes) > MaximumReleaseChangesPerCandidate {
		return fmt.Errorf("release candidate changes exceed %d", MaximumReleaseChangesPerCandidate)
	}
	for index, change := range candidate.Changes {
		if err := change.Validate(); err != nil {
			return fmt.Errorf("release candidate change %d: %w", index, err)
		}
	}
	return nil
}

// ReleaseNotesProvider parses one bounded fetched representation. It is a
// network-free adapter; live access remains owned by SnapshotCaptureService.
type ReleaseNotesProvider interface {
	ID() string
	Discover(context.Context, FetchedSource) ([]ReleaseCandidate, error)
}

type ReleaseDiscoveryRequest struct {
	TechnologyID research.ID
	Topic        research.ResearchTopic
	Profile      research.AuthorityProfile
	Sources      []ReleaseDiscoverySource
	MaximumBytes int64
}

func (request ReleaseDiscoveryRequest) Validate() error {
	if err := request.TechnologyID.Validate(); err != nil {
		return fmt.Errorf("release discovery technology: %w", err)
	}
	if err := request.Topic.Validate(); err != nil {
		return err
	}
	if err := request.Profile.Validate(); err != nil {
		return err
	}
	if len(request.Sources) == 0 || len(request.Sources) > MaximumReleaseDiscoverySources {
		return fmt.Errorf("release discovery sources must contain between 1 and %d entries", MaximumReleaseDiscoverySources)
	}
	seen := make(map[research.SourceID]struct{}, len(request.Sources))
	for index, source := range request.Sources {
		if err := source.Validate(); err != nil {
			return fmt.Errorf("release discovery source %d: %w", index, err)
		}
		if _, exists := seen[source.SourceID]; exists {
			return fmt.Errorf("release discovery contains duplicate source %q", source.SourceID.String())
		}
		seen[source.SourceID] = struct{}{}
	}
	if request.MaximumBytes <= 0 || request.MaximumBytes > MaximumReleaseFeedBytes {
		return fmt.Errorf("release feed maximum bytes must be between 1 and %d", MaximumReleaseFeedBytes)
	}
	return nil
}

type ReleaseDiscoveryResult struct {
	Releases         []research.TechnologyRelease
	Evidence         []research.Evidence
	Claims           []research.Claim
	CurrentStable    *research.TechnologyRelease
	PreviewReleases  []research.TechnologyRelease
	DuplicateCount   int
	AlgorithmVersion string
}

type ReleaseDiscoveryService interface {
	Discover(context.Context, ResearchMode, ReleaseDiscoveryRequest) (ReleaseDiscoveryResult, error)
}

const (
	MinimumDeprecationStrongInferenceConfidence = 0.8
	MaximumDeprecationSignals                   = 32
)

// DeprecationSignalKind is deliberately closed: absence from documentation is
// not a signal and cannot be represented as valid input.
type DeprecationSignalKind string

const (
	DeprecationSignalExplicitStatement DeprecationSignalKind = "explicit_statement"
	DeprecationSignalStrongInference   DeprecationSignalKind = "strong_inference"
)

func (kind DeprecationSignalKind) Validate() error {
	switch kind {
	case DeprecationSignalExplicitStatement, DeprecationSignalStrongInference:
		return nil
	default:
		return fmt.Errorf("invalid deprecation signal kind %q", kind)
	}
}

// DeprecationSignal references one structured deprecation claim and one of its
// evidence excerpts. The application service resolves and validates the full
// claim -> evidence -> source relationship before recording a conclusion.
type DeprecationSignal struct {
	Kind         DeprecationSignalKind
	ClaimID      research.ClaimID
	EvidenceID   research.ID
	SourceID     research.SourceID
	Status       research.DeprecationStatus
	IntroducedIn *research.SourceVersion
	DeprecatedIn *research.SourceVersion
	RemovedIn    *research.SourceVersion
	Replacement  string
}

func (signal DeprecationSignal) Validate() error {
	if err := signal.Kind.Validate(); err != nil {
		return err
	}
	if err := signal.ClaimID.Validate(); err != nil {
		return err
	}
	if err := signal.EvidenceID.Validate(); err != nil {
		return fmt.Errorf("deprecation signal evidence: %w", err)
	}
	if err := signal.SourceID.Validate(); err != nil {
		return err
	}
	return validateDeprecationConclusion(signal.Status, signal.IntroducedIn, signal.DeprecatedIn, signal.RemovedIn, signal.Replacement)
}

type DeprecationAssessmentRequest struct {
	Subject string
	Signals []DeprecationSignal
}

func (request DeprecationAssessmentRequest) Validate() error {
	if err := requireText("deprecation subject", request.Subject); err != nil {
		return err
	}
	if len(request.Signals) == 0 || len(request.Signals) > MaximumDeprecationSignals {
		return fmt.Errorf("deprecation signals must contain between 1 and %d entries", MaximumDeprecationSignals)
	}
	seen := make(map[string]struct{}, len(request.Signals))
	var kind DeprecationSignalKind
	for index, signal := range request.Signals {
		if err := signal.Validate(); err != nil {
			return fmt.Errorf("deprecation signal %d: %w", index, err)
		}
		if index == 0 {
			kind = signal.Kind
		} else if signal.Kind != kind {
			return fmt.Errorf("deprecation assessment cannot mix explicit and inferred signals")
		}
		if index > 0 && !sameDeprecationConclusion(request.Signals[0], signal) {
			return fmt.Errorf("deprecation signals disagree on status, versions, or replacement")
		}
		key := signal.ClaimID.String() + "\x00" + signal.EvidenceID.String() + "\x00" + signal.SourceID.String()
		if _, exists := seen[key]; exists {
			return fmt.Errorf("deprecation assessment contains a duplicate signal")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validateDeprecationConclusion(
	status research.DeprecationStatus,
	introducedIn, deprecatedIn, removedIn *research.SourceVersion,
	replacement string,
) error {
	if err := status.Validate(); err != nil {
		return err
	}
	for _, item := range []struct {
		name    string
		version *research.SourceVersion
	}{
		{"introduced version", introducedIn},
		{"deprecated version", deprecatedIn},
		{"removed version", removedIn},
	} {
		if item.version != nil {
			if err := item.version.Validate(); err != nil {
				return fmt.Errorf("%s: %w", item.name, err)
			}
		}
	}
	return validateOptionalText("deprecation replacement", replacement)
}

func sameDeprecationConclusion(left, right DeprecationSignal) bool {
	return left.Status == right.Status &&
		optionalVersionEqual(left.IntroducedIn, right.IntroducedIn) &&
		optionalVersionEqual(left.DeprecatedIn, right.DeprecatedIn) &&
		optionalVersionEqual(left.RemovedIn, right.RemovedIn) &&
		left.Replacement == right.Replacement
}

func optionalVersionEqual(left, right *research.SourceVersion) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

type DeprecationAssessmentResult struct {
	Record research.DeprecationRecord
}

type DeprecationIntelligenceService interface {
	Assess(context.Context, DeprecationAssessmentRequest) (DeprecationAssessmentResult, error)
	Get(context.Context, research.ID) (research.DeprecationRecord, error)
	History(context.Context, string) ([]research.DeprecationRecord, error)
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

type GenerateCitationRequest struct {
	ID           research.ID
	SourceID     research.SourceID
	SnapshotID   research.ID
	EvidenceID   research.ID
	LastVerified research.Timestamp
	Target       citation.Target
}

type CitationService interface {
	Generate(context.Context, GenerateCitationRequest) (research.Citation, error)
	Get(context.Context, research.ID) (research.Citation, error)
	ListForEvidence(context.Context, research.ID) ([]research.Citation, error)
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
	ClassifyTemporalScope(context.Context, research.SourceID, research.SourceTemporalScope) error
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

// SourceRegistryStore scopes read-only source registry, provenance, and stale
// schedule commands plus the underlying workspace database lifetime without
// exposing SQLite.
type SourceRegistryStore interface {
	Registry() SourceRegistryService
	Provenance() ProvenanceService
	Freshness() FreshnessService
	Close() error
}

type SourceRegistryStoreFactory interface {
	Open(context.Context, string) (SourceRegistryStore, error)
}

type VerificationService interface {
	Verify(context.Context, research.ClaimID) (research.VerificationResult, error)
	Get(context.Context, research.ID) (research.VerificationResult, error)
	Latest(context.Context, research.ClaimID) (research.VerificationResult, error)
}

type ConflictObservationRef struct {
	ClaimID  research.ClaimID
	SourceID research.SourceID
}

func (reference ConflictObservationRef) Validate() error {
	if err := reference.ClaimID.Validate(); err != nil {
		return err
	}
	return reference.SourceID.Validate()
}

type ConflictAssessmentRequest struct {
	Relation     conflictpolicy.Relation
	Observations []ConflictObservationRef
}

func (request ConflictAssessmentRequest) Validate() error {
	if err := request.Relation.Validate(); err != nil {
		return err
	}
	if len(request.Observations) != 2 {
		return fmt.Errorf("conflict assessment requires exactly 2 observations")
	}
	for index, observation := range request.Observations {
		if err := observation.Validate(); err != nil {
			return fmt.Errorf("conflict observation %d: %w", index, err)
		}
	}
	if request.Observations[0].ClaimID == request.Observations[1].ClaimID {
		return fmt.Errorf("conflict observations repeat a claim")
	}
	return nil
}

type ConflictResolutionService interface {
	Assess(context.Context, ConflictAssessmentRequest) (research.Conflict, error)
	Get(context.Context, research.ID) (research.Conflict, error)
	ListForClaim(context.Context, research.ClaimID) ([]research.Conflict, error)
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
