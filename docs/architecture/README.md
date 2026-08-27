# Kelyro architecture

Kelyro is being built as a local-first, cross-platform system whose core stays
independent from presentation frameworks, storage engines, external services,
AI providers, and operating-system details.

I-02 Student & Learning Core is complete in source. The records below are the
stable v1 contracts and adapter boundaries that later implementations consume;
they do not imply that Research, production Learning Packs, generated
exercises, or AI runtime behavior already exist.

The Foundation package boundaries, dependency rules, and stable contracts are
documented in [foundation.md](foundation.md).

The Student & Learning Core domain vocabulary and invariants are documented in
[student-learning-domain.md](student-learning-domain.md).

The Research & Source Intelligence domain vocabulary, traceability graph,
value objects, invariants, and deferred adapter/policy boundaries are documented
in [research-domain.md](research-domain.md).

Research repository ports, external adapter contracts, application services,
error taxonomy, and deterministic memory fakes are documented in
[research-application.md](research-application.md).

The additive Research SQLite schema, bounded retention policy, indexes,
constraints, migration compatibility, and production repository adapter are
documented in [research-persistence.md](research-persistence.md).

The contextual authority tiers, six independent trust dimensions, deterministic
decision precedence, and reason-code contract are documented in
[trust-policy-v1.md](trust-policy-v1.md).

The data-driven authority-profile contract, strict fixture loader, topic-key
matching, precedence, fallback, and persistence compatibility are documented in
[authority-profiles-v1.md](authority-profiles-v1.md).

The reviewed source-family registry, canonical domain rules, contextual Trust
Policy signal, SQLite uniqueness contract, and initial read-only CLI are
documented in [trusted-source-registry.md](trusted-source-registry.md).

The mandatory Foundation privacy gate for live discovery, fetch, and release
lookup, including offline/online/auto semantics and explicit cache fallbacks,
is documented in
[research-network-privacy.md](research-network-privacy.md).

The reusable Research HTTP transport, bounded retry policy, response limits,
safe hooks, compression behavior, redaction boundary, and SSRF defenses are
documented in [research-http-client.md](research-http-client.md).

The `SourceFetcher` adapter, canonical content hash, conditional revalidation,
append-only snapshot capture, and raw-body disposition contract are documented
in [source-fetch-snapshots-v1.md](source-fetch-snapshots-v1.md).

The deterministic HTML/text/JSON/Markdown normalization pipeline, enriched
derived-source contract, sanitization boundary, canonical links, output limits,
and golden fixtures are documented in
[source-normalization-v1.md](source-normalization-v1.md).

The vendor-neutral source-discovery contracts, candidate normalization,
duplicate URL policy, exact rank preservation, privacy boundary, and static
network-free provider are documented in [source-discovery.md](source-discovery.md).

The pure `query-planner-v1` input/output contract, purpose variants,
authority-aware deterministic ordering, generic-topic behavior, and discovery
mapping are documented in [query-planner-v1.md](query-planner-v1.md).

The bounded Evidence contract, canonical excerpt hashing, structured Claim
scopes, multi-evidence relationships, copyright boundary, and forward-only
persistence compatibility are documented in
[evidence-claims-v1.md](evidence-claims-v1.md).

The bounded `provenance-graph-v1` DAG, typed relationships, historical and
multi-source paths, deterministic explain/export behavior, persistence, and
internal trace command are documented in
[provenance-graph-v1.md](provenance-graph-v1.md).

The deterministic `citation-v1` model, explicit anchor strategies, verified
source-code permalinks, canonical fallback, chronology, and persistence are
documented in [citations-deep-links-v1.md](citations-deep-links-v1.md).

The versioned evidence-age formula, Authority Profile TTL precedence, release
cadence cap, temporal invalidation triggers, state boundaries, and score are
documented in [freshness-v1.md](freshness-v1.md).

The versioned next-verification policy, trigger precedence, priorities,
deterministic due ordering, persistence, and read-only stale command are
documented in [refresh-scheduling-v1.md](refresh-scheduling-v1.md).

The deterministic technical/pedagogical rubric, weighted dimensions,
recommended-use precedence, explainability contract, and strict separation
from authority and freshness are documented in
[resource-quality-v1.md](resource-quality-v1.md).

The SemVer/date/opaque version identity, technology release entity, lifecycle
vocabulary, chronology, and existing persistence/application compatibility are
documented in [release-intelligence-model.md](release-intelligence-model.md).

The authority-ordered, privacy-gated release pipeline, JSON/Atom provider
adapters, deterministic current-stable/preview policy, duplicate handling, and
atomic version-scoped release-notes ingestion are documented in
[release-discovery-v1.md](release-discovery-v1.md).

The evidence-linked deprecation assessment policy, explicit versus
multi-source inference distinction, append-only status history, and legacy
migration behavior are documented in
[deprecation-intelligence-v1.md](deprecation-intelligence-v1.md).

The explicit current, historical, version-bound, and archived source scopes;
exact-version authority rule; durable citation warnings; temporally typed bundle
members; and legacy migration behavior are documented in
[historical-sources-v1.md](historical-sources-v1.md).

The pairwise conflict classification precedence, contextual temporal/version/
scope/authority rules, explainable resolved and unresolved outcomes, and
append-only compatibility are documented in
[conflict-resolver-v1.md](conflict-resolver-v1.md).

