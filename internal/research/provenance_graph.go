package research

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	ProvenanceGraphAlgorithmV1  = "provenance-graph-v1"
	MaximumProvenanceNodes      = 512
	MaximumProvenanceEdges      = 1024
	MaximumProvenanceLabelBytes = 1 << 10
	MaximumProvenanceToolBytes  = 256
	MaximumProvenanceJSONBytes  = 256 << 10
)

type ProvenanceNodeKind string

const (
	ProvenanceRequest          ProvenanceNodeKind = "research_request"
	ProvenanceRun              ProvenanceNodeKind = "research_run"
	ProvenanceQuery            ProvenanceNodeKind = "query"
	ProvenanceDiscoveredSource ProvenanceNodeKind = "discovered_source"
	ProvenanceSource           ProvenanceNodeKind = "source"
	ProvenanceSnapshot         ProvenanceNodeKind = "snapshot"
	ProvenanceEvidence         ProvenanceNodeKind = "evidence"
	ProvenanceClaim            ProvenanceNodeKind = "claim"
	ProvenanceSourceBundle     ProvenanceNodeKind = "source_bundle"
)

func (kind ProvenanceNodeKind) Validate() error {
	switch kind {
	case ProvenanceRequest, ProvenanceRun, ProvenanceQuery,
		ProvenanceDiscoveredSource, ProvenanceSource, ProvenanceSnapshot,
		ProvenanceEvidence, ProvenanceClaim, ProvenanceSourceBundle:
		return nil
	default:
		return fmt.Errorf("invalid provenance node kind %q", kind)
	}
}

func (kind ProvenanceNodeKind) requiresToolVersion() bool {
	switch kind {
	case ProvenanceQuery, ProvenanceDiscoveredSource, ProvenanceSnapshot, ProvenanceEvidence:
		return true
	default:
		return false
	}
}

// ProvenanceNode is bounded audit metadata. Label is explanatory metadata,
// never fetched content or evidence, and ToolVersion identifies automated
// query, discovery, fetch, and extraction steps.
type ProvenanceNode struct {
	ID          ID
	Kind        ProvenanceNodeKind
	Label       string
	OccurredAt  Timestamp
	ToolVersion string
}

func (node ProvenanceNode) Validate() error {
	if err := node.ID.Validate(); err != nil {
		return fmt.Errorf("provenance node: %w", err)
	}
	if err := node.Kind.Validate(); err != nil {
		return err
	}
	if err := requireText("provenance node label", node.Label); err != nil {
		return err
	}
	if !utf8.ValidString(node.Label) {
		return fmt.Errorf("provenance node label is not valid UTF-8")
	}
	if strings.IndexFunc(node.Label, unicode.IsControl) >= 0 {
		return fmt.Errorf("provenance node label contains control characters")
	}
	if len(node.Label) > MaximumProvenanceLabelBytes {
		return fmt.Errorf("provenance node label exceeds %d bytes", MaximumProvenanceLabelBytes)
	}
	if err := validateTimestamp("provenance node occurred at", node.OccurredAt); err != nil {
		return err
	}
	if node.Kind.requiresToolVersion() {
		if err := requireText("provenance node tool version", node.ToolVersion); err != nil {
			return err
		}
	} else if err := validateOptionalText("provenance node tool version", node.ToolVersion); err != nil {
		return err
	}
	if !utf8.ValidString(node.ToolVersion) {
		return fmt.Errorf("provenance node tool version is not valid UTF-8")
	}
	if strings.IndexFunc(node.ToolVersion, unicode.IsControl) >= 0 {
		return fmt.Errorf("provenance node tool version contains control characters")
	}
	if len(node.ToolVersion) > MaximumProvenanceToolBytes {
		return fmt.Errorf("provenance node tool version exceeds %d bytes", MaximumProvenanceToolBytes)
	}
	return nil
}

type ProvenanceEdge struct {
	From ID
	To   ID
}

