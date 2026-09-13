# Kelyro architecture

Kelyro is being built as a local-first, cross-platform system whose core stays
independent from presentation frameworks, storage engines, external services,
AI providers, and operating-system details.

I-02 Student & Learning Core and I-03 Research & Source Intelligence are
complete. I-03 closed after hosted Linux/macOS/Windows CI with Linux race
coverage. I-04 Curriculum Compiler & Learning Packs is now in progress. The
records below are stable contracts and adapter boundaries; they do not imply
that later compiler passes, production Learning Packs, generated exercises,
automatic learner migration, or an AI runtime already exist.

The portable container contract is documented in
[Learning Pack format v1](../specs/learning-pack-v1.md).
Its bounded directory/ZIP adapter and CLI behavior are documented in
[Secure Learning Pack loader v1](learning-pack-loader-v1.md).
The read-only I-03 hand-off is documented in
[Curriculum evidence ingestion v1](curriculum-evidence-ingestion-v1.md).
Evidence-backed, pack-declared goal expansion is documented in
[Goal decomposition v1](goal-decomposition-v1.md).
Professional outcome categories and completeness are documented in
[Professional outcome model v1](professional-outcomes-v1.md).
Evidence-backed competency construction is documented in
[Competency matrix v1](competency-matrix-v1.md).
Claim grouping into deterministic, evidence-backed semantic candidates is
documented in [Claim-to-concept candidates v1](claim-to-concept-candidates-v1.md).
The domain-neutral atomicity signals, result taxonomy and deterministic policy
are documented in [Atomic concept criteria v1](atomic-concept-criteria-v1.md).
Evidence-constrained candidate splitting and Claim-to-Concept mapping are
documented in [Concept Atomizer v1](concept-atomizer-v1.md).
Unbounded concept preservation, merge review and safe visual grouping are
documented in [Granularity Guard v1](granularity-guard-v1.md).
Evidence-backed direct semantic dependencies are documented in
[Prerequisite extraction v1](prerequisite-extraction-v1.md).
Recursive insertion of available foundations and unresolved prerequisite gaps
is documented in [Prerequisite expansion v1](prerequisite-expansion-v1.md).
Validated DAG compilation, reachability, components, critical paths and the
I-02 prerequisite projection are documented in
[Knowledge Graph Compiler v1](knowledge-graph-compiler-v1.md).
Canonical term, alias/acronym, observed-use and explicit domain-baseline
resolution are documented in [Vocabulary Graph v1](vocabulary-graph-v1.md).
Vocabulary introduction ordering across prerequisite paths and within a lesson
is documented in
[Definition-before-use Audit v1](definition-before-use-v1.md).
Independent competency, concept, evidence, theory, practice, production,
security and toolchain measurement is documented in
[Curriculum Coverage Engine v1](curriculum-coverage-v1.md).
Deterministic conversion of incomplete coverage and prerequisite/temporal
findings into prioritized gaps is documented in
[Curriculum Gap Scanner v1](curriculum-gap-scanner-v1.md).
Declared learner/domain baselines and evidence-backed checks for missing
foundations are documented in
[Zero-Assumption Audit v1](zero-assumption-audit-v1.md).
Evidence-gated insertion of foundational roots and explicit research-required
outcomes are documented in
[First-Principles Expansion v1](first-principles-expansion-v1.md).
Evidence-backed definition, mental-model, mechanism, tradeoff, and failure-mode
contracts for important competencies are documented in
[Theory Coverage v1](theory-coverage-v1.md).
Versioned practice-to-competency compatibility and the structured I-05 hand-off
are documented in
[Practice Coverage Contract v1](practice-coverage-v1.md).
Domain-specific operational categories and the evidence policy that separates
production guidance from tutorial-only material are documented in
[Production Reality Coverage v1](production-reality-coverage-v1.md).
Exact Environment Pack resolution, tool-level strength, introduction points,
platform notes, and evidence completeness are documented in
[Toolchain Coverage v1](toolchain-coverage-v1.md).

The Foundation package boundaries, dependency rules, and stable contracts are
documented in [foundation.md](foundation.md).

The Student & Learning Core domain vocabulary and invariants are documented in
[student-learning-domain.md](student-learning-domain.md).

The Research & Source Intelligence domain vocabulary, traceability graph,
value objects, invariants, and deferred adapter/policy boundaries are documented
in [research-domain.md](research-domain.md).

