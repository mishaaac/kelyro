# Student-safe Curriculum Migration Plan v1

## Boundary

`curriculum-migration-planner-v1` converts a validated old/new curriculum
classification into a deterministic, learner-neutral migration plan. It does
not open Student Core storage, activate a pack, create a backup, or write
mastery. I-02 remains the only owner of learner-state application.

The plan is keyed by curriculum ID and immutable from/to versions. Its ID is a
hash of the complete canonical action set, policy version, and unlock
recalculation signal; timestamps and input ordering cannot change it.

## Actions

- `preserve_state`: the same stable Concept ID exists in both versions.
  Mastery and evidence remain eligible for preservation. Deprecated, legacy,
  and historical targets additionally retain historical-evidence intent.
- `initialize_unknown`: a newly added Concept starts without exposure,
  mastery, or invented evidence.
- `preserve_historical`: a removed Concept remains readable through the old
  curriculum instance; no state is transferred into the target.
- `split_no_transfer`: one removed ID maps explicitly to multiple new IDs.
  Historical evidence stays with the source and every target starts unknown.
- `merge_no_transfer`: multiple removed IDs map explicitly to one new ID. The
  target starts unknown because aggregating mastery would invent knowledge.

Every old and new Concept ID appears in exactly one action. IDs and actions are
canonicalized before validation, so equivalent input order yields the same
plan and plan ID.

## Review and prerequisites

Mappings must be the same explicit mappings used by the change classifier.
Missing, duplicate, or inconsistent split/merge mappings fail closed. Removed,
split, merge, and affected review/breaking changes set the student-review flag.

A prerequisite diff sets `RecalculateUnlockEligibility`. The plan never copies
an unlock cache or review schedule. A hierarchy-only move preserves mastery by
stable Concept ID because display position is not learner identity.

## Dry-run upgrade

`kelyro packs upgrade <id> --dry-run` resolves the active pack for the
workspace, discovers the highest newer immutable version already installed,
loads it through the validating repository, classifies the curriculum diff,
checks the required SemVer transition, and renders the migration summary.
`<id>@<version>` selects an installed target explicitly.

Dry-run creates no backup, changes no activation, and writes no Student State.
Catalog entries are discovery metadata only and are never auto-installed.
