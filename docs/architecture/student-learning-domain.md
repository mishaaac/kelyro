# Student & Learning domain

The Student & Learning domain is Kelyro's persistence- and presentation-neutral
language for describing a learner, their intent, the curriculum they consume,
and auditable learning progress. Step 1 defines vocabulary and invariants only.
It does not select educational algorithms or implement use cases.

## Package boundary

The domain lives in `internal/learning` as one cohesive package, split into files
by area. This intentionally differs from the possible package tree in the I-02
plan: the concepts share small value objects and invariants, and separate
packages would create low-value boundaries or dependency cycles before real
application use cases reveal useful seams.

Dependencies continue to point inward:

```text
CLI / TUI / future APIs
          |
          v
application services
          |
          v
 internal/learning
          ^
          |
SQLite / files / future providers
```

`internal/learning` depends only on the standard library. It does not import
Bubble Tea, SQLite, operating-system adapters, GitHub, AI providers, or any
subject-specific technology.

## Identity and time

- `ID` is a stable, non-empty, whitespace-free machine identity. A title or
  display name is never an identity. For example, a concept may use
  `knowledge.statistics.arithmetic-mean` even if its visible title changes.
- `Timestamp` is a non-zero instant normalized to UTC when it enters the domain.
- `MasteryScore` is an evidence-derived value in the closed interval `[0, 1]`.
- `MasteryThreshold` is a policy value in the same interval. It remains a
  separate type because a threshold is not observed student performance.

These value objects prevent invalid primitive values from being passed between
domain areas. Zero-value instances remain invalid and are rejected by entity
validation.

## Curriculum and progress relationships

```text
Student ---- LearningGoal ---- CurriculumRef
   |
   +---- ConceptState ---- Concept <---- Prerequisite
   |          |               ^
   |          |               |
   |          +-- mastery     Topic <- Lesson <- Module <- Phase
   |          +-- exposure
   |
   +---- Evidence / Mistake / RetentionState / ReviewSchedule / WarmUpPlan
   |
   +---- LearningSession ---- StudyActivity
   |
   +---- Streak / Achievement / Milestone / AnalyticsSnapshot
   |
   +---- DailyPlan ---- DailyPlanItem
```

The curriculum hierarchy uses ordered ID references rather than embedding an
artificial depth limit. Later curriculum-consumption steps may add aggregate
loading and graph policies without changing the identity of concepts.

## Glossary

| Term | Meaning |
| --- | --- |
| `Student` | Stable learner identity and current profile. |
| `StudentProfile` | Learner-provided optional display name, general experience, preferred language, study preferences, timezone, and availability. |
| `ExperienceLevel` | Coarse starting point: novice, beginner, intermediate, or advanced. |
| `StudyPreference` | Preferred learning mode, independent from subject and UI. |
| `Availability` | Sustainable daily time budget, weekly-days target, and optional preferred weekdays. |
| `LearningGoal` | Intended learning outcome, open-text domain, goal-specific starting level, lifecycle status, and mastery threshold. |
| `CurriculumRef` | Stable identity plus version of deterministic curriculum supplied elsewhere. |
| `Phase`, `Module`, `Lesson`, `Topic` | Ordered curriculum containers connected through stable child IDs. |
| `Concept` | Smallest independently tracked unit of knowledge. |
| `Prerequisite` | Directed requirement from one concept to a different concept. |
| `ConceptState` | One student's exposure lifecycle and mastery score for a concept. |
| `MasteryScore` | Proficiency derived from evidence; never inferred solely from lifecycle state. |
| `Evidence` | Immutable scored observation with type, source, concept, and timestamp. |
| `Mistake` | Remembered error or misconception assigned to a known concept. |
| `LearningSession` | Completed bounded study period for a student and goal. |
| `StudyActivity` | Typed, concept-linked interval contained by a learning session. |
| `RetentionState` | Versioned point-in-time recall strength, stability, status, and due estimate; separate from mastery. |
| `ReviewSchedule` | Versioned next due instant, review type, time estimate, and prerequisite priority for an introduced concept. |
| `ReviewItem` | Trackable pending, completed, or skipped review with explicit outcome and postponement metadata. |
| `WarmUpPlan` | Versioned, bounded selection of contextual concept reviews with explicit reasons and priorities; not an exercise. |
| `Streak` | Versioned, recalculable current/longest/total active-day projection in the learner timezone; informational only. |
| `AchievementDefinition` | Versioned data describing one deterministic criterion and professional display text. |
| `Achievement` | Idempotent student recognition with historical unlock instant, explanatory context, and policy version. |
| `Milestone` | Meaningful goal progress event distinct from gamification. |
| `AnalyticsSnapshot` | Legacy stored summary retained for schema compatibility. |
| `LearningAnalyticsSnapshot` | Explainable, versioned point-in-time metrics recalculated from primary learning facts. |
| `DailyPlan` | Dated, ordered study proposal for a student and goal. |
| `DailyPlanItem` | Typed concept work with explicit position and estimated minutes. |