The Claim-type corroboration rules, reviewed organizational independence,
authority/scope metrics, conflict consumption, confidence caps, and conservative
legacy persistence are documented in
[multi-source-verification-v1.md](multi-source-verification-v1.md).

The immutable Source Bundle hand-off, primary/supporting/historical source
roles, conservative freshness aggregation, lifecycle-state precedence,
canonical JSON/hash contract, and legacy persistence behavior are documented in
[source-bundles-v1.md](source-bundles-v1.md).

The bounded student-facing resource selector, reviewed quality/trust/freshness
inputs, reading-level and access ranking, duplicate suppression, diversity
bonuses, and mandatory community/paywall/staleness disclosure are documented in
[further-reading-selection-v1.md](further-reading-selection-v1.md).

The domain-general Playground, Package Reference, and Standards metadata
union, canonical bounded encoding, trust/freshness integration, and additive
SQLite compatibility projection are documented in
[specialized-technical-sources-v1.md](specialized-technical-sources-v1.md).

Its repository ports, application services, error taxonomy, and transaction
boundary are documented in
[student-learning-application.md](student-learning-application.md).

The additive SQLite schema, normalization decisions, indexes, constraints, and
repository adapter are documented in
[student-learning-persistence.md](student-learning-persistence.md).

The separation between immutable curriculum definitions, learner curriculum
instances, and sparse instance-scoped concept state is documented in
[learner-curriculum-instances.md](learner-curriculum-instances.md).

The deterministic curriculum hierarchy, stable identities, validation, and
fixture-loading boundary are documented in
[curriculum-consumption-contract.md](curriculum-consumption-contract.md), while
prerequisite evaluation lives in
[knowledge-graph-prerequisite-engine.md](knowledge-graph-prerequisite-engine.md).

The deterministic initial diagnostic contract, scoring/confidence policy,
adaptive branching, evidence linkage, and resumable persistence are documented
in [deterministic-initial-diagnostic.md](deterministic-initial-diagnostic.md).

The immutable evidence contract, explicit `mastery-v1` formula, deterministic
calculation, explainability breakdown, and persistence compatibility are
documented in [mastery-v1.md](mastery-v1.md).

The transactional connection between evidence, mastery, instance concept
state, thresholds, and derived prerequisite unlock decisions is documented in
[concept-state-progression-v1.md](concept-state-progression-v1.md).

The recall-strength formula, status boundaries, mastery separation, and
durable due estimate are documented in [retention-v1.md](retention-v1.md).

The versioned review types, due priority, time budget, lifecycle, idempotency,
and persistence compatibility are documented in
[review-scheduler-v1.md](review-scheduler-v1.md).

The contextual prerequisite/review/mistake priority, rotation, bounded time
policy, and read-only Exercise Engine boundary are documented in
[warm-up-selector-v1.md](warm-up-selector-v1.md).

The significant-activity threshold, local-calendar/DST policy, full-history
recalculation, timezone-change behavior, and non-punitive presentation are
documented in
[non-punitive-study-streak-v1.md](non-punitive-study-streak-v1.md).

The deterministic milestone catalog, historical criteria, idempotent unlock
transaction, persistence compatibility, and restrained TUI message are
documented in
[learning-achievements-v1.md](learning-achievements-v1.md).

The explainable mastery, retention, pace, time, and activity projections are
documented in
[explainable-learning-analytics-v1.md](explainable-learning-analytics-v1.md).

The deterministic priority, prerequisite, time-budget, snapshot, and
regeneration policy for today's work is documented in
[daily-plan-v1.md](daily-plan-v1.md).

The shared read model that keeps CLI and TUI metrics semantically aligned is
documented in
[progress-dashboard-read-model.md](progress-dashboard-read-model.md).

The persistent, deduplicated mistake model, generic classification vocabulary,
immutable lifecycle history, application write boundary, and legacy migration
are documented in [mistake-memory.md](mistake-memory.md).

The persistent study-session lifecycle, versioned idle policy, bounded active
time calculation, crash recovery, and separation from Foundation app sessions
are documented in [study-session-lifecycle.md](study-session-lifecycle.md).

The resumable orchestration from onboarding through optional diagnostic and
transactional initial learner state is documented in
[integrated-learner-setup.md](integrated-learner-setup.md).

The human-readable Student Core command surface, shared dashboard routing,
empty-state behavior, and exit-code contract are documented in
[student-core-cli.md](student-core-cli.md).

The persistent terminal navigation, refresh, accessibility, and height-bounded
viewport behavior are documented in
[student-core-tui.md](student-core-tui.md).

The explicit human-readable learning snapshot, document contents, template
versions, privacy boundary, and safe regeneration policy are documented in
[student-learning-markdown-artifacts.md](student-learning-markdown-artifacts.md).

The internal algorithm suite, derived-state version metadata, dry-run impact,
transactional recalculation, backup, audit, and immutable-evidence guarantees
are documented in
[versioned-learning-state-recalculation.md](versioned-learning-state-recalculation.md).

The cross-aggregate integrity scan, privacy review, large deterministic fixture,
query-plan protections, and concurrent-write behavior are documented in
[student-core-hardening.md](student-core-hardening.md).
