# Concept state and progression-v1

Step 14 connects immutable evidence, `mastery-v1`, instance-scoped concept
state, `threshold-v1`, and prerequisite decisions. The policy remains in the
learning domain; application code supplies transaction boundaries and the
knowledge graph supplies unlock truth.

## Transactional flow

`ProgressionService.RecordEvidence` performs one educational update:

```text
validate learner + curriculum instance + concept
                    ↓
append immutable Evidence
                    ↓
calculate mastery-v1 from complete concept history
                    ↓
apply progression-v1 to InstanceConceptState
                    ↓
persist state in the same transaction
                    ↓
evaluate each direct dependent before and after the update
```

An evidence conflict, future timestamp, malformed calculation, state failure,
or graph failure rolls the transaction back. `Recalculate` follows the same
path without appending evidence, which permits future algorithm migrations
without rewriting history.

Evidence is longitudinal by stable learner/concept identity. Concept state is
still isolated by curriculum instance. If applicable historical evidence
predates an instance, its first/last-seen projection is clamped to the
instance creation time; the original evidence timestamp remains unchanged.

## Versioned state policy

Policy `progression-v1` first persists the known `mastery-v1` score. No
evidence produces unknown mastery and leaves state unchanged; it is never
converted into an observed zero.

When mastery is below the effective threshold, the furthest evidence stage
determines exposure:

| Evidence | Exposure floor |
|---|---|
| objective/self-report diagnostic, manual import | `introduced` |
| knowledge check | `learning` |
| practice success/failure, assessment, project, review recall | `practicing` |

Exposure does not move backward among these three learning stages merely
because later evidence has a weaker type. When known mastery is equal to or
above the resolved threshold, exposure becomes `mastered` and the first
mastery time is recorded from the latest contributing observation.

Mastery is reversible. If recalculation later falls below the threshold, a
previously mastered concept returns to `practicing`, while `MasteredAt`
preserves the historical fact that the threshold was once met.

`review_due` and `ReviewDueAt` remain owned by the future Retention and Review
policies. Progression may refresh their mastery score but does not clear or
reverse that lifecycle state.

## Thresholds and prerequisites

The service consumes the resolved `threshold-v1` value, including its source
and any validated pack override. Equality satisfies mastery. It never embeds
presets or threshold precedence.

Unlock is not stored. After updating the concept state, the service builds one
instance snapshot and calls `KnowledgeGraph.EvaluateIntroduction` for every
direct dependent. Each result reports:

- whether it was eligible before the update;
- the complete current prerequisite decision;
- whether it became newly eligible.

This keeps `CanIntroduce` authoritative. A dependent with several
prerequisites stays locked until every declared requirement is satisfied, and
a later mastery decrease can make a formerly eligible dependent ineligible
without reconciling a stale unlock table.

## Boundaries

Step 14 does not create exercises, infer mistakes, calculate retention,
schedule reviews, compile curricula, or add presentation commands. Manual
unlock overrides are not implemented. Evidence and graph definitions remain
immutable, and SQLite contains no educational formula.
