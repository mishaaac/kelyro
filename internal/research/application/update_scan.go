package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
)

type updateScanService struct {
	sources      SourceRepository
	snapshots    SnapshotRepository
	releases     ReleaseRepository
	deprecations DeprecationRepository
	freshness    FreshnessRepository
	conflicts    ConflictRepository
	provider     UpdateSignalProvider
}

func NewUpdateScanService(
	sources SourceRepository,
	snapshots SnapshotRepository,
	releases ReleaseRepository,
	deprecations DeprecationRepository,
	freshness FreshnessRepository,
	conflicts ConflictRepository,
	provider UpdateSignalProvider,
) UpdateScanService {
	return &updateScanService{
		sources: sources, snapshots: snapshots, releases: releases,
		deprecations: deprecations, freshness: freshness, conflicts: conflicts,
		provider: provider,
	}
}

func (service *updateScanService) Scan(
	ctx context.Context,
	mode ResearchMode,
	access NetworkResearchAccess,
	asOf research.Timestamp,
) (research.UpdateScan, error) {
	const operation = "scan research updates"
	if err := mode.Validate(); err != nil {
		return research.UpdateScan{}, invalid(operation, err)
	}
	if err := asOf.Validate(); err != nil {
		return research.UpdateScan{}, invalid(operation, err)
	}
	for name, dependency := range map[string]any{
		"source repository": service.sources, "snapshot repository": service.snapshots,
		"release repository": service.releases, "deprecation repository": service.deprecations,
		"freshness repository": service.freshness, "conflict repository": service.conflicts,
	} {
		if err := requireDependency(operation, name, dependency); err != nil {
			return research.UpdateScan{}, err
		}
	}

	sources, err := service.sources.List(ctx)
	if err != nil {
		return research.UpdateScan{}, repositoryError(operation, err)
	}
	releases, err := service.releases.List(ctx)
	if err != nil {
		return research.UpdateScan{}, repositoryError(operation, err)
	}
	deprecations, err := service.deprecations.List(ctx)
	if err != nil {
		return research.UpdateScan{}, repositoryError(operation, err)
	}
	due, err := service.freshness.ListDue(ctx, asOf)
	if err != nil {
		return research.UpdateScan{}, repositoryError(operation, err)
	}
	conflicts, err := service.conflicts.ListUnresolved(ctx)
	if err != nil {
		return research.UpdateScan{}, repositoryError(operation, err)
	}

	signals := make(map[string]research.UpdateSignal)
	add := func(signal research.UpdateSignal) error {
		if err := signal.Validate(); err != nil {
			return err
		}
		signals[updateSignalKey(signal)] = signal
		return nil
	}
	localSources, err := service.localSourceSignals(ctx, sources)
	if err != nil {
		return research.UpdateScan{}, repositoryError(operation, err)
	}
	for _, signal := range localSources {
		if err := add(signal); err != nil {
			return research.UpdateScan{}, invalid(operation, err)
		}
	}
	for _, signal := range localReleaseSignals(releases) {
		if err := add(signal); err != nil {
			return research.UpdateScan{}, invalid(operation, err)
		}
	}
	for _, record := range latestDeprecationsBySubject(deprecations) {
		detail := fmt.Sprintf("%s is recorded as %s", record.Subject, record.Status)
		if err := add(research.UpdateSignal{Type: research.UpdateSignalDeprecatedSubject, Reference: record.ID.String(), Detail: detail, Origin: research.UpdateSignalStoredMetadata, ObservedAt: record.VerifiedAt}); err != nil {
			return research.UpdateScan{}, invalid(operation, err)
		}
	}
	for _, record := range due {
		detail := fmt.Sprintf("freshness is %s and reverification is due", record.State)
		if err := add(research.UpdateSignal{Type: research.UpdateSignalStaleEvidence, Reference: record.SubjectID.String(), Detail: detail, Origin: research.UpdateSignalStoredMetadata, ObservedAt: asOf}); err != nil {
			return research.UpdateScan{}, invalid(operation, err)
		}
	}
	for _, conflict := range conflicts {
		if err := add(research.UpdateSignal{Type: research.UpdateSignalUnresolvedConflict, Reference: conflict.ID.String(), Detail: conflict.Reason, Origin: research.UpdateSignalStoredMetadata, ObservedAt: conflict.DetectedAt}); err != nil {
			return research.UpdateScan{}, invalid(operation, err)
		}
	}

	reasons := make([]research.UpdateScanIncompleteReason, 0, 1)
	decision, err := newNetworkResearchAccess(access).decide(ctx, mode, NetworkOperationUpdateScan)
	if err != nil {
		return research.UpdateScan{}, err
	}
	if !decision.live {
		reasons = append(reasons, research.UpdateScanNetworkDisabled)
	} else if service.provider == nil {
		reasons = append(reasons, research.UpdateScanProviderUnavailable)
	} else {
		lookup := UpdateSignalLookup{Sources: sources, Releases: releases, AsOf: asOf}
		if err := lookup.Validate(); err != nil {
			return research.UpdateScan{}, invalid(operation, err)
		}
		current, providerErr := service.provider.Scan(ctx, lookup)
		if providerErr != nil {
			if errors.Is(providerErr, context.Canceled) || errors.Is(providerErr, context.DeadlineExceeded) {
				return research.UpdateScan{}, Classify(ErrorUnavailable, operation, providerErr)
			}
			reasons = append(reasons, research.UpdateScanProviderFailed)
		} else {
			for _, signal := range current {
				if signal.Origin != research.UpdateSignalCurrentLookup {
					return research.UpdateScan{}, invalid(operation, errors.New("live provider returned a non-live signal"))
				}
				if err := add(signal); err != nil {
					return research.UpdateScan{}, boundaryError(ErrorExternalFailure, operation, err)
				}
			}
		}
	}

	technologyIDs := make(map[research.ID]struct{})
	for _, release := range releases {
		technologyIDs[release.TechnologyID] = struct{}{}
	}
	ordered := make([]research.UpdateSignal, 0, len(signals))
	for _, signal := range signals {
		ordered = append(ordered, signal)
	}
	result := research.UpdateScan{
		ScannedAt: asOf,
		Inventory: research.UpdateScanInventory{
			KnownTechnologies: len(technologyIDs), KnownReleases: len(releases),
			TrackedSources: len(sources), FreshnessDue: len(due),
		},
		Signals: research.SortUpdateSignalsV1(ordered), IncompleteReasons: reasons,
		AlgorithmVersion: research.UpdateScanAlgorithmV1,
	}
	if err := result.Validate(); err != nil {
		return research.UpdateScan{}, invalid(operation, err)
	}
	return result, nil
}

