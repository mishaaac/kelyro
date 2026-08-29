# Research Audit Trail and reproducibility metadata v1

## Purpose

`research-audit-v1` records how a Research Run was planned and observed so a
human can audit the inputs and decisions behind its future Source Bundles. It
records bounded metadata and hashes, never raw fetched bodies, and does not
claim that a changing external service can be replayed byte-for-byte later.

```text
ResearchRequest
  └─ ResearchRun
       ├─ ResearchRunAudit (planned)
       ├─ ResearchRunAudit (running, optional)
       └─ ResearchRunAudit (completed | failed | cancelled)
            └─ exact queries + algorithms + source/snapshot/hash refs
```

The records are lifecycle checkpoints, not mutable status rows. Appending a
later checkpoint never rewrites the facts visible at an earlier point.

## Metadata contract

Every checkpoint contains:

```text
audit_id
run_id
recorded_at
started_at
completed_at optional
outcome
query_planner_version
trust_policy_version
freshness_version
conflict_resolver_version
providers_used[]
network_mode
network_allowed
cache_hits
source_count
bytes_fetched
queries[]
sources[] { source_id, locator, snapshot_id, snapshot_hash }
target_technology
target_version optional
additional_algorithms[] { stage, version }
algorithm_version
content_hash
```

The four first-class algorithm versions name the policies selected for the
run. They do not by themselves prove that every stage produced an output;
outcome, source records, bundles, and additional durable results show what was
actually produced. `providers_used` contains only providers actually called,
not configured providers. Zero providers, cache hits, sources, or fetched bytes
remain explicit rather than being filled with invented activity.

`network_mode` is `offline`, `online`, or `auto`. `network_allowed` preserves
the resolved Foundation privacy gate independently from intent; `offline`
cannot claim network was allowed. No audit record grants network permission or
bypasses `privacy.allow_network`.

`outcome` reuses the Research Run lifecycle (`planned`, `running`, `completed`,
`failed`, `cancelled`). Terminal checkpoints require the exact durable
completion time; non-terminal checkpoints cannot contain one.

## Reproducibility and integrity

Query order is preserved because planner priority is meaningful. Providers,
source/snapshot refs, and additional algorithm stages are canonicalized into
stable order. Duplicate queries, providers, snapshots, or algorithm stages are
rejected.

Each source record binds all of:

```text
stable Source ID
exact HTTP(S) locator fetched
immutable Snapshot ID
canonical SHA-256 decoded-content hash
```

The memory and SQLite adapters verify those values against durable Source and
Snapshot data before append. A locator or hash mismatch is invalid state; a
missing run/source/snapshot is not found. Search candidates without a durable
snapshot are not included as observed sources.

The entire canonical JSON representation, except `content_hash` itself, is
bound to `sha256:<64 lowercase hex>`. Parsing rejects unknown fields, trailing
data, invalid hashes, and payloads larger than 256 KiB. Queries and free text
are individually bounded, and the model caps queries, providers, algorithms,
and source records. No excerpts, page bodies, transcripts, credentials,
headers, workspace paths, or secrets belong in this metadata.

This supports reproducibility of stored inputs, selected algorithms, and
decisions. It intentionally makes the narrower promise:

> Stored metadata can reproduce the run inputs and decisions; it cannot
> guarantee that the future Internet will return the same content.

The durable Snapshot hash is therefore the historical fact. A future fetch
with different bytes creates a new Snapshot and audit checkpoint; it does not
invalidate or rewrite the old record.

## Application service

`ResearchService.RecordAudit` validates the sealed record against the durable
Research Run and Request before append:

- run ID, start/completion timestamps, and outcome must match the current run;
- target technology/version must match the immutable request;
- source/snapshot/locator/hash tuples must match persistence;
- duplicate audit IDs or checkpoint timestamps conflict.

`ResearchService.AuditTrail` returns defensive copies in deterministic
`recorded_at, audit_id` order. Recording is explicit after lifecycle changes;
updating a run cannot infer providers, cache behavior, sources, or algorithm
outputs and therefore does not fabricate a checkpoint automatically.

The manual `research topic` workflow does know its planning inputs, so it
describes them at the application level by appending an initial
`planned` checkpoint immediately after creating the run. It records the exact
query plan, all selected policy versions, `auto` network intent, the resolved
privacy gate, and zero activity because no live adapter was called.

## SQLite compatibility

Forward-only migration v43 adds `research_run_audit` with a Research Run
foreign key, unique `(run_id, recorded_at)`, deterministic history index,
bounded JSON, canonical hash and `research-audit-v1` constraints. Update and
delete triggers make checkpoints append-only.

Existing runs receive no synthetic audit row: absence remains visible as "not
recorded". This avoids pretending that legacy providers, queries, algorithms,
network decisions, cache hits, locators, or hashes are known.

## CLI

```bash
kelyro research show <run-id>
```

The command reads only workspace-local durable data and prints the request/run,
ordered checkpoints, lifecycle times, four required algorithm versions,
providers, network intent and gate, counters, queries, exact snapshot hashes,
additional algorithms, and audit hash. A run with no records says so; it does
not synthesize metadata. Every result includes the future-Internet disclaimer.

`research status <run-id>` remains the concise current-state/bundle view;
`research show <run-id>` is the detailed reproducibility view.

## Boundaries

Research audit does not replay network calls, fetch sources, verify Claims,
resolve conflicts, calculate freshness, create Source Bundles, compile
curriculum, migrate student progress, or export external content. External
source strings remain untrusted data and are never interpreted as
instructions.
