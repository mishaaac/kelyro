# Research Engine application boundaries

The Research & Source Intelligence application boundary lives in
`internal/research/application`. It exposes persistence-neutral use cases and
adapter-neutral external operations while keeping `internal/research`
independent from databases, HTTP, search providers, parsers, clocks, and UI.

## Dependency direction

```text
future CLI / TUI / compiler consumers
                 |
                 v
       application service interfaces
          |                   |
          v                   v
  narrow repository ports   external ports
          ^                   ^
          |                   |
 SQLite / memory adapters   search/fetch/normalize adapters

                 application
                      |
                      v
              research domain values
```

The domain never imports application. Application depends on the research
domain and Foundation's narrow `privacy.NetworkGate` contract, plus the Go
standard library. No concrete adapter or resolved configuration is selected
here.

## Repository ports

Persistence is divided by aggregate or durable output:

| Port | Responsibility |
| --- | --- |
| `SourceRepository` | Stable source identities, canonical locators, and source metadata. |
| `SnapshotRepository` | Immutable fetch snapshots ordered by `fetched_at`. |
| `EvidenceRepository` | Immutable evidence tied to a source and snapshot. |
| `ProvenanceRepository` | Immutable bounded claim graphs and latest trace lookup. |
| `ResearchRunRepository` | Research requests and one or more runs for each request. |
| `TrustRegistryRepository` | Authority profiles and versioned trust decisions. |
| `SourceRegistryRepository` | Reviewed source-family entries with deterministic list/show access. |
| `ReleaseRepository` | Evidence-backed technology release records. |
| `FreshnessRepository` | Versioned freshness outputs and due-state queries. |
| `VerificationRepository` | Immutable verification results by claim. |
| `DriftRepository` | Immutable drift reports. |
| `ImpactRepository` | Immutable impact reports linked to drift. |
| `ResearchCacheRepository` | Opaque, bounded cache entries that are never evidence truth. |

There is deliberately no `ResearchRepository` mega-interface. `Repositories`
is only a wiring struct for adapters and tests; services receive the individual
ports they need. No transaction abstraction is introduced in this step because
the initial services perform single-record operations. A later atomic use case
must introduce a transaction boundary based on concrete consistency needs, not
speculation.

Singular reads return `not_found` when absent. Collection reads return an empty,
deterministically ordered slice. Immutable records use `Create` or `Append`, so
duplicate stable identities are `conflict`; materialized outputs such as
freshness and authority profiles use explicit `Save` semantics.

The research-run fake accepts multiple distinct runs for the same immutable
request. Reusing a request ID with different request data is a conflict.

## External adapter ports

The following application-owned contracts prevent provider and transport types
from entering domain or services:

- `SearchProvider` accepts a normalized `SearchQuery` and separate validated
  `SearchOptions`, then returns candidate `SearchResult` values. Results remain
  candidates, never evidence. The service deterministically normalizes and
  deduplicates locators while preserving provider order and ranks.
- `SourceFetcher` accepts a bounded `FetchRequest` and returns `FetchedSource`
  bytes plus transport-neutral `FetchMetadata`; the privacy-gated service marks
  the result origin as `live` or `cache`.
- `ReleaseLookupProvider` accepts a validated technology/channel query and
  returns release candidates for later evidence and verification work.
- `SearchCache`, `SourceFetchCache`, and `ReleaseLookupCache` expose explicitly
  offline fallback reads and cannot be confused with their live counterparts.
- `SourceNormalizer` converts fetched bytes into a `NormalizedSource` without
  exposing parser-specific nodes.
- `MetadataExtractor` derives `SourceMetadata` from normalized data.
- `Clock` supplies a validated research timestamp to time-dependent use cases.