func (edge ProvenanceEdge) Validate() error {
	if err := edge.From.Validate(); err != nil {
		return fmt.Errorf("provenance edge from: %w", err)
	}
	if err := edge.To.Validate(); err != nil {
		return fmt.Errorf("provenance edge to: %w", err)
	}
	if edge.From == edge.To {
		return fmt.Errorf("provenance edge forms a self-cycle at %q", edge.From)
	}
	return nil
}

// ProvenanceGraph is an immutable, exportable explanation of one claim. It
// contains audit metadata only and deliberately stops at SourceBundle: future
// curriculum concepts belong to I-04.
type ProvenanceGraph struct {
	ID               ID
	ClaimID          ClaimID
	Nodes            []ProvenanceNode
	Edges            []ProvenanceEdge
	RecordedAt       Timestamp
	AlgorithmVersion string
}

func (graph ProvenanceGraph) Validate() error {
	if err := graph.ID.Validate(); err != nil {
		return fmt.Errorf("provenance graph: %w", err)
	}
	if err := graph.ClaimID.Validate(); err != nil {
		return err
	}
	if graph.AlgorithmVersion != ProvenanceGraphAlgorithmV1 {
		return fmt.Errorf("invalid provenance graph algorithm %q", graph.AlgorithmVersion)
	}
	if err := validateTimestamp("provenance graph recorded at", graph.RecordedAt); err != nil {
		return err
	}
	if len(graph.Nodes) == 0 || len(graph.Nodes) > MaximumProvenanceNodes {
		return fmt.Errorf("provenance graph node count must be between 1 and %d", MaximumProvenanceNodes)
	}
	if len(graph.Edges) == 0 || len(graph.Edges) > MaximumProvenanceEdges {
		return fmt.Errorf("provenance graph edge count must be between 1 and %d", MaximumProvenanceEdges)
	}

	nodes := make(map[ID]ProvenanceNode, len(graph.Nodes))
	kindCounts := make(map[ProvenanceNodeKind]int)
	for _, node := range graph.Nodes {
		if err := node.Validate(); err != nil {
			return err
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("provenance graph contains duplicate node %q", node.ID)
		}
		if node.OccurredAt.After(graph.RecordedAt) {
			return fmt.Errorf("provenance node %q occurs after graph recording", node.ID)
		}
		nodes[node.ID] = node
		kindCounts[node.Kind]++
	}
	if kindCounts[ProvenanceRequest] != 1 || kindCounts[ProvenanceRun] != 1 || kindCounts[ProvenanceClaim] != 1 {
		return fmt.Errorf("provenance graph requires exactly one request, run, and claim")
	}
	claimNode, exists := nodeByKind(nodes, ProvenanceClaim)
	if !exists || claimNode.ID.String() != graph.ClaimID.String() {
		return fmt.Errorf("provenance graph claim identity does not match")
	}

	adjacency := make(map[ID][]ID, len(nodes))
	reverse := make(map[ID][]ID, len(nodes))
	seenEdges := make(map[string]struct{}, len(graph.Edges))
	for _, edge := range graph.Edges {
		if err := edge.Validate(); err != nil {
			return err
		}
		from, fromExists := nodes[edge.From]
		to, toExists := nodes[edge.To]
		if !fromExists || !toExists {
			return fmt.Errorf("provenance edge %q -> %q references a missing node", edge.From, edge.To)
		}
		if !validProvenanceTransition(from.Kind, to.Kind) {
			return fmt.Errorf("invalid provenance transition %s -> %s", from.Kind, to.Kind)
		}
		key := edge.From.String() + "\x00" + edge.To.String()
		if _, exists := seenEdges[key]; exists {
			return fmt.Errorf("provenance graph contains duplicate edge %q -> %q", edge.From, edge.To)
		}
		seenEdges[key] = struct{}{}
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
		reverse[edge.To] = append(reverse[edge.To], edge.From)
	}
	if err := validateProvenanceDegree(nodes, adjacency, reverse); err != nil {
		return err
	}
	if hasProvenanceCycle(nodes, adjacency) {
		return fmt.Errorf("provenance graph contains a cycle")
	}
	requestNode, _ := nodeByKind(nodes, ProvenanceRequest)
	if len(reachableProvenanceNodes(requestNode.ID, adjacency)) != len(nodes) {
		return fmt.Errorf("provenance graph contains a node disconnected from its request")
	}
	canReachClaim := reachableProvenanceNodes(claimNode.ID, reverse)
	for id, node := range nodes {
		if node.Kind != ProvenanceSourceBundle {
			if _, exists := canReachClaim[id]; !exists {
				return fmt.Errorf("provenance node %q does not support claim %q", id, graph.ClaimID)
			}
		}
	}
	return nil
}

