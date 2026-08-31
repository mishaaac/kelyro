package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

const LiveResearchOrchestratorV1 = "live-research-orchestrator-v1"

type LiveResearchStage string

const (
	LiveResearchStageSearch             LiveResearchStage = "search"
	LiveResearchStageRegisterCandidates LiveResearchStage = "register_candidates"
	LiveResearchStageFetch              LiveResearchStage = "fetch"
	LiveResearchStageSnapshot           LiveResearchStage = "snapshot"
	LiveResearchStageNormalize          LiveResearchStage = "normalize"
	LiveResearchStageExtract            LiveResearchStage = "extract"
	LiveResearchStageVerify             LiveResearchStage = "verify"
	LiveResearchStageBundle             LiveResearchStage = "bundle"
	LiveResearchStageFinalize           LiveResearchStage = "finalize"
)

var liveResearchStageOrder = []LiveResearchStage{
	LiveResearchStageSearch,
	LiveResearchStageRegisterCandidates,
	LiveResearchStageFetch,
	LiveResearchStageSnapshot,
	LiveResearchStageNormalize,
	LiveResearchStageExtract,
	LiveResearchStageVerify,
	LiveResearchStageBundle,
	LiveResearchStageFinalize,
}

type LiveResearchOrchestrationRequest struct {
	QueueItemID research.ID
	RunID       research.ID
	Mode        ResearchMode
}

func (request LiveResearchOrchestrationRequest) Validate() error {
	if err := request.QueueItemID.Validate(); err != nil {
		return fmt.Errorf("live research queue item: %w", err)
	}
	if err := request.RunID.Validate(); err != nil {
		return fmt.Errorf("live research run: %w", err)
	}
	return request.Mode.Validate()
}

// LiveResearchArtifacts is the in-process hand-off between stages. Search
// results remain candidates; only persisted Evidence can support Claims.
// Fetched bodies and normalized content are transient and bounded by the
// services that create them.
type LiveResearchArtifacts struct {
	SearchResults          []SearchResult
	Candidates             []SourceCandidate
	DeduplicatedCandidates []DeduplicatedSourceCandidate
	Discoveries            []research.DiscoveredSource
	Sources                []research.Source
	FetchedSources         []FetchedSource
	FetchFailures          []SourceFetchFailure
	FetchMaximumBytes      int64
	Snapshots              []research.SourceSnapshot
	NormalizationInputs    []FetchedSource
	SnapshotFailures       []SourceSnapshotFailure
	CacheFailures          []SourceSnapshotFailure
	NormalizedSources      []NormalizedSource
	NormalizationFailures  []SourceNormalizationFailure
	EvidenceCandidates     []EvidenceCandidate
	Evidence               []research.Evidence
	ClaimCandidates        []ClaimCandidate
	Claims                 []research.Claim
	Citations              []research.Citation
	Verifications          []research.VerificationResult
	Bundle                 *research.SourceBundle
}

type LiveResearchStageInput struct {
	QueueItem research.ResearchQueueItem
	Request   research.ResearchRequest
	Run       research.ResearchRun
	Mode      ResearchMode
	Artifacts LiveResearchArtifacts
}

type LiveResearchOrchestrationResult struct {
	QueueItem        research.ResearchQueueItem
	Request          research.ResearchRequest
	Run              research.ResearchRun
	Artifacts        LiveResearchArtifacts
	CompletedStages  []LiveResearchStage
	AlgorithmVersion string
}

type LiveResearchOrchestratorDependencies struct {
	Queue              ResearchTriggerService
	Research           ResearchService
	Clock              Clock
	Search             LiveResearchStageService
	RegisterCandidates LiveResearchStageService
	Fetch              LiveResearchStageService
	Snapshot           LiveResearchStageService
	Normalize          LiveResearchStageService
	Extract            LiveResearchStageService
	Verify             LiveResearchStageService
	Bundle             LiveResearchStageService
	Finalize           LiveResearchStageService
}

type liveResearchOrchestrator struct {
	dependencies LiveResearchOrchestratorDependencies
}

