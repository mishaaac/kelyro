# Research Engine application boundaries

The Research & Source Intelligence application boundary lives in
`internal/research/application`. It exposes persistence-neutral use cases and
adapter-neutral external operations while keeping `internal/research`
independent from databases, HTTP, search providers, parsers, clocks, and UI.

## Dependency direction

```text
   CLI / TUI / compiler consumers
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
| `SourceRepository` | Stable source identities, canonical locators, metadata, optional specialized/video details, and explicit temporal-scope classification. |
| `SnapshotRepository` | Immutable fetch snapshots ordered by `fetched_at`. |
| `EvidenceRepository` | Immutable evidence tied to a source and snapshot. |
| `ClaimRepository` | Structured claims with validated source/evidence relationships. |
| `CitationRepository` | Immutable `citation-v1` references and evidence-scoped reads. |
| `ProvenanceRepository` | Immutable bounded claim graphs and latest trace lookup. |
| `ResearchRunRepository` | Research requests and one or more runs for each request. |
| `TrustRegistryRepository` | Authority profiles and versioned trust decisions. |
| `SourceRegistryRepository` | Reviewed source-family entries with deterministic list/show access. |
| `ReleaseRepository` | Evidence-backed `TechnologyRelease` records with compatible `ReleaseRecord` naming. |
| `ReleaseIngestionRepository` | Atomic Evidence/Claim/release/status batches produced by release discovery. |
| `DeprecationRepository` | Append-only, evidence-linked deprecation conclusions and subject history. |
| `FreshnessRepository` | Versioned freshness outputs and due-state queries. |
| `VerificationRepository` | Immutable verification results by claim. |
| `ConflictRepository` | Append-only explainable conflict outcomes and per-Claim history. |
| `SourceBundleRepository` | Append-only canonical Source Bundles with exact and research-run history reads. |
| `DriftRepository` | Immutable drift reports. |
| `ImpactRepository` | Immutable impact reports linked to drift. |
| `ResearchCacheRepository` | Opaque, bounded cache entries that are never evidence truth. |

There is deliberately no `ResearchRepository` mega-interface. `Repositories`
is only a wiring struct for adapters and tests; services receive the individual
ports they need. Step 20 adds one purpose-specific atomic port for the concrete
multi-record consistency need of release-note ingestion; it does not turn the
bundle into a generic Unit of Work.

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
- `source-candidate-mapper-v1` converts provider DTOs into explicit transient
  candidates with query/provider/rank/time provenance; it performs no
  registration, trust, Evidence, or Claim work.
- `source-candidate-deduplication-v1` merges canonical URLs across queries,
  preserves all distinct discovery observations, and read-only matches stable
  Source locators before registration.
- `SourceFetcher` accepts a bounded `FetchRequest` and returns `FetchedSource`
  bytes plus transport-neutral `FetchMetadata`; the privacy-gated service marks
  the result origin as `live` or `cache`.
- `ReleaseLookupProvider` accepts a validated technology/channel query and
  returns metadata candidates; Step 20 additionally defines the network-free
  `ReleaseNotesProvider` parser boundary over already privacy-gated fetched
  bytes.
- `SearchCache`, `SourceFetchCache`, and `ReleaseLookupCache` expose explicitly
  offline fallback reads and cannot be confused with their live counterparts.
- `SourceNormalizer` converts fetched bytes into a `NormalizedSource` without
  exposing parser-specific nodes.
- `MetadataExtractor` derives `SourceMetadata` from normalized data.
- `Clock` supplies a validated research timestamp to time-dependent use cases.

Every live search and fetch call is protected by Foundation's privacy gate.
The production composition root selects the Brave Web Search adapter through
the provider-neutral `LiveSearchProviderFactory`, resolves its credential from
Foundation Secrets, and wraps it with cost control and the workspace search
cache. The hardened source-fetch adapter, deterministic source normalizer, and
filesystem cache implement the remaining external boundaries. The static
network-free provider remains available for deterministic tests.

`query-planner-v1` produces query text, desired kind, authority threshold, and
execution priority. The production search stage adds the durable request ID,
bounded result limit, and target-version options before calling
`DiscoveryService`. JSON and Atom release-feed adapters remain separate from
general web discovery. The optional `MetadataExtractor` port is not part of the
live query-to-bundle path; normalization, evidence extraction, and post-fetch
source classification use their own explicit services.
`MaximumBytes` is enforced by the fetch adapter as a request-specific limit
below the transport's configured global ceiling. A safe redirect may change
the returned locator without changing `SourceID`.

## Initial application services

The initial services are deliberately thin:

- `ResearchService` creates and updates validated request/run state;
- `DiscoveryService` normalizes and validates queries/options, enforces research
  mode/privacy, delegates to either the live provider or explicit offline cache,
  and returns bounded unique candidates without reranking them;
- `source-candidate-registration-v1` reuses durable Sources, creates only new
  canonical locators as unclassified `other` Sources, and appends immutable
  request/query/provider discovery observations without assigning trust;
- `FetchService` applies the same boundary to live/cached source retrieval;
- `live-source-fetch-v2` selects the ordered Source prefix allowed by the
  durable remaining fetch/byte budget, allocates bytes before concurrent work,
  preserves per-Source failures for partial success, and delegates privacy and
  transport behavior to `FetchService` and the hardened adapter;
- `live-source-snapshot-v1` persists successful fetched observations with
  `SnapshotCaptureService`, writes bounded disposable bodies through the
  existing filesystem cache, resolves cache/304 normalization inputs without
  duplicating history, and preserves partial snapshot/cache failures;
- `live-source-normalization-v1` verifies each transient input against its
  durable snapshot, delegates HTML/Markdown/JSON/text parsing to the existing
  `SourceNormalizer`, preserves bounded document-level partial failures, and
  is followed by `source-classifier-v1`, which classifies only successfully
  normalized content without trusting search rank, title, or snippet;
- `ResearchProcessingService` executes prepared discovery and fetch work in
  separate bounded pools, preserves input-index result ordering, and enforces
  the versioned whole-run candidate, fetch, Claim, and byte budget;
- `SnapshotCaptureService` resolves the latest source snapshot, sends its
  conditional validators through `FetchService`, verifies canonical content
  metadata, and appends a new immutable observation;
- `ReleaseLookupService` applies it to live/cached release lookups without
  persisting candidates automatically;
- `SourceService` registers sources, records snapshots for known sources, and
  explicitly classifies their temporal scope;
- `SourceRegistryService` saves and queries reviewed registry entries;
- `ProvenanceService` records, traces, and exports validated claim graphs;
- `CitationService` loads a source/snapshot/evidence chain, generates one
  deterministic stable citation, and exposes offline citation reads;
- `VerificationService` loads a Claim's complete stored source/trust/registry/
  conflict context, applies `multi-source-verification-v2` for live work,
  appends the immutable result, and exposes offline result reads;
- `FreshnessService` stores already-computed, versioned freshness outputs;
  the pure `internal/research/freshness` model produces `freshness-v1`
  assessments with an injected clock and no adapter dependency, while
  `FreshnessRecordFromAssessment` maps only known-verification results without
  calculating `next_verify_at`; `refresh-scheduling-v1` separately derives
  versioned deadlines, reasons, and priorities, and
  `FreshnessRecordFromSchedule` combines matching outputs;
- `ReleaseIntelligenceService` records and reads release facts;
- `ReleaseDiscoveryService` orders Authority Profile sources, requires accepted
  Trust Decisions, captures feeds through the privacy boundary, deduplicates
  releases, selects stable/preview families, and atomically ingests bounded
  release-note Evidence and version-scoped Claims;
- `DeprecationIntelligenceService` validates structured deprecation signals
  against their Claim/Evidence/Source chain, applies the versioned explicit or
  multi-source inference admission policy, appends the conclusion, and reads
  exact subject history;
- `ConflictResolutionService` resolves stored Claim/source references through
  accepted TrustDecision tiers, invokes the pure pairwise
  `conflict-resolver-v1` policy, appends the explainable outcome, and exposes
  exact and per-Claim history;
- `SourceBundleService` loads a running or completed run and selected Claims, checks every
  declared Evidence identity, consumes latest verification/conflict/freshness
  state, classifies source roles, invokes `source-bundle-v1`, appends the
  immutable result, and exposes offline read/export/history operations;
- `DriftService` deterministically detects v1 drift without writing, exposes
  unresolved comparisons, and separately records/reads reviewed reports;
- `ImpactService` loads a persisted v1 DriftReport, deterministically assesses
  affected identities and explicit future references, and separately
  records/reads reviewed impact reports.

These services validate input, enforce immediate identity relationships,
delegate bounded work, and translate errors. Snapshot capture reads prior
immutable metadata before appending a new observation; raw bodies stay in the
disposable filesystem cache rather than SQLite. The live extraction stage
composes `evidence-extractor-v2`, `claim-extractor-v2`, trust evaluation, and
temporal/freshness evaluation. It persists only literal bounded Evidence and
Evidence-backed Claims, reuses stable identities on retry, and keeps selection
scores transient. Conflict discovery is not inferred from search ranking or
missing content. No live stage invokes curriculum compilation or mutates
Student Core.

`LiveResearchOrchestrator` fixes the production query-to-bundle stage order:

```text
search
→ candidate deduplication/registration
→ bounded fetch
→ snapshot/cache
→ normalization/classification
→ Evidence/Claim/trust/freshness extraction
→ multi-source verification/diversity
→ supported-Claim Source Bundle
→ provenance
→ durable-bundle validation
```

`ResearchService.TransitionRun` applies `research-run-lifecycle-v1`: the
orchestrator enters `running` before stage work. It does not claim queue items
or persist terminal run state. `ResearchQueueConsumer` owns that outer boundary
through [`research-queue-worker-v1`](research-queue-worker-v1.md), invokes the
orchestrator at most once for each acquired execution claim, and atomically
persists the terminal run, queue execution, queue acknowledgement, final audit,
and cost metadata. `completed` is possible only after a durable matching bundle
has survived the finalization stage; failure and cancellation preserve the
artifacts already written and produce terminal audit state.

Search, fetch, evidence, Claim, and byte limits apply to the whole run before
or around bounded concurrent work. A discovery result remains a candidate
until registration, fetch, snapshot, normalization, and literal extraction
have succeeded. The complete concurrency and budget contract is in
[research-performance-limits-v1.md](research-performance-limits-v1.md).

Step 19 keeps these ports stable through the `ReleaseRecord` alias while adding
the explicit `TechnologyRelease` entity and deterministic
`VersionIdentifier` classification. It introduces no provider, precedence, or
discovery behavior.

Step 20 permits lifecycle-only release status updates so a newly discovered
stable can supersede the prior current record. All other release identity,
source, version, channel, chronology, and verification fields remain immutable.
The full policy is in
[release-discovery-v1.md](release-discovery-v1.md).

Step 21 does not parse prose or infer from missing documentation. Signal
producers must classify a bounded observation as an explicit statement or a
strong inference and attach one deprecation Claim, Evidence ID, source, status,
and optional version/replacement fields. All signals in one assessment must
agree. The service checks the stored relationships; inferred conclusions also
require at least two distinct sources and confidence `>= 0.8` for every Claim.
This admission rule is not the general multi-source verification algorithm
reserved for Step 24. The full contract is in
[deprecation-intelligence-v1.md](deprecation-intelligence-v1.md).

Step 22 exposes temporal reclassification through `SourceService` and the
`SourceRepository` port. The service validates the stable source identity and
closed scope; the repository additionally enforces that `version_bound` has an
existing source version. The pure applicability decision remains in
`internal/research/temporal`, while `CitationService` copies the source scope and
deterministic warning into each new immutable citation. The full contract is in
[historical-sources-v1.md](historical-sources-v1.md).

Step 23 adds the append-only `ConflictRepository` and
`ConflictResolutionService`. The service requires two stored Claims, their
selected declared sources, accepted latest trust decisions, and a valid clock;
it never reads external content. Pair ordering is canonicalized by Claim ID so
stable identity and output do not depend on request order. The full policy is
in [conflict-resolver-v1.md](conflict-resolver-v1.md).

Step 24 replaces the raw verification-recording surface with
`VerificationService.Verify`. It assesses every declared Claim source, treats
missing reviewed trust/organization as unknown, counts organizations through
the Source Registry, and consumes the latest visible outcome for each conflict
pair without rerunning conflict resolution. The complete policy is in
[multi-source-verification-v1.md](multi-source-verification-v1.md).

Step 25 adds the append-only `SourceBundleRepository` and
`SourceBundleService`. Assembly is entirely offline over persisted Research
records. Missing Evidence, verification, or Claim freshness becomes explicit
`incomplete` state; no source content or missing conclusion is invented. The
complete hand-off contract is in
[source-bundles-v1.md](source-bundles-v1.md).

Step 27 keeps the `SourceRepository`/`SourceService` ports stable while Source
gains optional `specialized-source-metadata-v1` details. Memory and SQLite
adapters preserve the closed Playground/Package Reference/Standard union and
return defensive deep copies. No new external provider or network path is
introduced. The complete contract is in
[specialized-technical-sources-v1.md](specialized-technical-sources-v1.md).

Step 29 keeps the same ports while Source gains optional host-neutral video
metadata. Memory and SQLite clone timestamp deep links defensively and persist
only bounded metadata/availability, never transcript text. The complete
contract is in [video-learning-resources-v1.md](video-learning-resources-v1.md).

Step 31 keeps `EvidenceRepository` host-neutral and requires its adapters to
validate the loaded Source against any `source-code-evidence-v1` locator before
append. Citation generation consumes the persisted locator rather than a
second caller-owned permalink. Memory and SQLite return defensive copies. The
complete contract is in
[real-source-code-evidence-v1.md](real-source-code-evidence-v1.md).

Step 32 adds the separate `ResearchCacheStore`/`ResearchCacheService` boundary
for disposable workspace files. The service owns versioned layer TTLs,
per-entry/global limits, explicit hit/stale outcomes, warnings, status, clear,
and deterministic eviction; the filesystem adapter owns strict envelopes and
Foundation paths. `SearchCache`, `SourceFetchCache`, and `ReleaseLookupCache`
are now backed by a concrete network-free codec adapter. The older generic
SQLite cache repository remains compatibility-only and is not historical
evidence. The complete contract is in
[research-cache-offline-v1.md](research-cache-offline-v1.md).

Step 30 adds `SourceDiversityService` without a dedicated result repository. It
composes existing Claim, Source, Trust Decision, and applicable Source Registry
organization reads with caller-reviewed
dependency/perspective/technical-role annotations.
Exact annotation coverage is mandatory and failures use the existing error
taxonomy. The returned `source-diversity-v1` assessment is not persisted and
does not rewrite immutable Verification Results or Source Bundles. The complete
contract is in [source-diversity-v1.md](source-diversity-v1.md).

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
- source/snapshot/evidence/claim/citation, source/release,
  source/deprecation/evidence, source/verification, claim/source/conflict, and
  run/claim/source/conflict/bundle, and drift/impact
  relationship checks;
- defensive copies for pointer, slice, byte, nested specialized-source, and
  video deep-link fields so callers cannot mutate stored state through returned
  values;
- support for multiple runs belonging to one immutable request.

The fake is a test adapter, not a persistence format, cache implementation, or
transaction substitute. Production uses the SQLite repositories and the
workspace filesystem cache through the same application ports.

## Current boundaries

The shipped binary has a production live search provider and a public
`research topic` query-to-bundle workflow. It deliberately has no background
daemon, recursive crawler, browser renderer, arbitrary prose/LLM extractor, or
hidden retry loop. Provider configuration does not authorize network access:
every live operation still requires `privacy.allow_network`, while durable
sources, snapshots, Evidence, Claims, bundles, audit, and usable cache remain
readable offline. Curriculum compilation and student-state mutation remain
outside Research; the offline Foundation and Student Core behavior is
unchanged.
