# Provenance Graph v1

Step 14 makes claim origin queryable without treating discovery metadata as
evidence. `provenance-graph-v1` is a bounded, directed, acyclic audit graph for
one stable `ClaimID`:

```text
ResearchRequest
      ↓
ResearchRun
      ↓
Query ───────────────┐
      ↓              │ manual reviewed source
DiscoveredSource     │
      ↓              ↓
Source
      ↓
SourceSnapshot
      ↓
Evidence
      ↓
Claim
      ↓ optional
SourceBundle
```

The graph stops at `SourceBundle`. Curriculum concepts are deliberately absent
because they belong to I-04.

## Nodes and relationships

Every node has a stable ID, closed kind, bounded human label, UTC occurrence
time, and optional tool version. Query, discovery, snapshot, and evidence nodes
require their tool version so planning, provider, fetch, and extraction output
can be attributed. The graph itself always carries the immutable algorithm
identifier `provenance-graph-v1` and a recording timestamp.

Labels and tool versions must be valid UTF-8 without control characters. This
keeps untrusted source metadata from emitting terminal control sequences when a
trace is explained by the CLI.

Allowed relationships are closed:

```text
request -> run
run -> query
query -> discovered_source
discovered_source -> source
run -> source                 # manually registered/reviewed source
source -> snapshot
snapshot -> evidence
evidence -> claim
claim -> source_bundle
```

One graph represents exactly one request, run, and claim. It may contain many
queries, candidates, sources, snapshots, and evidence records. Every pre-claim
node must participate in a path to the claim, and every node must be reachable
from the request. This prevents unrelated audit metadata from being attached to
a claim explanation.

The validator rejects duplicate IDs/edges, missing endpoints, invalid node
kinds or transitions, disconnected paths, self-cycles, general cycles, future
timestamps, missing evidence, and invalid terminal bundle links. The closed
forward-only transition vocabulary also makes most cycles unrepresentable.

## Historical snapshots and multiple sources

A snapshot node identifies the exact observation used; it is not replaced by a
newer snapshot during trace lookup. Consequently a claim may remain reproducible
against historical evidence while another branch uses a current snapshot.
Multiple evidence branches converge on the single claim node, preserving each
independent source path.

Discovery is optional. A reviewed source that was manually registered uses the
explicit `run -> source` relationship. A provider result, snippet, or rank never
skips directly to Evidence.

## Explain and export

`ProvenanceGraph.Explain` returns a deterministic, human-readable node and edge
listing. It contains bounded labels and identities, not source bodies or
evidence excerpts. `ExportJSON` emits nodes by lifecycle order and edges by
stable identity; `ParseProvenanceGraphJSON` applies a strict unknown-field,
size, and full-graph validation before accepting an export.

Safety limits are explicit:

| Value | Maximum |
| --- | ---: |
| nodes | 512 |
| edges | 1,024 |
| node label | 1 KiB UTF-8 |
| tool version | 256 bytes UTF-8 |
| encoded graph | 256 KiB |

## Persistence and use case

Forward-only SQLite migration v27 appends immutable graph exports in
`provenance_graphs`, indexed by claim and recording time. JSON identity and
algorithm fields must match their indexed columns, and the 256 KiB limit is
repeated in SQLite. The repository returns the newest recording for a claim;
older graphs remain available as audit history rather than being overwritten.

`ProvenanceService` records, traces, and exports validated graphs. The memory
adapter preserves defensive ownership and the SQLite adapter validates stored
JSON again when reading it.

The internal read command is:

```text
kelyro sources trace <claim-id>
```

It opens the workspace-local research store and prints `Explain`. It performs
no network access and does not manufacture a graph when the claim has no stored
provenance.

## Boundaries

Step 14 adds no Citation/Deep Link generation, evidence extraction, claim
verification, freshness or conflict algorithms, live search, Curriculum
Compiler behavior, AI inference, or Student Core mutation.
