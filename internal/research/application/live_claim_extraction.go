package application

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/citation"
)

const LiveClaimExtractionV1 = "live-claim-extraction-v1"

type LiveClaimExtractionRequest struct {
	Topic         research.ResearchTopic
	Purpose       research.ResearchPurpose
	TargetVersion *research.SourceVersion
	Sources       []research.Source
	Snapshots     []research.SourceSnapshot
	Evidence      []research.Evidence
}

type LiveClaimExtractionResult struct {
	Candidates       []ClaimCandidate
	Claims           []research.Claim
	Citations        []research.Citation
	AlgorithmVersion string
}

func (result LiveClaimExtractionResult) Validate() error {
	if len(result.Candidates) == 0 || len(result.Claims) == 0 || len(result.Citations) == 0 {
		return errors.New("live claim extraction requires candidates, Claims, and citations")
	}
	if len(result.Candidates) > MaximumClaimCandidatesPerRun || len(result.Claims) > MaximumClaimCandidatesPerRun {
		return fmt.Errorf("live claim extraction exceeds %d candidates or Claims", MaximumClaimCandidatesPerRun)
	}
	cited := make(map[research.ID]struct{}, len(result.Citations))
	for index, item := range result.Citations {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("live claim citation %d: %w", index, err)
		}
		if _, duplicate := cited[item.EvidenceID]; duplicate {
			return fmt.Errorf("live claim extraction repeats a citation for Evidence %q", item.EvidenceID)
		}
		cited[item.EvidenceID] = struct{}{}
	}
	seenClaims := make(map[research.ClaimID]struct{}, len(result.Claims))
	for index, claim := range result.Claims {
		if err := claim.Validate(); err != nil {
			return fmt.Errorf("live Claim %d: %w", index, err)
		}
		if _, duplicate := seenClaims[claim.ID]; duplicate {
			return fmt.Errorf("live claim extraction repeats Claim %q", claim.ID)
		}
		seenClaims[claim.ID] = struct{}{}
		for _, evidenceID := range claim.EvidenceIDs {
			if _, exists := cited[evidenceID]; !exists {
				return fmt.Errorf("live Claim %q lacks a citation for Evidence %q", claim.ID, evidenceID)
			}
		}
	}
	for index, candidate := range result.Candidates {
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("live claim candidate %d: %w", index, err)
		}
	}
	if result.AlgorithmVersion != LiveClaimExtractionV1 {
		return fmt.Errorf("live claim extraction algorithm must be %q", LiveClaimExtractionV1)
	}
	return nil
}

type liveClaimExtractionService struct {
	extractor ClaimExtractor
	evidence  EvidenceRepository
	claims    ClaimRepository
	citations CitationRepository
	clock     Clock
}

func NewLiveClaimExtractionService(
	extractor ClaimExtractor,
	evidence EvidenceRepository,
	claims ClaimRepository,
	citations CitationRepository,
	clock Clock,
) (LiveClaimExtractionService, error) {
	const operation = "configure live claim extraction"
	for _, dependency := range []struct {
		name  string
		value any
	}{{"claim extractor", extractor}, {"evidence repository", evidence}, {"claim repository", claims},
		{"citation repository", citations}, {"clock", clock}} {
		if err := requireDependency(operation, dependency.name, dependency.value); err != nil {
			return nil, err
		}
	}
	return &liveClaimExtractionService{extractor: extractor, evidence: evidence, claims: claims, citations: citations, clock: clock}, nil
}

type liveClaimAggregateV1 struct {
	candidate   ClaimCandidate
	sourceIDs   []research.SourceID
	evidenceIDs []research.ID
}

