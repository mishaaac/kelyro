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
translate errors. They do not calculate mastery, retention, scheduling,
streaks, achievements, analytics, or daily-plan selection. Those policies stay
deferred to their dedicated versioned I-02 steps.

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
- `ProgressService` for stored concept state, evidence, and mistakes;
- `SessionService` and `ReviewService` for study history and review records;
- `AnalyticsService` for auditable stored snapshots;
- `DailyPlanService` for stored plans selected by student, goal, and date.

`ConceptProgress` is deliberately a read model of stored facts. It does not
derive a mastery score from evidence or change exposure state.

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
