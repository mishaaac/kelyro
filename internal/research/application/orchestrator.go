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
	SearchResults     []SearchResult
	Sources           []research.Source
	FetchedSources    []FetchedSource
	Snapshots         []research.SourceSnapshot
	NormalizedSources []NormalizedSource
	Evidence          []research.Evidence
	Claims            []research.Claim
	Verifications     []research.VerificationResult
	Bundle            *research.SourceBundle
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
	for _, stage := range liveResearchStageOrder {
		service := liveResearchStageService(orchestrator.dependencies, stage)
		artifacts, stageErr := service.Execute(ctx, LiveResearchStageInput{
			QueueItem: queueItem, Request: durableRequest, Run: result.Run, Mode: request.Mode,
			Artifacts: cloneLiveResearchArtifacts(result.Artifacts),
		})
		if stageErr != nil {
			return result, boundaryError(ErrorUnavailable, "execute live research stage "+string(stage), stageErr)
		}
		result.Artifacts = artifacts
		result.CompletedStages = append(result.CompletedStages, stage)
	}
	return result, nil
}

func validateLiveResearchOrchestratorDependencies(dependencies LiveResearchOrchestratorDependencies) error {
	const operation = "configure live research orchestrator"
	for _, dependency := range []struct {
		name  string
		value any
	}{
		{"research trigger service", dependencies.Queue},
		{"research service", dependencies.Research},
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
	result.Sources = append([]research.Source(nil), artifacts.Sources...)
	result.FetchedSources = append([]FetchedSource(nil), artifacts.FetchedSources...)
	result.Snapshots = append([]research.SourceSnapshot(nil), artifacts.Snapshots...)
	result.NormalizedSources = append([]NormalizedSource(nil), artifacts.NormalizedSources...)
	result.Evidence = append([]research.Evidence(nil), artifacts.Evidence...)
	result.Claims = append([]research.Claim(nil), artifacts.Claims...)
	result.Verifications = append([]research.VerificationResult(nil), artifacts.Verifications...)
	if artifacts.Bundle != nil {
		bundle := *artifacts.Bundle
		result.Bundle = &bundle
	}
	return result
}

var _ LiveResearchOrchestrator = (*liveResearchOrchestrator)(nil)
