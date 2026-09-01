package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/authority"
	freshnesspolicy "github.com/mishaaac/kelyro/internal/research/freshness"
	temporalpolicy "github.com/mishaaac/kelyro/internal/research/temporal"
)

const (
	LiveTemporalEvaluationV1               = "live-temporal-evaluation-v1"
	LiveTemporalMetadataV1                 = "live-temporal-metadata-v1"
	MaximumLiveTemporalIntelligenceRecords = 5000
)

type LiveTemporalSourceObservation struct {
	SourceID         research.SourceID
	PublishedAt      *research.Timestamp
	UpdatedAt        *research.Timestamp
	VersionHints     []string
	Temporal         temporalpolicy.Assessment
	AlgorithmVersion string
}

func (observation LiveTemporalSourceObservation) Validate() error {
	if err := observation.SourceID.Validate(); err != nil {
		return err
	}
	if err := validateOptionalNormalizedTimestamp("live temporal published at", observation.PublishedAt); err != nil {
		return err
	}
	if err := validateOptionalNormalizedTimestamp("live temporal updated at", observation.UpdatedAt); err != nil {
		return err
	}
	if observation.PublishedAt != nil && observation.UpdatedAt != nil && observation.UpdatedAt.Before(*observation.PublishedAt) {
		return errors.New("live temporal update precedes publication")
	}
	if len(observation.VersionHints) > MaximumNormalizedVersionHints {
		return fmt.Errorf("live temporal version hints exceed %d", MaximumNormalizedVersionHints)
	}
	for index, hint := range observation.VersionHints {
		if _, err := research.NewSourceVersion(hint); err != nil {
			return fmt.Errorf("live temporal version hint %d: %w", index, err)
		}
	}
	if err := observation.Temporal.Validate(); err != nil {
		return err
	}
	if observation.Temporal.SourceID != observation.SourceID {
		return errors.New("live temporal assessment identifies another Source")
	}
	if observation.AlgorithmVersion != LiveTemporalMetadataV1 {
		return fmt.Errorf("live temporal metadata algorithm must be %q", LiveTemporalMetadataV1)
	}
	return nil
}

type LiveClaimSourceFreshness struct {
	ClaimID             research.ClaimID
	SourceID            research.SourceID
	Assessment          freshnesspolicy.Assessment
	KnownNewRelease     bool
	DeprecationVerified bool
	ReleaseVerified     bool
}

func (item LiveClaimSourceFreshness) Validate() error {
	if err := item.ClaimID.Validate(); err != nil {
		return err
	}
	if err := item.SourceID.Validate(); err != nil {
		return err
	}
	if err := item.Assessment.Validate(); err != nil {
		return err
	}
	return nil
}

type LiveTemporalEvaluationRequest struct {
	Topic             research.ResearchTopic
	Purpose           research.ResearchPurpose
	TargetVersion     *research.SourceVersion
	Sources           []research.Source
	NormalizedSources []NormalizedSource
	Claims            []research.Claim
	Citations         []research.Citation
}

type LiveTemporalEvaluationResult struct {
	TemporalObservations []LiveTemporalSourceObservation
	FreshnessAssessments []LiveClaimSourceFreshness
	FreshnessRecords     []FreshnessRecord
	TrustDecisions       []research.TrustDecision
	AlgorithmVersion     string
}

func (result LiveTemporalEvaluationResult) Validate() error {
	if len(result.TemporalObservations) == 0 || len(result.FreshnessAssessments) == 0 ||
		len(result.FreshnessRecords) == 0 || len(result.TrustDecisions) == 0 {
		return errors.New("live temporal evaluation requires temporal, freshness, and trust output")
	}
	for index, observation := range result.TemporalObservations {
		if err := observation.Validate(); err != nil {
			return fmt.Errorf("live temporal observation %d: %w", index, err)
		}
	}
	for index, item := range result.FreshnessAssessments {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("live freshness assessment %d: %w", index, err)
		}
	}
	for index, record := range result.FreshnessRecords {
		if err := record.Validate(); err != nil {
			return fmt.Errorf("live freshness record %d: %w", index, err)
		}
	}
	for index, decision := range result.TrustDecisions {
		if err := decision.Validate(); err != nil {
			return fmt.Errorf("live temporal trust decision %d: %w", index, err)
		}
	}
	if result.AlgorithmVersion != LiveTemporalEvaluationV1 {
		return fmt.Errorf("live temporal algorithm must be %q", LiveTemporalEvaluationV1)
	}
	return nil
}