The Curriculum Compiler domain vocabulary, Source Bundle reference boundary,
Learning Pack shapes, closed status types, and structural invariants are
documented in [curriculum-domain.md](curriculum-domain.md).

Curriculum repository ports, service contracts, external adapter boundaries,
error taxonomy, and deterministic in-memory fakes are documented in
[curriculum-application.md](curriculum-application.md).

The additive I-04 SQLite schema, reuse of the published I-02 curriculum tables,
immutable version/install separation, Source Bundle foreign keys, indexes, and
bounded metadata retention are documented in
[curriculum-persistence.md](curriculum-persistence.md).

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

The [Research queue worker v1](research-queue-worker-v1.md) atomically claims
and settles work from the existing durable trigger queue while delegating the
single run lifecycle to the live orchestrator. The manual topic command can
invoke an assembled worker synchronously with a bounded context, without a
daemon.

The [Source Candidates v1](source-candidates-v1.md) boundary preserves
query/provider/rank/time provenance while keeping web search results separate
from registered Sources, trust, and Evidence.

The reusable Research HTTP transport, bounded retry policy, response limits,
safe hooks, compression behavior, redaction boundary, and SSRF defenses are
documented in [research-http-client.md](research-http-client.md).

The versioned Research run budget, bounded discovery/fetch worker pools,
global/per-host HTTP concurrency and rate policy, SQLite batch boundary, and
offline performance fixture are documented in
[research-performance-limits-v1.md](research-performance-limits-v1.md).

The complete untrusted-input threat model, redirect/content/parser/cache/log
hardening, fuzz surface, and future AI prompt-injection gate are documented in
[research-security-hardening-v1.md](research-security-hardening-v1.md).

The public-Internet-free Research Engine E2E topology, build-tagged loopback
allowlist, controlled endpoint inventory, persisted pipeline, scenario matrix,
and cross-platform CI contract are documented in
[research-e2e-fixture-v1.md](research-e2e-fixture-v1.md).

The explicit opt-in boundary, stable unauthenticated source inventory, strict
timeouts, resilient assertion policy, and non-blocking role of controlled live
Research checks are documented in
[research-live-integration.md](research-live-integration.md).

The `SourceFetcher` adapter, canonical content hash, conditional revalidation,
append-only snapshot capture, and raw-body disposition contract are documented
in [source-fetch-snapshots-v1.md](source-fetch-snapshots-v1.md).

The copyright, licensing, minimal-excerpt, `no-store`, bounded-cache, export,
and video/transcript retention boundary is documented in
[source-content-retention-policy.md](source-content-retention-policy.md).

The deterministic HTML/text/JSON/Markdown normalization pipeline, enriched
derived-source contract, sanitization boundary, canonical links, output limits,
and golden fixtures are documented in
[source-normalization-v1.md](source-normalization-v1.md).

The vendor-neutral source-discovery contracts, candidate normalization,
duplicate URL policy, exact rank preservation, privacy boundary, and static
network-free provider are documented in [source-discovery.md](source-discovery.md).

The provider-neutral live-search configuration keys, safe defaults, bounded
limits, nested TOML representation, and credential-free readiness states are
documented in
[research-search-configuration-v1.md](research-search-configuration-v1.md).

The reference production discovery-provider decision, current evidence,
alternatives, mapping, and operational constraints are recorded in
[ADR-001](adr/ADR-001-research-search-provider.md).

The fixed-endpoint Brave search transport, TLS/timeouts/redirect/body policy,
secret-safe diagnostics, rate-limit metadata and strict separation between the
search API endpoint and untrusted fetched result URLs are documented in
[research-search-transport-v1.md](research-search-transport-v1.md).

The pure `query-planner-v1` input/output contract, purpose variants,
authority-aware deterministic ordering, generic-topic behavior, and discovery
mapping are documented in [query-planner-v1.md](query-planner-v1.md).

The single durable Research Run state machine, legal transitions, cancellation
and failure terminality, and bundle-before-completed ordering are documented in
[research-run-lifecycle-v1.md](research-run-lifecycle-v1.md).

The bounded Evidence contract, canonical excerpt hashing, structured Claim
scopes, multi-evidence relationships, copyright boundary, and forward-only
persistence compatibility are documented in
[evidence-claims-v1.md](evidence-claims-v1.md).

