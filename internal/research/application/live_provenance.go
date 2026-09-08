package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/queryplanner"
)

const LiveResearchProvenanceV1 = "live-research-provenance-v1"

type LiveResearchProvenanceRequest struct {
	Request       research.ResearchRequest
	Run           research.ResearchRun
	Discoveries   []research.DiscoveredSource
	Sources       []research.Source
	Snapshots     []research.SourceSnapshot
	Evidence      []research.Evidence
	Claims        []research.Claim
	Verifications []research.VerificationResult
	Bundle        *research.SourceBundle
}

type LiveResearchProvenanceResult struct {
	Graphs           []research.ProvenanceGraph
	AlgorithmVersion string
}

func (result LiveResearchProvenanceResult) Validate() error {
	if len(result.Graphs) == 0 || len(result.Graphs) > MaximumClaimCandidatesPerRun {
		return fmt.Errorf("live provenance graphs must contain between 1 and %d entries", MaximumClaimCandidatesPerRun)
	}
	for index, graph := range result.Graphs {
		if err := graph.Validate(); err != nil {
			return fmt.Errorf("live provenance graph %d: %w", index, err)
		}
		if index > 0 && graph.ClaimID.String() <= result.Graphs[index-1].ClaimID.String() {
			return errors.New("live provenance graphs must be unique and ordered by Claim")
		}
	}
	if result.AlgorithmVersion != LiveResearchProvenanceV1 {
		return fmt.Errorf("live provenance algorithm must be %q", LiveResearchProvenanceV1)
	}
	return nil
}

type liveResearchProvenanceService struct {
	provenance ProvenanceService
	clock      Clock
}

func NewLiveResearchProvenanceService(provenance ProvenanceService, clock Clock) (LiveResearchProvenanceService, error) {
	const operation = "configure live research provenance"
	if err := requireDependency(operation, "provenance service", provenance); err != nil {
		return nil, err
	}
	if err := requireDependency(operation, "clock", clock); err != nil {
		return nil, err
	}
	return &liveResearchProvenanceService{provenance: provenance, clock: clock}, nil
}

