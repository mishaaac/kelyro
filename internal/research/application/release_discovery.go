package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/authority"
)

const releaseClaimConfidenceV1 = 0.8

type releaseDiscoveryService struct {
	sources                    SourceRepository
	trust                      TrustRegistryRepository
	capture                    SnapshotCaptureService
	releases                   ReleaseRepository
	ingestion                  ReleaseIngestionRepository
	providers                  map[string]ReleaseNotesProvider
	providerConfigurationError error
}

func NewReleaseDiscoveryService(
	sources SourceRepository,
	trust TrustRegistryRepository,
	capture SnapshotCaptureService,
	releases ReleaseRepository,
	ingestion ReleaseIngestionRepository,
	providers ...ReleaseNotesProvider,
) ReleaseDiscoveryService {
	registry := make(map[string]ReleaseNotesProvider, len(providers))
	var configurationError error
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		id := provider.ID()
		if strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) {
			configurationError = errors.Join(configurationError, fmt.Errorf("release provider has invalid ID %q", id))
			continue
		}
		if _, exists := registry[id]; exists {
			configurationError = errors.Join(configurationError, fmt.Errorf("duplicate release provider ID %q", id))
			continue
		}
		registry[id] = provider
	}
	return &releaseDiscoveryService{
		sources: sources, trust: trust, capture: capture, releases: releases,
		ingestion: ingestion, providers: registry, providerConfigurationError: configurationError,
	}
}

type authorizedReleaseSource struct {
	config ReleaseDiscoverySource
	source research.Source
	rank   int
}

type observedReleaseChange struct {
	sourceID   research.SourceID
	snapshotID research.ID
	change     ReleaseChange
}

type accumulatedRelease struct {
	version    research.VersionIdentifier
	channel    research.ReleaseChannel
	releasedAt *research.Timestamp
	sourceIDs  []research.SourceID
	changes    []observedReleaseChange
	observedAt research.Timestamp
}

