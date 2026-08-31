package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

const LiveEvidenceExtractionV1 = "live-evidence-extraction-v1"

type LiveEvidenceExtractionRequest struct {
	Topic             research.ResearchTopic
	Purpose           research.ResearchPurpose
	TargetVersion     *research.SourceVersion
	Sources           []research.Source
	Snapshots         []research.SourceSnapshot
	NormalizedSources []NormalizedSource
}

type LiveEvidenceExtractionResult struct {
	Candidates       []EvidenceCandidate
	Evidence         []research.Evidence
	AlgorithmVersion string
}

func (result LiveEvidenceExtractionResult) Validate() error {
	if len(result.Candidates) == 0 || len(result.Evidence) == 0 || len(result.Candidates) != len(result.Evidence) {
		return errors.New("live evidence extraction requires one persisted Evidence per candidate")
	}
	if len(result.Candidates) > MaximumEvidenceCandidatesPerRun {
		return fmt.Errorf("live evidence candidate count exceeds %d", MaximumEvidenceCandidatesPerRun)
	}
	seen := make(map[research.ID]struct{}, len(result.Evidence))
	for index := range result.Candidates {
		candidate := result.Candidates[index]
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("live evidence candidate %d: %w", index, err)
		}
		evidence := result.Evidence[index]
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("live Evidence %d: %w", index, err)
		}
		if evidence.SourceID != candidate.SourceID || evidence.SnapshotID != candidate.SnapshotID ||
			evidence.Location != candidate.Location || evidence.Excerpt != candidate.Excerpt ||
			evidence.ExcerptHash != candidate.ExcerptHash || evidence.ContextBefore != candidate.ContextBefore ||
			evidence.ContextAfter != candidate.ContextAfter || evidence.ExtractorVersion != candidate.ExtractorVersion {
			return fmt.Errorf("live Evidence %d does not match its candidate", index)
		}
		if _, duplicate := seen[evidence.ID]; duplicate {
			return fmt.Errorf("live evidence repeats ID %q", evidence.ID)
		}
		seen[evidence.ID] = struct{}{}
	}
	if result.AlgorithmVersion != LiveEvidenceExtractionV1 {
		return fmt.Errorf("live evidence extraction algorithm must be %q", LiveEvidenceExtractionV1)
	}
	return nil
}

type liveEvidenceExtractionService struct {
	extractor EvidenceExtractor
	evidence  EvidenceRepository
	clock     Clock
}

func NewLiveEvidenceExtractionService(extractor EvidenceExtractor, evidence EvidenceRepository, clock Clock) (LiveEvidenceExtractionService, error) {
	const operation = "configure live evidence extraction"
	for _, dependency := range []struct {
		name  string
		value any
	}{
		{"evidence extractor", extractor},
		{"evidence repository", evidence},
		{"clock", clock},
	} {
		if err := requireDependency(operation, dependency.name, dependency.value); err != nil {
			return nil, err
		}
	}
	return &liveEvidenceExtractionService{extractor: extractor, evidence: evidence, clock: clock}, nil
}

