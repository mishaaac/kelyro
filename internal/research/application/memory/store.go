package memory

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

// Store owns independent maps for each narrow repository port.
type Store struct {
	mu sync.RWMutex

	sources         map[research.SourceID]research.Source
	sourceLocators  map[string]research.SourceID
	snapshots       map[research.ID]research.SourceSnapshot
	evidence        map[research.ID]research.Evidence
	claims          map[research.ClaimID]research.Claim
	citations       map[research.ID]research.Citation
	provenance      map[research.ClaimID][]research.ProvenanceGraph
	requests        map[research.ID]research.ResearchRequest
	runs            map[research.ID]research.ResearchRun
	profiles        map[research.ID]research.AuthorityProfile
	registryEntries map[research.ID]research.SourceRegistryEntry
	decisions       map[research.SourceID][]research.TrustDecision
	releases        map[research.ID]research.ReleaseRecord
	deprecations    map[research.ID]research.DeprecationRecord
	freshness       map[research.ID]application.FreshnessRecord
	verification    map[research.ID]research.VerificationResult
	conflicts       map[research.ID]research.Conflict
	bundles         map[research.ID]research.SourceBundle
	drift           map[research.ID]research.DriftReport
	impact          map[research.ID]research.ImpactReport
	cache           map[string]application.CacheEntry
}

func New() *Store {
	return &Store{
		sources:         make(map[research.SourceID]research.Source),
		sourceLocators:  make(map[string]research.SourceID),
		snapshots:       make(map[research.ID]research.SourceSnapshot),
		evidence:        make(map[research.ID]research.Evidence),
		claims:          make(map[research.ClaimID]research.Claim),
		citations:       make(map[research.ID]research.Citation),
		provenance:      make(map[research.ClaimID][]research.ProvenanceGraph),
		requests:        make(map[research.ID]research.ResearchRequest),
		runs:            make(map[research.ID]research.ResearchRun),
		profiles:        make(map[research.ID]research.AuthorityProfile),
		registryEntries: make(map[research.ID]research.SourceRegistryEntry),
		decisions:       make(map[research.SourceID][]research.TrustDecision),
		releases:        make(map[research.ID]research.ReleaseRecord),
		deprecations:    make(map[research.ID]research.DeprecationRecord),
		freshness:       make(map[research.ID]application.FreshnessRecord),
		verification:    make(map[research.ID]research.VerificationResult),
		conflicts:       make(map[research.ID]research.Conflict),
		bundles:         make(map[research.ID]research.SourceBundle),
		drift:           make(map[research.ID]research.DriftReport),
		impact:          make(map[research.ID]research.ImpactReport),
		cache:           make(map[string]application.CacheEntry),
	}
}

func (store *Store) Repositories() application.Repositories {
	return application.Repositories{
		Sources:          sourceRepository{store},
		Snapshots:        snapshotRepository{store},
		Evidence:         evidenceRepository{store},
		Claims:           claimRepository{store},
		Citations:        citationRepository{store},
		Provenance:       provenanceRepository{store},
		Runs:             researchRunRepository{store},
		TrustRegistry:    trustRegistryRepository{store},
		SourceRegistry:   sourceRegistryRepository{store},
		Releases:         releaseRepository{store},
		ReleaseIngestion: releaseIngestionRepository{store},
		Deprecations:     deprecationRepository{store},
		Freshness:        freshnessRepository{store},
		Verification:     verificationRepository{store},
		Conflicts:        conflictRepository{store},
		Bundles:          sourceBundleRepository{store},
		Drift:            driftRepository{store},
		Impact:           impactRepository{store},
		Cache:            cacheRepository{store},
	}
}

