# Student & Learning Core persistence

The production adapter for Student Core persistence lives in
`internal/storage/sqlite`. The domain and application packages do not import
SQLite, SQL handles, or driver types.

## Schema evolution

Schema version 4 is the first Student Core migration. Versions 1 through 3 are
the published Foundation history and remain byte-for-byte unchanged. The new
migration is additive and non-destructive, so existing workspace state and the
Foundation backup/restore format continue to work without a pre-migration
backup.

All domain timestamps are encoded as RFC 3339 with nanoseconds after conversion
to UTC. Database constraints require the stored representation to end in `Z`.
Stable domain IDs are stored as text primary keys; display names and titles are
never used as identity.

## Normalization and source of truth

The schema separates learner identity, profile, ordered preferences and
availability. Goals, concept state, immutable evidence, mistakes, retention,
sessions and activities, review schedules and review items, streak state,
achievements, milestones, analytics snapshots, and daily plans each have an
explicit table boundary.

Curricula are stored as versioned instances with generic hierarchical nodes and
prerequisite edges. A small concept registry provides stable foreign-key targets
for student facts that intentionally outlive a particular curriculum version.
The fixture seeder only installs deterministic test curriculum data; it is not a
Curriculum Compiler or an ingestion engine.

No computed mastery, scheduling, streak, or daily-plan cache was introduced by
the initial schema. Current state tables are authoritative values supplied by
versioned policies. Since Step 18, `retention_state` stores the replaceable,
version-labelled output of `retention-v1`; immutable evidence remains the
recalculation source. Since Step 19, review schedules and items carry
`review-scheduler-v1` type, priority, time-budget, and lifecycle metadata while
the domain remains the source of scheduling policy. Legacy analytics snapshots
and daily plans are retained because they are auditable historical outputs,
not transparent caches. The richer `learning-analytics-v1` snapshot is
recalculated from primary facts and does not read or write the legacy analytics
table; no new cache or migration is introduced in Step 23.

Step 20 adds no persistence. `warm-up-selector-v1` reads existing curriculum,
review, mistake, and profile state; its recent-selection rotation context and
result remain caller-owned until a later daily-plan or session use case chooses
to persist an auditable plan.

Forward-only migration v18 extends `streak_state` with total active days, the
last active local date, captured timezone, minimum active minutes, and policy
version. Existing v4 rows remain `legacy-streak/v0`; the application replaces
them through a complete `streak-v1` recalculation. SQLite guards aggregate
shape only—the educational and local-calendar policy remains in the domain.

Forward-only migration v19 extends the published v4 achievement tables with
definition criteria/configuration, description, visibility and version, plus
student unlock context and evaluator version. Existing rows remain readable as
`legacy-achievement/v0`. Definition and unlock writes stay separate repository
operations so the application can install deterministic data and use
insert-if-absent semantics inside one unit of work.

Forward-only migration v20 extends the published v4 daily-plan tables with the
curriculum instance, local-date context, budget totals, result status,
regeneration reason, source fingerprint, policy version, and explainable item
metadata required by `daily-plan-v1`. Existing rows remain readable as
`legacy-daily-plan/v0`. The unique student/goal/local-date row is an auditable
daily snapshot; explicit regeneration atomically replaces only that day's
invalidated snapshot. SQLite guards enforce snapshot shape and budget, while
selection priorities remain in the domain.

## Integrity and access paths

Foreign keys reject student facts for unknown students or concepts and cascade
learner-owned records when a student is removed. Check constraints mirror the
closed enums, score ranges, temporal ordering, and optional-timestamp rules of
the domain.

Indexes cover:

- curriculum concept and parent lookups;
- active goals and student goal lists;
- concept evidence and mistake timelines;
- pending and scheduled reviews by due time;
- one pending review per student and concept through a partial unique index;
- study history and session ranges;
- milestone and analytics timelines.

## Repository boundary

`Database.LearningRepositories` implements the narrow ports from
`internal/learning/application`. `Database.WithinTransaction` implements its
unit-of-work contract. Composite writes such as students, sessions, legacy
achievement saves, curriculum fixtures, and daily plans use a transaction even when
called outside an application unit of work.

The adapter validates values before writing and reconstructs and validates
domain entities after reading. SQLite failures are translated before crossing
the boundary:

- missing rows become `not_found`;
- unique and primary-key violations become `conflict`;
- other constraint violations become `invalid_state`;
- cancellation, deadlines, locks, I/O and availability failures become
  `unavailable`;
- malformed persisted values and unclassified driver failures become
  `persistence_failure`.
