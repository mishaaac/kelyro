# Learning goals

Step 5 makes a learner's intended outcome explicit and persistent without
coupling it to curriculum generation, diagnosis, exercises, or AI.

## Goal data

Each `LearningGoal` belongs to the workspace's stable `student.primary` learner
and contains:

- a generated opaque ID, independent from the visible title;
- title and optional description;
- an open-text domain, so new subjects require no enum or schema change;
- a concrete target outcome;
- an explicitly supplied goal-specific starting level;
- the mastery threshold policy value;
- lifecycle status plus creation, update, first-activation, and completion
  timestamps.

The goal-specific starting level is never copied from the general learner
profile. A later diagnostic may refine goal-specific knowledge, but Step 5 does
not implement that policy.

## Lifecycle

New domain goals begin as `draft`. The workspace use case immediately activates
a goal created by `goal set`. Supported domain transitions are:

```text
draft  ----> active <----> paused
                 \          /
                  completed

draft / active / paused / completed ----> archived
```

`activated_at` records the first activation and remains stable after resume.
`completed_at` exists only for completed goals. Transitions reject time moving
backwards and invalid source states.

Step 5 exposes `set`, `pause`, and `resume`; completion and archival remain
domain capabilities for later workflows rather than extra CLI surface.

## Single-active policy and history

A workspace currently supports at most one active goal. `goal set` creates a
new ID and pauses any current active goal; it never overwrites or deletes prior
goals. `goal resume` selects the most recently updated paused goal and pauses a
different current active goal. That deterministic selection keeps the command
usable without introducing multi-goal UX before it is planned.

The pause/create and pause/resume writes run in one `UnitOfWork`. A failure rolls
back every status change. SQLite also enforces the invariant with a unique
partial index on `student_id` for rows whose status is `active`; the in-memory
adapter mirrors this behavior.

## Persistence compatibility

Forward-only migration v6 adds goal details and lifecycle timestamps. Existing
rows receive general, valid defaults; their title becomes the target outcome.
Previously active, paused, or completed goals receive compatible lifecycle
timestamps. If an older database contains multiple active goals, the most
recent remains active and the others become paused, preserving every row before
the unique index is created.

## CLI

```text
kelyro goal show
kelyro goal set --title TITLE --domain DOMAIN --target-outcome OUTCOME
  [--description TEXT] [--starting-level LEVEL] [--mastery-threshold SCORE]
kelyro goal pause
kelyro goal resume
```

`goal show` displays all retained goals and their statuses. The CLI only parses
and renders; lifecycle policy remains in `internal/learning/application` and
SQLite remains behind `internal/infra/learningdb`.
