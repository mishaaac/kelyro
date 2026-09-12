# Granularity Guard v1

`granularity-guard-v1` prevents curriculum structure from being compacted for
presentation convenience. It reviews Concepts, proposed semantic merges and
visual Lesson groupings without changing Concept identities.

## Unbounded structure

The guard has no module, lesson, topic or Concept count setting and no
"roadmap must fit one screen" rule. Runtime and memory use grow with the actual
input. A large curriculum is valid when its concepts are pedagogically needed.

## Atomicity and merge review

Concepts marked `needs_split` are returned as forced splits. Unknown and
too-fragmented Concepts produce structured warnings. Every proposed semantic
merge supplies explicit `atomic-concept-criteria-v1` signals and is allowed only
when the merged result remains atomic, independently explainable, practicable
and assessable. The guard reports merge decisions; it does not perform merges.

## Visual grouping

A `VisualConceptGroup` associates two or more existing stable Concept IDs with
a LessonSpec ID. This is presentation metadata, not a semantic merge: every
Concept remains independently addressable and learner-trackable. A Concept may
appear in at most one safe visual group in one review.

Inputs and outputs are copied and sorted by stable IDs. Results contain
structured warnings, forced-split IDs, safe visual groupings, merge decisions
and `granularity-guard-v1`. No hierarchy order is interpreted as a
prerequisite, and no I-05 lesson runtime is created.