func (service *liveClaimExtractionService) ExtractClaims(ctx context.Context, request LiveClaimExtractionRequest) (LiveClaimExtractionResult, error) {
	const operation = "extract live research Claims"
	if ctx == nil {
		return LiveClaimExtractionResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveClaimExtractionResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.Topic.Validate(); err != nil {
		return LiveClaimExtractionResult{}, invalid(operation, err)
	}
	if err := request.Purpose.Validate(); err != nil {
		return LiveClaimExtractionResult{}, invalid(operation, err)
	}
	if request.TargetVersion != nil {
		if err := request.TargetVersion.Validate(); err != nil {
			return LiveClaimExtractionResult{}, invalid(operation, err)
		}
	}
	if len(request.Evidence) == 0 || len(request.Evidence) > MaximumClaimCandidatesPerRun {
		return LiveClaimExtractionResult{}, invalid(operation, fmt.Errorf("Evidence count must be between 1 and %d", MaximumClaimCandidatesPerRun))
	}
	sources, snapshots, evidenceByID, err := service.validateInputs(ctx, request)
	if err != nil {
		return LiveClaimExtractionResult{}, err
	}
	createdAt := service.clock.Now()
	if err := createdAt.Validate(); err != nil {
		return LiveClaimExtractionResult{}, invalid(operation, fmt.Errorf("claim extraction clock: %w", err))
	}

	result := LiveClaimExtractionResult{AlgorithmVersion: LiveClaimExtractionV1}
	aggregates := make([]liveClaimAggregateV1, 0, len(request.Evidence))
	byKey := make(map[string]int, len(request.Evidence))
	for _, item := range request.Evidence {
		if createdAt.Before(item.ExtractedAt) {
			return cloneLiveClaimExtractionResult(result), invalid(operation, errors.New("claim extraction precedes Evidence extraction"))
		}
		extracted, extractErr := service.extractor.Extract(ctx, ClaimExtractionRequest{
			Topic: request.Topic, Purpose: request.Purpose, TargetVersion: cloneSourceVersion(request.TargetVersion), Evidence: item,
		})
		if extractErr != nil {
			return cloneLiveClaimExtractionResult(result), boundaryError(ErrorInvalidState, operation, extractErr)
		}
		if err := extracted.Validate(); err != nil {
			return cloneLiveClaimExtractionResult(result), invalid(operation, err)
		}
		if len(result.Candidates)+len(extracted.Candidates) > MaximumClaimCandidatesPerRun {
			return cloneLiveClaimExtractionResult(result), invalid(operation, fmt.Errorf("live claim candidate count exceeds %d", MaximumClaimCandidatesPerRun))
		}
		for _, candidate := range extracted.Candidates {
			if candidate.SourceID != item.SourceID || candidate.SnapshotID != item.SnapshotID || candidate.EvidenceID != item.ID {
				return cloneLiveClaimExtractionResult(result), invalid(operation, errors.New("claim extractor changed Evidence identity"))
			}
			result.Candidates = append(result.Candidates, cloneClaimCandidate(candidate))
			key := claimCandidateSemanticKeyV1(candidate)
			aggregateIndex, exists := byKey[key]
			if !exists {
				byKey[key] = len(aggregates)
				aggregates = append(aggregates, liveClaimAggregateV1{candidate: candidate})
				aggregateIndex = len(aggregates) - 1
			}
			aggregate := &aggregates[aggregateIndex]
			aggregate.sourceIDs = appendUniqueSourceIDV1(aggregate.sourceIDs, candidate.SourceID)
			aggregate.evidenceIDs = appendUniqueResearchIDV1(aggregate.evidenceIDs, candidate.EvidenceID)
		}
	}
	if len(aggregates) == 0 {
		return cloneLiveClaimExtractionResult(result), invalid(operation, errors.New("Evidence contains no unambiguous claim statement"))
	}

	for _, aggregate := range aggregates {
		for _, evidenceID := range aggregate.evidenceIDs {
			if _, already := citationForEvidenceV1(result.Citations, evidenceID); already {
				continue
			}
			item := evidenceByID[evidenceID]
			generated, persistErr := service.persistCitation(ctx, sources[item.SourceID], snapshots[item.SnapshotID], item, createdAt)
			if persistErr != nil {
				return cloneLiveClaimExtractionResult(result), persistErr
			}
			result.Citations = append(result.Citations, generated)
		}
		claim, persistErr := service.persistClaim(ctx, request.Topic, aggregate, createdAt)
		if persistErr != nil {
			return cloneLiveClaimExtractionResult(result), persistErr
		}
		result.Claims = append(result.Claims, claim)
	}
	if err := result.Validate(); err != nil {
		return LiveClaimExtractionResult{}, invalid(operation, err)
	}
	return cloneLiveClaimExtractionResult(result), nil
}

func (service *liveClaimExtractionService) validateInputs(ctx context.Context, request LiveClaimExtractionRequest) (
	map[research.SourceID]research.Source, map[research.ID]research.SourceSnapshot, map[research.ID]research.Evidence, error,
) {
	const operation = "validate live claim extraction inputs"
	sources := make(map[research.SourceID]research.Source, len(request.Sources))
	for index, source := range request.Sources {
		if err := source.Validate(); err != nil {
			return nil, nil, nil, invalid(operation, fmt.Errorf("source %d: %w", index, err))
		}
		if _, duplicate := sources[source.ID]; duplicate {
			return nil, nil, nil, invalid(operation, fmt.Errorf("source %q is repeated", source.ID))
		}
		sources[source.ID] = source
	}
	snapshots := make(map[research.ID]research.SourceSnapshot, len(request.Snapshots))
	for index, snapshot := range request.Snapshots {
		if err := snapshot.Validate(); err != nil {
			return nil, nil, nil, invalid(operation, fmt.Errorf("snapshot %d: %w", index, err))
		}
		if _, duplicate := snapshots[snapshot.ID]; duplicate {
			return nil, nil, nil, invalid(operation, fmt.Errorf("snapshot %q is repeated", snapshot.ID))
		}
		snapshots[snapshot.ID] = snapshot
	}
	evidenceByID := make(map[research.ID]research.Evidence, len(request.Evidence))
	evidenceBySource := make(map[research.SourceID]int, len(sources))
	for index, item := range request.Evidence {
		if err := item.Validate(); err != nil {
			return nil, nil, nil, invalid(operation, fmt.Errorf("Evidence %d: %w", index, err))
		}
		if _, duplicate := evidenceByID[item.ID]; duplicate {
			return nil, nil, nil, invalid(operation, fmt.Errorf("Evidence %q is repeated", item.ID))
		}
		source, sourceExists := sources[item.SourceID]
		snapshot, snapshotExists := snapshots[item.SnapshotID]
		if !sourceExists || !snapshotExists || snapshot.SourceID != source.ID || snapshot.Locator != source.Locator {
			return nil, nil, nil, invalid(operation, fmt.Errorf("Evidence %q lacks an exact Source/snapshot chain", item.ID))
		}
		evidenceBySource[item.SourceID]++
		if evidenceBySource[item.SourceID] > MaximumEvidenceCandidatesPerSource {
			return nil, nil, nil, invalid(operation, fmt.Errorf(
				"source %q exceeds %d Evidence and the %d Claim candidate bound",
				item.SourceID, MaximumEvidenceCandidatesPerSource, MaximumClaimCandidatesPerSource,
			))
		}
		stored, err := service.evidence.Get(ctx, item.ID)
		if err != nil {
			return nil, nil, nil, repositoryError(operation, err)
		}
		if !reflect.DeepEqual(stored, item) {
			return nil, nil, nil, invalid(operation, fmt.Errorf("Evidence %q does not match its persisted record", item.ID))
		}
		evidenceByID[item.ID] = item
	}
	return sources, snapshots, evidenceByID, nil
}

func (service *liveClaimExtractionService) persistClaim(ctx context.Context, topic research.ResearchTopic, aggregate liveClaimAggregateV1, at research.Timestamp) (research.Claim, error) {
	const operation = "persist live research Claim"
	candidate := aggregate.candidate
	claim := research.Claim{
		ID:    stableClaimID("claim.live", candidate.ExtractorVersion, claimCandidateSemanticKeyV1(candidate)),
		Topic: topic, Statement: candidate.Statement, Type: candidate.Family.ClaimType(), Scope: candidate.Scope,
		VersionScope: cloneSourceVersion(candidate.VersionScope), StatusScope: candidate.StatusScope, Confidence: candidate.Confidence,
		SourceIDs: append([]research.SourceID(nil), aggregate.sourceIDs...), EvidenceIDs: append([]research.ID(nil), aggregate.evidenceIDs...), CreatedAt: at,
	}
	if err := claim.Validate(); err != nil {
		return research.Claim{}, invalid(operation, err)
	}
	existing, err := service.claims.Get(ctx, claim.ID)
	if err == nil {
		if !sameLiveClaimV1(existing, claim) {
			return research.Claim{}, invalid(operation, errors.New("stable Claim ID identifies different content"))
		}
		return cloneLiveClaimV1(existing), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return research.Claim{}, repositoryError(operation, err)
	}
	if err := service.claims.Append(ctx, claim); err != nil {
		if errors.Is(err, ErrConflict) {
			existing, getErr := service.claims.Get(ctx, claim.ID)
			if getErr == nil && sameLiveClaimV1(existing, claim) {
				return cloneLiveClaimV1(existing), nil
			}
		}
		return research.Claim{}, repositoryError(operation, err)
	}
	return cloneLiveClaimV1(claim), nil
}

func (service *liveClaimExtractionService) persistCitation(ctx context.Context, source research.Source, snapshot research.SourceSnapshot, item research.Evidence, at research.Timestamp) (research.Citation, error) {
	const operation = "persist live research citation"
	id := stableResearchID("citation.live", ClaimExtractorV1, item.ID.String())
	generated, err := citation.GenerateV1(citation.Request{
		ID: id, Source: source, Snapshot: snapshot, Evidence: item, LastVerified: at,
		Target: citation.Target{Section: item.Location},
	})
	if err != nil {
		return research.Citation{}, invalid(operation, err)
	}
	existing, err := service.citations.Get(ctx, id)
	if err == nil {
		if !sameLiveCitationV1(existing, generated) {
			return research.Citation{}, invalid(operation, errors.New("stable citation ID identifies different Evidence"))
		}
		return cloneCitationArtifact(existing), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return research.Citation{}, repositoryError(operation, err)
	}
	if err := service.citations.Append(ctx, generated); err != nil {
		if errors.Is(err, ErrConflict) {
			existing, getErr := service.citations.Get(ctx, id)
			if getErr == nil && sameLiveCitationV1(existing, generated) {
				return cloneCitationArtifact(existing), nil
			}
		}
		return research.Citation{}, repositoryError(operation, err)
	}
	return cloneCitationArtifact(generated), nil
}

func (service *liveClaimExtractionService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.ExtractClaims(ctx, LiveClaimExtractionRequest{
		Topic: input.Request.Topic, Purpose: input.Request.Purpose, TargetVersion: cloneSourceVersion(input.Request.TargetVersion),
		Sources: input.Artifacts.Sources, Snapshots: input.Artifacts.Snapshots, Evidence: input.Artifacts.Evidence,
	})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.ClaimCandidates = make([]ClaimCandidate, len(result.Candidates))
	for index, candidate := range result.Candidates {
		artifacts.ClaimCandidates[index] = cloneClaimCandidate(candidate)
	}
	artifacts.Claims = make([]research.Claim, len(result.Claims))
	for index, claim := range result.Claims {
		artifacts.Claims[index] = cloneLiveClaimV1(claim)
	}
	artifacts.Citations = make([]research.Citation, len(result.Citations))
	for index, item := range result.Citations {
		artifacts.Citations[index] = cloneCitationArtifact(item)
	}
	return artifacts, err
}

type liveResearchExtractionStage struct {
	stages []LiveResearchStageService
}

func NewLiveResearchExtractionStage(evidence, claims LiveResearchStageService, afterClaims ...LiveResearchStageService) (LiveResearchStageService, error) {
	const operation = "configure live research extraction stage"
	if err := requireDependency(operation, "evidence extraction stage", evidence); err != nil {
		return nil, err
	}
	if err := requireDependency(operation, "claim extraction stage", claims); err != nil {
		return nil, err
	}
	for index, stage := range afterClaims {
		if err := requireDependency(operation, fmt.Sprintf("post-Claim stage %d", index), stage); err != nil {
			return nil, err
		}
	}
	stages := append([]LiveResearchStageService{evidence, claims}, afterClaims...)
	return &liveResearchExtractionStage{stages: stages}, nil
}

func (stage *liveResearchExtractionStage) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	for _, service := range stage.stages {
		input.Artifacts = artifacts
		var err error
		artifacts, err = service.Execute(ctx, input)
		if err != nil {
			return artifacts, err
		}
	}
	return artifacts, nil
}

