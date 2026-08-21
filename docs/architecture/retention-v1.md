# Retention Model v1

Step 18 distinguishes evidence-derived proficiency from the current estimate
that the learner can recall a concept. `retention-v1` is deterministic domain
logic. It does not query SQLite, read a timezone, create exercises, or schedule
review items.

## Inputs and state

The calculation receives one validated `mastery-v1` calculation, the complete
immutable evidence history for the same learner and stable concept, and an
explicit UTC measurement instant. The application service supplies that
instant through an injectable clock and saves the result transactionally.

Each durable snapshot records:

- last successful recall and last recall-bearing practice;
- review, successful-review, and failed-review counts;
- stability estimate in whole seconds;
- predicted recall strength, status, next due instant, measurement instant,
  and `retention-v1` algorithm version.

Objective diagnostic answers, knowledge checks, practice outcomes,
assessments, project evidence, and review recalls start or refresh the retention
clock. Self-report and manual imports remain visible to mastery but cannot by
themselves prove recall. A score of `0.70` or greater is a successful recall.
Only `review_recall` evidence increments review counters; a completed review
without recall evidence has no invented outcome.

No known mastery or no recall-bearing evidence produces `unknown`, with no due
instant. Future evidence relative to the supplied clock is rejected rather
than silently ignored.

## Formula

The latest recall-bearing observation supplies normalized difficulty `d`.
With mastery `m`, successful reviews `s`, failed reviews `f`, and latest result
`r`:

```text
base_days        = 1 + 6m
difficulty_factor = 1.25 - 0.50d
review_factor     = clamp(1 + 0.50s - 0.25f, 0.50, 4.00)

recent_factor = 0.25  when r < 0.70
                1.25  when the latest result is a successful review
                1.00  otherwise

stability = clamp(
  base_days × difficulty_factor × review_factor × recent_factor,
  0.25 days,
  90 days
)

next_due_at = last_practice + stability
strength    = mastery × exp(-elapsed_since_last_practice / stability)
```

Stability is rounded to the nearest whole second before calculating the due
instant and strength. A successful review therefore extends the estimate; a
recent failure sharply shortens it. Difficulty is general normalized metadata,
not a language- or subject-specific scale.

## Status boundaries

- `unknown`: no supported estimate exists;
- `fresh`: successful recall within `clamp(stability / 4, 6h, 24h)`;
- `stable`: before due while predicted strength is at least `0.70`;
- `weakening`: before due with lower predicted strength, or immediately after
  a failed recall;
- `due`: at or after `next_due_at`, through one additional stability interval;
- `overdue`: strictly later than that additional interval.

The exact due instant belongs to `due`; the overdue boundary is strict. All
arithmetic uses UTC instants. Timezones are presentation concerns only.

## Mastery and progression semantics

Strength is a prediction, not mastery. Time passing never rewrites immutable
evidence or lowers the stored mastery score. `due` and `overdue` mean recall
should be checked, not that forgetting has been proven.

When recalculation sees an existing instance state in `mastered`, it stores the
next due instant and changes exposure to `review_due` only when due. A later
successful recall can return that exposure to `mastered`. The transition
preserves the mastery score and the historical `mastered_at` instant. States
currently learning or practicing are not promoted or overwritten by retention.

## Persistence and compatibility

Forward-only migration v16 extends the published `retention_state` table and
adds aggregate guards. Snapshots written before the policy are preserved as
`legacy-retention/v0`, `unknown`; their old strength and measurement instant
remain inspectable, but they are not treated as a v1 calculation.

The in-memory and SQLite adapters share the same `RetentionRepository` port.
`RetentionService.Recalculate` reads evidence, calculates mastery and
retention, persists the snapshot, and projects due state inside one unit of
work. `State` returns the last durable snapshot.

## Scheduling boundary

Retention still does not create `ReviewSchedule` or `ReviewItem` records.
`review-scheduler-v1` consumes its durable due instant, strength, and status to
select review metadata and a bounded queue. Retention remains the sole owner of
recall decay and the next interval; the scheduler owns review type, priority,
explicit deferral, and lifecycle.
