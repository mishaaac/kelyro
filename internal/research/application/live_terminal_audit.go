package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

const LiveResearchTerminalAuditV1 = "live-research-terminal-audit-v1"

type LiveResearchTerminalAuditRequest struct {
	Run         research.ResearchRun
	Outcome     research.ResearchRunStatus
	FinalizedAt research.Timestamp
	Mode        ResearchMode
	FailureKind ErrorKind
	Artifacts   LiveResearchArtifacts
}

type LiveResearchTerminalAuditResult struct {
	Audit            research.ResearchRunAudit
	Cost             research.ResearchCostMetadata
	AlgorithmVersion string
}

type LiveResearchTerminalAuditService interface {
	Prepare(context.Context, LiveResearchTerminalAuditRequest) (LiveResearchTerminalAuditResult, error)
}

type liveResearchTerminalAuditService struct {
	research ResearchService
	costs    ResearchCostService
}

func NewLiveResearchTerminalAuditService(researchService ResearchService, costs ResearchCostService) (LiveResearchTerminalAuditService, error) {
	const operation = "configure live research terminal audit"
	if err := requireDependency(operation, "research service", researchService); err != nil {
		return nil, err
	}
	if err := requireDependency(operation, "research cost service", costs); err != nil {
		return nil, err
	}
	return &liveResearchTerminalAuditService{research: researchService, costs: costs}, nil
}