func (service *liveEvidenceExtractionService) ExtractEvidence(ctx context.Context, request LiveEvidenceExtractionRequest) (LiveEvidenceExtractionResult, error) {
	const operation = "extract live research evidence"
	if ctx == nil {
		return LiveEvidenceExtractionResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveEvidenceExtractionResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.Topic.Validate(); err != nil {
		return LiveEvidenceExtractionResult{}, invalid(operation, err)
	}
	if err := request.Purpose.Validate(); err != nil {
		return LiveEvidenceExtractionResult{}, invalid(operation, err)
	}
	if request.TargetVersion != nil {
		if err := request.TargetVersion.Validate(); err != nil {
			return LiveEvidenceExtractionResult{}, invalid(operation, err)
		}
	}
	if len(request.NormalizedSources) == 0 || len(request.NormalizedSources) > MaximumFetchesPerRun {
		return LiveEvidenceExtractionResult{}, invalid(operation, fmt.Errorf("normalized source count must be between 1 and %d", MaximumFetchesPerRun))
	}

	sources, snapshots, err := validateEvidenceExtractionInputs(request)
	if err != nil {
		return LiveEvidenceExtractionResult{}, invalid(operation, err)
	}
	extractedAt := service.clock.Now()
	if err := extractedAt.Validate(); err != nil {
		return LiveEvidenceExtractionResult{}, invalid(operation, fmt.Errorf("evidence extraction clock: %w", err))
	}
	result := LiveEvidenceExtractionResult{AlgorithmVersion: LiveEvidenceExtractionV1}
	for _, normalized := range request.NormalizedSources {
		if err := ctx.Err(); err != nil {
			return cloneLiveEvidenceExtractionResult(result), Classify(ErrorUnavailable, operation, err)
		}
		source := sources[normalized.SourceID]
		// Generic v1 cannot satisfy the reviewed line/permalink contract required
		// for a Source explicitly classified as source_code.
		if source.Kind == research.SourceCode {
			continue
		}
		snapshot := snapshots[normalized.SourceID]
		if extractedAt.Before(snapshot.FetchedAt) {
			return cloneLiveEvidenceExtractionResult(result), invalid(operation, errors.New("evidence extraction precedes source snapshot"))
		}
		extraction, extractErr := service.extractor.Extract(ctx, EvidenceExtractionRequest{
			Topic: request.Topic, Purpose: request.Purpose, TargetVersion: cloneSourceVersion(request.TargetVersion),
			Source: cloneNormalizedSource(normalized), Snapshot: snapshot,
		})
		if extractErr != nil {
			return cloneLiveEvidenceExtractionResult(result), boundaryError(ErrorInvalidState, operation, extractErr)
		}
		if err := extraction.Validate(); err != nil {
			return cloneLiveEvidenceExtractionResult(result), invalid(operation, err)
		}
		if len(result.Candidates)+len(extraction.Candidates) > MaximumEvidenceCandidatesPerRun {
			return cloneLiveEvidenceExtractionResult(result), invalid(operation, fmt.Errorf("live evidence candidate count exceeds %d", MaximumEvidenceCandidatesPerRun))
		}
		for _, candidate := range extraction.Candidates {
			if candidate.SourceID != normalized.SourceID || candidate.SnapshotID != snapshot.ID {
				return cloneLiveEvidenceExtractionResult(result), invalid(operation, errors.New("extractor candidate changed source or snapshot identity"))
			}
			evidence, persistErr := service.persistCandidate(ctx, candidate, extractedAt)
			if persistErr != nil {
				return cloneLiveEvidenceExtractionResult(result), persistErr
			}
			result.Candidates = append(result.Candidates, cloneEvidenceCandidate(candidate))
			result.Evidence = append(result.Evidence, evidence)
		}
	}
	if len(result.Evidence) == 0 {
		return cloneLiveEvidenceExtractionResult(result), invalid(operation, errors.New("normalized sources contain no relevant evidence"))
	}
	if err := result.Validate(); err != nil {
		return LiveEvidenceExtractionResult{}, invalid(operation, err)
	}
	return cloneLiveEvidenceExtractionResult(result), nil
}

func validateEvidenceExtractionInputs(request LiveEvidenceExtractionRequest) (map[research.SourceID]research.Source, map[research.SourceID]research.SourceSnapshot, error) {
	sources := make(map[research.SourceID]research.Source, len(request.Sources))
	for index, source := range request.Sources {
		if err := source.Validate(); err != nil {
			return nil, nil, fmt.Errorf("evidence source %d: %w", index, err)
		}
		if _, duplicate := sources[source.ID]; duplicate {
			return nil, nil, fmt.Errorf("evidence source %q is repeated", source.ID)
		}
		sources[source.ID] = source
	}
	snapshots := make(map[research.SourceID]research.SourceSnapshot, len(request.Snapshots))
	for index, snapshot := range request.Snapshots {
		if err := snapshot.Validate(); err != nil {
			return nil, nil, fmt.Errorf("evidence snapshot %d: %w", index, err)
		}
		if _, duplicate := snapshots[snapshot.SourceID]; duplicate {
			return nil, nil, fmt.Errorf("evidence snapshot for source %q is repeated", snapshot.SourceID)
		}
		snapshots[snapshot.SourceID] = snapshot
	}
	seenNormalized := make(map[research.SourceID]struct{}, len(request.NormalizedSources))
	for index, normalized := range request.NormalizedSources {
		if err := normalized.Validate(); err != nil {
			return nil, nil, fmt.Errorf("normalized evidence source %d: %w", index, err)
		}
		if _, duplicate := seenNormalized[normalized.SourceID]; duplicate {
			return nil, nil, fmt.Errorf("normalized evidence source %q is repeated", normalized.SourceID)
		}
		seenNormalized[normalized.SourceID] = struct{}{}
		source, sourceExists := sources[normalized.SourceID]
		snapshot, snapshotExists := snapshots[normalized.SourceID]
		if !sourceExists || !snapshotExists || source.Locator != normalized.Locator || snapshot.Locator != normalized.Locator {
			return nil, nil, fmt.Errorf("normalized evidence source %q lacks an exact Source/snapshot chain", normalized.SourceID)
		}
	}
	return sources, snapshots, nil
}

func (service *liveEvidenceExtractionService) persistCandidate(ctx context.Context, candidate EvidenceCandidate, extractedAt research.Timestamp) (research.Evidence, error) {
	const operation = "persist live research evidence"
	id := stableResearchID("evidence.live", candidate.ExtractorVersion, candidate.SourceID.String(), candidate.SnapshotID.String(), candidate.Location, candidate.ExcerptHash)
	item := research.Evidence{
		ID: id, SourceID: candidate.SourceID, SnapshotID: candidate.SnapshotID,
		Location: candidate.Location, Excerpt: candidate.Excerpt, ExcerptHash: candidate.ExcerptHash,
		ContextBefore: candidate.ContextBefore, ContextAfter: candidate.ContextAfter,
		ExtractedAt: extractedAt, ExtractorVersion: candidate.ExtractorVersion,
	}
	if err := item.Validate(); err != nil {
		return research.Evidence{}, invalid(operation, err)
	}
	existing, err := service.evidence.Get(ctx, id)
	if err == nil {
		if !sameExtractedEvidence(existing, item) {
			return research.Evidence{}, invalid(operation, errors.New("stable evidence ID identifies different content"))
		}
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return research.Evidence{}, repositoryError(operation, err)
	}
	if err := service.evidence.Append(ctx, item); err != nil {
		if errors.Is(err, ErrConflict) {
			existing, getErr := service.evidence.Get(ctx, id)
			if getErr == nil && sameExtractedEvidence(existing, item) {
				return existing, nil
			}
		}
		return research.Evidence{}, repositoryError(operation, err)
	}
	return item, nil
}

func sameExtractedEvidence(left, right research.Evidence) bool {
	return left.ID == right.ID && left.SourceID == right.SourceID && left.SnapshotID == right.SnapshotID &&
		left.Location == right.Location && left.Excerpt == right.Excerpt && left.ExcerptHash == right.ExcerptHash &&
		left.ContextBefore == right.ContextBefore && left.ContextAfter == right.ContextAfter &&
		left.ExtractorVersion == right.ExtractorVersion && left.SourceCode == nil && right.SourceCode == nil
}

func cloneLiveEvidenceExtractionResult(result LiveEvidenceExtractionResult) LiveEvidenceExtractionResult {
	clone := result
	clone.Candidates = make([]EvidenceCandidate, len(result.Candidates))
	for index, candidate := range result.Candidates {
		clone.Candidates[index] = cloneEvidenceCandidate(candidate)
	}
	clone.Evidence = append([]research.Evidence(nil), result.Evidence...)
	return clone
}

func (service *liveEvidenceExtractionService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.ExtractEvidence(ctx, LiveEvidenceExtractionRequest{
		Topic: input.Request.Topic, Purpose: input.Request.Purpose, TargetVersion: cloneSourceVersion(input.Request.TargetVersion),
		Sources: input.Artifacts.Sources, Snapshots: input.Artifacts.Snapshots, NormalizedSources: input.Artifacts.NormalizedSources,
	})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.EvidenceCandidates = make([]EvidenceCandidate, len(result.Candidates))
	for index, candidate := range result.Candidates {
		artifacts.EvidenceCandidates[index] = cloneEvidenceCandidate(candidate)
	}
	artifacts.Evidence = append([]research.Evidence(nil), result.Evidence...)
	return artifacts, err
}

var (
	_ LiveEvidenceExtractionService = (*liveEvidenceExtractionService)(nil)
	_ LiveResearchStageService      = (*liveEvidenceExtractionService)(nil)
)