Step 07 protects every live call with Foundation's privacy gate. Steps 08–09
implement the hardened HTTP transport and `SourceFetcher`, Step 10 implements
the deterministic `SourceNormalizer`, and Step 11 completes the vendor-neutral
search contract with a static network-free provider. Step 12 adds the pure
`query-planner-v1` producer for query text, desired kind, authority threshold,
and execution priority; callers add request ID, result limit, and target-version
options when mapping a plan item into these discovery contracts. A production
search adapter, cache encoding, release discovery, and separate metadata
extraction remain unimplemented.
`MaximumBytes` is enforced by the fetch adapter as a request-specific limit
below the transport's configured global ceiling. A safe redirect may change
the returned locator without changing `SourceID`.

## Initial application services

The initial services are deliberately thin:

- `ResearchService` creates and updates validated request/run state;
- `DiscoveryService` normalizes and validates queries/options, enforces research
  mode/privacy, delegates to either the live provider or explicit offline cache,
  and returns bounded unique candidates without reranking them;
- `FetchService` applies the same boundary to live/cached source retrieval;
- `SnapshotCaptureService` resolves the latest source snapshot, sends its
  conditional validators through `FetchService`, verifies canonical content
  metadata, and appends a new immutable observation;
- `ReleaseLookupService` applies it to live/cached release lookups without
  persisting candidates automatically;
- `SourceService` registers sources and records snapshots for known sources;
- `SourceRegistryService` saves and queries reviewed registry entries;
- `ProvenanceService` records, traces, and exports validated claim graphs;
- `VerificationService` records and retrieves verification results;
- `FreshnessService` stores already-computed, versioned freshness outputs;
- `ReleaseIntelligenceService` records and reads release facts;
- `DriftService` records and reads drift reports;
- `ImpactService` records and reads impact reports.

They validate input, enforce immediate identity relationships, require their
dependencies, delegate bounded operations, and translate errors. Snapshot
capture is the first orchestration that reads prior immutable metadata before
one append; it does not update history or persist raw bodies. They do not
implement Trust Policy, authority matching, query execution orchestration,
evidence extraction, verification rules, freshness formulas, release discovery,
conflict resolution, drift detection, or impact analysis. Those remain future
versioned policies and orchestration steps.

## Error taxonomy

Every error crossing the application boundary has one stable kind:

| Kind | Meaning |
| --- | --- |
| `not_found` | A requested singular record does not exist. |
| `conflict` | A stable identity or immutable record already exists incompatibly. |
| `invalid_state` | Input or an immediate relationship violates a domain/use-case invariant. |
| `unavailable` | A dependency is missing, or context was cancelled/timed out. |
| `persistence_failure` | An unclassified repository/storage operation failed. |
| `external_failure` | An unclassified external provider/adapter operation failed. |
| `network_research_blocked` | The requested live research is disallowed and no usable offline result is available. |

`application.Error` preserves the underlying cause for diagnostics while
allowing callers to branch with `errors.Is` or `KindOf`. Repositories and
external providers have distinct fallback mappings, so a search outage cannot
masquerade as SQLite corruption and a database error cannot masquerade as a
provider outage. Already-classified errors retain their kind when a service
adds operation context.

The research modes and privacy authorization sequence are specified in
[research-network-privacy.md](research-network-privacy.md).

## Deterministic memory fake

`internal/research/application/memory` implements every repository port using
mutex-protected maps. It provides:

- deterministic ordering for collection and latest-record queries;
- classified invalid, not-found, conflict, and cancellation errors;
- source/snapshot/evidence, source/release, source/verification, and
  drift/impact relationship checks;
- defensive copies for pointer, slice, and byte fields so callers cannot mutate
  stored state through returned values;
- support for multiple runs belonging to one immutable request.

The fake is a test adapter, not a persistence format, cache implementation, or
transaction substitute. SQLite schema and production repositories begin only
in Step 03.

## Deferred boundaries

The current boundary does not add search/release providers, credentials,
background work, evidence extraction, source bundles, public research commands,
curriculum compilation, or student-state mutations. The Student Core remains
offline and unchanged.
