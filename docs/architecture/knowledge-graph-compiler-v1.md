# Knowledge Graph Compiler v1

`knowledge-graph-compiler-v1` compiles atomic Concepts and their typed direct
prerequisites into a deterministic DAG artifact. It has no repository, network,
UI, learner-state, or hierarchy dependency.

## Graph semantics

Stored prerequisite edges retain the domain direction `dependent -> required`.
Topological output uses the learning direction and always places the required
Concept before its dependents. Exact duplicate typed edges, missing Concepts,
self-references, non-atomic Concepts, and cycles are rejected.

The result includes:

- every stable Concept ID and typed prerequisite edge in canonical order;
- deterministic topological order and root Concepts;
- weakly connected components;
- Concepts unreachable from an explicitly foundational root;
- the longest prerequisite path globally and per component.

An isolated Concept is its own component and critical path. A root is any
Concept with no prerequisite. Reachability is intentionally stricter: traversal
starts only at roots explicitly marked `Foundational`. This exposes an
unanchored root or component instead of silently treating it as assumed prior
knowledge. Equal-length critical paths use lexicographic Concept-ID order as a
stable tie-break.

## I-02 consumption projection

The artifact declares `curriculum-consumption/v1` and emits a bounded
prerequisite projection using the exact `introduced` and `mastered` requirement
vocabulary accepted by Student Core:

- `hard` becomes `mastered`;
- `exposure_only`, `tool_dependency`, and `vocabulary` become `introduced`;
- `recommended` remains graph/review metadata and is not converted into a
  blocking Student Core prerequisite;
- when several typed edges share a Concept pair, `mastered` wins.

This step does not build the complete I-02 `Curriculum`: hierarchy and the
remaining pedagogical node metadata belong to later I-04 passes. It also never
creates a learner instance or modifies mastery.

## Scale and determinism

Kahn traversal uses a lexicographic priority queue. Adjacency, component
membership, roots, unreachable IDs, paths, and the consumption projection all
have stable ordering independent of input order. No concept, module, lesson, or
edge limit is imposed. The large fixture compiles a 5,000-Concept chain.
