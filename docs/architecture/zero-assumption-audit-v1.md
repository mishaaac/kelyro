# Zero-Assumption Audit v1

`zero-assumption-v1` verifies that a curriculum advertised for a declared
learner profile does not silently depend on missing foundations.

This audit is implemented and release-gated as part of the completed I-04
compiler in `v0.3.0-alpha.1`.

## Declared input, never a universal checklist

The audit accepts `zero`, `some_experience`, and `domain_experienced` learner
profiles. An immutable `AssumptionBaseline` binds that selection to an exact
Domain Profile ID/version and contains evidence-backed `FoundationRequirement`
records scoped to explicit competencies. Therefore terminal use, files and
directories, processes, editors, version control, networking, or any other
foundation are checked only when the pack's domain profile and verified Claims
declare them relevant to the goal.

A `zero` baseline cannot contain assumed Concept IDs. The two experienced
profiles may declare exact assumed Concepts; this is visible metadata rather
than an implicit global list. This compiler metadata is learner-neutral and
does not read or modify I-02 student state.

## Audit rule

For every non-assumed foundation and every Concept mapped to its target
competencies, the foundation must:

1. exist in the compiled Concept set;
2. be explicitly marked `Foundational`; and
3. be reachable as a prerequisite of the target Concept in the validated DAG.

Violations distinguish `missing_foundation_concept`, `foundation_not_root`, and
`missing_foundation_prerequisite`. Each finding preserves the requirement,
foundation, competency, target Concept, evidence, severity, and a human-readable
reason. Missing curriculum Concepts outside this relation remain the Coverage
Engine's responsibility.

## Validation and determinism

The baseline must match the exact Domain Profile. Competency areas must belong
to that profile, the supplied Concepts must equal the Knowledge Graph Concept
set, and all profile, foundation, competency, and Concept evidence must resolve
to accepted Claims in the supplied I-03-derived evidence sets.

Requirements and findings are sorted by stable IDs. Equivalent input ordering
therefore produces the same result. The audit performs no research, network,
persistence, hierarchy mutation, I-05 content generation, or student-state
mutation.
