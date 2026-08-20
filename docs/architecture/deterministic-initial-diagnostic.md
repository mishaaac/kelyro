# Deterministic initial diagnostic

Step 11 estimates prior knowledge without an LLM and without implementing the
Exercise Engine. A diagnostic is content supplied for one exact
`CurriculumRef`; an attempt belongs to one learner and one explicit
`CurriculumInstance`.

## Versioned contracts

The content contract is `diagnostic/v1` and the evaluation policy is
`diagnostic-scoring/v1`. A definition contains stable IDs for the diagnostic,
sections, items, concepts, answer options, and branch requirements. Its
canonical SHA-256 fingerprint is stored with the first attempt. Resume rejects
a changed definition even if its visible ID and version were reused.

The four initial item kinds deliberately remain small and generic:

- single choice;
- multiple choice with exact-set evaluation;
- short answer with deterministic case/whitespace normalization and an explicit
  accepted-answer set;
- self-report calibration with explicit option scores.

No item executes code, calls an AI provider, or updates concept mastery.

## Scoring and confidence

Every answered objective item has weight `1.0`; self-report has weight `0.25`.
For a concept with observations `i`, estimated mastery is:

```text
estimate = Σ(score_i × weight_i) / Σ(weight_i)
```

Confidence is intentionally separate:

```text
confidence = min(1, Σ(weight_i) / 2.0)
```

Consequences of v1 include:

- one perfect objective answer produces estimate `1.0` with confidence `0.5`;
- one perfect self-report produces estimate `1.0` with confidence `0.125`;
- a concept with no answered item is reported as `unknown`;
- an in-progress result is explicitly `partial`;
- a result is always estimated mastery, never confirmed mastery.

The service does not write `InstanceConceptState`, change exposure, or mark a
concept mastered. Step 13 may later consume evidence under its own versioned
Mastery Engine policy.

## Deterministic adaptive behavior

Items retain source order. A branch requirement references an earlier item and
a minimum score. If the requirement fails, that dependent branch and its
transitive descendants are skipped. A positive answer continues into the
branch so it can be validated by additional questions.

After two objective observations for the same concept, confidence reaches
`1.0`; later items for that concept are treated as redundant and skipped. These
rules are pure domain behavior and are reproducible after restart from the
persisted scored observations.

## Attempts, privacy, and evidence

An attempt is `in_progress`, `completed`, or `skipped`. Starting the same
diagnostic for the same learner and curriculum instance resumes the existing
attempt. Skipping is permitted only before any answer. Terminal attempts are
immutable.

Raw answer text is evaluated at the application boundary and is not persisted.
The durable observation records only item ID, concept ID, score, evidence ID,
and timestamp. Each submission transactionally:

1. appends immutable `EvidenceDiagnostic` evidence;
2. appends the scored observation that references that evidence;
3. advances or completes the attempt.

The evidence source records diagnostic ID/version, curriculum instance,
attempt, item, and scoring-policy version. This makes provenance explicit while
leaving evidence independent from the current mastery projection.

## Persistence

Forward-only migration v10 adds:

- `diagnostic_attempts`, unique by learner, curriculum instance, and diagnostic
  ID/version;
- `diagnostic_observations`, ordered and append-only, with foreign keys to the
  concept registry and `learning_evidence`.

The attempt foreign key includes both curriculum-instance ID and student ID, so
diagnostic state cannot cross learner boundaries. Upgrade v9 to v10 is additive
and retains all Step 10 data.

## Development fixture and boundaries

`testdata/curricula/foundation-demo/diagnostic.json` is a deterministic,
versioned development fixture associated with `foundation-demo@1.0.0`. The
strict JSON adapter rejects unknown fields and does not compile or enrich the
content.

Real pack questions remain future curricular content. Step 11 does not add a
code runner, generated questions, AI evaluation, full setup integration, or
automatic Student State initialization; the latter flow belongs to Step 12.
