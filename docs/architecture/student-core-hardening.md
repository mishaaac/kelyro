# Student Core integrity, privacy, and performance hardening

Step 30 hardens the connections between the already-published Student Core
subsystems. It does not introduce a new learning policy or product feature.

## Integrity layers

SQLite continues to enforce row-local ranges, lifecycle shapes, foreign keys,
and uniqueness. In particular, partial unique indexes prevent more than one
active goal, active study session, or pending review for the same owner, while
the student-achievement key prevents duplicate unlocks.

Forward-only migration v22 adds cross-aggregate guards that SQLite foreign keys
cannot express on their own:

- an instance concept state must name a concept node in that instance's exact
  immutable curriculum ID and version;
- a diagnostic observation, its attempt, and its evidence must have the same
  student and concept ownership, and that concept must belong to the attempt's
  curriculum instance.

Database open now follows physical `quick_check` and migration with a bounded
Student Core scan. It runs `foreign_key_check`, checks duplicate active/pending
aggregates, mastery ranges, curriculum-state membership, diagnostic ownership,
and all persisted IANA timezone identifiers. Findings return a stable integrity
category and finding name, never learner IDs, answers, mistake text, evidence
sources, or other row content. The read-only backup validator applies the same
relational and version-appropriate semantic checks, so a physically readable
but relationally corrupt snapshot is not accepted as recoverable.

Historic daily-plan and streak timezones may legitimately differ from the
current profile after a profile edit. Integrity therefore validates every
identifier but does not treat a valid historic timezone difference as
corruption. Recalculation and refresh policies remain responsible for producing
new current-state snapshots.

## Privacy review

The profile schema remains limited to display preference, general experience,
language, sustainable availability, learning-mode preferences, preferred days,
and timezone. A schema allowlist regression test detects accidental collection
of unrelated personal fields.

Progress Markdown remains a derived allowlisted projection. Its existing
privacy tests exclude internal IDs and normalize student-authored titles; it
does not include profile details, goal descriptions, evidence, mistakes, or
diagnostic responses.

Rejected onboarding and diagnostic answers are no longer interpolated into
domain errors. The application logging boundary also marks onboarding, setup,
profile, and goal text inputs as sensitive, so defensive log sanitization
redacts them if a lower-level error ever repeats them. Technical audit events
continue to contain only operation metadata and aggregate counts. No Student
Core package adds a network client; normal learning, export, integrity, and
maintenance operations remain local-only.

## Scale and query behavior

The deterministic `student-core-scale/v1` test fixture contains 50 phases, 150
modules, 500 lessons, 500 topics, 2,000 concepts, 1,500 prerequisite edges,
2,000 instance states, and 6,000 evidence records. It exercises immutable
curriculum installation plus outline, planning, state, and evidence projections
without imposing a machine-specific elapsed-time threshold.

The fixture instead verifies result cardinality and SQLite query plans. Migration
v22 adds covering timeline indexes for curriculum instances, study sessions, and
all review items; the existing evidence index serves the student/concept/time
projection. These read paths stay bulk projections whose query count does not
grow with the number of concepts. Graph and dashboard tests separately cover
large in-memory traversal and assembly.

## Concurrency and failure behavior

Each workspace database instance uses one pooled SQLite connection, foreign keys,
a bounded busy timeout, and explicit transactions for compound writes. Multiple
processes may still open the same local database. Regression tests verify that
simultaneous active-goal writes produce one committed winner and one classified
conflict, and that a write blocked beyond the busy timeout is reported as
`unavailable`, not as corrupt state or an unclassified driver error. Existing
transaction tests continue to verify rollback of compound Student Core writes.

## Boundaries

This hardening does not correct or delete learner evidence, change a published
algorithm, infer a new curriculum, generate exercises, add background work, or
introduce Research Engine, Curriculum Compiler, AI provider, plugin, telemetry,
or network behavior. Migration v22 is forward-only and leaves published
migrations unchanged.
