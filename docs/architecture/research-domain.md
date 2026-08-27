# Research & Source Intelligence domain

I-03 introduces `internal/research` as Kelyro's transport-, persistence-, and
presentation-independent vocabulary for research and evidence. The package
contains domain data and validation. Its `trust` subpackage adds the pure,
versioned Trust Policy v1, while `queryplanner` adds deterministic discovery
intent planning and `conflict` adds the pairwise deterministic Conflict
Resolver v1; none of these packages searches the web, performs HTTP
requests, parses documents, persists records, compiles curriculum, or modifies
learner state.

## Package shape

The first domain step uses one cohesive Go package split into files by area:

```text
internal/research/
├── value.go         identities, timestamps, locators, topics, scores
├── content.go       canonical fetched-content hashing v1
├── source.go        sources, metadata, fetch metadata, snapshots
├── research.go      requests, runs, authority, trust, discovery candidates
├── evidence.go      evidence, claims, provenance, citations, bundles
└── intelligence.go releases, deprecations, verification, drift, impact
```

This deliberately avoids a package per noun. The vocabulary has many shared
value objects and relationship invariants, while no application-service seams
exist yet to justify more boundaries. Later steps may extract packages only
when dependencies and cohesive use cases make the split real.

`internal/research` depends only on the Go standard library. In particular, it
does not import Bubble Tea, SQLite, `net/http`, GitHub adapters, AI providers,
the learning domain, or operating-system packages.

## Stable identities and locators

`ID` is the common stable identity for research records. `SourceID` and
`ClaimID` are distinct value objects so source and claim references cannot be
silently interchanged. Every identity rejects empty, padded, and whitespace
containing values.

`SourceID` is independent from `SourceLocator`. A canonical URL can move or
gain aliases without changing the source's identity, and repeated fetches do
not create new sources. `SourceLocator` currently accepts normalized absolute
HTTP(S) URLs with a host and rejects embedded credentials. This validation is
a domain syntax boundary, not the SSRF policy: address resolution, private-host
blocking, redirect checks, and other network security belong to the hardened
HTTP adapter steps.

`SourceVersion` is non-empty and remains compatible with opaque ecosystems.
Its release-oriented alias `VersionIdentifier` classifies strict SemVer,
supported date-based forms, and opaque fallbacks without rewriting identity or
requiring SemVer for documentation, revisions, editions, or other schemes.

## Domain-general topics

`ResearchTopic` requires a subject and permits optional domain and technology
labels. Technology is not mandatory, so the model can represent mathematics,
standards, science, and other fields. Initial `ResearchPurpose` values describe
why evidence is needed, including definitions, current usage, version behavior,
release and deprecation checks, prerequisites, production practice, and
security guidance.

## Sources and snapshots

`Source` owns stable identity, classification, canonical locator, optional
version scope, descriptive metadata, and creation time. The initial source-kind
vocabulary is:

```text
official_documentation  specification       standard
release_notes           official_blog       package_reference
official_tutorial       source_code          issue_tracker
community_article       community_forum      video
paper                   book_reference       other
```

Source classification does not imply trust. An official resource is still
evaluated in context by the separate, versioned trust policy.

Step 22 adds the independent temporal scopes `current`, `historical`,
`version_bound`, and `archived`. A version-bound source requires an opaque
version. The pure `source-temporal-policy-v1` model decides only whether a
source is current guidance, exact-version authority, historical context, or not
applicable. Non-current sources always carry deterministic warnings when cited
or bundled; exact old-version authority is preserved without treating it as
current guidance. The full contract is documented in
[historical-sources-v1.md](historical-sources-v1.md).

`SourceSnapshot` records one immutable fetch identity, the source and locator
used, `fetched_at`, and transport-neutral `FetchMetadata`. It contains metadata
only; it neither requires nor authorizes retaining an unbounded response body.
Snapshots preserve changes over time instead of overwriting prior observations.

`CanonicalContentHashV1` defines `sha256:<lowercase hex>` over the exact bounded,
decoded fetch bytes. It is deterministic domain logic with no HTTP dependency;
the Step 09 adapter and capture service share it so snapshot content identity
cannot drift between layers.

## Research, authority, trust, and discovery

`ResearchRequest` binds a topic, purpose, optional target version, and request
time. `ResearchRun` gives an execution a separate lifecycle and validates
terminal completion timestamps.

