# Versioned learning-state recalculation

Step 29 provides a controlled path for changing replaceable Student Core
algorithms without rewriting the observations that produced learner progress.
The advanced local command is:

```text
kelyro maintenance recalculate --dry-run
kelyro maintenance recalculate
```

The first form computes the same deterministic projection as an apply but
writes nothing and creates no backup. The second form requires a successful
Foundation backup before opening the Student Core maintenance transaction.

## Versioned algorithm suite

`LearningAlgorithmSuite` selects one implementation each for mastery,
retention, and daily planning. The production suite wraps `mastery-v1`,
`retention-v1`, and `daily-plan-v1`; it does not duplicate their formulas.
Internal construction can inject another supported suite, which lets migration
tests simulate v2 results without shipping a product v2 policy early.

Every algorithm must identify its version, and every returned projection must
match that configured version. A mismatch fails before commit. Existing
retention, review, and daily-plan aggregates already record their algorithm or
policy versions. Forward-only schema migration v21 adds
`mastery_algorithm_version` and `progression_policy_version` to
`learner_curriculum_concept_states`, backfilling published state as
`mastery-v1`/`progression-v1`. Newly persisted pre-calculation sparse state uses
the explicit compatibility marker `unversioned/v0` rather than pretending a
calculation occurred.

Future production versions remain explicit code and schema changes. Accepting
an arbitrary version label does not silently make a new formula supported; the
configured suite and its forward migration define that compatibility.

## Recalculation transaction

The service loads all evidence for the local student once, groups it by stable
concept ID, and scans every curriculum-instance state. Evidence for a concept
that belongs to an instance can materialize its missing sparse state. It then
projects, in order:

1. mastery and progression metadata;
2. retention and review-due exposure;
3. active-curriculum review schedules and pending review items;
4. today's active-goal daily plan.

Completed/skipped reviews and historical daily plans are not rewritten.
Existing explicit review postponements remain later than an algorithmic due
date. The current daily plan is replaced only when its source fingerprint or
policy version changes. Equivalent derived state preserves its timestamp, so a
same-clock v1 recalculation is idempotent.

All apply writes occur through one `UnitOfWork`. A failure in any later stage
rolls back concept state, retention, review, and daily-plan changes together.
The impact returned by both modes includes previous and target version sets,
evidence/concept counts, and changed counts for every derived aggregate.

## Backup, audit, and immutable evidence

An apply without a backup ID is rejected by the application service. The CLI
path creates an allowlisted Foundation backup with reason
`learning-algorithm-recalculation`; a failed backup prevents the recalculation
service from being called. Successful apply records the safe audit event
`learning.recalculation.completed` with the backup ID, target versions, and
aggregate counts. No learner content or evidence payload is included.

`EvidenceRepository` exposes append and read operations only. Recalculation
uses `ListByStudent`, and tests compare the complete evidence sequence before
and after v1/v2-simulated runs. Correction of historical evidence remains a
separate, explicit future workflow rather than an implicit side effect of
algorithm migration.

## Boundaries

This step does not implement mastery-v2, retention-v2, a new review scheduler,
or daily-plan-v2. It adds no Exercise Engine, Curriculum Compiler, Research
Engine, AI provider, plugin, network activity, integrity scan, or performance
hardening reserved for later steps.