type liveTemporalEvaluationService struct {
	freshness    FreshnessService
	trust        LiveTrustEvaluationService
	profiles     TrustRegistryRepository
	releases     ReleaseRepository
	deprecations DeprecationRepository
	clock        Clock
}

func NewLiveTemporalEvaluationService(
	freshness FreshnessService,
	trust LiveTrustEvaluationService,
	profiles TrustRegistryRepository,
	releases ReleaseRepository,
	deprecations DeprecationRepository,
	clock Clock,
) (LiveTemporalEvaluationService, error) {
	const operation = "configure live temporal evaluation"
	for _, dependency := range []struct {
		name  string
		value any
	}{{"freshness service", freshness}, {"trust evaluation service", trust}, {"authority profile repository", profiles},
		{"release repository", releases}, {"deprecation repository", deprecations}, {"clock", clock}} {
		if err := requireDependency(operation, dependency.name, dependency.value); err != nil {
			return nil, err
		}
	}
	return &liveTemporalEvaluationService{
		freshness: freshness, trust: trust, profiles: profiles, releases: releases,
		deprecations: deprecations, clock: clock,
	}, nil
}

func (service *liveTemporalEvaluationService) EvaluateTemporal(ctx context.Context, request LiveTemporalEvaluationRequest) (LiveTemporalEvaluationResult, error) {
	const operation = "evaluate live research freshness"
	if ctx == nil {
		return LiveTemporalEvaluationResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveTemporalEvaluationResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.Topic.Validate(); err != nil {
		return LiveTemporalEvaluationResult{}, invalid(operation, err)
	}
	if err := request.Purpose.Validate(); err != nil {
		return LiveTemporalEvaluationResult{}, invalid(operation, err)
	}
	if request.TargetVersion != nil {
		if err := request.TargetVersion.Validate(); err != nil {
			return LiveTemporalEvaluationResult{}, invalid(operation, err)
		}
	}
	sources, normalized, claims, citations, err := validateLiveTemporalInputsV1(request)
	if err != nil {
		return LiveTemporalEvaluationResult{}, invalid(operation, err)
	}
	profile, err := service.matchAuthorityProfile(ctx, request.Topic)
	if err != nil {
		return LiveTemporalEvaluationResult{}, err
	}
	releases, err := service.releases.List(ctx)
	if err != nil {
		return LiveTemporalEvaluationResult{}, repositoryError(operation, err)
	}
	if len(releases) > MaximumLiveTemporalIntelligenceRecords {
		return LiveTemporalEvaluationResult{}, invalid(operation, fmt.Errorf("release intelligence exceeds %d records", MaximumLiveTemporalIntelligenceRecords))
	}
	deprecations, err := service.deprecations.ListBySubject(ctx, request.Topic.Subject)
	if err != nil {
		return LiveTemporalEvaluationResult{}, repositoryError(operation, err)
	}
	if len(deprecations) > MaximumLiveTemporalIntelligenceRecords {
		return LiveTemporalEvaluationResult{}, invalid(operation, fmt.Errorf("deprecation intelligence exceeds %d records", MaximumLiveTemporalIntelligenceRecords))
	}

	result := LiveTemporalEvaluationResult{AlgorithmVersion: LiveTemporalEvaluationV1}
	sourceIDs := sortedLiveTemporalSourceIDsV1(sources)
	observations := make(map[research.SourceID]LiveTemporalSourceObservation, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		source := sources[sourceID]
		assessment, assessErr := temporalpolicy.AssessV1(temporalpolicy.Input{
			Source: source, Purpose: request.Purpose, TargetVersion: cloneSourceVersion(request.TargetVersion),
		})
		if assessErr != nil {
			return cloneLiveTemporalEvaluationResult(result), invalid(operation, assessErr)
		}
		publishedAt := cloneTimestampValue(source.Metadata.PublishedAt)
		updatedAt := cloneTimestampValue(source.Metadata.UpdatedAt)
		versionHints := []string(nil)
		if item, exists := normalized[sourceID]; exists {
			if item.PublishedAt != nil {
				publishedAt = cloneTimestampValue(item.PublishedAt)
			}
			if item.UpdatedAt != nil {
				updatedAt = cloneTimestampValue(item.UpdatedAt)
			}
			versionHints = append([]string(nil), item.VersionHints...)
		}
		observation := LiveTemporalSourceObservation{
			SourceID: sourceID, PublishedAt: publishedAt, UpdatedAt: updatedAt,
			VersionHints: versionHints, Temporal: assessment, AlgorithmVersion: LiveTemporalMetadataV1,
		}
		if err := observation.Validate(); err != nil {
			return cloneLiveTemporalEvaluationResult(result), invalid(operation, err)
		}
		observations[sourceID] = observation
		result.TemporalObservations = append(result.TemporalObservations, observation)
	}

	freshnessStates := make(map[research.SourceID]research.FreshnessState)
	model := freshnesspolicy.NewModelV1(service.clock)
	for _, claim := range claims {
		claimAssessments := make([]LiveClaimSourceFreshness, 0, len(claim.SourceIDs))
		for _, sourceID := range claim.SourceIDs {
			lastVerified, found := lastVerifiedForClaimSourceV1(claim, sourceID, citations)
			if !found {
				return cloneLiveTemporalEvaluationResult(result), invalid(operation, fmt.Errorf("Claim %q lacks a citation for Source %q", claim.ID, sourceID))
			}
			deprecationVerified := false
			if verifiedAt, matched := matchingDeprecationVerificationV1(claim, sourceID, deprecations); matched {
				deprecationVerified = true
				if verifiedAt.After(lastVerified) {
					lastVerified = verifiedAt
				}
			}
			releaseVerified := false
			if verifiedAt, matched := matchingReleaseVerificationV1(claim, sourceID, releases); matched {
				releaseVerified = true
				if verifiedAt.After(lastVerified) {
					lastVerified = verifiedAt
				}
			}
			baseline := request.TargetVersion
			if claim.VersionScope != nil {
				baseline = claim.VersionScope
			}
			knownNewRelease := knownNewReleaseV1(sourceID, baseline, releases)
			assessment, assessErr := model.Assess(freshnesspolicy.Input{
				LastVerifiedAt: &lastVerified, SourceUpdatedAt: cloneTimestampValue(observations[sourceID].UpdatedAt),
				ClaimType: claim.Type, SourceKind: sources[sourceID].Kind, KnownNewRelease: knownNewRelease,
				AuthorityProfile: profile,
			})
			if assessErr != nil {
				return cloneLiveTemporalEvaluationResult(result), invalid(operation, assessErr)
			}
			item := LiveClaimSourceFreshness{
				ClaimID: claim.ID, SourceID: sourceID, Assessment: assessment,
				KnownNewRelease: knownNewRelease, DeprecationVerified: deprecationVerified, ReleaseVerified: releaseVerified,
			}
			claimAssessments = append(claimAssessments, item)
			result.FreshnessAssessments = append(result.FreshnessAssessments, item)
			freshnessStates[sourceID] = conservativeFreshnessStateV1(freshnessStates[sourceID], assessment.State)
		}
		record, recordErr := aggregateClaimFreshnessV1(claim.ID, claimAssessments)
		if recordErr != nil {
			return cloneLiveTemporalEvaluationResult(result), invalid(operation, recordErr)
		}
		if saveErr := service.freshness.Save(ctx, record); saveErr != nil {
			return cloneLiveTemporalEvaluationResult(result), boundaryError(ErrorPersistenceFailure, operation, saveErr)
		}
		result.FreshnessRecords = append(result.FreshnessRecords, record)
	}

	trustResult, err := service.trust.EvaluateTrust(ctx, LiveTrustEvaluationRequest{
		Topic: request.Topic, Purpose: request.Purpose, Sources: request.Sources,
		Claims: request.Claims, FreshnessStates: freshnessStates,
	})
	if err != nil {
		return cloneLiveTemporalEvaluationResult(result), err
	}
	result.TrustDecisions = trustResult.Decisions
	if err := result.Validate(); err != nil {
		return LiveTemporalEvaluationResult{}, invalid(operation, err)
	}
	return cloneLiveTemporalEvaluationResult(result), nil
}

func validateLiveTemporalInputsV1(request LiveTemporalEvaluationRequest) (
	map[research.SourceID]research.Source, map[research.SourceID]NormalizedSource,
	[]research.Claim, map[research.ID]research.Citation, error,
) {
	if len(request.Sources) == 0 || len(request.Sources) > MaximumFetchesPerRun {
		return nil, nil, nil, nil, fmt.Errorf("temporal Sources must contain between 1 and %d entries", MaximumFetchesPerRun)
	}
	sources := make(map[research.SourceID]research.Source, len(request.Sources))
	for index, source := range request.Sources {
		if err := source.Validate(); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("temporal Source %d: %w", index, err)
		}
		if _, duplicate := sources[source.ID]; duplicate {
			return nil, nil, nil, nil, fmt.Errorf("temporal Source %q is repeated", source.ID)
		}
		sources[source.ID] = source
	}
	normalized := make(map[research.SourceID]NormalizedSource, len(request.NormalizedSources))
	for index, item := range request.NormalizedSources {
		if err := item.Validate(); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("temporal normalized Source %d: %w", index, err)
		}
		source, exists := sources[item.SourceID]
		if !exists || item.Locator != source.Locator {
			return nil, nil, nil, nil, fmt.Errorf("temporal normalized Source %q lacks its Source", item.SourceID)
		}
		if _, duplicate := normalized[item.SourceID]; duplicate {
			return nil, nil, nil, nil, fmt.Errorf("temporal normalized Source %q is repeated", item.SourceID)
		}
		normalized[item.SourceID] = item
	}
	if len(request.Claims) == 0 || len(request.Claims) > MaximumClaimCandidatesPerRun {
		return nil, nil, nil, nil, fmt.Errorf("temporal Claims must contain between 1 and %d entries", MaximumClaimCandidatesPerRun)
	}
	if len(request.Citations) == 0 || len(request.Citations) > MaximumClaimCandidatesPerRun {
		return nil, nil, nil, nil, fmt.Errorf("temporal citations must contain between 1 and %d entries", MaximumClaimCandidatesPerRun)
	}
	claims := make([]research.Claim, len(request.Claims))
	for index, claim := range request.Claims {
		if err := claim.Validate(); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("temporal Claim %d: %w", index, err)
		}
		for _, sourceID := range claim.SourceIDs {
			if _, exists := sources[sourceID]; !exists {
				return nil, nil, nil, nil, fmt.Errorf("temporal Claim %q references missing Source %q", claim.ID, sourceID)
			}
		}
		claims[index] = cloneLiveClaimV1(claim)
	}
	sort.Slice(claims, func(i, j int) bool { return claims[i].ID.String() < claims[j].ID.String() })
	citations := make(map[research.ID]research.Citation, len(request.Citations))
	for index, item := range request.Citations {
		if err := item.Validate(); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("temporal citation %d: %w", index, err)
		}
		if _, duplicate := citations[item.EvidenceID]; duplicate {
			return nil, nil, nil, nil, fmt.Errorf("temporal citation repeats Evidence %q", item.EvidenceID)
		}
		citations[item.EvidenceID] = item
	}
	return sources, normalized, claims, citations, nil
}

