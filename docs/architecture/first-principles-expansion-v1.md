# First-Principles Expansion v1

`first-principles-expansion-v1` consumes findings from a validated
`zero-assumption-v1` audit and closes them only with pack-authored,
evidence-backed foundation candidates.

## Expansion contract

Each candidate identifies one baseline requirement, an atomic Concept already
marked `Foundational`, a prerequisite kind, evidence for the Concept, evidence
for the proposed relation, and a reason. Candidate identity must match the
foundation named by the audit finding.

For a missing foundation, the pass adds the candidate as a root and adds an
edge from every affected target Concept to that root. For a foundation already
present, only missing edges are added. Existing Concept records are immutable:
the pass never silently turns a published non-foundational Concept into a root.
Existing edges are retained, exact duplicates are skipped, and any proposed
edge that would close a cycle is rejected.

`ExpandedRootConcepts` and `ExpandedPrerequisites` contain only additions, not
the full input graph. A later compiler orchestration step is responsible for
recompiling the Knowledge Graph from current and expanded values.

## Research-required boundary

Automatic expansion requires both Concept evidence and prerequisite-relation
evidence to resolve to usable Claims in the exact I-03-derived evidence sets.
The pass does not search, fetch, infer Claims from prose, or synthesize missing
evidence.

When a candidate is unavailable, evidence is absent/unavailable, an immutable
Concept conflicts, or a cycle would result, the output contains a structured
`FirstPrinciplesResearchNeed`. This is the explicit “research required” gap for
a future compilation after I-03 has produced new verified evidence.

## Determinism and boundaries

Requirements, roots, edges, target IDs, evidence refs, and research needs have
stable ordering. Equivalent input order produces equivalent output. The pass
does not mutate the original curriculum, a published pack, hierarchy, learner
mastery, or any I-05 runtime contract.
