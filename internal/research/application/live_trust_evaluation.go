package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
	registrycatalog "github.com/mishaaac/kelyro/internal/research/registry"
	trustpolicy "github.com/mishaaac/kelyro/internal/research/trust"
)

const LiveTrustEvaluationV1 = "live-trust-evaluation-v1"

type LiveTrustEvaluationRequest struct {
	Topic           research.ResearchTopic
	Purpose         research.ResearchPurpose
	Sources         []research.Source
	Claims          []research.Claim
	FreshnessStates map[research.SourceID]research.FreshnessState
}

type LiveTrustEvaluationResult struct {
	Decisions        []research.TrustDecision
	AlgorithmVersion string
}

func (result LiveTrustEvaluationResult) Validate() error {
	if len(result.Decisions) == 0 || len(result.Decisions) > MaximumFetchesPerRun {
		return fmt.Errorf("live trust decisions must contain between 1 and %d entries", MaximumFetchesPerRun)
	}
	seen := make(map[research.SourceID]struct{}, len(result.Decisions))
	for index, decision := range result.Decisions {
		if err := decision.Validate(); err != nil {
			return fmt.Errorf("live trust decision %d: %w", index, err)
		}
		if decision.Policy != trustpolicy.PolicyVersionV1 {
			return fmt.Errorf("live trust decision %d does not use %q", index, trustpolicy.PolicyVersionV1)
		}
		if _, duplicate := seen[decision.SourceID]; duplicate {
			return fmt.Errorf("live trust repeats Source %q", decision.SourceID)
		}
		seen[decision.SourceID] = struct{}{}
	}
	if result.AlgorithmVersion != LiveTrustEvaluationV1 {
		return fmt.Errorf("live trust algorithm must be %q", LiveTrustEvaluationV1)
	}
	return nil
}

type liveTrustEvaluationService struct {
	repository TrustRegistryRepository
	registry   SourceRegistryService
	clock      Clock
}

func NewLiveTrustEvaluationService(repository TrustRegistryRepository, registry SourceRegistryService, clock Clock) (LiveTrustEvaluationService, error) {
	const operation = "configure live trust evaluation"
	for _, dependency := range []struct {
		name  string
		value any
	}{{"trust registry repository", repository}, {"source registry service", registry}, {"clock", clock}} {
		if err := requireDependency(operation, dependency.name, dependency.value); err != nil {
			return nil, err
		}
	}
	return &liveTrustEvaluationService{repository: repository, registry: registry, clock: clock}, nil
}

type liveSourceClaimContextV1 struct {
	source        research.Source
	freshness     research.FreshnessState
	relevance     trustpolicy.Relevance
	stability     trustpolicy.Stability
	corroboration trustpolicy.Corroboration
}

