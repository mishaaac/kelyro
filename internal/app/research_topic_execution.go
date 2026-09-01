package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchTopicExecutionConfigured() bool {
	return service.researchSearch != nil && service.secrets != nil && service.configs != nil &&
		service.researchFetcher != nil && service.researchNormalizer != nil && service.researchSourceCaches != nil
}

// executeResearchTopic is the production, synchronous composition root for
// one queue item. It assembles existing stages only for this invocation and
// never retains the workspace store or starts detached work.
func (service *Service) executeResearchTopic(ctx context.Context, request ResearchTopicExecutionRequest) (researchapp.ResearchQueueConsumeResult, error) {
	if err := request.Validate(); err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	store := request.Store
	candidates, err := newResearchTopicCandidateStage(store.CandidateDeduplication(), store.CandidateRegistration())
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	fetch, err := service.researchFetchForRun(ctx, Command{ConfigOverrides: cloneConfigSettings(request.ConfigOverrides)}, request.Workspace, store, request.RunID)
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	snapshot, err := service.researchSnapshotForRun(ctx, request.Workspace, store)
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	normalize, err := service.researchNormalizationForRun()
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	extract, err := service.researchEvidenceForRun(store)
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	verify, err := service.researchVerificationForRun(store)
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	bundle, err := service.researchBundleForRun(store)
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	provenance, err := service.researchProvenanceForRun(store)
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	finalizationStage, finalization, err := service.researchFinalizationForRun(store)
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	terminalAudit, err := service.researchTerminalAuditForRun(store)
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	search := &researchTopicSearchStage{service: service, request: request}
	orchestrator, err := researchapp.NewLiveResearchOrchestrator(researchapp.LiveResearchOrchestratorDependencies{
		Queue: store.Triggers(), Research: store.Research(), Clock: researchSearchClock{now: service.researchClock},
		Search: search, RegisterCandidates: candidates, Fetch: fetch, Snapshot: snapshot, Normalize: normalize,
		Extract: extract, Verify: verify, Bundle: bundle, Provenance: provenance, Finalize: finalizationStage,
	})
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	consumer, err := researchapp.NewResearchQueueConsumer(researchapp.ResearchQueueConsumerDependencies{
		Queue: store.Triggers(), Research: store.Research(), Orchestrator: orchestrator,
		Finalization: finalization, TerminalAudit: terminalAudit, Clock: researchSearchClock{now: service.researchClock},
	})
	if err != nil {
		return researchapp.ResearchQueueConsumeResult{}, err
	}
	return consumer.Consume(ctx, researchapp.ResearchQueueConsumeRequest{
		QueueItemID: request.QueueItemID, RunID: request.RunID, Mode: request.Mode,
	})
}

type researchTopicSearchStage struct {
	service *Service
	request ResearchTopicExecutionRequest
}