func nodeByKind(nodes map[ID]ProvenanceNode, kind ProvenanceNodeKind) (ProvenanceNode, bool) {
	for _, node := range nodes {
		if node.Kind == kind {
			return node, true
		}
	}
	return ProvenanceNode{}, false
}

func validProvenanceTransition(from, to ProvenanceNodeKind) bool {
	switch from {
	case ProvenanceRequest:
		return to == ProvenanceRun
	case ProvenanceRun:
		return to == ProvenanceQuery || to == ProvenanceSource
	case ProvenanceQuery:
		return to == ProvenanceDiscoveredSource
	case ProvenanceDiscoveredSource:
		return to == ProvenanceSource
	case ProvenanceSource:
		return to == ProvenanceSnapshot
	case ProvenanceSnapshot:
		return to == ProvenanceEvidence
	case ProvenanceEvidence:
		return to == ProvenanceClaim
	case ProvenanceClaim:
		return to == ProvenanceSourceBundle
	default:
		return false
	}
}

func validateProvenanceDegree(nodes map[ID]ProvenanceNode, adjacency, reverse map[ID][]ID) error {
	for id, node := range nodes {
		incoming, outgoing := len(reverse[id]), len(adjacency[id])
		switch node.Kind {
		case ProvenanceRequest:
			if incoming != 0 || outgoing != 1 {
				return fmt.Errorf("provenance request must have no parent and exactly one run")
			}
		case ProvenanceRun, ProvenanceQuery, ProvenanceDiscoveredSource, ProvenanceSource, ProvenanceSnapshot:
			if incoming != 1 || outgoing == 0 {
				return fmt.Errorf("provenance %s %q requires one parent and at least one child", node.Kind, id)
			}
		case ProvenanceEvidence:
			if incoming != 1 || outgoing != 1 {
				return fmt.Errorf("provenance evidence %q requires one snapshot and one claim", id)
			}
		case ProvenanceClaim:
			if incoming == 0 {
				return fmt.Errorf("provenance claim requires at least one evidence")
			}
		case ProvenanceSourceBundle:
			if incoming != 1 || outgoing != 0 {
				return fmt.Errorf("provenance source bundle %q must terminate one claim path", id)
			}
		}
	}
	return nil
}

func hasProvenanceCycle(nodes map[ID]ProvenanceNode, adjacency map[ID][]ID) bool {
	const (
		unseen = iota
		visiting
		visited
	)
	state := make(map[ID]int, len(nodes))
	var visit func(ID) bool
	visit = func(id ID) bool {
		if state[id] == visiting {
			return true
		}
		if state[id] == visited {
			return false
		}
		state[id] = visiting
		for _, next := range adjacency[id] {
			if visit(next) {
				return true
			}
		}
		state[id] = visited
		return false
	}
	for id := range nodes {
		if state[id] == unseen && visit(id) {
			return true
		}
	}
	return false
}

