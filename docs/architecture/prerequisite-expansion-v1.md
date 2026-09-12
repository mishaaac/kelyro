# Prerequisite expansion v1

`prerequisite-expansion-v1` expands a set of atomic Concepts with missing
prerequisite Concepts that are available from verified evidence. It consumes
current Concepts and direct edges, semantic prerequisite declarations, an
evidence-backed catalog of available Concepts and accepted I-03 evidence sets.

## Expansion

The pass starts from every current Concept and follows semantic dependencies
recursively. When the required Concept is absent from the current curriculum
but present in the available catalog, it is added with its stable identity and
evidence, the edge is emitted, and that Concept's own dependencies are
expanded. Existing direct edges are preserved. Concepts and Claims are copied
defensively.

An available Concept must be valid, atomic and have verified Claim references.
The pass never synthesizes a Concept from a missing ID or from prose. Its input
has no visual hierarchy, so lesson/module order cannot become a dependency.

## Unresolved gaps

The result distinguishes:

- `missing_root_boundary`: a non-foundational reachable Concept has neither a
  direct prerequisite nor declared prerequisite semantics;
- `prerequisite_concept_unavailable`: semantics require a Concept that is not
  present in the verified catalog;
- `cycle_prevented`: adding a declared edge would create a cycle.

Each gap records the dependent Concept, optional required Concept and kind,
reason, and supporting evidence. Cycle-producing edges are not emitted. Input
cycles are rejected and the final result validates as acyclic; full graph
topology, reachability and critical-path analysis remain Step 16 work.

## Determinism and output

`PrerequisiteExpansionResult` contains added prerequisite Concepts, the full
expanded prerequisite edge set, unresolved gaps, machine-stable reasons and
`prerequisite-expansion-v1`. Concepts, edges, gaps and reasons are sorted by
stable identity. The same inputs produce the same ordering regardless of
catalog or semantic declaration order.