func (service *liveTemporalEvaluationService) matchAuthorityProfile(ctx context.Context, topic research.ResearchTopic) (*research.AuthorityProfile, error) {
	profiles, err := service.profiles.ListProfiles(ctx)
	if err != nil {
		return nil, repositoryError("match live freshness authority profile", err)
	}
	if len(profiles) == 0 {
		return nil, nil
	}
	catalog, err := authority.NewCatalog(profiles)
	if err != nil {
		return nil, invalid("match live freshness authority profile", err)
	}
	profile, found, err := catalog.Match(topic)
	if err != nil || !found {
		return nil, err
	}
	return &profile, nil
}

func sortedLiveTemporalSourceIDsV1(sources map[research.SourceID]research.Source) []research.SourceID {
	result := make([]research.SourceID, 0, len(sources))
	for sourceID := range sources {
		result = append(result, sourceID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func lastVerifiedForClaimSourceV1(claim research.Claim, sourceID research.SourceID, citations map[research.ID]research.Citation) (research.Timestamp, bool) {
	var result research.Timestamp
	found := false
	for _, evidenceID := range claim.EvidenceIDs {
		item, exists := citations[evidenceID]
		if !exists || item.SourceID != sourceID {
			continue
		}
		if !found || item.LastVerified.Before(result) {
			result, found = item.LastVerified, true
		}
	}
	return result, found
}

func matchingDeprecationVerificationV1(claim research.Claim, sourceID research.SourceID, records []research.DeprecationRecord) (research.Timestamp, bool) {
	if claim.Type != research.ClaimDeprecation {
		return research.Timestamp{}, false
	}
	var latest research.Timestamp
	found := false
	for _, record := range records {
		if !containsSourceID(record.SourceIDs, sourceID) || !idSetsOverlapV1(record.EvidenceIDs, claim.EvidenceIDs) {
			continue
		}
		if !found || record.VerifiedAt.After(latest) {
			latest, found = record.VerifiedAt, true
		}
	}
	return latest, found
}

func matchingReleaseVerificationV1(claim research.Claim, sourceID research.SourceID, records []research.ReleaseRecord) (research.Timestamp, bool) {
	if claim.Type != research.ClaimVersionChange || claim.VersionScope == nil {
		return research.Timestamp{}, false
	}
	var latest research.Timestamp
	found := false
	for _, record := range records {
		if record.Version.String() != claim.VersionScope.String() || !containsSourceID(record.SourceIDs, sourceID) {
			continue
		}
		if !found || record.VerifiedAt.After(latest) {
			latest, found = record.VerifiedAt, true
		}
	}
	return latest, found
}

func knownNewReleaseV1(sourceID research.SourceID, baseline *research.SourceVersion, records []research.ReleaseRecord) bool {
	if baseline == nil {
		return false
	}
	for _, record := range records {
		if record.Status == research.ReleaseCurrent && record.Version.String() != baseline.String() && containsSourceID(record.SourceIDs, sourceID) {
			return true
		}
	}
	return false
}

func idSetsOverlapV1(left, right []research.ID) bool {
	for _, candidate := range left {
		for _, value := range right {
			if candidate == value {
				return true
			}
		}
	}
	return false
}

func aggregateClaimFreshnessV1(claimID research.ClaimID, assessments []LiveClaimSourceFreshness) (FreshnessRecord, error) {
	if len(assessments) == 0 {
		return FreshnessRecord{}, errors.New("cannot aggregate empty Claim freshness")
	}
	state := assessments[0].Assessment.State
	score := assessments[0].Assessment.Score
	lastVerified := *assessments[0].Assessment.LastVerifiedAt
	for _, item := range assessments[1:] {
		state = conservativeFreshnessStateV1(state, item.Assessment.State)
		if item.Assessment.Score.Value() < score.Value() {
			score = item.Assessment.Score
		}
		if item.Assessment.LastVerifiedAt.Before(lastVerified) {
			lastVerified = *item.Assessment.LastVerifiedAt
		}
	}
	subjectID, err := research.NewID(claimID.String())
	if err != nil {
		return FreshnessRecord{}, err
	}
	record := FreshnessRecord{
		SubjectID: subjectID, State: state, Score: score, LastVerifiedAt: lastVerified,
		AlgorithmVersion: research.FreshnessAlgorithmV1,
	}
	return record, record.Validate()
}

func conservativeFreshnessStateV1(left, right research.FreshnessState) research.FreshnessState {
	if left == "" {
		return right
	}
	rank := map[research.FreshnessState]int{
		research.FreshnessFresh: 0, research.FreshnessAging: 1,
		research.FreshnessStale: 2, research.FreshnessUnknown: 3,
	}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func (service *liveTemporalEvaluationService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.EvaluateTemporal(ctx, LiveTemporalEvaluationRequest{
		Topic: input.Request.Topic, Purpose: input.Request.Purpose, TargetVersion: cloneSourceVersion(input.Request.TargetVersion),
		Sources: input.Artifacts.Sources, NormalizedSources: input.Artifacts.NormalizedSources,
		Claims: input.Artifacts.Claims, Citations: input.Artifacts.Citations,
	})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.TemporalObservations = make([]LiveTemporalSourceObservation, len(result.TemporalObservations))
	for index, item := range result.TemporalObservations {
		artifacts.TemporalObservations[index] = cloneLiveTemporalSourceObservation(item)
	}
	artifacts.FreshnessAssessments = make([]LiveClaimSourceFreshness, len(result.FreshnessAssessments))
	for index, item := range result.FreshnessAssessments {
		artifacts.FreshnessAssessments[index] = cloneLiveClaimSourceFreshness(item)
	}
	artifacts.FreshnessRecords = make([]FreshnessRecord, len(result.FreshnessRecords))
	for index, item := range result.FreshnessRecords {
		artifacts.FreshnessRecords[index] = cloneFreshnessRecordArtifact(item)
	}
	artifacts.TrustDecisions = make([]research.TrustDecision, len(result.TrustDecisions))
	for index, item := range result.TrustDecisions {
		artifacts.TrustDecisions[index] = cloneTrustDecisionArtifact(item)
	}
	return artifacts, err
}

func cloneLiveTemporalSourceObservation(item LiveTemporalSourceObservation) LiveTemporalSourceObservation {
	clone := item
	clone.PublishedAt = cloneTimestampValue(item.PublishedAt)
	clone.UpdatedAt = cloneTimestampValue(item.UpdatedAt)
	clone.VersionHints = append([]string(nil), item.VersionHints...)
	clone.Temporal.SourceVersion = cloneSourceVersion(item.Temporal.SourceVersion)
	clone.Temporal.TargetVersion = cloneSourceVersion(item.Temporal.TargetVersion)
	return clone
}

func cloneLiveClaimSourceFreshness(item LiveClaimSourceFreshness) LiveClaimSourceFreshness {
	clone := item
	clone.Assessment.LastVerifiedAt = cloneTimestampValue(item.Assessment.LastVerifiedAt)
	clone.Assessment.Reasons = append([]freshnesspolicy.Reason(nil), item.Assessment.Reasons...)
	return clone
}

func cloneFreshnessRecordArtifact(item FreshnessRecord) FreshnessRecord {
	clone := item
	clone.NextVerifyAt = cloneTimestampValue(item.NextVerifyAt)
	return clone
}

func cloneLiveTemporalEvaluationResult(result LiveTemporalEvaluationResult) LiveTemporalEvaluationResult {
	clone := result
	clone.TemporalObservations = make([]LiveTemporalSourceObservation, len(result.TemporalObservations))
	for index, item := range result.TemporalObservations {
		clone.TemporalObservations[index] = cloneLiveTemporalSourceObservation(item)
	}
	clone.FreshnessAssessments = make([]LiveClaimSourceFreshness, len(result.FreshnessAssessments))
	for index, item := range result.FreshnessAssessments {
		clone.FreshnessAssessments[index] = cloneLiveClaimSourceFreshness(item)
	}
	clone.FreshnessRecords = make([]FreshnessRecord, len(result.FreshnessRecords))
	for index, item := range result.FreshnessRecords {
		clone.FreshnessRecords[index] = cloneFreshnessRecordArtifact(item)
	}
	clone.TrustDecisions = make([]research.TrustDecision, len(result.TrustDecisions))
	for index, item := range result.TrustDecisions {
		clone.TrustDecisions[index] = cloneTrustDecisionArtifact(item)
	}
	return clone
}

var _ LiveTemporalEvaluationService = (*liveTemporalEvaluationService)(nil)
