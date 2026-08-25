package research

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestProvenanceGraphValidatesFullMultiSourceHistoricalChain(t *testing.T) {
	t.Parallel()

	graph := provenanceGraphFixture(t)
	if err := graph.Validate(); err != nil {
		t.Fatalf("ProvenanceGraph.Validate() error = %v", err)
	}
	explanation, err := graph.Explain()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"claim.interfaces",
		"query.spec",
		"source.release",
		"snapshot.spec.historical",
		"evidence.release",
		"bundle.interfaces",
		"tool query-planner-v1",
	} {
		if !strings.Contains(explanation, want) {
			t.Fatalf("Explain() = %q, missing %q", explanation, want)
		}
	}
}

func TestProvenanceGraphRejectsMissingNodeAndInvalidCycles(t *testing.T) {
	t.Parallel()

	t.Run("missing node", func(t *testing.T) {
		graph := provenanceGraphFixture(t)
		graph.Nodes = graph.Nodes[:len(graph.Nodes)-1]
		if err := graph.Validate(); err == nil || !strings.Contains(err.Error(), "missing node") {
			t.Fatalf("Validate() error = %v, want missing node", err)
		}
	})

	t.Run("self cycle", func(t *testing.T) {
		graph := provenanceGraphFixture(t)
		graph.Edges = append(graph.Edges, ProvenanceEdge{From: mustID(t, "claim.interfaces"), To: mustID(t, "claim.interfaces")})
		if err := graph.Validate(); err == nil || !strings.Contains(err.Error(), "self-cycle") {
			t.Fatalf("Validate() error = %v, want self-cycle", err)
		}
	})

	t.Run("disconnected node", func(t *testing.T) {
		graph := provenanceGraphFixture(t)
		graph.Nodes = append(graph.Nodes, ProvenanceNode{
			ID: mustID(t, "query.unused"), Kind: ProvenanceQuery, Label: "unused query",
			OccurredAt: provenanceTimestamp(t, 2025, 1, 1), ToolVersion: "query-planner-v1",
		})
		if err := graph.Validate(); err == nil {
			t.Fatal("Validate() accepted a disconnected query")
		}
	})

	t.Run("unbounded tool version", func(t *testing.T) {
		graph := provenanceGraphFixture(t)
		for index := range graph.Nodes {
			if graph.Nodes[index].Kind == ProvenanceQuery {
				graph.Nodes[index].ToolVersion = strings.Repeat("v", MaximumProvenanceToolBytes+1)
				break
			}
		}
		if err := graph.Validate(); err == nil || !strings.Contains(err.Error(), "tool version exceeds") {
			t.Fatalf("Validate() error = %v, want bounded tool version", err)
		}
	})

	t.Run("unsafe display metadata", func(t *testing.T) {
		graph := provenanceGraphFixture(t)
		graph.Nodes[0].Label = "request\x1b[31m"
		if err := graph.Validate(); err == nil || !strings.Contains(err.Error(), "control characters") {
			t.Fatalf("Validate() error = %v, want control-character rejection", err)
		}
	})
}

func TestProvenanceGraphSupportsManualSourcesAndStableJSONExport(t *testing.T) {
	t.Parallel()

	recorded := provenanceTimestamp(t, 2025, 2, 1)
	graph := ProvenanceGraph{
		ID: mustID(t, "graph.manual"), ClaimID: mustClaimID(t, "manual"),
		RecordedAt: recorded, AlgorithmVersion: ProvenanceGraphAlgorithmV1,
		Nodes: []ProvenanceNode{
			{ID: mustID(t, "request.manual"), Kind: ProvenanceRequest, Label: "manual research", OccurredAt: provenanceTimestamp(t, 2025, 1, 1)},
			{ID: mustID(t, "run.manual"), Kind: ProvenanceRun, Label: "manual run", OccurredAt: provenanceTimestamp(t, 2025, 1, 2)},
			{ID: mustID(t, "source.manual"), Kind: ProvenanceSource, Label: "reviewed source", OccurredAt: provenanceTimestamp(t, 2024, 1, 1)},
			{ID: mustID(t, "snapshot.manual"), Kind: ProvenanceSnapshot, Label: "historical snapshot", OccurredAt: provenanceTimestamp(t, 2024, 6, 1), ToolVersion: "fetch/v1"},
			{ID: mustID(t, "evidence.manual"), Kind: ProvenanceEvidence, Label: "section 2", OccurredAt: provenanceTimestamp(t, 2025, 1, 3), ToolVersion: "extract/v1"},
			{ID: mustID(t, "claim.manual"), Kind: ProvenanceClaim, Label: "manual claim", OccurredAt: provenanceTimestamp(t, 2025, 1, 4)},
		},
		Edges: []ProvenanceEdge{
			{From: mustID(t, "request.manual"), To: mustID(t, "run.manual")},
			{From: mustID(t, "run.manual"), To: mustID(t, "source.manual")},
			{From: mustID(t, "source.manual"), To: mustID(t, "snapshot.manual")},
			{From: mustID(t, "snapshot.manual"), To: mustID(t, "evidence.manual")},
			{From: mustID(t, "evidence.manual"), To: mustID(t, "claim.manual")},
		},
	}
	first, err := graph.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := ParseProvenanceGraphJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := loaded.ExportJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("export is not stable:\n%s\n---\n%s", first, second)
	}
	if !bytes.Contains(first, []byte(ProvenanceGraphAlgorithmV1)) {
		t.Fatalf("export = %s", first)
	}
}