func (service *updateScanService) localSourceSignals(ctx context.Context, sources []research.Source) ([]research.UpdateSignal, error) {
	result := make([]research.UpdateSignal, 0)
	for _, source := range sources {
		snapshots, err := service.snapshots.ListBySource(ctx, source.ID)
		if err != nil {
			return nil, err
		}
		if len(snapshots) == 0 {
			continue
		}
		latest := snapshots[len(snapshots)-1]
		if latest.Fetch.StatusCode == 404 || latest.Fetch.StatusCode == 410 {
			result = append(result, research.UpdateSignal{
				Type: research.UpdateSignalChangedSource, Reference: source.ID.String(),
				Detail: "latest source fetch reports that the source is gone", Origin: research.UpdateSignalStoredMetadata,
				ObservedAt: latest.FetchedAt,
			})
			continue
		}
		latestHash := latest.Fetch.ContentHash
		if latestHash == "" {
			continue
		}
		for index := len(snapshots) - 2; index >= 0; index-- {
			previousHash := snapshots[index].Fetch.ContentHash
			if previousHash == "" {
				continue
			}
			if previousHash != latestHash {
				result = append(result, research.UpdateSignal{
					Type: research.UpdateSignalChangedSource, Reference: source.ID.String(),
					Detail: "latest stored snapshot has a different content hash", Origin: research.UpdateSignalStoredMetadata,
					ObservedAt: latest.FetchedAt,
				})
			}
			break
		}
	}
	return result, nil
}

func localReleaseSignals(releases []research.ReleaseRecord) []research.UpdateSignal {
	byTechnology := make(map[research.ID][]research.ReleaseRecord)
	for _, release := range releases {
		byTechnology[release.TechnologyID] = append(byTechnology[release.TechnologyID], release)
	}
	result := make([]research.UpdateSignal, 0)
	for _, records := range byTechnology {
		if len(records) < 2 {
			continue
		}
		sort.Slice(records, func(i, j int) bool {
			if records[i].VerifiedAt.Time().Equal(records[j].VerifiedAt.Time()) {
				return records[i].ID.String() < records[j].ID.String()
			}
			return records[i].VerifiedAt.Before(records[j].VerifiedAt)
		})
		latest := records[len(records)-1]
		if latest.Status != research.ReleaseCurrent {
			continue
		}
		result = append(result, research.UpdateSignal{
			Type: research.UpdateSignalNewRelease, Reference: latest.ID.String(),
			Detail: fmt.Sprintf("version %s is the current release for %s", latest.Version, latest.TechnologyID),
			Origin: research.UpdateSignalStoredMetadata, ObservedAt: latest.VerifiedAt,
		})
	}
	return result
}

func latestDeprecationsBySubject(records []research.DeprecationRecord) []research.DeprecationRecord {
	latest := make(map[string]research.DeprecationRecord)
	for _, record := range records {
		stored, exists := latest[record.Subject]
		if !exists || record.VerifiedAt.After(stored.VerifiedAt) ||
			(record.VerifiedAt.Time().Equal(stored.VerifiedAt.Time()) && record.ID.String() > stored.ID.String()) {
			latest[record.Subject] = record
		}
	}
	result := make([]research.DeprecationRecord, 0, len(latest))
	for _, record := range latest {
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Subject < result[j].Subject })
	return result
}

func updateSignalKey(signal research.UpdateSignal) string {
	return string(signal.Type) + "\x00" + signal.Reference
}