func (service *releaseDiscoveryService) Discover(
	ctx context.Context,
	mode ResearchMode,
	request ReleaseDiscoveryRequest,
) (ReleaseDiscoveryResult, error) {
	const operation = "discover technology releases"
	if err := request.Validate(); err != nil {
		return ReleaseDiscoveryResult{}, invalid(operation, err)
	}
	if err := mode.Validate(); err != nil {
		return ReleaseDiscoveryResult{}, invalid(operation, err)
	}
	for name, dependency := range map[string]any{
		"source repository":            service.sources,
		"trust registry repository":    service.trust,
		"snapshot capture service":     service.capture,
		"release repository":           service.releases,
		"release ingestion repository": service.ingestion,
	} {
		if err := requireDependency(operation, name, dependency); err != nil {
			return ReleaseDiscoveryResult{}, err
		}
	}
	if service.providerConfigurationError != nil {
		return ReleaseDiscoveryResult{}, Classify(ErrorUnavailable, operation, service.providerConfigurationError)
	}
	catalog, err := authority.NewCatalog([]research.AuthorityProfile{request.Profile})
	if err != nil {
		return ReleaseDiscoveryResult{}, invalid(operation, err)
	}
	matched, found, err := catalog.Match(request.Topic)
	if err != nil || !found || matched.ID != request.Profile.ID {
		if err == nil {
			err = errors.New("authority profile does not apply to the release topic")
		}
		return ReleaseDiscoveryResult{}, invalid(operation, err)
	}

	ordered, err := service.authorizedSources(ctx, request)
	if err != nil {
		return ReleaseDiscoveryResult{}, err
	}
	accumulated := make(map[string]*accumulatedRelease)
	duplicateCount := 0
	for _, item := range ordered {
		if err := ctx.Err(); err != nil {
			return ReleaseDiscoveryResult{}, Classify(ErrorUnavailable, operation, err)
		}
		provider, exists := service.providers[item.config.Provider]
		if !exists {
			return ReleaseDiscoveryResult{}, Classify(ErrorUnavailable, operation,
				fmt.Errorf("release provider %q is not configured", item.config.Provider))
		}
		capture, captureErr := service.capture.Capture(ctx, mode, SnapshotCaptureRequest{
			SourceID: item.source.ID, MaximumBytes: request.MaximumBytes,
			BodyPolicy: SnapshotNormalizedExcerpt,
		})
		if captureErr != nil {
			return ReleaseDiscoveryResult{}, captureErr
		}
		// A valid 304 snapshots the revalidation but carries no new body. Stored
		// release intelligence remains available and is considered below.
		if capture.NormalizationInput == nil {
			continue
		}
		candidates, providerErr := provider.Discover(ctx, *capture.NormalizationInput)
		if providerErr != nil {
			return ReleaseDiscoveryResult{}, externalError(operation,
				fmt.Errorf("provider %q: %w", provider.ID(), providerErr))
		}
		if len(candidates) > MaximumReleaseCandidates {
			return ReleaseDiscoveryResult{}, externalError(operation,
				fmt.Errorf("provider %q returned more than %d releases", provider.ID(), MaximumReleaseCandidates))
		}
		for index, candidate := range candidates {
			if err := validateReleaseCandidate(candidate, capture.Snapshot.FetchedAt); err != nil {
				return ReleaseDiscoveryResult{}, externalError(operation,
					fmt.Errorf("provider %q result %d: %w", provider.ID(), index, err))
			}
			key := releaseIdentity(candidate.Version, candidate.Channel)
			entry, duplicate := accumulated[key]
			if !duplicate {
				entry = &accumulatedRelease{
					version: candidate.Version, channel: candidate.Channel,
					releasedAt: cloneTimestampValue(candidate.ReleasedAt),
					observedAt: capture.Snapshot.FetchedAt,
				}
				accumulated[key] = entry
			} else {
				duplicateCount++
				if err := mergeReleaseDate(entry, candidate.ReleasedAt); err != nil {
					return ReleaseDiscoveryResult{}, externalError(operation, err)
				}
				if capture.Snapshot.FetchedAt.After(entry.observedAt) {
					entry.observedAt = capture.Snapshot.FetchedAt
				}
			}
			entry.sourceIDs = appendUniqueSourceID(entry.sourceIDs, item.source.ID)
			for _, change := range candidate.Changes {
				observation := observedReleaseChange{
					sourceID: item.source.ID, snapshotID: capture.Snapshot.ID, change: change,
				}
				if !containsObservedChange(entry.changes, observation) {
					entry.changes = append(entry.changes, observation)
				}
			}
		}
	}

	existing, err := service.releases.ListByTechnology(ctx, request.TechnologyID)
	if err != nil {
		return ReleaseDiscoveryResult{}, repositoryError(operation, err)
	}
	existingByKey := make(map[string]research.TechnologyRelease, len(existing))
	for _, record := range existing {
		existingByKey[releaseIdentity(record.Version, record.Channel)] = record
	}

	newRecords := make([]research.TechnologyRelease, 0, len(accumulated))
	newEntries := make(map[string]*accumulatedRelease, len(accumulated))
	for key, entry := range accumulated {
		if _, duplicate := existingByKey[key]; duplicate {
			duplicateCount++
			continue
		}
		record := research.TechnologyRelease{
			ID:           stableResearchID("release", request.TechnologyID.String(), entry.version.String(), string(entry.channel)),
			TechnologyID: request.TechnologyID, Version: entry.version,
			Channel: entry.channel, Status: research.ReleaseStatusUnknown,
			SourceIDs:  append([]research.SourceID(nil), entry.sourceIDs...),
			ReleasedAt: cloneTimestampValue(entry.releasedAt),
			VerifiedAt: entry.observedAt,
		}
		newRecords = append(newRecords, record)
		newEntries[key] = entry
	}

	all := append(cloneReleases(existing), newRecords...)
	currentStableIndex, previewIndexes, err := classifyCurrentReleases(all)
	if err != nil {
		return ReleaseDiscoveryResult{}, invalid(operation, err)
	}
	confidence, err := research.NewClaimConfidence(releaseClaimConfidenceV1)
	if err != nil {
		return ReleaseDiscoveryResult{}, invalid(operation, err)
	}
	result := ReleaseDiscoveryResult{
		DuplicateCount: duplicateCount, AlgorithmVersion: ReleaseDiscoveryAlgorithmV1,
	}
	for _, record := range all[len(existing):] {
		entry := newEntries[releaseIdentity(record.Version, record.Channel)]
		evidence, claims, buildErr := buildReleaseNotes(record, entry, request.Topic, confidence)
		if buildErr != nil {
			return ReleaseDiscoveryResult{}, invalid(operation, buildErr)
		}
		result.Releases = append(result.Releases, record)
		result.Evidence = append(result.Evidence, evidence...)
		result.Claims = append(result.Claims, claims...)
	}
	batch := ReleaseIngestionBatch{
		Evidence: result.Evidence, Claims: result.Claims, Releases: result.Releases,
		StatusUpdates: changedReleaseStatuses(existing, all),
	}
	if len(batch.Evidence) > 0 || len(batch.Claims) > 0 || len(batch.Releases) > 0 || len(batch.StatusUpdates) > 0 {
		if err := service.ingestion.Commit(ctx, batch); err != nil {
			return ReleaseDiscoveryResult{}, repositoryError(operation, err)
		}
	}
	if currentStableIndex >= 0 {
		current := all[currentStableIndex]
		result.CurrentStable = &current
	}
	for _, index := range previewIndexes {
		result.PreviewReleases = append(result.PreviewReleases, all[index])
	}
	sortReleaseOutput(result.Releases)
	sortReleaseOutput(result.PreviewReleases)
	return result, nil
}

