# Research & Source Intelligence persistence

Step 03 adds schema version 23 to the workspace-local Foundation SQLite
database. Step 05 adds forward-only migration v24 for topic-aware authority
profiles, and Step 06 adds v25 for the Trusted Source Registry. The 22
migrations that shipped Student Core and migrations v23/v24 remain unchanged,
and an I-02 database is upgraded without rewriting its learning state.

## Adapter boundary

`Database.Repositories().Research` exposes the twelve narrow repository ports
defined by `internal/research/application`. The adapter depends on the research
domain and application contracts; neither package imports SQLite.

Repository behavior matches the deterministic memory adapter:

- singular misses are `not_found`, immutable duplicate identities are
  `conflict`, and invalid relationships are `invalid_state`;
- source locator and stable source identity are independently unique;
- request identity is immutable and may own multiple research runs;
- snapshots, evidence, verification results, drift reports, and impact reports
  are append-only through their ports;
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
verification by claim, and due cache/freshness records. UTC timestamps continue
to use Foundation's fixed-width representation so chronological `TEXT` ordering
is stable.

## Retention boundary

Snapshots contain transport metadata and a content hash, never a fetched web
body. Evidence stores the excerpt separately from metadata and limits it to
8 KiB. Opaque cache payloads are capped at 1 MiB. These limits are enforced by
both adapters and SQLite constraints, preventing this initial persistence layer
from becoming an unbounded raw-content archive.

No credential or secret columns exist. Source aliases, release/deprecation
records, claims, citations, bundles, and conflicts have schema representation
but no new application services in this step; later authorized steps may add
ports without changing the domain's dependency direction.

## Deferred behavior

Step 03 does not implement Trust Policy, authority matching, network access,
fetching, parsing, evidence extraction, freshness formulas, release discovery,
conflict resolution, cache eviction, drift detection, impact analysis, CLI/TUI
surfaces, curriculum compilation, or Student Core mutations.
