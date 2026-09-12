# Definition-before-use Audit v1

`definition-before-use-v1` checks every non-baseline resolved vocabulary use
against the compiled prerequisite DAG and, when available, its Topic/Concept
position inside a lesson. It is deterministic, read-only, and does not infer
vocabulary from lesson prose.

## Ordering rules

A use is valid when any of these conditions holds:

1. the same Concept introduces and uses the term;
2. the introduction Concept is a transitive prerequisite of the use Concept;
3. both Concepts are in the same lesson and the introduction has an earlier
   Topic order or an earlier position within the same Topic.

The graph remains pedagogical truth. If the use is a prerequisite of the
introduction, the audit reports `introduced_after_use` even when visual
placement suggests otherwise. If a valid prerequisite exists but the same
lesson renders the use first, it reports `same_lesson_out_of_order`. Unrelated
Concepts in different lessons produce `missing_vocabulary_prerequisite`.

## Violations

Every violation records its stable code, canonical term, observed spelling,
use Concept, expected introduction Concept, severity, and reason. A safe
suggested prerequisite is emitted only for a missing relationship. It is
omitted when the edge already exists or would create a cycle, where ordering or
graph repair is required instead.

Aliases and acronyms need no special-case policy here because the Vocabulary
Graph has already resolved them while retaining the observed spelling.

## Baseline exceptions

Uses resolved against an explicit `DomainVocabularyBaselineTerm` are counted
and exempted. The audit contains no built-in common-word list. The baseline
scope and reason remain part of the Vocabulary Graph artifact for review.

## Result and boundaries

`DefinitionBeforeUseAuditResult` includes pass/fail, audited-use and baseline
use counts, sorted violations, and `definition-before-use-v1`. Any violation
fails this audit. Optional Topics affect only same-lesson ordering; Concepts
without a hierarchy position must be justified through the prerequisite graph.

This step does not change the graph or hierarchy, generate lessons, run I-05,
or touch student mastery.