## Lifecycle states

A learning goal is exactly one of `draft`, `active`, `paused`, `completed`, or
`archived`.

Concept exposure is exactly one of `not_seen`, `introduced`, `learning`,
`practicing`, `mastered`, or `review_due`. Exposure and mastery answer different
questions: exposure describes workflow; mastery is a numeric conclusion from
evidence. A `mastered` exposure value therefore does not manufacture a score,
and a score does not silently change exposure state.

## Domain invariants

- Every entity and relationship ID is valid and stable; titles are display text.
- Mastery scores and thresholds are finite values within `[0, 1]`.
- All internal timestamps are non-zero UTC instants.
- Creation, update, resolution, session, and activity ranges are chronological.
- A concept cannot require itself.
- A learning goal always has valid details and lifecycle timestamps consistent
  with its status; its domain is open text rather than a rigid taxonomy.
- Evidence always records student, concept, type, source, score, and observation
  time.
- A mistake can be checked against an in-memory curriculum concept set without
  coupling the domain to a repository.
- A non-imported review requires an introduction timestamp and cannot be due
  before introduction. Explicit imports are the only exception.
- A v1 review has one of three fixed type/time pairs, and at most one pending
  item exists for a student and concept. Skipped items have no score or outcome;
  completed items have a score-consistent success or failure outcome.
- A v1 streak has one captured timezone and threshold, deduplicates active
  local dates, and cannot have a longest run above its total active days.
- A non-`not_seen` concept has an introduction instant; a `not_seen` concept does
  not.
- Session activities are individually valid and contained by the session range.
- Counts, durations, and analytics metrics cannot be negative; mastered concepts
  cannot exceed introduced concepts. Learning Analytics v1 excludes `not_seen`
  mastery rather than averaging it as zero.
- Completed reviews and unlocked achievements have corresponding timestamps;
  other states do not.
- A v1 achievement references a valid v1 definition, is always unlocked, and
  carries valid explanatory context plus `achievement-v1`.
- Collections that represent identity or ordering reject duplicate IDs or
  positions where ambiguity would result.

Cross-aggregate rules take the required aggregates as ordinary values. For
example, mistake-to-concept membership is validated against a concept slice.
This keeps the domain deterministic and avoids a premature repository interface.

## Deferred policy

The base domain did not define educational policies. Mastery, diagnostic,
retention, progression, spaced-repetition, warm-up, streak, and achievement
policies are now documented in their dedicated versioned architecture records.
Adaptive daily-plan selection and exercise generation remain deferred to their
explicit later steps. Analytics aggregation is defined by
`learning-analytics-v1` and documented in
[`explainable-learning-analytics-v1.md`](explainable-learning-analytics-v1.md).
The base domain also does not define database schemas, migrations,
repositories, commands, screens, generated curricula, exercise engines,
research, or AI behavior.