func NewLiveResearchOrchestrator(dependencies LiveResearchOrchestratorDependencies) (LiveResearchOrchestrator, error) {
	if err := validateLiveResearchOrchestratorDependencies(dependencies); err != nil {
		return nil, err
	}
	return &liveResearchOrchestrator{dependencies: dependencies}, nil
}

func (orchestrator *liveResearchOrchestrator) Execute(ctx context.Context, request LiveResearchOrchestrationRequest) (LiveResearchOrchestrationResult, error) {
	const operation = "execute live research orchestration"
	if err := request.Validate(); err != nil {
		return LiveResearchOrchestrationResult{}, invalid(operation, err)
	}
	if ctx == nil {
		return LiveResearchOrchestrationResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveResearchOrchestrationResult{}, Classify(ErrorUnavailable, operation, err)
	}

	queueItem, err := orchestrator.dependencies.Queue.Get(ctx, request.QueueItemID)
	if err != nil {
		return LiveResearchOrchestrationResult{}, boundaryError(ErrorUnavailable, operation, err)
	}
	if queueItem.Status == research.ResearchQueueCancelled {
		return LiveResearchOrchestrationResult{}, invalid(operation, errors.New("research queue item is cancelled"))
	}
	run, err := orchestrator.dependencies.Research.Run(ctx, request.RunID)
	if err != nil {
		return LiveResearchOrchestrationResult{}, boundaryError(ErrorUnavailable, operation, err)
	}
	durableRequest, err := orchestrator.dependencies.Research.Request(ctx, run.RequestID)
	if err != nil {
		return LiveResearchOrchestrationResult{}, boundaryError(ErrorUnavailable, operation, err)
	}
	if run.RequestID != queueItem.Request.ID || !sameLiveResearchRequest(durableRequest, queueItem.Request) {
		return LiveResearchOrchestrationResult{}, invalid(operation, errors.New("queue item, request, and run do not identify the same research work"))
	}
	if run.Status == research.ResearchRunCompleted || run.Status == research.ResearchRunFailed || run.Status == research.ResearchRunCancelled {
		return LiveResearchOrchestrationResult{}, invalid(operation, fmt.Errorf("research run is already terminal with status %q", run.Status))
	}

	result := LiveResearchOrchestrationResult{
		QueueItem: queueItem, Request: durableRequest, Run: run,
		AlgorithmVersion: LiveResearchOrchestratorV1,
	}
	if err := ctx.Err(); err != nil {
		return orchestrator.stop(ctx, result, research.ResearchRunCancelled, Classify(ErrorUnavailable, operation, err))
	}
	if run.Status == research.ResearchRunPlanned {
		run, err = orchestrator.dependencies.Research.TransitionRun(ctx, run.ID, research.ResearchRunRunning, orchestrator.dependencies.Clock.Now())
		if err != nil {
			classified := boundaryError(ErrorUnavailable, operation, err)
			if ctx.Err() != nil {
				return orchestrator.stop(ctx, result, research.ResearchRunCancelled, classified)
			}
			return orchestrator.stop(ctx, result, research.ResearchRunFailed, classified)
		}
		result.Run = run
	}
	for _, stage := range liveResearchStageOrder {
		if err := ctx.Err(); err != nil {
			return orchestrator.stop(ctx, result, research.ResearchRunCancelled, Classify(ErrorUnavailable, operation, err))
		}
		service := liveResearchStageService(orchestrator.dependencies, stage)
		artifacts, stageErr := service.Execute(ctx, LiveResearchStageInput{
			QueueItem: queueItem, Request: durableRequest, Run: result.Run, Mode: request.Mode,
			Artifacts: cloneLiveResearchArtifacts(result.Artifacts),
		})
		if stageErr != nil {
			result.Artifacts = cloneLiveResearchArtifacts(artifacts)
			classified := boundaryError(ErrorUnavailable, "execute live research stage "+string(stage), stageErr)
			status := research.ResearchRunFailed
			if errors.Is(stageErr, context.Canceled) || errors.Is(stageErr, context.DeadlineExceeded) || ctx.Err() != nil {
				status = research.ResearchRunCancelled
			}
			return orchestrator.stop(ctx, result, status, classified)
		}
		result.Artifacts = artifacts
		result.CompletedStages = append(result.CompletedStages, stage)
		if stage == LiveResearchStageBundle {
			if err := validateOrchestratedBundle(result.Run.ID, result.Artifacts.Bundle); err != nil {
				return orchestrator.stop(ctx, result, research.ResearchRunFailed, invalid(operation, err))
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return orchestrator.stop(ctx, result, research.ResearchRunCancelled, Classify(ErrorUnavailable, operation, err))
	}
	completed, err := orchestrator.dependencies.Research.TransitionRun(ctx, result.Run.ID, research.ResearchRunCompleted, orchestrator.dependencies.Clock.Now())
	if err != nil {
		return orchestrator.stop(ctx, result, research.ResearchRunFailed, boundaryError(ErrorUnavailable, operation, err))
	}
	result.Run = completed
	return result, nil
}

func (orchestrator *liveResearchOrchestrator) stop(ctx context.Context, result LiveResearchOrchestrationResult, status research.ResearchRunStatus, cause error) (LiveResearchOrchestrationResult, error) {
	transitionContext := ctx
	if ctx.Err() != nil {
		transitionContext = context.WithoutCancel(ctx)
	}
	terminal, err := orchestrator.dependencies.Research.TransitionRun(transitionContext, result.Run.ID, status, orchestrator.dependencies.Clock.Now())
	if err != nil {
		return result, errors.Join(cause, boundaryError(ErrorUnavailable, "finalize live research run", err))
	}
	result.Run = terminal
	return result, cause
}

func validateOrchestratedBundle(runID research.ID, bundle *research.SourceBundle) error {
	if bundle == nil {
		return errors.New("bundle stage returned no durable bundle")
	}
	if err := bundle.ID.Validate(); err != nil {
		return fmt.Errorf("bundle stage returned invalid bundle identity: %w", err)
	}
	if bundle.RunID != runID {
		return errors.New("bundle stage returned a bundle for a different research run")
	}
	return nil
}

func validateLiveResearchOrchestratorDependencies(dependencies LiveResearchOrchestratorDependencies) error {
	const operation = "configure live research orchestrator"
	for _, dependency := range []struct {
		name  string
		value any
	}{
		{"research trigger service", dependencies.Queue},
		{"research service", dependencies.Research},
		{"clock", dependencies.Clock},
		{"search stage", dependencies.Search},
		{"candidate registration stage", dependencies.RegisterCandidates},
		{"fetch stage", dependencies.Fetch},
		{"snapshot stage", dependencies.Snapshot},
		{"normalization stage", dependencies.Normalize},
		{"extraction stage", dependencies.Extract},
		{"verification stage", dependencies.Verify},
		{"bundle stage", dependencies.Bundle},
		{"finalization stage", dependencies.Finalize},
	} {
		if err := requireDependency(operation, dependency.name, dependency.value); err != nil {
			return err
		}
	}
	return nil
}

func liveResearchStageService(dependencies LiveResearchOrchestratorDependencies, stage LiveResearchStage) LiveResearchStageService {
	switch stage {
	case LiveResearchStageSearch:
		return dependencies.Search
	case LiveResearchStageRegisterCandidates:
		return dependencies.RegisterCandidates
	case LiveResearchStageFetch:
		return dependencies.Fetch
	case LiveResearchStageSnapshot:
		return dependencies.Snapshot
	case LiveResearchStageNormalize:
		return dependencies.Normalize
	case LiveResearchStageExtract:
		return dependencies.Extract
	case LiveResearchStageVerify:
		return dependencies.Verify
	case LiveResearchStageBundle:
		return dependencies.Bundle
	case LiveResearchStageFinalize:
		return dependencies.Finalize
	default:
		panic("unknown live research stage " + string(stage))
	}
}

func sameLiveResearchRequest(left, right research.ResearchRequest) bool {
	if left.ID != right.ID || left.Topic != right.Topic || left.Purpose != right.Purpose || !left.RequestedAt.Time().Equal(right.RequestedAt.Time()) {
		return false
	}
	if left.TargetVersion == nil || right.TargetVersion == nil {
		return left.TargetVersion == nil && right.TargetVersion == nil
	}
	return *left.TargetVersion == *right.TargetVersion
}

func cloneLiveResearchArtifacts(artifacts LiveResearchArtifacts) LiveResearchArtifacts {
	result := artifacts
	result.SearchResults = append([]SearchResult(nil), artifacts.SearchResults...)
	result.Candidates = make([]SourceCandidate, len(artifacts.Candidates))
	for index, candidate := range artifacts.Candidates {
		result.Candidates[index] = cloneSourceCandidate(candidate)
	}
	result.DeduplicatedCandidates = make([]DeduplicatedSourceCandidate, len(artifacts.DeduplicatedCandidates))
	for index, candidate := range artifacts.DeduplicatedCandidates {
		result.DeduplicatedCandidates[index].Candidate = cloneSourceCandidate(candidate.Candidate)
		if candidate.ExistingSourceID != nil {
			id := *candidate.ExistingSourceID
			result.DeduplicatedCandidates[index].ExistingSourceID = &id
		}
	}
	result.Sources = append([]research.Source(nil), artifacts.Sources...)
	result.Discoveries = make([]research.DiscoveredSource, len(artifacts.Discoveries))
	for index, discovery := range artifacts.Discoveries {
		result.Discoveries[index] = cloneDiscoveredSource(discovery)
	}
	result.FetchedSources = make([]FetchedSource, len(artifacts.FetchedSources))
	for index, fetched := range artifacts.FetchedSources {
		result.FetchedSources[index] = cloneFetchedSource(fetched)
	}
	result.FetchFailures = append([]SourceFetchFailure(nil), artifacts.FetchFailures...)
	result.NormalizationInputs = make([]FetchedSource, len(artifacts.NormalizationInputs))
	for index, fetched := range artifacts.NormalizationInputs {
		result.NormalizationInputs[index] = cloneFetchedSource(fetched)
	}
	result.SnapshotFailures = append([]SourceSnapshotFailure(nil), artifacts.SnapshotFailures...)
	result.CacheFailures = append([]SourceSnapshotFailure(nil), artifacts.CacheFailures...)
	result.Snapshots = append([]research.SourceSnapshot(nil), artifacts.Snapshots...)
	result.NormalizedSources = make([]NormalizedSource, len(artifacts.NormalizedSources))
	for index, normalized := range artifacts.NormalizedSources {
		result.NormalizedSources[index] = cloneNormalizedSource(normalized)
	}
	result.NormalizationFailures = append([]SourceNormalizationFailure(nil), artifacts.NormalizationFailures...)
	result.EvidenceCandidates = make([]EvidenceCandidate, len(artifacts.EvidenceCandidates))
	for index, candidate := range artifacts.EvidenceCandidates {
		result.EvidenceCandidates[index] = cloneEvidenceCandidate(candidate)
	}
	result.Evidence = append([]research.Evidence(nil), artifacts.Evidence...)
	result.ClaimCandidates = make([]ClaimCandidate, len(artifacts.ClaimCandidates))
	for index, candidate := range artifacts.ClaimCandidates {
		result.ClaimCandidates[index] = cloneClaimCandidate(candidate)
	}
	result.Claims = append([]research.Claim(nil), artifacts.Claims...)
	result.Citations = make([]research.Citation, len(artifacts.Citations))
	for index, item := range artifacts.Citations {
		result.Citations[index] = cloneCitationArtifact(item)
	}
	result.Verifications = append([]research.VerificationResult(nil), artifacts.Verifications...)
	if artifacts.Bundle != nil {
		bundle := *artifacts.Bundle
		result.Bundle = &bundle
	}
	return result
}

func cloneCitationArtifact(item research.Citation) research.Citation {
	clone := item
	if item.DeepLink != nil {
		link := *item.DeepLink
		clone.DeepLink = &link
	}
	clone.VersionScope = cloneSourceVersion(item.VersionScope)
	return clone
}

var _ LiveResearchOrchestrator = (*liveResearchOrchestrator)(nil)
