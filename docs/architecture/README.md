# Kelyro architecture

Kelyro is being built as a local-first, cross-platform system whose core stays
independent from presentation frameworks, storage engines, external services,
AI providers, and operating-system details.

The Foundation package boundaries, dependency rules, and stable contracts are
documented in [foundation.md](foundation.md).

The Student & Learning Core domain vocabulary and invariants are documented in
[student-learning-domain.md](student-learning-domain.md).

Its repository ports, application services, error taxonomy, and transaction
boundary are documented in
[student-learning-application.md](student-learning-application.md).

The additive SQLite schema, normalization decisions, indexes, constraints, and
repository adapter are documented in
[student-learning-persistence.md](student-learning-persistence.md).

The separation between immutable curriculum definitions, learner curriculum
instances, and sparse instance-scoped concept state is documented in
[learner-curriculum-instances.md](learner-curriculum-instances.md).

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
