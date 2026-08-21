# Student & Learning application boundary

The Student Core application boundary lives in `internal/learning/application`.
It lets CLI, TUI, tests, and future adapters invoke learning use cases without
knowing whether data comes from SQLite, an in-memory fixture, or another local
implementation.

## Dependency direction

```text
CLI / TUI
    |
    v
application service interfaces
    |
    +----> learning domain values and invariants
    |
    v
small repository ports <---- SQLite / memory adapters
```

Application services validate domain values, orchestrate repository calls, and
translate errors. Educational formulas stay in the domain: application code
loads their inputs, supplies transaction and clock boundaries, and persists
their durable versioned outputs. Spaced-repetition scheduling, read-only
warm-up selection, full-history streak recalculation, historical achievement
evaluation, explainable analytics aggregation, and adaptive daily-plan
selection follow that boundary.

## Repository ports

Persistence is split by aggregate or use case instead of hidden behind one
`LearningRepository`. The ports cover students, goals, versioned curriculum
reads, concept and retention state, evidence, mistakes, sessions, reviews,
streaks, achievements and milestones, analytics snapshots, and daily plans.

Read methods return `not_found` for an absent singular record and an empty slice
for a valid query with no results. Immutable records such as evidence and
sessions use append/create operations so duplicate stable IDs can be reported
as conflicts. Implementations must return domain values and classified errors;
database handles, rows, SQL errors, and persistence structs never cross the
port.

## Initial services

The initial service contracts are:

- `StudentService` and `GoalService` for validated aggregate lifecycle calls;
- `ProgressService` for stored concept state and evidence projections;
- `MistakeMemoryService` for deduplicated mistake lifecycle and history;
- `StudySessionLifecycleService` for active study timing and crash recovery;
- `ReviewSchedulerService` for idempotent review scheduling, bounded due queues,
  explicit postponement/skip, and transactional recall outcomes;
- `WarmUpSelectorService` for contextual, time-bounded selection over existing
  due reviews and mistake memory without mutating either source;
- `StreakService` for rebuilding and repairing the materialized consistency
  projection from durable history in the current profile timezone;
- `AchievementService` for installing versioned definitions, rebuilding
  eligibility from durable facts, and atomically recording only new unlocks;
- `LearningAnalyticsService` for rebuilding a described, versioned snapshot
  from primary concept, retention, review, session, and history facts;
- `AdaptiveDailyPlanService` for reusing or explicitly regenerating today's
  persisted, explainable `daily-plan-v1` snapshot from primary facts;
- `ProgressDashboardService` for assembling the active goal/curriculum,
  progress, location, mastery, reviews, current plan, time, streak, milestone,
  and weak-concept projection behind one presentation-neutral query;
- the legacy `SessionService` projection and `ReviewService` for completed
  activity records and review records;
- the legacy `AnalyticsService` for stored snapshot compatibility; Learning
  Analytics v1 never treats those rows as truth;
- the legacy `DailyPlanService` for direct stored-plan compatibility.

`ConceptProgress` is deliberately a read model of stored facts. It does not
derive a mastery score from evidence or change exposure state.

The curriculum read port also exposes a compact hierarchy outline. Dashboard
assembly loads this projection once and indexes it in memory, so neither CLI nor
TUI needs storage access or per-concept queries. See
[`progress-dashboard-read-model.md`](progress-dashboard-read-model.md).

## Error taxonomy

Every error visible at the application boundary has one stable kind:

| Kind | Meaning |
| --- | --- |
| `not_found` | A requested singular record does not exist. |
| `conflict` | A stable identity or immutable record already exists. |
| `invalid_state` | Input violates a domain invariant or use-case precondition. |
| `unavailable` | The operation cannot run now, including cancellation or timeout. |
| `persistence_failure` | An unclassified repository/storage operation failed. |

Callers classify with `errors.Is` or `application.KindOf`. The typed error keeps
its underlying cause for diagnostics without requiring presentation code to
understand a storage driver.

## Transaction boundary

`UnitOfWork.WithinTransaction` supplies a `Repositories` value whose individual
ports share one commit/rollback boundary. `Repositories` is only a transaction
scope; it is not itself a repository and is not exposed to presentation code.
This supports later atomic use cases such as appending evidence, updating
concept state, scheduling a review, and appending history without leaking
`sql.Tx` into application or domain code.

The `application/memory` package is a deterministic fake adapter for tests. It
runs transactions against an isolated copy and publishes the copy only after a
successful callback, so service and rollback tests require no SQLite database.