func appendUniqueSourceIDV1(values []research.SourceID, candidate research.SourceID) []research.SourceID {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func appendUniqueResearchIDV1(values []research.ID, candidate research.ID) []research.ID {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func citationForEvidenceV1(values []research.Citation, evidenceID research.ID) (research.Citation, bool) {
	for _, value := range values {
		if value.EvidenceID == evidenceID {
			return value, true
		}
	}
	return research.Citation{}, false
}

func sameLiveClaimV1(left, right research.Claim) bool {
	left.CreatedAt = research.Timestamp{}
	right.CreatedAt = research.Timestamp{}
	return reflect.DeepEqual(left, right)
}

func sameLiveCitationV1(left, right research.Citation) bool {
	left.LastVerified = research.Timestamp{}
	right.LastVerified = research.Timestamp{}
	return reflect.DeepEqual(left, right)
}

func cloneLiveClaimV1(claim research.Claim) research.Claim {
	clone := claim
	clone.VersionScope = cloneSourceVersion(claim.VersionScope)
	clone.SourceIDs = append([]research.SourceID(nil), claim.SourceIDs...)
	clone.EvidenceIDs = append([]research.ID(nil), claim.EvidenceIDs...)
	return clone
}

func cloneLiveClaimExtractionResult(result LiveClaimExtractionResult) LiveClaimExtractionResult {
	clone := result
	clone.Candidates = make([]ClaimCandidate, len(result.Candidates))
	for index, candidate := range result.Candidates {
		clone.Candidates[index] = cloneClaimCandidate(candidate)
	}
	clone.Claims = make([]research.Claim, len(result.Claims))
	for index, claim := range result.Claims {
		clone.Claims[index] = cloneLiveClaimV1(claim)
	}
	clone.Citations = make([]research.Citation, len(result.Citations))
	for index, item := range result.Citations {
		clone.Citations[index] = cloneCitationArtifact(item)
	}
	return clone
}

var (
	_ LiveClaimExtractionService = (*liveClaimExtractionService)(nil)
	_ LiveResearchStageService   = (*liveResearchExtractionStage)(nil)
)