func reachableProvenanceNodes(start ID, adjacency map[ID][]ID) map[ID]struct{} {
	reached := map[ID]struct{}{start: {}}
	queue := []ID{start}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if _, exists := reached[next]; exists {
				continue
			}
			reached[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return reached
}

// Explain returns a deterministic, human-readable trace without excerpts or
// other unbounded external content.
func (graph ProvenanceGraph) Explain() (string, error) {
	if err := graph.Validate(); err != nil {
		return "", err
	}
	nodes, edges := canonicalProvenanceOrder(graph.Nodes, graph.Edges)
	lines := []string{
		"Claim provenance: " + graph.ClaimID.String(),
		"Graph: " + graph.ID.String(),
		"Algorithm: " + graph.AlgorithmVersion,
		"Recorded: " + graph.RecordedAt.Time().Format(time.RFC3339),
		"Nodes:",
	}
	for _, node := range nodes {
		line := fmt.Sprintf("- %s %s — %s — %s", node.Kind, node.ID, node.Label, node.OccurredAt.Time().Format(time.RFC3339))
		if node.ToolVersion != "" {
			line += " — tool " + node.ToolVersion
		}
		lines = append(lines, line)
	}
	lines = append(lines, "Relationships:")
	for _, edge := range edges {
		lines = append(lines, fmt.Sprintf("- %s -> %s", edge.From, edge.To))
	}
	return strings.Join(lines, "\n"), nil
}

type provenanceGraphJSON struct {
	ID               string               `json:"graph_id"`
	ClaimID          string               `json:"claim_id"`
	Nodes            []provenanceNodeJSON `json:"nodes"`
	Edges            []provenanceEdgeJSON `json:"edges"`
	RecordedAt       string               `json:"recorded_at"`
	AlgorithmVersion string               `json:"algorithm_version"`
}

type provenanceNodeJSON struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Label       string `json:"label"`
	OccurredAt  string `json:"occurred_at"`
	ToolVersion string `json:"tool_version,omitempty"`
}

type provenanceEdgeJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ExportJSON returns stable JSON ordering suitable for audit/export tooling.
func (graph ProvenanceGraph) ExportJSON() ([]byte, error) {
	if err := graph.Validate(); err != nil {
		return nil, err
	}
	nodes, edges := canonicalProvenanceOrder(graph.Nodes, graph.Edges)
	payload := provenanceGraphJSON{
		ID: graph.ID.String(), ClaimID: graph.ClaimID.String(),
		RecordedAt:       graph.RecordedAt.Time().Format(time.RFC3339Nano),
		AlgorithmVersion: graph.AlgorithmVersion,
		Nodes:            make([]provenanceNodeJSON, 0, len(nodes)),
		Edges:            make([]provenanceEdgeJSON, 0, len(edges)),
	}
	for _, node := range nodes {
		payload.Nodes = append(payload.Nodes, provenanceNodeJSON{
			ID: node.ID.String(), Kind: string(node.Kind), Label: node.Label,
			OccurredAt: node.OccurredAt.Time().Format(time.RFC3339Nano), ToolVersion: node.ToolVersion,
		})
	}
	for _, edge := range edges {
		payload.Edges = append(payload.Edges, provenanceEdgeJSON{From: edge.From.String(), To: edge.To.String()})
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode provenance graph: %w", err)
	}
	if len(encoded) > MaximumProvenanceJSONBytes {
		return nil, fmt.Errorf("provenance graph export exceeds %d bytes", MaximumProvenanceJSONBytes)
	}
	return encoded, nil
}