func (service *releaseDiscoveryService) authorizedSources(ctx context.Context, request ReleaseDiscoveryRequest) ([]authorizedReleaseSource, error) {
	const operation = "discover technology releases"
	preferred := make(map[research.SourceKind]int, len(request.Profile.PreferredKinds))
	for index, kind := range request.Profile.PreferredKinds {
		preferred[kind] = index
	}
	result := make([]authorizedReleaseSource, 0, len(request.Sources))
	for _, config := range request.Sources {
		source, err := service.sources.Get(ctx, config.SourceID)
		if err != nil {
			return nil, repositoryError(operation, err)
		}
		rank, authorizedKind := preferred[source.Kind]
		if !authorizedKind {
			return nil, invalid(operation, fmt.Errorf("source %q kind %q is not preferred by authority profile", source.ID.String(), source.Kind))
		}
		decision, err := service.trust.LatestDecision(ctx, source.ID)
		if err != nil {
			return nil, repositoryError(operation, err)
		}
		if decision.State != research.TrustAccepted || !tierMeetsMinimum(decision.Tier, request.Profile.MinimumTier) {
			return nil, invalid(operation, fmt.Errorf("source %q is not authorized for release evidence", source.ID.String()))
		}
		result = append(result, authorizedReleaseSource{config: config, source: source, rank: rank})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].rank != result[j].rank {
			return result[i].rank < result[j].rank
		}
		return result[i].source.ID.String() < result[j].source.ID.String()
	})
	return result, nil
}

func validateReleaseCandidate(candidate ReleaseCandidate, observedAt research.Timestamp) error {
	if err := candidate.Validate(); err != nil {
		return err
	}
	if candidate.ReleasedAt != nil && candidate.ReleasedAt.After(observedAt) {
		return fmt.Errorf("release date follows feed observation")
	}
	if semantic, ok := candidate.Version.Semantic(); ok && candidate.Channel == research.ReleaseStable && semantic.IsPrerelease() {
		return fmt.Errorf("stable channel contains a prerelease semantic version")
	}
	return nil
}

