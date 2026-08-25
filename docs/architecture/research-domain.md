# Research & Source Intelligence domain

I-03 introduces `internal/research` as Kelyro's transport-, persistence-, and
presentation-independent vocabulary for research and evidence. The package
contains domain data and validation. Its `trust` subpackage adds the pure,
versioned Trust Policy v1, while `queryplanner` adds deterministic discovery
intent planning; none of these packages searches the web, performs HTTP
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

`SourceVersion` is opaque and non-empty. SemVer is not required because release
and documentation ecosystems may use date versions, revisions, editions, or
other schemes.

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
evaluated in context by a later, versioned trust policy.

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
snapshot, with an excerpt hash, extraction timestamp, and extractor version.
It is not a complete mirrored document.

`Claim` is a structured assertion. It requires at least one `SourceID` and one
evidence identity, a closed claim type, a confidence in `[0,1]`, and an optional
opaque version scope. Empty claims and claims without evidence are invalid.

`Provenance` names the request, run, source, snapshot, evidence, and claim in a
single trace record; discovery is optional for manually registered sources.
`ValidateProvenanceRelationships` verifies that the supplied aggregates form
the same chain rather than only checking that each ID is non-empty.

`Citation` always names a source, snapshot, and evidence item. It may carry a
more specific `DeepLink`, but the canonical locator remains available as a
fallback. `ValidateCitationRelationships` checks source, snapshot, evidence,
and locator consistency.

`SourceBundle` groups claim and source identities for a research run. This step
defines only identity, topic, purpose, target version, state, and verification
time. Deterministic serialization, hashing, conflict assembly, and compiler
eligibility are later responsibilities.

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

`FreshnessScore` and `ClaimConfidence` are finite values in the closed interval
`[0,1]`. `FreshnessState` distinguishes `fresh`, `aging`, `stale`, and
`unknown`. No formula is implemented in this step; the future formula must be
explicitly versioned.

`ReleaseRecord` supports opaque versions, stable/preview/beta/RC/experimental/
nightly/unknown channels, and current/superseded/legacy/EOL/unknown lifecycle
states. A release requires evidence-bearing source identities and a verification
time. `DeprecationRecord` likewise requires sources and evidence; absence from
documentation is not represented as a deprecation.

`Conflict`, `VerificationResult`, `DriftReport`, and `ImpactReport` preserve
explicit states and affected relationships. This vocabulary does not resolve
conflicts, corroborate claims, detect semantic drift, or recommend curriculum
mutations yet. Those algorithms remain in their dedicated, versioned steps.

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
- authority-profile matching, quality, freshness, conflict, verification,
  drift, impact, trigger, or cost algorithms;
- raw-body retention, caches, audit adapters, CLI, and TUI;
- production curriculum compilation and any mutation of student mastery.

These boundaries keep Step 01 a domain-language foundation rather than a
premature Research Engine implementation.
