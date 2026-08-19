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

No computed mastery, retention, scheduling, streak, or daily-plan cache is
introduced here. Current state tables are authoritative values supplied by
later versioned policies. Analytics snapshots and daily plans are retained
because they are auditable historical outputs, not transparent caches.

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
- study history and session ranges;
- milestone and analytics timelines.

## Repository boundary

`Database.LearningRepositories` implements the narrow ports from
`internal/learning/application`. `Database.WithinTransaction` implements its
unit-of-work contract. Composite writes such as students, sessions,
achievements, curriculum fixtures, and daily plans use a transaction even when
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