func (service *liveTrustEvaluationService) EvaluateTrust(ctx context.Context, request LiveTrustEvaluationRequest) (LiveTrustEvaluationResult, error) {
	const operation = "evaluate live research source trust"
	if ctx == nil {
		return LiveTrustEvaluationResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveTrustEvaluationResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.Topic.Validate(); err != nil {
		return LiveTrustEvaluationResult{}, invalid(operation, err)
	}
	if err := request.Purpose.Validate(); err != nil {
		return LiveTrustEvaluationResult{}, invalid(operation, err)
	}
	contexts, err := liveTrustSourceContextsV1(request)
	if err != nil {
		return LiveTrustEvaluationResult{}, invalid(operation, err)
	}
	entries, err := service.registry.List(ctx)
	if err != nil {
		return LiveTrustEvaluationResult{}, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	catalog, err := registrycatalog.NewCatalog(entries)
	if err != nil {
		return LiveTrustEvaluationResult{}, invalid(operation, err)
	}
	evaluatedAt := service.clock.Now()
	if err := evaluatedAt.Validate(); err != nil {
		return LiveTrustEvaluationResult{}, invalid(operation, fmt.Errorf("trust evaluation clock: %w", err))
	}

	sourceIDs := make([]research.SourceID, 0, len(contexts))
	for sourceID := range contexts {
		sourceIDs = append(sourceIDs, sourceID)
	}
	sort.Slice(sourceIDs, func(i, j int) bool { return sourceIDs[i].String() < sourceIDs[j].String() })
	result := LiveTrustEvaluationResult{AlgorithmVersion: LiveTrustEvaluationV1}
	policy := trustpolicy.PolicyV1{}
	for _, sourceID := range sourceIDs {
		if err := ctx.Err(); err != nil {
			return cloneLiveTrustEvaluationResult(result), Classify(ErrorUnavailable, operation, err)
		}
		item := contexts[sourceID]
		var matched *research.SourceRegistryEntry
		entry, found, matchErr := catalog.MatchLocator(item.source.Locator)
		if matchErr != nil {
			return cloneLiveTrustEvaluationResult(result), invalid(operation, matchErr)
		}
		if found && registrycatalog.AppliesTo(entry, request.Topic, item.source.Kind) {
			matched = &entry
		}
		decision, evaluateErr := policy.Evaluate(trustpolicy.Input{
			Source: item.source, Topic: request.Topic, Purpose: request.Purpose,
			UseCase: liveTrustUseCaseV1(request.Purpose, item.source), Freshness: item.freshness,
			Relevance: item.relevance, Directness: trustpolicy.DirectnessPrimary,
			Stability: item.stability, Corroboration: item.corroboration,
			Registry: matched, EvaluatedAt: evaluatedAt,
		})
		if evaluateErr != nil {
			return cloneLiveTrustEvaluationResult(result), invalid(operation, evaluateErr)
		}
		if persistErr := service.repository.SaveDecision(ctx, decision); persistErr != nil {
			return cloneLiveTrustEvaluationResult(result), repositoryError(operation, persistErr)
		}
		result.Decisions = append(result.Decisions, decision)
	}
	if err := result.Validate(); err != nil {
		return LiveTrustEvaluationResult{}, invalid(operation, err)
	}
	return cloneLiveTrustEvaluationResult(result), nil
}

func liveTrustSourceContextsV1(request LiveTrustEvaluationRequest) (map[research.SourceID]liveSourceClaimContextV1, error) {
	if len(request.Sources) == 0 || len(request.Sources) > MaximumFetchesPerRun {
		return nil, fmt.Errorf("trust Sources must contain between 1 and %d entries", MaximumFetchesPerRun)
	}
	if len(request.Claims) == 0 || len(request.Claims) > MaximumClaimCandidatesPerRun {
		return nil, fmt.Errorf("trust Claims must contain between 1 and %d entries", MaximumClaimCandidatesPerRun)
	}
	sources := make(map[research.SourceID]research.Source, len(request.Sources))
	for index, source := range request.Sources {
		if err := source.Validate(); err != nil {
			return nil, fmt.Errorf("trust Source %d: %w", index, err)
		}
		if _, duplicate := sources[source.ID]; duplicate {
			return nil, fmt.Errorf("trust Source %q is repeated", source.ID)
		}
		sources[source.ID] = source
	}
	for sourceID, state := range request.FreshnessStates {
		if _, exists := sources[sourceID]; !exists {
			return nil, fmt.Errorf("trust freshness references missing Source %q", sourceID)
		}
		if err := state.Validate(); err != nil {
			return nil, err
		}
	}
	contexts := make(map[research.SourceID]liveSourceClaimContextV1)
	for index, claim := range request.Claims {
		if err := claim.Validate(); err != nil {
			return nil, fmt.Errorf("trust Claim %d: %w", index, err)
		}
		for _, sourceID := range claim.SourceIDs {
			source, exists := sources[sourceID]
			if !exists {
				return nil, fmt.Errorf("trust Claim %q references missing Source %q", claim.ID, sourceID)
			}
			relevance := liveTrustRelevanceV1(claim.Statement, request.Topic)
			stability := liveTrustStabilityV1(claim.StatusScope)
			corroboration := trustpolicy.CorroborationSingleSource
			freshnessState := research.FreshnessUnknown
			if state, exists := request.FreshnessStates[sourceID]; exists {
				freshnessState = state
			}
			if len(claim.SourceIDs) > 1 {
				// Multiple locators do not establish organizational independence.
				// Step 29 owns corroboration and diversity evaluation.
				corroboration = trustpolicy.CorroborationUnknown
			}
			current, found := contexts[sourceID]
			if !found {
				contexts[sourceID] = liveSourceClaimContextV1{source: source, freshness: freshnessState, relevance: relevance, stability: stability, corroboration: corroboration}
				continue
			}
			current.relevance = conservativeTrustRelevanceV1(current.relevance, relevance)
			current.stability = conservativeTrustStabilityV1(current.stability, stability)
			if current.corroboration != corroboration {
				current.corroboration = trustpolicy.CorroborationUnknown
			}
			contexts[sourceID] = current
		}
	}
	if len(contexts) == 0 {
		return nil, errors.New("trust Claims reference no Sources")
	}
	return contexts, nil
}

func liveTrustUseCaseV1(purpose research.ResearchPurpose, source research.Source) trustpolicy.UseCase {
	if purpose == research.PurposeSecurityGuidance {
		return trustpolicy.UseCaseSecurityAdvisory
	}
	if purpose == research.PurposeVersionBehavior || source.TemporalScope == research.SourceTemporalHistorical || source.TemporalScope == research.SourceTemporalArchived {
		return trustpolicy.UseCaseHistoricalBehavior
	}
	if source.Kind == research.SourcePackageReference || source.Kind == research.SourceCode {
		return trustpolicy.UseCasePackageAPI
	}
	if purpose == research.PurposeConceptDefinition && (source.Kind == research.SourceSpecification || source.Kind == research.SourceStandard) {
		return trustpolicy.UseCaseLanguageSpecification
	}
	return trustpolicy.UseCaseGeneral
}

func liveTrustRelevanceV1(statement string, topic research.ResearchTopic) trustpolicy.Relevance {
	words := evidenceWords(statement)
	if containsEvidencePhrase(words, evidenceWords(topic.Subject)) {
		return trustpolicy.RelevanceExact
	}
	if topic.Technology != "" && containsEvidencePhrase(words, evidenceWords(topic.Technology)) {
		return trustpolicy.RelevanceStrong
	}
	return trustpolicy.RelevanceUnknown
}

func liveTrustStabilityV1(scope research.ClaimStatusScope) trustpolicy.Stability {
	switch scope {
	case research.ClaimStatusStable:
		return trustpolicy.StabilityStable
	case research.ClaimStatusPreview:
		return trustpolicy.StabilityPreview
	case research.ClaimStatusExperimental:
		return trustpolicy.StabilityExperimental
	case research.ClaimStatusLegacy:
		return trustpolicy.StabilityLegacy
	default:
		return trustpolicy.StabilityUnknown
	}
}

func conservativeTrustRelevanceV1(left, right trustpolicy.Relevance) trustpolicy.Relevance {
	rank := map[trustpolicy.Relevance]int{trustpolicy.RelevanceExact: 0, trustpolicy.RelevanceStrong: 1, trustpolicy.RelevancePartial: 2, trustpolicy.RelevanceUnknown: 3, trustpolicy.RelevanceUnrelated: 4}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func conservativeTrustStabilityV1(left, right trustpolicy.Stability) trustpolicy.Stability {
	rank := map[trustpolicy.Stability]int{trustpolicy.StabilityStable: 0, trustpolicy.StabilityLegacy: 1, trustpolicy.StabilityPreview: 2, trustpolicy.StabilityExperimental: 3, trustpolicy.StabilityUnknown: 4}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func (service *liveTrustEvaluationService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.EvaluateTrust(ctx, LiveTrustEvaluationRequest{
		Topic: input.Request.Topic, Purpose: input.Request.Purpose,
		Sources: input.Artifacts.Sources, Claims: input.Artifacts.Claims,
	})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.TrustDecisions = make([]research.TrustDecision, len(result.Decisions))
	for index, decision := range result.Decisions {
		artifacts.TrustDecisions[index] = cloneTrustDecisionArtifact(decision)
	}
	return artifacts, err
}

func cloneLiveTrustEvaluationResult(result LiveTrustEvaluationResult) LiveTrustEvaluationResult {
	clone := result
	clone.Decisions = make([]research.TrustDecision, len(result.Decisions))
	for index, decision := range result.Decisions {
		clone.Decisions[index] = cloneTrustDecisionArtifact(decision)
	}
	return clone
}

var _ LiveTrustEvaluationService = (*liveTrustEvaluationService)(nil)