`AuthorityProfile`, `AuthorityTier`, `TrustDecision`, and `TrustReason` define
the stable input/output vocabulary. `internal/research/authority` implements
data-driven domain/topic matching and deterministic precedence, while
`internal/research/trust` implements the deterministic `trust-policy-v1`
decision policy without numeric scoring or I/O. A trust decision carries its
policy version and ordered human-readable reasons so authority, freshness,
relevance, directness, stability, corroboration, and the terminal decision
remain explainable. A matching Authority Profile is contextual preference data,
not a trust decision.

`DiscoveredSource` is explicitly a candidate with provider and rank metadata.
At the application boundary, `SearchResult` also carries an optional snippet
and publication hint supplied by the provider. Neither value is evidence.
Conversion into evidence requires later classification, fetch, normalization,
and extraction steps.

`SourceRegistryEntry`, `CanonicalDomain`, `RegistryAuthorityHint`, and
`RegistryStatus` model reviewed source-family metadata. The pure registry
catalog matches exact and wildcard DNS rules and preserves `trusted`,
`conditional`, `historical`, `deprecated`, and `blocked` status. Registry
metadata remains contextual input; it is never evidence or an automatic trust
decision.

## Evidence, claims, and provenance

The central relationship is:

```text
ResearchRequest
      ↓
ResearchRun
      ↓
optional DiscoveredSource
      ↓
Source
      ↓
SourceSnapshot
      ↓
Evidence
      ↓
Claim
      ↓
Citation / SourceBundle
```

`Evidence` is a bounded excerpt or observation tied to exactly one source and
snapshot. Its required excerpt is capped at 8 KiB; optional context before and
after is capped at 2 KiB each. `CanonicalEvidenceExcerptHashV1` verifies
canonical SHA-256 over the exact excerpt bytes, while extraction time and
extractor version retain temporal and algorithm identity. These are safety
ceilings: Evidence is not a complete mirrored document.

`Claim` is a structured assertion. It requires at least one distinct `SourceID`
and evidence identity, a closed claim type, bounded domain-general applicability
scope, explicit `all/stable/preview/experimental/legacy` status scope,
confidence in `[0,1]`, and an optional opaque version scope. Empty claims and
claims without evidence are invalid; multiple evidence records are supported.
The detailed model and copyright boundary are documented in
[evidence-claims-v1.md](evidence-claims-v1.md).

The initial `Provenance` record remains a single-path relationship validator.
Step 14 completes traceability with bounded `ProvenanceGraph` and the immutable
algorithm ID `provenance-graph-v1`. Its typed DAG includes optional
Query/Discovery branches, supports multiple source/evidence paths and exact
historical snapshots, rejects missing/disconnected/cyclic structure, and can
terminate at an optional SourceBundle. Deterministic explain and JSON export
never include raw source bodies or evidence excerpts. The complete contract is
documented in [provenance-graph-v1.md](provenance-graph-v1.md).

`Citation` always names a source, snapshot, and evidence item. Step 15 completes
it with the immutable `citation-v1` algorithm, a closed deep-link strategy,
required bounded section/path hint, exact snapshot date, optional source version
scope, and explicit `last_verified`. A more specific `DeepLink` is included only
when an explicit stable anchor or verified source-code permalink is available;
otherwise the canonical source locator remains available with the hint.
`ValidateCitationRelationships` checks source, snapshot, evidence, title,
canonical locator, snapshot date, version, temporal scope/warning, and
chronology consistency. Step 22 adds the separate immutable
`source-temporal-policy-v1` annotation without changing deep-link selection. The
full citation contract is documented in
[citations-deep-links-v1.md](citations-deep-links-v1.md).

`SourceBundle` is completed by Step 25 as the immutable, bounded hand-off from a
completed research run. `source-bundle-v1` groups Claim identities; classified
primary, supporting, and historical source references; latest visible conflict
identities; conservative Claim freshness; issue codes; state; summary; and
canonical content hash. Exact-target version-bound sources may support the
bundle; archived, historical, and mismatched version-bound sources remain
historical with explicit warnings. Deterministic JSON contains identities and
annotations rather than raw source bodies or Evidence excerpts. The complete
contract is documented in [source-bundles-v1.md](source-bundles-v1.md).

## Query planning

