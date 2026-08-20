# Evidence model and mastery-v1

Step 13 calculates concept mastery from immutable evidence. The calculation is
domain logic: it has no SQLite, curriculum traversal, presentation, retention,
or progression dependency. It never changes concept state.

## Evidence contract

Every evidence record identifies the learner and stable concept and includes:

- a stable evidence ID and semantic type;
- observed score in `[0,1]`;
- confidence in `(0,1]`;
- independence and normalized difficulty in `[0,1]`;
- UTC occurrence time and a source reference;
- the version of the evaluator or import algorithm that produced it.

The initial semantic types are `diagnostic_objective`,
`diagnostic_self_report`, `knowledge_check`, `practice_success`,
`practice_failure`, `assessment`, `project_evidence`, `review_recall`, and
`manual_import`. A failure is therefore observed evidence with a valid zero or
low score; absence of evidence is not encoded as failure.

Hints or closely dependent attempts can lower `independence`. Difficulty is a
normalized input supplied by the evidence producer and does not impose a
subject-specific curriculum scale. Invalid, non-finite, mismatched, or
duplicate evidence is rejected before calculation.

## Versioned formula

Policy `mastery-v1` assigns these type weights:

| Evidence type | Base weight |
|---|---:|
| assessment, project evidence | 1.00 |
| objective diagnostic, knowledge check, review recall | 0.90 |
| practice success or failure | 0.75 |
| manual import | 0.50 |
| diagnostic self-report | 0.40 |

For evidence item `i`:

```text
independence_factor_i = 0.25 + 0.75 × independence_i
difficulty_factor_i   = 0.75 + 0.50 × difficulty_i

weight_i = type_weight_i
         × confidence_i
         × independence_factor_i
         × difficulty_factor_i

mastery = Σ(score_i × weight_i) / Σ(weight_i)
```

The independence floor keeps a hinted observation visible while reducing its
effect. Difficulty ranges from a `0.75` modifier to `1.25`; the neutral value
`0.5` produces `1.0`. Positive confidence guarantees a positive total weight
whenever evidence exists.

No evidence produces `Known=false`, not a known score of zero. At least one
valid observation produces `Known=true`; that score may legitimately be zero.
Threshold comparison is inclusive and remains a separate `threshold-v1`
policy.

## Determinism and time

Before summing, evidence is ordered by `occurred_at` and then stable evidence
ID. Equal timestamps and repository/map ordering therefore produce the same
floating-point operation sequence and breakdown.

`mastery-v1` does not decay old evidence or privilege recent evidence. Step 18
owns retention and may later supply an explicit versioned policy. A future
mastery algorithm can recalculate from the append-only history without
rewriting that history.

## Explainability

`MasteryCalculationService` loads all evidence for one learner/concept and
returns both a calculation and a human summary. The structured contribution
for every item contains its score, type weight, metadata modifiers, effective
and normalized weights, weighted score, timestamp, and source reference.

The service only reads evidence. Persisting calculated mastery, changing
exposure, and applying progression are reserved for Step 14.

## Persistence compatibility

Forward-only migration v12 adds semantic type, confidence, independence,
difficulty, and producer algorithm version to the published
`learning_evidence` table. Its original coarse `evidence_type` column remains
as a storage compatibility category.

Legacy rows are classified deterministically: diagnostic becomes objective
diagnostic; zero-score practice becomes practice failure and other practice
becomes practice success; the remaining legacy categories map directly to the
closest v1 semantic type. They receive neutral metadata and
`legacy-evidence/v1`, so history is preserved rather than rewritten or
discarded.