func (service *liveResearchProvenanceService) Preserve(ctx context.Context, request LiveResearchProvenanceRequest) (LiveResearchProvenanceResult, error) {
	const operation = "preserve live research provenance"
	if ctx == nil {
		return LiveResearchProvenanceResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveResearchProvenanceResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := validateLiveResearchProvenanceRequest(request); err != nil {
		return LiveResearchProvenanceResult{}, invalid(operation, err)
	}

	recordedAt := service.clock.Now()
	if err := recordedAt.Validate(); err != nil {
		return LiveResearchProvenanceResult{}, invalid(operation, fmt.Errorf("provenance clock: %w", err))
	}
	claimsByID := make(map[research.ClaimID]research.Claim, len(request.Claims))
	for _, claim := range request.Claims {
		claimsByID[claim.ID] = claim
	}
	claims := make([]research.Claim, 0, len(request.Bundle.ClaimIDs))
	for _, claimID := range request.Bundle.ClaimIDs {
		claim, exists := claimsByID[claimID]
		if !exists {
			return LiveResearchProvenanceResult{}, invalid(operation,
				fmt.Errorf("bundle Claim %q is missing from live artifacts", claimID))
		}
		claims = append(claims, claim)
	}
	sort.Slice(claims, func(i, j int) bool { return claims[i].ID.String() < claims[j].ID.String() })
	result := LiveResearchProvenanceResult{AlgorithmVersion: LiveResearchProvenanceV1}
	for _, claim := range claims {
		if err := ctx.Err(); err != nil {
			return cloneLiveResearchProvenanceResult(result), Classify(ErrorUnavailable, operation, err)
		}
		graph, err := buildLiveResearchProvenanceGraph(request, claim, recordedAt)
		if err != nil {
			return cloneLiveResearchProvenanceResult(result), invalid(operation, err)
		}
		if err := service.provenance.Record(ctx, graph); err != nil {
			if !errors.Is(err, ErrConflict) {
				return cloneLiveResearchProvenanceResult(result), boundaryError(ErrorPersistenceFailure, operation, err)
			}
			stored, traceErr := service.provenance.Trace(ctx, claim.ID)
			if traceErr != nil || !sameLiveResearchProvenanceGraph(stored, graph) {
				return cloneLiveResearchProvenanceResult(result), boundaryError(ErrorPersistenceFailure, operation, errors.Join(err, traceErr))
			}
			graph = stored
		}
		result.Graphs = append(result.Graphs, cloneProvenanceGraphArtifact(graph))
	}
	if err := result.Validate(); err != nil {
		return LiveResearchProvenanceResult{}, invalid(operation, err)
	}
	return cloneLiveResearchProvenanceResult(result), nil
}

func (service *liveResearchProvenanceService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.Preserve(ctx, LiveResearchProvenanceRequest{
		Request: input.Request, Run: input.Run, Discoveries: input.Artifacts.Discoveries,
		Sources: input.Artifacts.Sources, Snapshots: input.Artifacts.Snapshots,
		Evidence: input.Artifacts.Evidence, Claims: input.Artifacts.Claims,
		Verifications: input.Artifacts.Verifications, Bundle: input.Artifacts.Bundle,
	})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.ProvenanceGraphs = make([]research.ProvenanceGraph, len(result.Graphs))
	for index, graph := range result.Graphs {
		artifacts.ProvenanceGraphs[index] = cloneProvenanceGraphArtifact(graph)
	}
	return artifacts, err
}

func validateLiveResearchProvenanceRequest(request LiveResearchProvenanceRequest) error {
	if err := request.Request.Validate(); err != nil {
		return err
	}
	if err := request.Run.Validate(); err != nil {
		return err
	}
	if request.Run.RequestID != request.Request.ID || request.Run.Status != research.ResearchRunRunning {
		return errors.New("live provenance requires the matching running Research Run")
	}
	if request.Bundle == nil {
		return errors.New("live provenance requires a Source Bundle")
	}
	if err := request.Bundle.Validate(); err != nil {
		return err
	}
	if request.Bundle.RunID != request.Run.ID {
		return errors.New("live provenance bundle belongs to another Research Run")
	}
	if len(request.Claims) == 0 || len(request.Claims) > MaximumClaimCandidatesPerRun {
		return fmt.Errorf("live provenance Claims must contain between 1 and %d entries", MaximumClaimCandidatesPerRun)
	}
	return nil
}

func buildLiveResearchProvenanceGraph(request LiveResearchProvenanceRequest, claim research.Claim, recordedAt research.Timestamp) (research.ProvenanceGraph, error) {
	if err := claim.Validate(); err != nil {
		return research.ProvenanceGraph{}, err
	}
	if claim.Topic != request.Request.Topic || !containsClaimID(request.Bundle.ClaimIDs, claim.ID) {
		return research.ProvenanceGraph{}, fmt.Errorf("Claim %q does not belong to the request and bundle", claim.ID)
	}
	verification, err := liveVerificationForClaim(request.Verifications, claim)
	if err != nil {
		return research.ProvenanceGraph{}, err
	}
	_ = verification // Validation is the durable Claim -> Verification -> Bundle gate.

	sources := make(map[research.SourceID]research.Source, len(request.Sources))
	for _, source := range request.Sources {
		if err := source.Validate(); err != nil {
			return research.ProvenanceGraph{}, err
		}
		sources[source.ID] = source
	}
	snapshots := make(map[research.ID]research.SourceSnapshot, len(request.Snapshots))
	for _, snapshot := range request.Snapshots {
		if err := snapshot.Validate(); err != nil {
			return research.ProvenanceGraph{}, err
		}
		snapshots[snapshot.ID] = snapshot
	}
	evidenceByID := make(map[research.ID]research.Evidence, len(request.Evidence))
	for _, evidence := range request.Evidence {
		if err := evidence.Validate(); err != nil {
			return research.ProvenanceGraph{}, err
		}
		evidenceByID[evidence.ID] = evidence
	}

	nodes := []research.ProvenanceNode{
		{ID: request.Request.ID, Kind: research.ProvenanceRequest, Label: "user research topic", OccurredAt: request.Request.RequestedAt},
		{ID: request.Run.ID, Kind: research.ProvenanceRun, Label: "live research run", OccurredAt: request.Run.StartedAt, ToolVersion: LiveResearchOrchestratorV1},
	}
	edges := []research.ProvenanceEdge{{From: request.Request.ID, To: request.Run.ID}}
	queryNodes := make(map[string]research.ID)
	sourceNodes := make(map[research.SourceID]research.ID)
	snapshotNodes := make(map[research.ID]struct{})

	evidenceItems := make([]research.Evidence, 0, len(claim.EvidenceIDs))
	for _, evidenceID := range claim.EvidenceIDs {
		evidence, exists := evidenceByID[evidenceID]
		if !exists || !containsSourceIDV1(claim.SourceIDs, evidence.SourceID) {
			return research.ProvenanceGraph{}, fmt.Errorf("Claim %q references Evidence outside its live artifacts", claim.ID)
		}
		evidenceItems = append(evidenceItems, evidence)
	}
	sort.Slice(evidenceItems, func(i, j int) bool { return evidenceItems[i].ID.String() < evidenceItems[j].ID.String() })
	for _, evidence := range evidenceItems {
		snapshot, exists := snapshots[evidence.SnapshotID]
		if !exists || snapshot.SourceID != evidence.SourceID {
			return research.ProvenanceGraph{}, fmt.Errorf("Evidence %q does not match a live snapshot", evidence.ID)
		}
		source, exists := sources[evidence.SourceID]
		if !exists || source.ID != snapshot.SourceID || source.Locator != snapshot.Locator {
			return research.ProvenanceGraph{}, fmt.Errorf("Evidence %q does not match a registered live Source", evidence.ID)
		}
		if _, exists := sourceNodes[source.ID]; !exists {
			discovery, err := selectLiveDiscovery(request.Discoveries, request.Request.ID, source.ID)
			if err != nil {
				return research.ProvenanceGraph{}, err
			}
			queryID, exists := queryNodes[discovery.Query]
			if !exists {
				queryID = stableLiveProvenanceID("query", request.Request.ID.String(), discovery.Query)
				queryNodes[discovery.Query] = queryID
				nodes = append(nodes, research.ProvenanceNode{ID: queryID, Kind: research.ProvenanceQuery, Label: discovery.Query, OccurredAt: request.Run.StartedAt, ToolVersion: queryplanner.AlgorithmVersion})
				edges = append(edges, research.ProvenanceEdge{From: request.Run.ID, To: queryID})
			}
			providerTool := "search-provider:" + discovery.Provider
			nodes = append(nodes,
				research.ProvenanceNode{ID: discovery.ID, Kind: research.ProvenanceDiscoveredSource, Label: fmt.Sprintf("search result rank %d", discovery.Rank), OccurredAt: discovery.DiscoveredAt, ToolVersion: providerTool},
				research.ProvenanceNode{ID: mustResearchID(source.ID.String()), Kind: research.ProvenanceSource, Label: "registered source", OccurredAt: source.CreatedAt},
			)
			sourceNodeID := mustResearchID(source.ID.String())
			sourceNodes[source.ID] = sourceNodeID
			edges = append(edges, research.ProvenanceEdge{From: queryID, To: discovery.ID}, research.ProvenanceEdge{From: discovery.ID, To: sourceNodeID})
		}
		sourceNodeID := sourceNodes[source.ID]
		if _, exists := snapshotNodes[snapshot.ID]; !exists {
			nodes = append(nodes, research.ProvenanceNode{ID: snapshot.ID, Kind: research.ProvenanceSnapshot, Label: "immutable source snapshot", OccurredAt: snapshot.FetchedAt, ToolVersion: snapshot.Fetch.FetchVersion})
			edges = append(edges, research.ProvenanceEdge{From: sourceNodeID, To: snapshot.ID})
			snapshotNodes[snapshot.ID] = struct{}{}
		}
		nodes = append(nodes, research.ProvenanceNode{ID: evidence.ID, Kind: research.ProvenanceEvidence, Label: "bounded evidence", OccurredAt: evidence.ExtractedAt, ToolVersion: evidence.ExtractorVersion})
		edges = append(edges, research.ProvenanceEdge{From: snapshot.ID, To: evidence.ID}, research.ProvenanceEdge{From: evidence.ID, To: mustResearchID(claim.ID.String())})
	}
	claimNodeID := mustResearchID(claim.ID.String())
	nodes = append(nodes,
		research.ProvenanceNode{ID: claimNodeID, Kind: research.ProvenanceClaim, Label: "evidence-backed claim", OccurredAt: claim.CreatedAt, ToolVersion: LiveClaimExtractionV1},
		research.ProvenanceNode{ID: request.Bundle.ID, Kind: research.ProvenanceSourceBundle, Label: "verified source bundle", OccurredAt: request.Bundle.VerifiedAt, ToolVersion: request.Bundle.AlgorithmVersion},
	)
	edges = append(edges, research.ProvenanceEdge{From: claimNodeID, To: request.Bundle.ID})
	graph := research.ProvenanceGraph{
		ID:      stableLiveProvenanceID("graph", request.Run.ID.String(), claim.ID.String(), request.Bundle.ID.String()),
		ClaimID: claim.ID, Nodes: nodes, Edges: edges, RecordedAt: recordedAt,
		AlgorithmVersion: research.ProvenanceGraphAlgorithmV1,
	}
	if err := graph.Validate(); err != nil {
		return research.ProvenanceGraph{}, err
	}
	return graph, nil
}

func liveVerificationForClaim(verifications []research.VerificationResult, claim research.Claim) (research.VerificationResult, error) {
	var matched *research.VerificationResult
	for index := range verifications {
		verification := verifications[index]
		if err := verification.Validate(); err != nil {
			return research.VerificationResult{}, err
		}
		if verification.ClaimID != claim.ID {
			continue
		}
		if matched != nil {
			return research.VerificationResult{}, fmt.Errorf("Claim %q has multiple verification results", claim.ID)
		}
		copy := verification
		matched = &copy
	}
	if matched == nil || !sameSourceIDSet(matched.SourceIDs, claim.SourceIDs) {
		return research.VerificationResult{}, fmt.Errorf("Claim %q has no matching verification result", claim.ID)
	}
	return *matched, nil
}

func selectLiveDiscovery(discoveries []research.DiscoveredSource, requestID research.ID, sourceID research.SourceID) (research.DiscoveredSource, error) {
	matches := make([]research.DiscoveredSource, 0)
	for _, discovery := range discoveries {
		if err := discovery.Validate(); err != nil {
			return research.DiscoveredSource{}, err
		}
		if discovery.RequestID == requestID && discovery.SourceID == sourceID {
			matches = append(matches, discovery)
		}
	}
	if len(matches) == 0 {
		return research.DiscoveredSource{}, fmt.Errorf("Source %q has no discovery for the live request", sourceID)
	}
	sort.Slice(matches, func(i, j int) bool {
		left := strings.Join([]string{matches[i].Query, matches[i].Provider, fmt.Sprint(matches[i].Rank), matches[i].ID.String()}, "\x00")
		right := strings.Join([]string{matches[j].Query, matches[j].Provider, fmt.Sprint(matches[j].Rank), matches[j].ID.String()}, "\x00")
		return left < right
	})
	return matches[0], nil
}

func stableLiveProvenanceID(kind string, values ...string) research.ID {
	hash := sha256.Sum256([]byte(strings.Join(append([]string{LiveResearchProvenanceV1, kind}, values...), "\x00")))
	id, err := research.NewID("provenance.live." + kind + "." + hex.EncodeToString(hash[:16]))
	if err != nil {
		panic(err)
	}
	return id
}

func mustResearchID(value string) research.ID {
	id, err := research.NewID(value)
	if err != nil {
		panic(err)
	}
	return id
}

func containsClaimID(values []research.ClaimID, target research.ClaimID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsSourceIDV1(values []research.SourceID, target research.SourceID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func sameSourceIDSet(left, right []research.SourceID) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[research.SourceID]struct{}, len(left))
	for _, value := range left {
		seen[value] = struct{}{}
	}
	for _, value := range right {
		if _, exists := seen[value]; !exists {
			return false
		}
	}
	return true
}

func sameLiveResearchProvenanceGraph(left, right research.ProvenanceGraph) bool {
	leftJSON, leftErr := left.ExportJSON()
	rightJSON, rightErr := right.ExportJSON()
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}

func cloneLiveResearchProvenanceResult(result LiveResearchProvenanceResult) LiveResearchProvenanceResult {
	clone := result
	clone.Graphs = make([]research.ProvenanceGraph, len(result.Graphs))
	for index, graph := range result.Graphs {
		clone.Graphs[index] = cloneProvenanceGraphArtifact(graph)
	}
	return clone
}

type LiveResearchProvenanceService interface {
	Preserve(context.Context, LiveResearchProvenanceRequest) (LiveResearchProvenanceResult, error)
	LiveResearchStageService
}

var (
	_ LiveResearchProvenanceService = (*liveResearchProvenanceService)(nil)
	_ LiveResearchStageService      = (*liveResearchProvenanceService)(nil)
)