func mergeReleaseDate(target *accumulatedRelease, candidate *research.Timestamp) error {
	if candidate == nil {
		return nil
	}
	if target.releasedAt == nil {
		target.releasedAt = cloneTimestampValue(candidate)
		return nil
	}
	if !target.releasedAt.Time().Equal(candidate.Time()) {
		return fmt.Errorf("duplicate release %q in channel %q has conflicting dates", target.version.String(), target.channel)
	}
	return nil
}

func buildReleaseNotes(record research.TechnologyRelease, entry *accumulatedRelease, topic research.ResearchTopic, confidence research.ClaimConfidence) ([]research.Evidence, []research.Claim, error) {
	if entry == nil {
		return nil, nil, fmt.Errorf("release ingestion entry is missing")
	}
	evidence := make([]research.Evidence, 0, len(entry.changes))
	claimGroups := make(map[string]*research.Claim)
	for _, observation := range entry.changes {
		evidenceID := stableResearchID("evidence.release", observation.sourceID.String(), observation.snapshotID.String(), observation.change.Location, observation.change.Excerpt)
		item := research.Evidence{
			ID: evidenceID, SourceID: observation.sourceID, SnapshotID: observation.snapshotID,
			Location: observation.change.Location, Excerpt: observation.change.Excerpt,
			ExcerptHash: research.CanonicalEvidenceExcerptHashV1(observation.change.Excerpt),
			ExtractedAt: record.VerifiedAt, ExtractorVersion: ReleaseNotesIngestionAlgorithmV1,
		}
		if err := item.Validate(); err != nil {
			return nil, nil, err
		}
		evidence = append(evidence, item)
		statusScope := claimStatusForChannel(record.Channel)
		claimKey := string(statusScope) + "\x00" + observation.change.Statement
		claim := claimGroups[claimKey]
		if claim == nil {
			version := research.SourceVersion(record.Version)
			value := research.Claim{
				ID:    stableClaimID("claim.release", record.TechnologyID.String(), record.Version.String(), string(statusScope), observation.change.Statement),
				Topic: topic, Statement: observation.change.Statement, Type: research.ClaimVersionChange,
				Scope: "release notes", VersionScope: &version, StatusScope: statusScope,
				Confidence: confidence, CreatedAt: record.VerifiedAt,
			}
			claimGroups[claimKey] = &value
			claim = &value
		}
		claim.SourceIDs = appendUniqueSourceID(claim.SourceIDs, observation.sourceID)
		claim.EvidenceIDs = append(claim.EvidenceIDs, evidenceID)
		claimGroups[claimKey] = claim
	}
	claims := make([]research.Claim, 0, len(claimGroups))
	for _, claim := range claimGroups {
		if err := claim.Validate(); err != nil {
			return nil, nil, err
		}
		claims = append(claims, *claim)
	}
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].ID.String() < evidence[j].ID.String() })
	sort.Slice(claims, func(i, j int) bool { return claims[i].ID.String() < claims[j].ID.String() })
	return evidence, claims, nil
}

