# Persistent study-session lifecycle

Step 16 adds a durable lifecycle for intentional study time. It is separate
from the Foundation app session, does not record user input, and does not infer
lesson completion.

## Model and states

Every new `StudySession` identifies one student, active learning goal, and
learner curriculum instance. It stores:

- an opaque session ID;
- start, optional end, and last meaningful activity timestamps in UTC;
- `active`, `completed`, `interrupted`, or `recovered` status;
- accumulated active duration and meaningful activity count;
- the policy version and idle timeout captured when it started.

`active` is the only open state and has no end timestamp. The other states are
terminal. `completed` means the learner explicitly stopped the study period;
it does not mean that a lesson or concept was completed. `interrupted` is an
explicit abnormal/early close. `recovered` means Kelyro found an abandoned
active session after its idle window and closed it deterministically.

## Idle policy v1

The replaceable policy is `study-session-v1`. Its default idle timeout is 15
minutes and new sessions can receive a different positive duration through
the application/factory configuration. Each session persists its timeout, so
changing configuration does not reinterpret an already open session.

For a meaningful activity or explicit stop observed at `t`:

```text
elapsed = t - last_activity_at
counted = min(elapsed, idle_timeout)
active_duration = active_duration + counted
```

Only `RecordActivity` increments `activity_count` and advances
`last_activity_at`. Presentation adapters call it for educational events such
as submitting a future exercise or completing a review, never for keypresses,
mouse movement, view navigation, or a periodic app heartbeat.

The active duration therefore remains bounded even if a process stays open
indefinitely. A normal stop also counts at most one final idle window. A
session is stale only when the observation is strictly later than its idle
boundary; at the exact boundary it remains resumable.

## Crash recovery

`Recover` behaves differently according to the stored policy:

- a recent active session remains active and can be resumed after a process
  restart;
- a stale session becomes `recovered`, ends at
  `last_activity_at + idle_timeout`, and accumulates at most that final idle
  window.

Starting a new study session performs the same stale check in one transaction.
It rejects a recent duplicate active session; if the existing session is
stale, it recovers that session and creates the replacement atomically.

## Ownership and persistence

The application service verifies that the goal and curriculum instance belong
to the workspace learner, match each other, and are both active. Domain and
application code remain independent of SQLite.

Forward-only migration v14 creates `study_session_lifecycle` and a composite
parent index for learner curriculum instances. Foreign keys bind every session
to its exact `(instance, student, goal)` scope. A partial unique index enforces
one `active` session per student/workspace, while constraints guard statuses,
timestamps, counters, policy version, and positive idle time.

The published v4 `study_sessions` and activity tables are retained unchanged
for compatibility with pre-lifecycle completed records. Migration v14 does not
fabricate curriculum-instance ownership for those legacy rows. New lifecycle
sessions use the v14 table; Step 17 can combine historical projections without
rewriting published history.

The memory adapter mirrors create, active lookup, update, canonical listing,
and uniqueness behavior for deterministic application tests. SQLite adapter
tests cover roundtrip, constraints, legacy migration, and persistence across
real store reopenings.

## Presentation boundary

The CLI provides:

```text
kelyro session status
kelyro session stop
```

There is intentionally no CLI `start`: a study period begins when an
educational surface knows the exact active goal and curriculum instance.
Kelyro currently has no lesson/exercise study screen, so opening Foundation,
visiting Home/Roadmap, onboarding, running the initial diagnostic, or closing
the TUI does not create or complete a study session. The lifecycle service is
the explicit integration point for a future TUI study surface, without
coupling educational policy to Bubble Tea.

## Explicit boundaries

Study sessions do not capture keystrokes, editor activity, telemetry, exercise
payloads, mistakes, Evidence, retention, reviews, streaks, achievements, or
analytics. Study History and time-range aggregation remain Step 17; retention,
spaced repetition, warm-ups, and the full Exercise Engine remain later steps.
