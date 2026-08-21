# Adaptive Daily Plan v1

`daily-plan-v1` is the deterministic policy that selects what the active
student should work on today. It selects concepts and kinds of work; it does
not generate lessons, exercises, assessments, feedback, or educational
content. Those remain owned by I-05.

## Boundary and inputs

The pure policy lives in `internal/learning`. The application service resolves
the workspace profile, active goal, active curriculum instance, mastery
threshold, and local date, then reads the following primary facts in one unit
of work:

- the hierarchy-ordered curriculum concept projection and prerequisite edges;
- instance-scoped concept exposure and mastery;
- pending reviews whose due instant has arrived;
- retention state used to distinguish overdue reviews;
- durable mistake memory;
- study history, reduced to the last studied instant per concept;
- the profile's daily time budget.

The policy does not read the Learning Analytics snapshot. Analytics is an
explainable view, not an authoritative planning cache. No AI provider,
Research Engine, Curriculum Compiler, plugin, SQLite type, CLI, or TUI type is
part of the domain policy.

## Curriculum order and eligibility

Persistence exposes a minimal `DailyPlanCurriculumConcept` projection. Concept
sequence follows the ordered hierarchy path (each ancestor's position and
stable ID), while prerequisite IDs come from the curriculum knowledge graph.
The projection is general: it does not assume a programming language or a
particular subject structure.

The planner walks that canonical order and stops at the first concept below the
resolved mastery requirement. This strict frontier prevents a later concept
from being introduced merely because it is otherwise available.

- If the frontier was already seen but remains below the requirement, it is a
  blocking weakness and receives reinforcement.
- If the frontier is unseen and any prerequisite is below the requirement, one
  deterministic blocking prerequisite receives reinforcement.
- If the frontier is unseen and every prerequisite satisfies the requirement,
  it is the new-learning candidate.
- If every curriculum concept satisfies the requirement, there is no
  new-learning candidate.

Blocking weaknesses are ordered by unresolved mistake occurrences descending,
mastery ascending, oldest or absent study history first, curriculum sequence,
then concept ID. This is a deterministic tie-break policy, not a new mastery
formula.

## Selection order

The planner fills the budget in this order:

1. one critical overdue prerequisite as a short warm-up;
2. other important due reviews;
3. one reinforcement for a blocking weakness;
4. the next eligible unseen concept;
5. at most one optional reinforcement for a previously seen concept with an
   unresolved mistake.

A review is overdue when the current instant is later than
`next_due_at + stability_estimate`. Critical overdue reviews sort before every
other review. Remaining review ties use criticality, lower retention strength,
earlier due time, and stable review ID. A concept appears at most once in a
plan, so a due review also satisfies its warm-up/reinforcement slot for the
day.

Every selected item stores a role, a machine-readable reason, a human-readable
explanation, one concept ID, estimated minutes, and a contiguous position.

## Time budget v1

The default configuration is:

| Slot | Minutes |
| --- | ---: |
| warm-up | 5 |
| reinforcement | 10 |
| new-learning target | 25 |
| minimum useful new-learning block | 10 |
| buffer | 5 |

The buffer is reserved when the available time can hold the minimum useful
new-learning block plus the buffer. Reviews use their persisted estimate. The
new-learning slot may shrink to the remaining minutes, but never below its
minimum. Other slots are indivisible in v1. The hard invariant is:

```text
planned_minutes + buffer_minutes <= available_minutes
```

The planner therefore never exceeds the profile budget. It deliberately uses
a simple greedy policy rather than pretending to solve a perfect scheduling
optimization problem.

## Result states

The snapshot has one of four states:

- `ready`: contains an eligible new-learning item;
- `review_only`: contains reviews or reinforcement but no new learning;
- `nothing_urgent`: there is no due, blocking, new, or optional work;
- `time_limited`: work exists, but no whole v1 slot fits the budget.

An empty result is still a valid persisted daily plan and explains whether the
cause was completion/no urgency or insufficient time.

## Determinism, snapshots, and regeneration

The policy version is `daily-plan-v1`. Plan IDs and item IDs derive from stable
student, goal, local-date, policy, role, concept, and position inputs. All
unordered repository results are canonicalized before selection.

Each candidate includes a SHA-256 source fingerprint over the local date,
timezone, time budget, curriculum version/order/prerequisites, resolved mastery
policy, concept state, due-review/retention inputs, unresolved mistakes, study
history relevant to curriculum concepts, and the complete daily-plan policy
configuration. The exact generation clock instant is intentionally excluded,
so repeated calls with unchanged facts produce and reuse the same snapshot.

`AdaptiveDailyPlanService.Today` is the explicit generation boundary:

- no existing plan for the local date creates an `initial` snapshot;
- matching policy version and source fingerprint returns the stored snapshot
  unchanged;
- a different policy version regenerates with `policy_changed`;
- a changed source fingerprint regenerates with `source_changed`.

Thus mastery, due reviews, mistakes, study history, availability, timezone, or
curriculum changes can invalidate today's plan without causing regeneration
merely because the clock advanced. Calls without one active goal, or without an
active curriculum instance for it, return `not_found` and do not create a plan.

## Persistence and compatibility

Forward-only SQLite migration v20 extends the published daily-plan tables with
curriculum instance identity, local-date context, budget totals, status,
generation reason, source fingerprint, policy version, item role, selection
reason, and explanation. Existing v4 rows remain readable as
`legacy-daily-plan/v0`; no published migration is modified.

The existing `(student_id, goal_id, plan_date)` uniqueness remains the daily
snapshot boundary. Regeneration atomically replaces that day's obsolete
snapshot and its items, while snapshots for earlier dates remain available as
history. Domain validation and SQLite guards both enforce budget and v1 shape;
the educational ordering policy remains exclusively in the domain.

## Verification scope

Deterministic fixtures cover a brand-new student, due reviews with a critical
warm-up, a blocked next lesson, fully mastered current content, a tiny budget,
absence of an active goal, input-order independence, snapshot reuse,
state-driven regeneration, hierarchy/prerequisite projection, migration of
legacy rows, v1 round trips, and database budget guards.