func (stage *researchTopicSearchStage) Execute(ctx context.Context, input researchapp.LiveResearchStageInput) (researchapp.LiveResearchArtifacts, error) {
	artifacts := input.Artifacts
	build, err := stage.service.researchSearchForRun(
		ctx,
		Command{ConfigOverrides: cloneConfigSettings(stage.request.ConfigOverrides)},
		stage.request.Workspace,
		stage.request.RunID,
		stage.request.Store.Costs(),
	)
	if err != nil {
		if ctx.Err() != nil {
			return artifacts, ctx.Err()
		}
		return artifacts, researchapp.Classify(researchapp.ErrorInvalidState, "assemble research topic search", err)
	}
	if build.Discovery == nil || build.ProviderID == "" || build.AdapterVersion == "" {
		return artifacts, researchapp.Classify(researchapp.ErrorUnavailable, "assemble research topic search", errors.New("live search adapter is incomplete"))
	}

	metadata := researchapp.LiveSearchExecutionMetadata{
		ProviderID: build.ProviderID, AdapterVersion: build.AdapterVersion,
		AlgorithmVersion: researchapp.LiveSearchExecutionMetadataV1,
	}
	artifacts.SearchExecution = &metadata
	for _, planned := range stage.request.Plan.Queries {
		remaining := researchapp.MaximumDiscoveryCandidatesPerRun - len(artifacts.Candidates)
		if remaining == 0 {
			break
		}
		limit := min(stage.request.MaxResultsPerQuery, remaining)
		query := researchapp.SearchQuery{RequestID: input.Request.ID, Text: planned.Query}
		kind := planned.DesiredSourceKind
		metadata.QueryCount++
		results, searchErr := build.Discovery.Search(ctx, input.Mode, query, researchapp.SearchOptions{
			DesiredKind: &kind, TargetVersion: input.Request.TargetVersion, Limit: limit,
		})
		artifacts.SearchResults = append(artifacts.SearchResults, results...)
		metadata.ResultCount = len(artifacts.SearchResults)
		if costErr := stage.updateProviderCalls(ctx, &metadata); costErr != nil {
			artifacts.SearchExecution = &metadata
			return artifacts, costErr
		}
		artifacts.SearchExecution = &metadata
		if searchErr != nil {
			return artifacts, searchErr
		}
		discoveredAt := researchSearchClock{now: stage.service.researchClock}.Now()
		if err := discoveredAt.Validate(); err != nil {
			return artifacts, researchapp.Classify(researchapp.ErrorInvalidState, "map research topic search results", err)
		}
		mapped, mapErr := researchapp.MapSearchResultsToSourceCandidatesV1(ctx, researchapp.SourceCandidateMappingInput{
			Query: query, Results: results, DiscoveredAt: discoveredAt,
		})
		if mapErr != nil {
			return artifacts, researchapp.Classify(researchapp.ErrorInvalidState, "map research topic search results", mapErr)
		}
		artifacts.Candidates = append(artifacts.Candidates, mapped...)
	}
	if len(artifacts.Candidates) == 0 {
		return artifacts, researchapp.Classify(researchapp.ErrorInvalidState, "search research topic", errors.New("search provider returned no results"))
	}
	return artifacts, nil
}

func (stage *researchTopicSearchStage) updateProviderCalls(ctx context.Context, metadata *researchapp.LiveSearchExecutionMetadata) error {
	cost, err := stage.request.Store.Costs().Metadata(ctx, stage.request.RunID)
	if err != nil {
		return researchapp.Classify(researchapp.ErrorPersistenceFailure, "read research search cost", err)
	}
	metadata.ProviderAPICalls = cost.Used.ProviderAPICalls
	return nil
}

type researchTopicCandidateStage struct {
	deduplication researchapp.SourceCandidateDeduplicationService
	registration  researchapp.SourceCandidateRegistrationService
}

func newResearchTopicCandidateStage(deduplication researchapp.SourceCandidateDeduplicationService, registration researchapp.SourceCandidateRegistrationService) (researchapp.LiveResearchStageService, error) {
	if deduplication == nil || registration == nil {
		return nil, fmt.Errorf("research candidate services are unavailable")
	}
	return &researchTopicCandidateStage{deduplication: deduplication, registration: registration}, nil
}

func (stage *researchTopicCandidateStage) Execute(ctx context.Context, input researchapp.LiveResearchStageInput) (researchapp.LiveResearchArtifacts, error) {
	artifacts := input.Artifacts
	deduplicated, err := stage.deduplication.Deduplicate(ctx, input.Artifacts.Candidates)
	artifacts.DeduplicatedCandidates = append([]researchapp.DeduplicatedSourceCandidate(nil), deduplicated.Candidates...)
	if err != nil {
		return artifacts, err
	}
	registered, err := stage.registration.Register(ctx, researchapp.SourceCandidateRegistrationRequest{
		RequestID: input.Request.ID, Candidates: deduplicated.Candidates,
	})
	artifacts.Sources = append([]research.Source(nil), registered.Sources...)
	artifacts.Discoveries = append([]research.DiscoveredSource(nil), registered.Discoveries...)
	return artifacts, err
}

var _ researchapp.LiveResearchStageService = (*researchTopicSearchStage)(nil)
var _ researchapp.LiveResearchStageService = (*researchTopicCandidateStage)(nil)

func cloneConfigSettings(settings config.Settings) config.Settings {
	if settings == nil {
		return nil
	}
	clone := make(config.Settings, len(settings))
	for key, value := range settings {
		clone[key] = value
	}
	return clone
}