func contextError(operation string, ctx context.Context) error {
	if ctx == nil {
		return application.Classify(application.ErrorInvalidState, operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return application.Classify(application.ErrorUnavailable, operation, err)
	}
	return nil
}

func invalid(operation string, err error) error {
	return application.Classify(application.ErrorInvalidState, operation, err)
}

func notFound(operation string) error {
	return application.Classify(application.ErrorNotFound, operation, errors.New("record does not exist"))
}

func conflict(operation string) error {
	return application.Classify(application.ErrorConflict, operation, errors.New("record already exists"))
}

func validateKey(operation, key string) error {
	if strings.TrimSpace(key) == "" || key != strings.TrimSpace(key) {
		return invalid(operation, errors.New("cache key is invalid"))
	}
	return nil
}

func cloneSource(source research.Source) research.Source {
	clone := source
	if source.Version != nil {
		version := *source.Version
		clone.Version = &version
	}
	clone.Metadata.PublishedAt = cloneTimestamp(source.Metadata.PublishedAt)
	clone.Metadata.UpdatedAt = cloneTimestamp(source.Metadata.UpdatedAt)
	if source.Specialization != nil {
		clone.Specialization = source.Specialization.Clone()
	}
	if source.Video != nil {
		clone.Video = source.Video.Clone()
	}
	return clone
}

func cloneEvidence(evidence research.Evidence) research.Evidence {
	clone := evidence
	if evidence.SourceCode != nil {
		clone.SourceCode = evidence.SourceCode.Clone()
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

func cloneRequest(request research.ResearchRequest) research.ResearchRequest {
	clone := request
	if request.TargetVersion != nil {
		version := *request.TargetVersion
		clone.TargetVersion = &version
	}
	return clone
}

func cloneRun(run research.ResearchRun) research.ResearchRun {
	clone := run
	clone.CompletedAt = cloneTimestamp(run.CompletedAt)
	return clone
}

func cloneProvenanceGraph(graph research.ProvenanceGraph) research.ProvenanceGraph {
	clone := graph
	clone.Nodes = append([]research.ProvenanceNode(nil), graph.Nodes...)
	clone.Edges = append([]research.ProvenanceEdge(nil), graph.Edges...)
	return clone
}

func cloneCitation(citation research.Citation) research.Citation {
	clone := citation
	if citation.DeepLink != nil {
		link := *citation.DeepLink
		clone.DeepLink = &link
	}
	if citation.VersionScope != nil {
		version := *citation.VersionScope
		clone.VersionScope = &version
	}
	return clone
}

func cloneProfile(profile research.AuthorityProfile) research.AuthorityProfile {
	clone := profile
	clone.PreferredKinds = append([]research.SourceKind(nil), profile.PreferredKinds...)
	clone.PreferredDomains = append([]string(nil), profile.PreferredDomains...)
	clone.PreferredOrganizations = append([]string(nil), profile.PreferredOrganizations...)
	clone.AllowedSupplementaryKinds = append([]research.SourceKind(nil), profile.AllowedSupplementaryKinds...)
	clone.FreshnessTTLHints = cloneFreshnessTTLHints(profile.FreshnessTTLHints)
	return clone
}

func cloneFreshnessTTLHints(hints []research.FreshnessTTLHint) []research.FreshnessTTLHint {
	result := make([]research.FreshnessTTLHint, len(hints))
	for index, hint := range hints {
		result[index] = hint
		if hint.ClaimType != nil {
			value := *hint.ClaimType
			result[index].ClaimType = &value
		}
		if hint.SourceKind != nil {
			value := *hint.SourceKind
			result[index].SourceKind = &value
		}
	}
	return result
}

func cloneDecision(decision research.TrustDecision) research.TrustDecision {
	clone := decision
	clone.Reasons = append([]research.TrustReason(nil), decision.Reasons...)
	return clone
}

func cloneRegistryEntry(entry research.SourceRegistryEntry) research.SourceRegistryEntry {
	clone := entry
	clone.CanonicalDomains = append([]research.CanonicalDomain(nil), entry.CanonicalDomains...)
	clone.SourceKinds = append([]research.SourceKind(nil), entry.SourceKinds...)
	clone.AuthorityHints = append([]research.RegistryAuthorityHint(nil), entry.AuthorityHints...)
	clone.ResearchDomains = append([]string(nil), entry.ResearchDomains...)
	clone.TopicPatterns = append([]string(nil), entry.TopicPatterns...)
	return clone
}

func cloneRelease(record research.ReleaseRecord) research.ReleaseRecord {
	clone := record
	clone.SourceIDs = append([]research.SourceID(nil), record.SourceIDs...)
	clone.ReleasedAt = cloneTimestamp(record.ReleasedAt)
	return clone
}

func cloneDeprecation(record research.DeprecationRecord) research.DeprecationRecord {
	clone := record
	clone.IntroducedIn = cloneVersion(record.IntroducedIn)
	clone.DeprecatedIn = cloneVersion(record.DeprecatedIn)
	clone.RemovedIn = cloneVersion(record.RemovedIn)
	clone.SourceIDs = append([]research.SourceID(nil), record.SourceIDs...)
	clone.EvidenceIDs = append([]research.ID(nil), record.EvidenceIDs...)
	return clone
}

func cloneVersion(version *research.SourceVersion) *research.SourceVersion {
	if version == nil {
		return nil
	}
	clone := *version
	return &clone
}

func cloneClaim(claim research.Claim) research.Claim {
	clone := claim
	if claim.VersionScope != nil {
		version := *claim.VersionScope
		clone.VersionScope = &version
	}
	clone.SourceIDs = append([]research.SourceID(nil), claim.SourceIDs...)
	clone.EvidenceIDs = append([]research.ID(nil), claim.EvidenceIDs...)
	return clone
}

func cloneFreshness(record application.FreshnessRecord) application.FreshnessRecord {
	clone := record
	clone.NextVerifyAt = cloneTimestamp(record.NextVerifyAt)
	return clone
}

func cloneVerification(result research.VerificationResult) research.VerificationResult {
	clone := result
	clone.SourceIDs = append([]research.SourceID(nil), result.SourceIDs...)
	clone.ReasonCodes = append([]research.ClaimVerificationReason(nil), result.ReasonCodes...)
	return clone
}

func cloneConflict(result research.Conflict) research.Conflict {
	clone := result
	clone.ClaimIDs = append([]research.ClaimID(nil), result.ClaimIDs...)
	if result.WinningClaimID != nil {
		id := *result.WinningClaimID
		clone.WinningClaimID = &id
	}
	if result.WinningSourceID != nil {
		id := *result.WinningSourceID
		clone.WinningSourceID = &id
	}
	return clone
}

func cloneSourceBundle(bundle research.SourceBundle) research.SourceBundle {
	clone := bundle
	clone.TargetVersion = cloneVersion(bundle.TargetVersion)
	clone.ClaimIDs = append([]research.ClaimID(nil), bundle.ClaimIDs...)
	clone.Sources = make([]research.SourceBundleSource, len(bundle.Sources))
	for index, source := range bundle.Sources {
		clone.Sources[index] = source
		clone.Sources[index].VersionScope = cloneVersion(source.VersionScope)
	}
	clone.ConflictIDs = append([]research.ID(nil), bundle.ConflictIDs...)
	clone.Freshness.LastVerifiedAt = cloneTimestamp(bundle.Freshness.LastVerifiedAt)
	clone.Freshness.MissingClaimIDs = append([]research.ClaimID(nil), bundle.Freshness.MissingClaimIDs...)
	clone.Freshness.SourceAlgorithms = append([]string(nil), bundle.Freshness.SourceAlgorithms...)
	clone.Issues = append([]research.SourceBundleIssue(nil), bundle.Issues...)
	return clone
}

func cloneDrift(report research.DriftReport) research.DriftReport {
	clone := report
	if report.NewBundleID != nil {
		id := *report.NewBundleID
		clone.NewBundleID = &id
	}
	clone.AffectedClaims = append([]research.ClaimID(nil), report.AffectedClaims...)
	clone.OldEvidence = append([]research.ID(nil), report.OldEvidence...)
	clone.NewEvidence = append([]research.ID(nil), report.NewEvidence...)
	return clone
}

func cloneImpact(report research.ImpactReport) research.ImpactReport {
	clone := report
	clone.AffectedBundleIDs = append([]research.ID(nil), report.AffectedBundleIDs...)
	clone.AffectedClaimIDs = append([]research.ClaimID(nil), report.AffectedClaimIDs...)
	return clone
}

func cloneCache(entry application.CacheEntry) application.CacheEntry {
	clone := entry
	clone.Payload = append([]byte(nil), entry.Payload...)
	clone.ExpiresAt = cloneTimestamp(entry.ExpiresAt)
	return clone
}

func sortedIDs[T any](records map[research.ID]T) []research.ID {
	ids := make([]research.ID, 0, len(records))
	for id := range records {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids
}