func classifyCurrentReleases(records []research.TechnologyRelease) (int, []int, error) {
	stable := make([]int, 0)
	stableCandidates := make([]int, 0)
	previews := make([]int, 0)
	previewCandidates := make([]int, 0)
	for index := range records {
		retired := records[index].Status == research.ReleaseLegacy || records[index].Status == research.ReleaseEOL
		switch records[index].Channel {
		case research.ReleaseStable:
			stable = append(stable, index)
			if !retired {
				stableCandidates = append(stableCandidates, index)
			}
		case research.ReleasePreview, research.ReleaseBeta, research.ReleaseRC,
			research.ReleaseExperimental, research.ReleaseNightly:
			previews = append(previews, index)
			if !retired {
				previewCandidates = append(previewCandidates, index)
			}
		}
	}
	stableCurrent, err := newestRelease(records, stableCandidates)
	if err != nil {
		return -1, nil, fmt.Errorf("select current stable: %w", err)
	}
	previewCurrent, err := newestRelease(records, previewCandidates)
	if err != nil {
		return -1, nil, fmt.Errorf("select current preview: %w", err)
	}
	for _, index := range stable {
		if records[index].Status == research.ReleaseLegacy || records[index].Status == research.ReleaseEOL {
			continue
		}
		if index == stableCurrent {
			records[index].Status = research.ReleaseCurrent
		} else {
			records[index].Status = research.ReleaseSuperseded
		}
	}
	for _, index := range previews {
		if records[index].Status == research.ReleaseLegacy || records[index].Status == research.ReleaseEOL {
			continue
		}
		if index == previewCurrent {
			records[index].Status = research.ReleaseCurrent
		} else {
			records[index].Status = research.ReleaseSuperseded
		}
	}
	sort.SliceStable(previews, func(i, j int) bool {
		comparison, compareErr := compareReleasePrecedence(records[previews[i]], records[previews[j]])
		return compareErr == nil && comparison > 0
	})
	return stableCurrent, previews, nil
}

func newestRelease(records []research.TechnologyRelease, indexes []int) (int, error) {
	if len(indexes) == 0 {
		return -1, nil
	}
	newest := indexes[0]
	for _, index := range indexes[1:] {
		comparison, err := compareReleasePrecedence(records[index], records[newest])
		if err != nil {
			return -1, err
		}
		if comparison > 0 {
			newest = index
		}
	}
	return newest, nil
}

func compareReleasePrecedence(left, right research.TechnologyRelease) (int, error) {
	if left.Version == right.Version {
		return 0, nil
	}
	if leftSemantic, leftOK := left.Version.Semantic(); leftOK {
		if rightSemantic, rightOK := right.Version.Semantic(); rightOK {
			if comparison := compareSemantic(leftSemantic, rightSemantic); comparison != 0 {
				return comparison, nil
			}
			return compareEqualPrecedenceIdentity(left, right)
		}
	}
	if leftDate, leftOK := left.Version.DateBased(); leftOK {
		if rightDate, rightOK := right.Version.DateBased(); rightOK {
			if comparison := compareDateVersion(leftDate, rightDate); comparison != 0 {
				return comparison, nil
			}
			return compareEqualPrecedenceIdentity(left, right)
		}
	}
	if left.ReleasedAt != nil && right.ReleasedAt != nil {
		if left.ReleasedAt.Before(*right.ReleasedAt) {
			return -1, nil
		}
		if left.ReleasedAt.After(*right.ReleasedAt) {
			return 1, nil
		}
	}
	return 0, fmt.Errorf("versions %q and %q have no deterministic precedence", left.Version.String(), right.Version.String())
}

func compareEqualPrecedenceIdentity(left, right research.TechnologyRelease) (int, error) {
	if left.Version == right.Version {
		return 0, nil
	}
	if left.ReleasedAt != nil && right.ReleasedAt != nil {
		if left.ReleasedAt.Before(*right.ReleasedAt) {
			return -1, nil
		}
		if left.ReleasedAt.After(*right.ReleasedAt) {
			return 1, nil
		}
	}
	return 0, fmt.Errorf("versions %q and %q have equal precedence and no distinct release dates", left.Version.String(), right.Version.String())
}

func compareSemantic(left, right research.SemanticVersion) int {
	for _, pair := range [][2]uint64{{left.Major, right.Major}, {left.Minor, right.Minor}, {left.Patch, right.Patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return comparePrerelease(left.Prerelease, right.Prerelease)
}

func comparePrerelease(left, right string) int {
	if left == right {
		return 0
	}
	if left == "" {
		return 1
	}
	if right == "" {
		return -1
	}
	leftParts, rightParts := strings.Split(left, "."), strings.Split(right, ".")
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		if leftParts[index] == rightParts[index] {
			continue
		}
		leftNumber, leftErr := strconv.ParseUint(leftParts[index], 10, 64)
		rightNumber, rightErr := strconv.ParseUint(rightParts[index], 10, 64)
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNumber < rightNumber {
				return -1
			}
			return 1
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		case leftParts[index] < rightParts[index]:
			return -1
		default:
			return 1
		}
	}
	if len(leftParts) < len(rightParts) {
		return -1
	}
	return 1
}