The conservative `evidence-extractor-v1` candidate contract, snapshot-local
locators, stable relevance scoring, admission anchors, and excerpt/context
bounds are documented in
[research-evidence-extractor-v1.md](research-evidence-extractor-v1.md).

The literal, ambiguity-rejecting `claim-extractor-v1` candidate contract, six
initial Claim families, qualifier rules, confidence constants, and exact-only
dedupe boundary are documented in
[research-claim-extractor-v1.md](research-claim-extractor-v1.md).

The post-dogfooding natural-topic compatibility rules used by the production
Evidence and Claim composition are documented in
[research-extractors-v2.md](research-extractors-v2.md). The provider-independent
classification of successfully normalized live Sources is documented in
[research-live-source-classifier-v1.md](research-live-source-classifier-v1.md).

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

The community resource types, supplementary-by-default rule, tightly bounded
Authority Profile elevation, explicit attribution, comment limitation,
freshness handling, and popularity exclusion are documented in
[community-resource-policy-v1.md](community-resource-policy-v1.md).

The host-neutral video metadata, normalized Source fields, transcript
availability-only retention, adapter-supplied timestamp links, supplementary
trust rule, and additive SQLite encoding are documented in
[video-learning-resources-v1.md](video-learning-resources-v1.md).

The proven-origin graph, organization/upstream dependency grouping,
kind/perspective/implementation-reference dimensions, normative single-source
exception, warnings, and deferred geography/language fields are documented in
[source-diversity-v1.md](source-diversity-v1.md).

The commit-pinned, host-neutral repository/path/line/symbol/version locator,
bounded excerpt, optional reviewed license metadata, normative-source
precedence, citation integration, and additive persistence are documented in
[real-source-code-evidence-v1.md](real-source-code-evidence-v1.md).

The five workspace-local cache layers, versioned TTL/size/eviction policy,
explicit hit and offline-stale warning, corruption detection, offline adapters,
safe status/clear CLI, and durable-evidence boundary are documented in
[research-cache-offline-v1.md](research-cache-offline-v1.md).

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

The provider-neutral budgets, cache-first/sufficiency decisions, atomic SQLite
ledger, explicit stop reasons, and `research stats` view are documented in
[research-cost-control-v1.md](research-cost-control-v1.md).

The eight deterministic research/research-refresh triggers, priority,
deduplication, durable queue metadata, and explicit no-scheduler boundary are
documented in
[research-trigger-policy-v1.md](research-trigger-policy-v1.md).

The human-first Research/Sources commands, durable manual query planning,
bundle-derived status output, privacy behavior, and no-provider boundary are
documented in [research-cli-v1.md](research-cli-v1.md).

The terminal Research, Sources, Source/Claim detail, Conflicts, and Freshness
views; persisted authority/freshness projection; compact URL policy; and native
browser-opening boundary are documented in
[research-source-transparency-tui-v1.md](research-source-transparency-tui-v1.md).

The read-only stored/live change inventory, deterministic signal ordering,
privacy-gated optional provider, explicit incomplete states, and curriculum
non-mutation boundary are documented in
[update-scan-v1.md](update-scan-v1.md).

The deterministic old/new Claim, snapshot, and release comparison; six drift
types; severity/confidence policy; unresolved-evidence behavior; and versioned
legacy-compatible persistence are documented in [drift-v1.md](drift-v1.md).

The deterministic projection from persisted drift to affected Evidence,
bundles, Claims, explicit future curriculum references, severity, and closed
recommended actions is documented in
[impact-analysis-v1.md](impact-analysis-v1.md).

The versioned I-03 → I-04 update envelope, knowledge-change classification,
stable concept continuity, selective migration classes, and student-state
non-destruction invariants are documented in
[research-to-curriculum-update-contract.md](research-to-curriculum-update-contract.md).

The transport-neutral I-04 research read API, conservative compile-eligibility
mapping, required reason codes, exact-version behavior, and critical-content
fail-closed gate are documented in
[source-driven-compiler-contract.md](source-driven-compiler-contract.md).

The append-only Research Run checkpoints, canonical reproducibility metadata,
snapshot/hash bindings, legacy-safe SQLite storage, and `research show` view
are documented in [research-audit-v1.md](research-audit-v1.md).
