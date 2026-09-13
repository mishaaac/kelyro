# Curriculum Hierarchy Builder v1

`curriculum-hierarchy-builder-v1` projects an atomic Concept DAG into the
visible `Phase -> Module -> Lesson -> Topic` hierarchy. The projection is for
navigation and presentation only. It neither copies nor replaces prerequisite
edges; `KnowledgeGraphCompilation` remains the pedagogical source of truth.

## Deterministic grouping

The builder validates that its Concept set exactly matches the compiled graph,
then applies stable, domain-neutral grouping keys:

- Phase: declared Concept difficulty, ordered from introductory to expert.
- Module: competency area within the phase.
- Lesson: primary competency within the area.
- Topic: optional pack-authored practice context, otherwise a neutral concepts
  and mental-models context.
- Concept order: graph topological position, then stable Concept ID.

When a Concept supports multiple competencies, the lexicographically first
area/competency pair is its display placement. A Concept not referenced by the
matrix is preserved under an explicit supporting-concepts group. These choices
affect only UX placement.

Node IDs are derived from complete grouping keys with SHA-256, so reordered
inputs produce byte-equivalent hierarchy values. IDs do not depend on titles or
slice positions.

## Scale and boundaries

The builder imposes no maximum number of phases, modules, lessons, topics, or
Concepts per topic. A 2,000-Concept fixture verifies that no Concept is dropped
or duplicated. Prerequisite edges may cross any hierarchy boundary.

Practice contexts are inert strings supplied by the pack. The builder does not
create lesson prose, exercises, assessments, learner paths, or mastery state.
