# Research & Source Intelligence persistence

Step 03 adds schema version 23 to the workspace-local Foundation SQLite
database. Step 05 adds forward-only migration v24 for topic-aware authority
profiles, Step 06 adds v25 for the Trusted Source Registry, Step 13 adds v26
for structured Evidence context and Claim scopes, and Step 14 adds v27 for
bounded claim provenance graphs. Step 15 adds v28 for stable citation metadata
and evidence lookup. Step 16 adds v29 for Authority Profile freshness TTL hints,
Step 17 adds v30 for refresh scheduling metadata, Step 21 adds v31 for versioned
deprecation conclusions, and Step 22 adds v32 for source temporal scopes. The 22
migrations that shipped Student Core and migrations v23–v31 remain unchanged,
and an I-02 database is upgraded
without rewriting its learning state.

## Adapter boundary

`Database.Repositories().Research` exposes the narrow Research repository ports
defined by `internal/research/application`. The adapter depends on the research
domain and application contracts; neither package imports SQLite.

Repository behavior matches the deterministic memory adapter:

- singular misses are `not_found`, immutable duplicate identities are
  `conflict`, and invalid relationships are `invalid_state`;
- source locator and stable source identity are independently unique;
- request identity is immutable and may own multiple research runs;
- snapshots, evidence, citations, deprecation conclusions, verification
  results, drift reports, and impact reports are append-only through their
  ports;
- provenance graphs are append-only and latest lookup is deterministic by
  claim, recording time, and stable graph ID;
- authority profiles, freshness state, and cache entries use explicit upsert
  semantics;
- collection and latest-record queries have deterministic ordering.

All database and context failures cross the adapter as the application error
taxonomy introduced in Step 02. Transactions created by Foundation receive the
same repository bundle and therefore keep Research writes inside the caller's
existing transaction boundary.

## Schema groups

Migration 23 creates the following durable groups:

- research requests/topics and runs;
- stable sources, locator aliases, and immutable source snapshots;
- authority profiles and append-only trust decisions;
- bounded evidence excerpts, claims, claim/source links, and citations;
- source bundles and ordered bundle items;
- release, deprecation, freshness, verification, and conflict records;
- bounded research cache entries;
- drift and impact reports.

Migration 24 extends `authority_profiles` with preferred domain and
organization JSON arrays, minimum corroboration, and supplementary source
kinds. Existing v23 records receive empty arrays and corroboration `1`.

Migration 25 creates `source_registry_entries` with validated arrays for
canonical domains, source kinds, authority hints, research domains, and topic
patterns. Insert/update triggers reject a canonical domain already owned by a
different registry entry; status/organization listing is indexed.

Migration 26 adds optional 2 KiB `context_before`/`context_after` fields to
Evidence and required bounded `scope` plus closed `status_scope` fields to
Claims. Existing rows receive empty contexts, `general` scope, and `all` status
scope. The Evidence adapter reads and writes the new context fields; Claim
repository behavior is added by Step 20 without changing this schema.

Migration 27 creates `provenance_graphs`. Each row stores a validated canonical
JSON graph capped at 256 KiB alongside its graph ID, claim ID, recording time,
and immutable `provenance-graph-v1` algorithm identifier. SQLite verifies that
the indexed identity/version columns agree with the JSON payload and indexes
latest lookup without overwriting older audit history.

Migration 28 extends `citations` with its closed deep-link strategy, required
2 KiB section/path hint, optional opaque version scope, and immutable
`citation-v1` algorithm identifier. Existing deep links are conservatively
classified as generic URL anchors; rows without one use canonical fallback. An
evidence/ID index provides deterministic `ListByEvidence` reads.

Migration 29 adds a bounded JSON array of `freshness-v1` TTL hints to Authority
Profiles. Existing profiles receive an empty array; repository reads validate
each claim/source selector and its 1–3,650 day TTL through the domain contract.

Migration 30 adds one bounded, checked scheduling JSON object containing the
verification reason, priority, and immutable `refresh-scheduling-v1` identifier
to `freshness_state`. Existing scheduled rows are conservatively read as normal
TTL deadlines; unscheduled rows remain unscheduled. Due reads order critical,
high, and normal records before applying deadline and stable-ID tie breakers.