func provenanceGraphFixture(t *testing.T) ProvenanceGraph {
	t.Helper()
	at := func(month time.Month, day int) Timestamp { return provenanceTimestamp(t, 2025, month, day) }
	node := func(id string, kind ProvenanceNodeKind, label, tool string, timestamp Timestamp) ProvenanceNode {
		return ProvenanceNode{ID: mustID(t, id), Kind: kind, Label: label, ToolVersion: tool, OccurredAt: timestamp}
	}
	edge := func(from, to string) ProvenanceEdge { return ProvenanceEdge{From: mustID(t, from), To: mustID(t, to)} }
	return ProvenanceGraph{
		ID: mustID(t, "graph.interfaces.001"), ClaimID: mustClaimID(t, "interfaces"),
		RecordedAt: at(time.February, 1), AlgorithmVersion: ProvenanceGraphAlgorithmV1,
		Nodes: []ProvenanceNode{
			node("request.interfaces", ProvenanceRequest, "interfaces research", "", at(time.January, 1)),
			node("run.interfaces", ProvenanceRun, "completed run", "", at(time.January, 2)),
			node("query.spec", ProvenanceQuery, "interface specification", "query-planner-v1", at(time.January, 2)),
			node("query.release", ProvenanceQuery, "interface release notes", "query-planner-v1", at(time.January, 2)),
			node("discovery.spec", ProvenanceDiscoveredSource, "spec candidate", "static-search/v1", at(time.January, 3)),
			node("discovery.release", ProvenanceDiscoveredSource, "release candidate", "static-search/v1", at(time.January, 3)),
			node("source.spec", ProvenanceSource, "language specification", "", at(time.January, 3)),
			node("source.release", ProvenanceSource, "release notes", "", at(time.January, 3)),
			node("snapshot.spec.historical", ProvenanceSnapshot, "specification snapshot", "fetch/v1", provenanceTimestamp(t, 2024, time.June, 1)),
			node("snapshot.release.current", ProvenanceSnapshot, "release snapshot", "fetch/v1", at(time.January, 4)),
			node("evidence.spec", ProvenanceEvidence, "section Interface types", "extract/v1", at(time.January, 5)),
			node("evidence.release", ProvenanceEvidence, "release heading Interfaces", "extract/v1", at(time.January, 5)),
			node("claim.interfaces", ProvenanceClaim, "interfaces changed in the target release", "", at(time.January, 6)),
			node("bundle.interfaces", ProvenanceSourceBundle, "ready source bundle", "", at(time.January, 7)),
		},
		Edges: []ProvenanceEdge{
			edge("request.interfaces", "run.interfaces"),
			edge("run.interfaces", "query.spec"), edge("run.interfaces", "query.release"),
			edge("query.spec", "discovery.spec"), edge("query.release", "discovery.release"),
			edge("discovery.spec", "source.spec"), edge("discovery.release", "source.release"),
			edge("source.spec", "snapshot.spec.historical"), edge("source.release", "snapshot.release.current"),
			edge("snapshot.spec.historical", "evidence.spec"), edge("snapshot.release.current", "evidence.release"),
			edge("evidence.spec", "claim.interfaces"), edge("evidence.release", "claim.interfaces"),
			edge("claim.interfaces", "bundle.interfaces"),
		},
	}
}

func provenanceTimestamp(t *testing.T, year int, month time.Month, day int) Timestamp {
	t.Helper()
	value, err := NewTimestamp(time.Date(year, month, day, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
