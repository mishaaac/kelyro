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
	requests        map[research.ID]research.ResearchRequest
	runs            map[research.ID]research.ResearchRun
	profiles        map[research.ID]research.AuthorityProfile
	registryEntries map[research.ID]research.SourceRegistryEntry
	decisions       map[research.SourceID][]research.TrustDecision
	releases        map[research.ID]research.ReleaseRecord
	freshness       map[research.ID]application.FreshnessRecord
	verification    map[research.ID]research.VerificationResult
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
		requests:        make(map[research.ID]research.ResearchRequest),
		runs:            make(map[research.ID]research.ResearchRun),
		profiles:        make(map[research.ID]research.AuthorityProfile),
		registryEntries: make(map[research.ID]research.SourceRegistryEntry),
		decisions:       make(map[research.SourceID][]research.TrustDecision),
		releases:        make(map[research.ID]research.ReleaseRecord),
		freshness:       make(map[research.ID]application.FreshnessRecord),
		verification:    make(map[research.ID]research.VerificationResult),
		drift:           make(map[research.ID]research.DriftReport),
		impact:          make(map[research.ID]research.ImpactReport),
		cache:           make(map[string]application.CacheEntry),
	}
}

func (store *Store) Repositories() application.Repositories {
	return application.Repositories{
		Sources:        sourceRepository{store},
		Snapshots:      snapshotRepository{store},
		Evidence:       evidenceRepository{store},
		Runs:           researchRunRepository{store},
		TrustRegistry:  trustRegistryRepository{store},
		SourceRegistry: sourceRegistryRepository{store},
		Releases:       releaseRepository{store},
		Freshness:      freshnessRepository{store},
		Verification:   verificationRepository{store},
		Drift:          driftRepository{store},
		Impact:         impactRepository{store},
		Cache:          cacheRepository{store},
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

func cloneProfile(profile research.AuthorityProfile) research.AuthorityProfile {
	clone := profile
	clone.PreferredKinds = append([]research.SourceKind(nil), profile.PreferredKinds...)
	clone.PreferredDomains = append([]string(nil), profile.PreferredDomains...)
	clone.PreferredOrganizations = append([]string(nil), profile.PreferredOrganizations...)
	clone.AllowedSupplementaryKinds = append([]research.SourceKind(nil), profile.AllowedSupplementaryKinds...)
	return clone
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

func cloneFreshness(record application.FreshnessRecord) application.FreshnessRecord {
	clone := record
	clone.NextVerifyAt = cloneTimestamp(record.NextVerifyAt)
	return clone
}

func cloneVerification(result research.VerificationResult) research.VerificationResult {
	clone := result
	clone.SourceIDs = append([]research.SourceID(nil), result.SourceIDs...)
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