Migration 31 adds a closed determination and algorithm version to
`deprecation_records`, plus an index for exact-subject chronological history.
Existing rows are conservatively marked `legacy_unclassified` with
`deprecation-unversioned-legacy`; they are never rewritten as explicit or
inferred evidence. New repository writes accept only
`deprecation-intelligence-v1`. SQLite rejects mismatched legacy/v1 markers and
single-source rows labeled as multi-source inference.

Migration 32 adds the closed `current`/`historical`/`version_bound`/`archived`
scope to sources and citations. Version-bound rows require a version. Citations
also store their deterministic warning and temporal algorithm marker; prior
rows remain readable as conservative current records with
`source-temporal-legacy-current`, while new repository writes require
`source-temporal-policy-v1`. Source bundle source items are backfilled as
current, claim items retain null scope, and insert/update triggers preserve that
typed distinction.

Step 19 requires no migration. The v23 `release_records.version` text preserves
the exact `VersionIdentifier`; strict semantic, supported date-based, and
opaque classification is reconstructed deterministically after reads. Existing
release rows therefore remain compatible without a scheme backfill.

Step 20 also requires no migration. `ClaimRepository` uses the existing
`claims`/`claim_sources` tables and validates that every declared source has
supporting Evidence. `ReleaseIngestionRepository` commits new Evidence, Claims,
release rows, and lifecycle-only status updates atomically. A release status may
change from current to superseded; its identity and all other fields remain
immutable.

Step 21 activates the existing deprecation table through an append-only
repository. Each write verifies that all declared sources exist, every Evidence
ID exists, Evidence belongs to a declared source, and every declared source has
supporting Evidence. Reads return exact-subject history ordered by verification
time and stable ID; later removed/legacy records do not overwrite earlier
deprecated guidance.

Step 22 permits an explicit source-scope update after validating the complete
stored source, including the version required by `version_bound`. It does not
rewrite immutable snapshots, Evidence, citations, or bundle item annotations.
The citation repository persists new temporal annotations only after validating
the full source/snapshot/evidence relationship.

The schema stores request topic fields directly in `research_topics`; its
`request_id` is the stable request identity referenced by one or more runs.
Small ordered identity collections and versioned reason records that do not
need independent queries are stored as validated JSON arrays. Queryable
relationships such as claim/source membership and bundle items have dedicated
tables.

## Integrity and query protection

SQLite foreign keys protect the core source → snapshot → evidence chain. A
composite snapshot key prevents evidence from naming a valid snapshot with the
wrong source. Run/request, trust/source, claim/source, bundle/run, citation
evidence, and impact/drift relationships are also constrained where their
records can be created together in this schema.

Indexes cover canonical locator lookup, aliases, latest snapshots, request
runs, claim topics, last-verified state, releases by technology/version,
verification and provenance by claim, citation lookup by evidence, and due
cache/freshness records. UTC timestamps continue to use Foundation's
fixed-width representation so
chronological `TEXT` ordering is stable.

## Retention boundary

Snapshots contain transport metadata and a content hash, never a fetched web
body. Evidence stores the excerpt separately from metadata and limits it to
8 KiB, with at most 2 KiB of optional context on each side. Opaque cache
payloads are capped at 1 MiB. These limits are enforced by both adapters and
SQLite constraints, preventing this persistence layer from becoming an
unbounded raw-content archive.

Step 09 uses this schema without a new migration. Each live `2xx` or `304`
observation appends a new row; the service never updates an earlier row. A
`304` row carries forward the canonical content identity and bounded metadata
of the snapshot it revalidated, while its status, fetch time, and fetch version
record the new observation.

No credential or secret columns exist. Source aliases, bundles, and conflicts
retain schema representation for later authorized application behavior.

## Deferred behavior

Step 03 does not implement Trust Policy, authority matching, network access,
fetching, parsing, evidence extraction, release discovery,
conflict resolution, cache eviction, drift detection, impact analysis, CLI/TUI
surfaces, curriculum compilation, or Student Core mutations.