func canonicalProvenanceOrder(nodes []ProvenanceNode, edges []ProvenanceEdge) ([]ProvenanceNode, []ProvenanceEdge) {
	orderedNodes := append([]ProvenanceNode(nil), nodes...)
	sort.Slice(orderedNodes, func(i, j int) bool {
		left, right := provenanceKindOrder(orderedNodes[i].Kind), provenanceKindOrder(orderedNodes[j].Kind)
		if left != right {
			return left < right
		}
		return orderedNodes[i].ID.String() < orderedNodes[j].ID.String()
	})
	orderedEdges := append([]ProvenanceEdge(nil), edges...)
	sort.Slice(orderedEdges, func(i, j int) bool {
		if orderedEdges[i].From != orderedEdges[j].From {
			return orderedEdges[i].From.String() < orderedEdges[j].From.String()
		}
		return orderedEdges[i].To.String() < orderedEdges[j].To.String()
	})
	return orderedNodes, orderedEdges
}

func provenanceKindOrder(kind ProvenanceNodeKind) int {
	switch kind {
	case ProvenanceRequest:
		return 0
	case ProvenanceRun:
		return 1
	case ProvenanceQuery:
		return 2
	case ProvenanceDiscoveredSource:
		return 3
	case ProvenanceSource:
		return 4
	case ProvenanceSnapshot:
		return 5
	case ProvenanceEvidence:
		return 6
	case ProvenanceClaim:
		return 7
	case ProvenanceSourceBundle:
		return 8
	default:
		return 9
	}
}

// ParseProvenanceGraphJSON loads only the bounded canonical export shape and
// validates the complete graph before returning it.
func ParseProvenanceGraphJSON(encoded []byte) (ProvenanceGraph, error) {
	if len(encoded) == 0 || len(encoded) > MaximumProvenanceJSONBytes {
		return ProvenanceGraph{}, fmt.Errorf("provenance graph JSON size must be between 1 and %d bytes", MaximumProvenanceJSONBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var payload provenanceGraphJSON
	if err := decoder.Decode(&payload); err != nil {
		return ProvenanceGraph{}, fmt.Errorf("decode provenance graph: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return ProvenanceGraph{}, fmt.Errorf("decode provenance graph: trailing data")
	}
	graphID, err := NewID(payload.ID)
	if err != nil {
		return ProvenanceGraph{}, err
	}
	claimID, err := NewClaimID(payload.ClaimID)
	if err != nil {
		return ProvenanceGraph{}, err
	}
	recordedAt, err := parseProvenanceTimestamp(payload.RecordedAt)
	if err != nil {
		return ProvenanceGraph{}, fmt.Errorf("provenance recorded at: %w", err)
	}
	graph := ProvenanceGraph{ID: graphID, ClaimID: claimID, RecordedAt: recordedAt, AlgorithmVersion: payload.AlgorithmVersion}
	graph.Nodes = make([]ProvenanceNode, 0, len(payload.Nodes))
	for _, raw := range payload.Nodes {
		id, idErr := NewID(raw.ID)
		if idErr != nil {
			return ProvenanceGraph{}, idErr
		}
		occurredAt, timeErr := parseProvenanceTimestamp(raw.OccurredAt)
		if timeErr != nil {
			return ProvenanceGraph{}, fmt.Errorf("provenance node occurred at: %w", timeErr)
		}
		graph.Nodes = append(graph.Nodes, ProvenanceNode{ID: id, Kind: ProvenanceNodeKind(raw.Kind), Label: raw.Label, OccurredAt: occurredAt, ToolVersion: raw.ToolVersion})
	}
	graph.Edges = make([]ProvenanceEdge, 0, len(payload.Edges))
	for _, raw := range payload.Edges {
		from, fromErr := NewID(raw.From)
		if fromErr != nil {
			return ProvenanceGraph{}, fromErr
		}
		to, toErr := NewID(raw.To)
		if toErr != nil {
			return ProvenanceGraph{}, toErr
		}
		graph.Edges = append(graph.Edges, ProvenanceEdge{From: from, To: to})
	}
	if err := graph.Validate(); err != nil {
		return ProvenanceGraph{}, err
	}
	return graph, nil
}

func parseProvenanceTimestamp(value string) (Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return Timestamp{}, err
	}
	return NewTimestamp(parsed)
}