func (service *liveResearchTerminalAuditService) Prepare(ctx context.Context, request LiveResearchTerminalAuditRequest) (LiveResearchTerminalAuditResult, error) {
	const operation = "prepare live research terminal audit"
	if ctx == nil {
		return LiveResearchTerminalAuditResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveResearchTerminalAuditResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := validateLiveResearchTerminalAuditRequest(request); err != nil {
		return LiveResearchTerminalAuditResult{}, invalid(operation, err)
	}
	trail, err := service.research.AuditTrail(ctx, request.Run.ID)
	if err != nil {
		return LiveResearchTerminalAuditResult{}, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	planned, err := plannedAuditForLiveRun(trail)
	if err != nil {
		return LiveResearchTerminalAuditResult{}, invalid(operation, err)
	}
	cost, err := service.costs.Metadata(ctx, request.Run.ID)
	if err != nil {
		return LiveResearchTerminalAuditResult{}, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	if err := cost.Validate(); err != nil {
		return LiveResearchTerminalAuditResult{}, invalid(operation, err)
	}

	providers, providersUsed, queryCount, resultCount, err := terminalSearchAccounting(request.Artifacts, cost)
	if err != nil {
		return LiveResearchTerminalAuditResult{}, invalid(operation, err)
	}
	fetchCount := len(request.Artifacts.FetchedSources) + len(request.Artifacts.FetchFailures)
	bytesFetched := int64(0)
	cacheHits := 0
	liveFetches := int64(0)
	cachedFetches := int64(0)
	liveBytes := int64(0)
	for _, result := range request.Artifacts.SearchResults {
		if result.CacheHit {
			cacheHits++
		}
	}
	for _, fetched := range request.Artifacts.FetchedSources {
		bytesFetched += int64(len(fetched.Body))
		if fetched.CacheHit {
			cacheHits++
		}
		if fetched.Origin == FetchOriginLive {
			liveFetches++
			liveBytes += int64(len(fetched.Body))
		} else if fetched.Origin == FetchOriginCache {
			cachedFetches++
		}
	}
	if cost.Used.FetchRequests < liveFetches || cost.Used.Bytes < liveBytes || cost.CacheSavings.FetchRequests < cachedFetches {
		return LiveResearchTerminalAuditResult{}, invalid(operation, errors.New("durable fetch cost does not cover observed live/cache artifacts"))
	}
	sources := make([]research.ResearchAuditSource, len(request.Artifacts.Snapshots))
	for index, snapshot := range request.Artifacts.Snapshots {
		if err := snapshot.Validate(); err != nil {
			return LiveResearchTerminalAuditResult{}, invalid(operation, err)
		}
		if err := research.ValidateCanonicalContentHashV1(snapshot.Fetch.ContentHash); err != nil {
			return LiveResearchTerminalAuditResult{}, invalid(operation, fmt.Errorf("terminal audit snapshot %q: %w", snapshot.ID, err))
		}
		sources[index] = research.ResearchAuditSource{
			SourceID: snapshot.SourceID, Locator: snapshot.Locator, SnapshotID: snapshot.ID, SnapshotHash: snapshot.Fetch.ContentHash,
		}
	}
	recordedAt, err := terminalAuditRecordedAt(request.FinalizedAt, trail)
	if err != nil {
		return LiveResearchTerminalAuditResult{}, invalid(operation, err)
	}
	execution := research.ResearchAuditExecution{
		Providers: providers, QueryCount: queryCount, ResultCount: resultCount, FetchCount: fetchCount,
		FailureKind: string(request.FailureKind), CostUsed: cost.Used, CacheSavings: cost.CacheSavings,
		StoppedByBudget: cost.StoppedByBudget, AlgorithmVersion: research.LiveResearchExecutionAuditV1,
	}
	if request.Outcome == research.ResearchRunCompleted {
		bundleID := request.Artifacts.Bundle.ID
		execution.BundleID = &bundleID
	}
	completedAt := request.FinalizedAt
	audit := research.ResearchRunAudit{
		ID: stableTerminalAuditID(request.Run.ID, request.Outcome, request.FinalizedAt), RunID: request.Run.ID,
		RecordedAt: recordedAt, StartedAt: request.Run.StartedAt, CompletedAt: &completedAt, Outcome: request.Outcome,
		QueryPlannerVersion: planned.QueryPlannerVersion, TrustPolicyVersion: planned.TrustPolicyVersion,
		FreshnessVersion: planned.FreshnessVersion, ConflictResolverVersion: planned.ConflictResolverVersion,
		ProvidersUsed: providersUsed, NetworkMode: auditNetworkMode(request.Mode), NetworkAllowed: planned.NetworkAllowed && request.Mode != ResearchModeOffline,
		CacheHits: cacheHits, BytesFetched: bytesFetched, Queries: append([]string(nil), planned.Queries...), Sources: sources,
		TargetTechnology: planned.TargetTechnology, TargetVersion: cloneSourceVersion(planned.TargetVersion),
		AdditionalAlgorithms: terminalAuditAlgorithms(planned.AdditionalAlgorithms, request.Artifacts), Execution: &execution,
	}
	sealed, err := research.SealResearchRunAuditV1(audit)
	if err != nil {
		return LiveResearchTerminalAuditResult{}, invalid(operation, err)
	}
	result := LiveResearchTerminalAuditResult{Audit: sealed, Cost: cost, AlgorithmVersion: LiveResearchTerminalAuditV1}
	return result, nil
}

func validateLiveResearchTerminalAuditRequest(request LiveResearchTerminalAuditRequest) error {
	if err := request.Run.Validate(); err != nil {
		return err
	}
	if request.Run.Status.IsTerminal() || request.Run.CompletedAt != nil {
		return errors.New("terminal audit requires a non-terminal Research Run")
	}
	if err := request.Outcome.Validate(); err != nil {
		return err
	}
	if !request.Outcome.IsTerminal() {
		return errors.New("terminal audit outcome is not terminal")
	}
	if err := request.FinalizedAt.Validate(); err != nil {
		return err
	}
	if request.FinalizedAt.Before(request.Run.StartedAt) {
		return errors.New("terminal audit precedes Research Run")
	}
	if err := request.Mode.Validate(); err != nil {
		return err
	}
	switch request.Outcome {
	case research.ResearchRunCompleted:
		if request.Run.Status != research.ResearchRunRunning || request.FailureKind != "" || request.Artifacts.Bundle == nil || request.Artifacts.Bundle.RunID != request.Run.ID {
			return errors.New("completed terminal audit requires the run bundle without failure")
		}
	case research.ResearchRunFailed:
		if !validResearchQueueFailureKind(request.FailureKind) {
			return errors.New("failed terminal audit requires a safe failure kind")
		}
	case research.ResearchRunCancelled:
		if request.FailureKind != "" {
			return errors.New("cancelled terminal audit cannot contain failure kind")
		}
	}
	return nil
}

func plannedAuditForLiveRun(trail []research.ResearchRunAudit) (research.ResearchRunAudit, error) {
	for _, audit := range trail {
		if audit.Outcome == research.ResearchRunPlanned {
			return audit, nil
		}
	}
	return research.ResearchRunAudit{}, errors.New("live Research Run has no planned audit checkpoint")
}

func terminalSearchAccounting(artifacts LiveResearchArtifacts, cost research.ResearchCostMetadata) ([]research.ResearchAuditProvider, []string, int, int, error) {
	if artifacts.SearchExecution == nil {
		if cost.Used.SearchRequests != 0 || cost.Used.ProviderAPICalls != 0 || len(artifacts.SearchResults) != 0 {
			return nil, nil, 0, 0, errors.New("live search activity has no adapter execution metadata")
		}
		return nil, nil, 0, 0, nil
	}
	metadata := *artifacts.SearchExecution
	if err := metadata.Validate(); err != nil {
		return nil, nil, 0, 0, err
	}
	if metadata.ResultCount != len(artifacts.SearchResults) || int64(metadata.QueryCount) < cost.Used.SearchRequests || metadata.ProviderAPICalls != cost.Used.ProviderAPICalls {
		return nil, nil, 0, 0, errors.New("live search execution metadata does not match artifacts or durable cost")
	}
	if metadata.ProviderAPICalls == 0 {
		return nil, nil, metadata.QueryCount, metadata.ResultCount, nil
	}
	provider := research.ResearchAuditProvider{ProviderID: metadata.ProviderID, AdapterVersion: metadata.AdapterVersion, APICalls: metadata.ProviderAPICalls}
	return []research.ResearchAuditProvider{provider}, []string{metadata.ProviderID}, metadata.QueryCount, metadata.ResultCount, nil
}

func terminalAuditRecordedAt(finalizedAt research.Timestamp, trail []research.ResearchRunAudit) (research.Timestamp, error) {
	recorded := finalizedAt.Time()
	for _, audit := range trail {
		if !recorded.After(audit.RecordedAt.Time()) {
			recorded = audit.RecordedAt.Time().Add(time.Nanosecond)
		}
	}
	return research.NewTimestamp(recorded)
}

func stableTerminalAuditID(runID research.ID, outcome research.ResearchRunStatus, finalizedAt research.Timestamp) research.ID {
	hash := sha256.Sum256([]byte(runID.String() + "\x00" + string(outcome) + "\x00" + finalizedAt.Time().Format(time.RFC3339Nano)))
	id, err := research.NewID("audit.live.terminal." + hex.EncodeToString(hash[:16]))
	if err != nil {
		panic(err)
	}
	return id
}

func auditNetworkMode(mode ResearchMode) research.ResearchAuditNetworkMode {
	switch mode {
	case ResearchModeOffline:
		return research.ResearchAuditNetworkOffline
	case ResearchModeOnline:
		return research.ResearchAuditNetworkOnline
	default:
		return research.ResearchAuditNetworkAuto
	}
}

func terminalAuditAlgorithms(planned []research.ResearchAuditAlgorithm, artifacts LiveResearchArtifacts) []research.ResearchAuditAlgorithm {
	versions := make(map[string]string, len(planned)+8)
	for _, algorithm := range planned {
		versions[algorithm.Stage] = algorithm.Version
	}
	versions["live_terminal_audit"] = LiveResearchTerminalAuditV1
	versions["orchestrator"] = LiveResearchOrchestratorV1
	if len(artifacts.FetchedSources)+len(artifacts.FetchFailures) > 0 {
		versions["live_fetch"] = artifacts.FetchAlgorithmVersion
		if versions["live_fetch"] == "" {
			versions["live_fetch"] = LiveSourceFetchV1
		}
	}
	if len(artifacts.Snapshots) > 0 {
		versions["live_snapshot"] = LiveSourceSnapshotV1
	}
	if len(artifacts.Verifications) > 0 {
		versions["verification"] = artifacts.Verifications[0].AlgorithmVersion
		versions["bundle_claim_selection"] = LiveBundleClaimSelectionV1
	}
	if artifacts.Bundle != nil {
		versions["live_bundle"] = LiveSourceBundleV1
	}
	if len(artifacts.ProvenanceGraphs) > 0 {
		versions["live_provenance"] = LiveResearchProvenanceV1
	}
	result := make([]research.ResearchAuditAlgorithm, 0, len(versions))
	for stage, version := range versions {
		result = append(result, research.ResearchAuditAlgorithm{Stage: stage, Version: version})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Stage < result[j].Stage })
	return result
}

var _ LiveResearchTerminalAuditService = (*liveResearchTerminalAuditService)(nil)
