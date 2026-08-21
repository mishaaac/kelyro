# Explainable Learning Analytics v1

`learning-analytics-v1` turns durable learning facts into a read-only,
point-in-time snapshot. The calculator lives in `internal/learning`; the
application service only loads a coherent set of primary facts, supplies the
profile timezone and clock, and invokes the policy. CLI, TUI, SQLite, AI
providers, and curriculum generation are outside this calculation.

## Snapshot contract

Every user-facing metric carries a non-empty description. Counts and durations
are non-negative, rates state their units, rankings contain stable concept and
curriculum-instance identities, and the complete result records its capture
instant, timezone, pace window, and policy version.

| Group | Metric | v1 definition | Primary source |
| --- | --- | --- | --- |
| Progress | concepts introduced | All instance concept states other than `not_seen`, including concepts later mastered | instance concept state |
| Progress | concepts learning | Current `introduced`, `learning`, or `practicing` states | instance concept state |
| Progress | concepts mastered | Current `mastered` or `review_due` states; becoming due does not erase mastery | instance concept state |
| Progress | reviews due | Pending review items with `due_at <= captured_at` | review items |
| Time | today | Active study duration anchored in the current local day | study sessions |
| Time | week | Active study duration in the local Monday-through-Sunday calendar week | study sessions |
| Time | month | Active study duration in the current local calendar month | study sessions |
| Time | total | All recorded active study duration | study sessions |
| Mastery | average known mastery | Arithmetic mean across studied instance concept states | instance concept state |
| Mastery | strongest concepts | Up to five known concepts ordered by descending mastery, then stable identity | instance concept state |
| Mastery | weakest concepts | Up to five known concepts ordered by ascending mastery, then stable identity | instance concept state |
| Retention | fresh | Known v1 estimates whose next due instant has not arrived | retention state |
| Retention | due | Known v1 estimates from due time through one stability interval past due | retention state |
| Retention | overdue | Known v1 estimates more than one stability interval past due | retention state |
| Activity | active days | Distinct local dates satisfying `streak-v1` meaningful-activity rules | study history and sessions |
| Activity | current streak | Latest consecutive active-day run, if it reaches today or yesterday | study history and sessions |
| Activity | longest streak | Longest historical consecutive active-day run | study history and sessions |
| Pace | concepts mastered per week | First mastery timestamps inside the rolling window divided by its week count | instance concept state |
| Pace | study time per week | Active minutes inside the rolling window divided by its week count | study sessions |

An unfinished session is anchored at its last activity instant; a finished
session is anchored at its end instant. Calendar windows are inclusive at the
start and exclusive at the end and are constructed in the profile's IANA
timezone, so DST and local midnight boundaries remain correct.

## Unknown mastery

`not_seen` means that mastery is unknown, not zero. Those states are excluded
from the average and both rankings. An empty profile therefore has no average
(`nil`) and empty rankings rather than an artificial zero score. A studied
concept with an actual score of zero remains known and is included.

Legacy or explicitly unknown retention rows are likewise excluded from the
fresh/due/overdue partition. For versioned estimates, the bucket is recalculated
at capture time from `next_due_at` and the stability interval; a previously
materialized label cannot become stale analytics truth.

## Pace and deterministic ordering

The default pace window is the 28 local calendar days ending at the next local
midnight, expressed as four weeks. The configurable v1 policy permits 1–52
weeks and ranking limits of 1–100 while retaining the same versioned formulas.
The snapshot deliberately provides no graduation or completion forecast.

Concept identities are instance-scoped. Rankings break equal scores by
`curriculum_instance_id` and then `concept_id`; the average is also summed in
that stable identity order. Reordering repository results therefore cannot
change the snapshot.

## Source of truth and persistence

`LearningAnalyticsService.Snapshot` reads curriculum instances, instance
concept states, retention, reviews, study sessions, and study history inside
one unit of work. It recalculates the snapshot for the workspace profile at the
injected clock instant.

The legacy `analytics_snapshots` table and `AnalyticsSnapshot` repository
contract remain readable for compatibility with the published schema, but v1
does not read or write them. No cache or migration is needed at current data
sizes. If profiling later justifies a cache, it must be replaceable, carry its
policy/input version, and never become the authority for the underlying facts.

## Boundaries

This step adds no dashboard, CLI or TUI presentation; those belong to later
I-02 steps. It also adds no adaptive plan, exercise engine, research engine,
curriculum compiler, AI provider, plugin, or background forecasting behavior.
