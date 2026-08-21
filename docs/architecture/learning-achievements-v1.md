# Learning achievements v1

## Purpose and boundary

`achievement-v1` recognizes durable learning progress with deterministic,
professional milestones. It does not award points, currency, levels, access,
mastery, review priority, or curriculum progression. An achievement is a
remembered recognition; the underlying study facts remain authoritative.

Definitions are versioned data. The Foundation catalog is returned by
`FoundationAchievementDefinitions` and initially contains:

| ID | Criterion | Configuration |
| --- | --- | --- |
| `first_session` | first completed meaningful study session | none |
| `first_concept_mastered` | first persisted concept mastery instant | none |
| `seven_active_days` | distinct active local dates from `streak-v1` | `count = 7` |
| `ten_hours_studied` | cumulative active session duration | `minutes = 600` |
| `first_review_completed` | first completed scheduled review | none |
| `module_mastered` | every concept in any one curriculum module is mastered | none |

Every definition carries title, description, criterion type, typed criterion
configuration, hidden flag, and `achievement-definition/v1`. Hidden affects
presentation only; it does not alter evaluation or persistence.

## Deterministic evaluation

The evaluator receives an explicit UTC `as_of`, current profile timezone,
Study History, Study Sessions, review items, and complete module membership
with instance-scoped concept states. Facts after `as_of`, facts belonging to a
different student, and duplicate identities are invalid input.

The criteria are:

- a first session requires `completed` plus at least one meaningful recorded
  activity; stopping an empty session is not progress;
- first concept mastery uses the earliest `MasteredAt` across curriculum
  instances;
- active days call the same `CalculateActiveStudyDaysV1` projection used by
  `streak-v1`, including the ten-minute default, local calendar, timezone, and
  DST behavior;
- studied time is `sum(StudySession.ActiveDuration)` in historical session
  order and crosses at the first session anchor where the configured duration
  is reached;
- first review uses the earliest `CompletedAt` from a completed review;
- a module crosses when every concept in that module and curriculum instance
  has a `MasteredAt`; its historical instant is the latest mastery instant
  among those concepts.

Where stored facts have the same timestamp, stable IDs break ties. Unlock
context records only explanatory identifiers and thresholds such as session,
concept, review, module, curriculum instance, active-day count, timezone, or
studied minutes. It contains no secret or generated content.

## Recalculation and idempotency

`AchievementService.Refresh` performs one transaction:

1. validate and upsert the current Foundation definitions;
2. load all durable facts required by the criteria;
3. evaluate every definition with `achievement-v1`;
4. insert satisfied student achievements using the unique pair
   `(student_id, achievement_key)`;
5. append `achievement.unlocked` Study History only when the insert was new;
6. return all stored achievements and the newly unlocked subset.

Student achievement IDs are deterministic from student and definition IDs.
The repository `Unlock` operation uses insert-if-absent semantics, so retries
and concurrent refreshes cannot unlock or announce the same definition twice.
The stored row is not eligibility evidence: deleting it and recalculating from
the retained educational history reproduces the recognition whenever the
source facts still exist.

## Persistence and compatibility

Forward-only migration v19 enriches the v4 achievement tables with definition
description, criterion/configuration, hidden flag, definition version, unlock
context, and evaluator policy version. Published v4 rows remain readable as
`legacy-achievement/v0`; no published migration is modified.

SQLite preserves the existing student/definition uniqueness and validates JSON,
closed criterion/version values, boolean visibility, required v1 descriptions,
criterion configuration shape, and unlocked-only v1 student records. The
domain remains responsible for educational evaluation.

## Presentation

On TUI initialization, a ready learning path runs one refresh. Newly created,
visible recognitions appear as a small block such as:

```text
Milestone unlocked
7 active study days
```

Previously stored achievements are not announced again. There is no public CLI
surface in this step and no points, rewards, penalties, notifications, or
content gates.
