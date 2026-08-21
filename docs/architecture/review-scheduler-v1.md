# Spaced Repetition Scheduler v1

Step 19 turns the due estimate from `retention-v1` into durable review
metadata and a bounded daily queue. `review-scheduler-v1` is deterministic
domain logic: it does not generate review exercises, query SQLite, or read the
system clock directly.

## Inputs and output

For each introduced concept in an active curriculum instance, the scheduler
receives its current concept state, a retention snapshot measured at the
injected UTC instant, review history, and whether the concept is a prerequisite
of another curriculum concept. It produces one `ReviewSchedule` containing:

- the next due instant from `RetentionState.NextDueAt`;
- one review type and its fixed time estimate;
- the critical-prerequisite flag;
- the `review-scheduler-v1` algorithm version.

`unknown` retention and `not_seen` concepts do not produce a schedule. The
three metadata-only review types are:

| Type | Estimated time | Selection policy |
| --- | ---: | --- |
| `quick_recall` | 5 minutes | Strength is at least `0.80`. |
| `standard_review` | 10 minutes | Strength is at least `0.50` and below `0.80`. |
| `deep_review` | 20 minutes | Strength is below `0.50`, or the latest v1 review failed. |

The type selects review depth only. Concrete questions, exercises, grading,
and adaptive exercise generation remain in I-05.

## Due queue and time budget

Only pending items whose due instant is at or before the injected queue instant
enter the due queue. Ordering is stable and applies this tuple:

```text
overdue first
lower retention strength first
critical prerequisite first
earlier due instant first
stable review item ID first
```

The student's `Availability.DailyMinutes` is a hard daily review budget. The
queue greedily accepts items in priority order while they fit. An item too large
for the remaining budget is deferred, but a later smaller item may still use
the available time. The result reports budget, used minutes, total due minutes,
and deferred items, so an excessive backlog is visible without presenting an
impossible daily plan.

## Review lifecycle

There is at most one pending review item per student and concept.

- Postponing moves its due instant strictly later and records the explicit
  postponement. Subsequent recalculation preserves that later choice.
- Skipping closes the item as `skipped` and creates a pending item deferred by
  24 hours. It emits no evidence and is never interpreted as success.
- Completing records the score and maps scores at or above `0.70` to `success`;
  lower scores map to `failure`.
- Completion appends immutable `review_recall` evidence, recalculates mastery
  and retention from evidence, projects `review_due`, and creates the next
  review. A failure therefore selects `deep_review` and receives the shorter
  next interval produced by `retention-v1`.

Completing the same item again with the same score is idempotent. A different
score conflicts with the immutable recorded outcome. Listing is also
idempotent: deterministic IDs plus the pending-item uniqueness rule prevent
duplicate work.

## Clock and timezone policy

Application services receive a clock function and normalize its value through
the domain UTC timestamp. Scheduling, ordering, due comparisons, and storage
all use UTC instants. `kelyro reviews` and `kelyro reviews due` convert due
instants only for display, using the learner profile timezone.

## Persistence and compatibility

Forward-only migration v17 extends `review_schedule` and `review_items` with
review type, time estimate, priority and lifecycle metadata, timestamps, and
algorithm version. Existing rows are preserved as `legacy-review/v0`. If old
data contains multiple pending items for a concept, migration keeps the
earliest deterministic item pending and marks the rest skipped before adding a
partial unique index.

SQLite triggers mirror v1 lifecycle and score/outcome constraints. The actual
scheduling and priority policy remains in `internal/learning`, shared by the
SQLite and in-memory application paths.

## Boundary with Step 20

The scheduler returns review work only. It does not choose warm-up material,
mix reviews with new learning, calculate streaks or achievements, or assemble
an adaptive daily plan. Those remain in their dedicated I-02 steps.
