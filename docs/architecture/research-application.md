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

The domain never imports application. Application depends only on the research
domain and the Go standard library. No concrete adapter is selected here.

## Repository ports

Persistence is divided by aggregate or durable output:

| Port | Responsibility |
| --- | --- |
| `SourceRepository` | Stable source identities, canonical locators, and source metadata. |
| `SnapshotRepository` | Immutable fetch snapshots ordered by `fetched_at`. |
| `EvidenceRepository` | Immutable evidence tied to a source and snapshot. |
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

- `SearchProvider` accepts a validated `SearchQuery` and returns candidate
  `SearchResult` values. Results remain candidates, never evidence.
- `SourceFetcher` accepts a bounded `FetchRequest` and returns `FetchedSource`
  bytes plus transport-neutral `FetchMetadata`.
- `SourceNormalizer` converts fetched bytes into a `NormalizedSource` without
  exposing parser-specific nodes.
- `MetadataExtractor` derives `SourceMetadata` from normalized data.
- `Clock` supplies a validated research timestamp to time-dependent use cases.

These are contracts only. Step 02 does not implement search, HTTP, retry,
privacy gating, normalization, metadata extraction, or a system clock adapter.
`MaximumBytes` makes the fetch boundary explicitly bounded, but enforcement and
security hardening remain adapter responsibilities in their dedicated steps.

## Initial application services

The initial services are deliberately thin:

- `ResearchService` creates and updates validated request/run state;
- `DiscoveryService` validates queries, delegates to one `SearchProvider`, and
  rejects invalid provider output;
- `SourceService` registers sources and records snapshots for known sources;
- `SourceRegistryService` saves and queries reviewed registry entries;
- `VerificationService` records and retrieves verification results;
- `FreshnessService` stores already-computed, versioned freshness outputs;
- `ReleaseIntelligenceService` records and reads release facts;
- `DriftService` records and reads drift reports;
- `ImpactService` records and reads impact reports.

They validate input, enforce immediate identity relationships, require their
dependencies, delegate one bounded operation, and translate errors. They do not
implement Trust Policy, authority matching, discovery planning, fetching,
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

`application.Error` preserves the underlying cause for diagnostics while
allowing callers to branch with `errors.Is` or `KindOf`. Repositories and
external providers have distinct fallback mappings, so a search outage cannot
masquerade as SQLite corruption and a database error cannot masquerade as a
provider outage. Already-classified errors retain their kind when a service
adds operation context.

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

Step 02 does not add SQLite migrations, network access, credentials, background
work, algorithms, evidence extraction, source bundles, CLI/TUI commands,
curriculum compilation, or student-state mutations. The Student Core remains
offline and unchanged.
