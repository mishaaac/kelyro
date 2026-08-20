# Persistent mistake memory

Step 15 introduces a durable memory of recurring learner error patterns. It
does not classify answers automatically, generate exercises, update mastery,
or schedule reviews.

## Aggregate identity and deduplication

A `Mistake` is identified by an opaque ID, while semantic deduplication uses:

```text
(student_id, concept_id, mistake_key)
```

`mistake_key` is supplied by the evaluator through the application service. It
is stable, bounded to 128 bytes, and is never derived from a display title.
The same key may be reused for unrelated concepts because concept identity is
part of its scope.

When a key already exists, a matching category and summary increments the
existing record instead of creating another row. Reusing a key for a different
category or summary is a conflict, preventing silent merging of unrelated
errors.

## Current projection

The aggregate stores:

- stable student and concept ownership;
- generic category and bounded human-readable summary;
- first and latest observed timestamps;
- total occurrence count;
- current `recent`, `reinforced`, or `resolved` status;
- the latest observation source reference;
- a resolution timestamp only while resolved.

The categories are `conceptual`, `syntax`, `procedure`, `misconception`,
`careless`, `tooling`, and `unknown`. They are deliberately subject-neutral;
syntax and tooling do not make the model programming-specific.

Summaries are limited to 500 bytes and source references to 256 bytes. The
contract is intended for concise explanations and traceable identifiers, not
complete submissions, large code blocks, prompts, or exercise payloads.

## Lifecycle and immutable history

Every successful mutation appends one `MistakeEvent`:

```text
observed     initial observation or recurrence
reinforced   the pattern was explicitly reinforced
resolved     the pattern was explicitly resolved
```

Events have independent opaque IDs and are ordered canonically by timestamp
and ID. Aggregate updates and event appends occur in one `UnitOfWork`, so a
partial history cannot be committed.

A recurrence increments `occurrences`, moves `last_seen_at`, updates the latest
source, and returns the status to `recent`. This reopens a resolved mistake by
clearing its current `resolved_at`; the prior resolved event remains in history.
The original `first_seen_at` never moves.

Reinforcement does not count as a new occurrence. A resolved mistake cannot be
marked reinforced until a real recurrence reopens it. Resolution does not
delete or rewrite previous observations.

## Application boundary

`MistakeMemoryService` owns `Record`, `Reinforce`, and `Resolve`. Evaluators and
future AI-assisted classifiers must call this service and cannot write SQLite
directly. The service also provides `List` and `Get`; `Get` returns the current
projection together with its complete immutable history.

The read-only CLI surface is:

```text
kelyro mistakes
kelyro mistakes show <id>
```

It exposes category, concept, count, timestamps, status, source, and history so
the learner can inspect what Kelyro remembers. Mutation commands are not added
in this step because observations currently originate from deterministic test
or evaluator fixtures.

## Persistence and compatibility

Forward-only migration v13 extends the published v4 `mistakes` table rather
than replacing it. It adds the dedupe key, category, first/latest timestamps,
count, status, and source reference; it also creates `mistake_events` and the
lookup indexes.

Legacy rows are preserved as one occurrence with category `unknown`, a
`legacy:<id>` key (or a bounded row key for an exceptionally long legacy ID),
and source `legacy:migration/v13`. Each receives an observed event and, when
applicable, a resolved event. Existing IDs, descriptions, timestamps,
ownership, and foreign keys remain intact; the bounded summary projection is
initialized without rewriting the published description column.
The migration runner renews the configured operation deadline per migration,
so a growing forward-only history does not consume one cumulative timeout.

SQLite enforces unique dedupe identity, enum values, positive counts, bounded
text, timestamp shape, lifecycle consistency, and event ownership. The memory
adapter mirrors the observable repository behavior for deterministic tests.

## Explicit boundaries

Mistake Memory does not emit `Evidence` implicitly. If an evaluator decides an
observation also affects mastery, it must submit Evidence through
`ProgressionService` as a separate explicit action. This avoids duplicating
`mastery-v1`, `progression-v1`, threshold resolution, or graph traversal.

Retention strength, review scheduling, warm-up selection, study sessions,
automatic classifiers, AI providers, and the full Exercise Engine remain in
their later implementation steps.