`internal/research/queryplanner` implements the pure `query-planner-v1`
algorithm. It combines a topic, optional target version, purpose, and
already-selected Authority Profile into bounded, ordered discovery intentions.
Every intention preserves query text, desired source kind, minimum authority
tier, and priority. The algorithm supports technology-free topics, and its
output remains candidate-search intent rather than evidence or trust.

The complete ordering policy, purpose matrix, validation contract, and mapping
to application discovery DTOs are documented in
[query-planner-v1.md](query-planner-v1.md).

## Freshness and change intelligence vocabulary

`FreshnessScore`, `ClaimConfidence`, and `QualityScore` are finite values in
the closed interval `[0,1]`. `FreshnessState` distinguishes `fresh`, `aging`,
`stale`, and `unknown`. Step 16 implements the deterministic `freshness-v1`
formula with explicit claim/source TTL defaults, optional Authority Profile
hints, a release cadence cap, known-release/source-update invalidation, and
clock-controlled age boundaries. It never substitutes publication or snapshot
time for `last_verified_at`. The complete policy is documented in
[freshness-v1.md](freshness-v1.md).

Step 18 implements the separate `resource-quality-v1` rubric over reviewed
accuracy-confidence, clarity, specificity, depth, maintainability, examples,
accessibility, and noise inputs. It recommends a resource use without reading
or changing authority, trust, freshness, or scheduling state. The complete
policy is documented in [resource-quality-v1.md](resource-quality-v1.md).

`TechnologyRelease` (with the compatible `ReleaseRecord` alias) supports
semantic, date-based, and opaque version identities;
stable/preview/beta/RC/experimental/nightly/unknown channels; and
current/superseded/legacy/EOL/unknown lifecycle states. A release requires
evidence-bearing source identities and a verification time, and a known release
date cannot follow verification. The complete contract is documented in
[release-intelligence-model.md](release-intelligence-model.md).
Step 20 adds deterministic precedence, stable/preview separation, deduplication,
and literal version-scoped release-note Claims through
[release-discovery-v1.md](release-discovery-v1.md). It does not authorize an
upgrade or curriculum mutation.
Step 21 completes `DeprecationRecord` with an immutable algorithm version and a
closed determination: explicit evidence, multi-source strong inference, or a
migration-only legacy-unclassified marker. V1 records require sources and
Evidence; inferred records require at least two distinct sources. The record
keeps optional introduced/deprecated/removed versions and replacement text,
while append-only records preserve the guidance that applied before a later
status. Absence from documentation is not represented as a valid signal. The
full contract is documented in
[deprecation-intelligence-v1.md](deprecation-intelligence-v1.md).

`Conflict`, `VerificationResult`, `DriftReport`, and `ImpactReport` preserve
explicit states and affected relationships. Step 23 completes `Conflict` with
confidence, reason, optional winner metadata, and an immutable algorithm ID.
The pure `conflict-resolver-v1` policy classifies an already-identified
incompatible Claim pair by temporal, version, scope, recommendation, and
authority precedence. It keeps unresolved equal-authority contradictions
visible. The complete contract is documented in
[conflict-resolver-v1.md](conflict-resolver-v1.md). Multi-source verification,
implemented by `multi-source-verification-v1`, adds a Claim requirement,
versioned reason codes, source/organization/authority/scope metrics, confidence,
and an immutable algorithm ID to `VerificationResult`. Its full contract is in
[multi-source-verification-v1.md](multi-source-verification-v1.md). Semantic
drift detection and curriculum impact remain in later dedicated steps.

## Temporal and enum invariants

All domain timestamps are non-zero and normalized to UTC by `NewTimestamp`.
Optional timestamps are validated when present. Timelines reject known
contradictions such as source updates before publication, research completion
before start, and citation verification before its snapshot.

Closed enums reject unknown values for source kind, research purpose, run
status, authority tier, trust decision, claim type, bundle state, freshness,
release channel/status, deprecation, conflict, verification, drift, severity,
and recommended action. Adding a state is therefore an explicit domain change.

## Deferred boundaries

The following remain intentionally absent from this step:

- repositories, units of work, application services, and SQLite schema;
- network privacy orchestration and HTTP clients;
- discovery providers and live query execution orchestration;
- authority-profile matching, multi-source verification,
  drift, impact, trigger, or cost algorithms;
- raw-body retention, caches, audit adapters, CLI, and TUI;
- production curriculum compilation and any mutation of student mastery.

These boundaries keep Step 01 a domain-language foundation rather than a
premature Research Engine implementation.