func compareDateVersion(left, right research.DateVersion) int {
	leftTime := left.Year*10000 + int(left.Month)*100 + left.Day
	rightTime := right.Year*10000 + int(right.Month)*100 + right.Day
	if leftTime < rightTime {
		return -1
	}
	if leftTime > rightTime {
		return 1
	}
	if left.Precision == right.Precision {
		return 0
	}
	if left.Precision == research.DatePrecisionMonth {
		return -1
	}
	return 1
}

func changedReleaseStatuses(existing, classified []research.TechnologyRelease) []research.TechnologyRelease {
	byID := make(map[research.ID]research.TechnologyRelease, len(classified))
	for _, record := range classified {
		byID[record.ID] = record
	}
	result := make([]research.TechnologyRelease, 0)
	for _, stored := range existing {
		updated := byID[stored.ID]
		if updated.Status != stored.Status {
			result = append(result, updated)
		}
	}
	return result
}

func claimStatusForChannel(channel research.ReleaseChannel) research.ClaimStatusScope {
	switch channel {
	case research.ReleaseStable:
		return research.ClaimStatusStable
	case research.ReleasePreview, research.ReleaseBeta, research.ReleaseRC:
		return research.ClaimStatusPreview
	case research.ReleaseExperimental, research.ReleaseNightly:
		return research.ClaimStatusExperimental
	default:
		return research.ClaimStatusAll
	}
}

func tierMeetsMinimum(actual, minimum research.AuthorityTier) bool {
	rank := map[research.AuthorityTier]int{
		research.AuthorityTierA: 0, research.AuthorityTierB: 1, research.AuthorityTierC: 2,
		research.AuthorityTierD: 3, research.AuthorityTierE: 4,
	}
	return rank[actual] <= rank[minimum]
}

func releaseIdentity(version research.VersionIdentifier, channel research.ReleaseChannel) string {
	return version.String() + "\x00" + string(channel)
}

func appendUniqueSourceID(values []research.SourceID, value research.SourceID) []research.SourceID {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func containsObservedChange(values []observedReleaseChange, value observedReleaseChange) bool {
	for _, existing := range values {
		if existing.sourceID == value.sourceID && existing.change.Location == value.change.Location &&
			existing.change.Excerpt == value.change.Excerpt && existing.change.Statement == value.change.Statement {
			return true
		}
	}
	return false
}

func cloneTimestampValue(value *research.Timestamp) *research.Timestamp {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneReleases(records []research.TechnologyRelease) []research.TechnologyRelease {
	result := make([]research.TechnologyRelease, len(records))
	for index, record := range records {
		result[index] = record
		result[index].SourceIDs = append([]research.SourceID(nil), record.SourceIDs...)
		result[index].ReleasedAt = cloneTimestampValue(record.ReleasedAt)
	}
	return result
}

func stableResearchID(prefix string, parts ...string) research.ID {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	id, err := research.NewID(prefix + "." + hex.EncodeToString(hash[:16]))
	if err != nil {
		panic(err)
	}
	return id
}

func stableClaimID(prefix string, parts ...string) research.ClaimID {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	id, err := research.NewClaimID(prefix + "." + hex.EncodeToString(hash[:16]))
	if err != nil {
		panic(err)
	}
	return id
}

func sortReleaseOutput(records []research.TechnologyRelease) {
	sort.SliceStable(records, func(i, j int) bool {
		comparison, err := compareReleasePrecedence(records[i], records[j])
		if err != nil || comparison == 0 {
			return records[i].ID.String() < records[j].ID.String()
		}
		return comparison > 0
	})
}

var _ ReleaseDiscoveryService = (*releaseDiscoveryService)(nil)
