# Safe Learning Pack Upgrade v1

## Flow and authority

`pack-upgrade-policy-v1` applies the deterministic plan produced by
`curriculum-migration-planner-v1`:

```text
discover installed version -> validate -> classify diff -> check SemVer
-> build migration plan -> backup -> require confirmation
-> migrate through I-02 -> activate -> integrity check -> audit
```

The candidate must already exist in the immutable global pack store. Loading
through `PackInstallationRepository` revalidates its archive and content hash.
Catalog metadata never authorizes installation, and upgrade v1 performs no
network access or automatic install. Users explicitly run `packs install`
before upgrading.

Every applied upgrade requires interactive confirmation or `--yes`.
`kelyro packs upgrade <id> --dry-run` performs the complete read-only planning
path and writes neither backup, activation, nor Student State.

## Student Core migration

I-04 never writes learner tables. The `learningmigration` adapter projects the
new definition to `curriculum-consumption/v1` and calls
`CurriculumInstanceService.Migrate`. That I-02 service performs one Unit of
Work containing:

- immutable installation of the new curriculum definition;
- creation of a new target curriculum instance for each applicable goal;
- copying existing derived state only for stable Concept IDs;
- explicit unknown state for additions and split/merge targets;
- archival of the source instance without deleting its concept state.

Evidence is append-only and keyed by stable learner/concept identity, so it is
not copied or reassigned. Removed and replaced identities remain inspectable on
the archived source instance. Split and merge targets never inherit mastery.
Prerequisite eligibility is evaluated against the target definition; there is
no unlock cache to copy. Retention and review schedules are likewise not
blindly transferred.

The `curriculum-consumption-projection-v1` projection is deterministic:
hierarchy IDs/order and Concept versions are
preserved, hard prerequisites become `mastered`, exposure/tool/vocabulary
requirements become `introduced`, recommendations remain outside the blocking
I-02 graph, and duplicate requirements prefer `mastered`. Concept difficulty
maps directly; its numeric level supplies the existing I-02 effort field at
30 minutes per level. Concept definition text supplies the required objective
and evidence expectation without generating I-05 content.

## Recovery and audit

Before application, Foundation backup captures `learning.db`, workspace
metadata/config, and `state/active-pack.json`. The configured
`backup.retention` policy is honored.

After migration and activation, the read-only SQLite snapshot validator runs
`quick_check`, migration-history validation, foreign-key validation, and all
Student Core integrity checks. A failure during migration, activation,
integrity, or success-audit recording restores the backup. Restoration reverts
both learner state and active-pack reference. A post-restore
`pack.upgrade.failed` event records the failed stage and whether recovery
succeeded; success records counts, versions, plan ID, backup ID, and policy.

Published pack directories are never changed or removed. A failed restore is
reported together with the original failure and backup ID so recovery remains
operator-visible rather than silent.
