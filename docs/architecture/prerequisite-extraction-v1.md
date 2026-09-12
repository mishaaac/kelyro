# Prerequisite extraction v1

`prerequisite-extractor-v1` derives direct prerequisite edges between atomic
Concepts from evidence-backed, domain-authored semantic dependencies.

Each `ConceptPrerequisiteSemantic` identifies the dependent Concept, the
required Concept, one closed prerequisite kind, exact Claim references and a
human-readable reason. The supported kinds are `hard`, `recommended`,
`exposure_only`, `tool_dependency` and `vocabulary`.

The extractor validates that:

- both Concepts exist and are atomic;
- the dependency is not a self-reference;
- every Claim reference belongs to an accepted I-03 evidence set;
- the semantic has evidence and a reason;
- exact concept/required/kind edges are unique.

The result contains ordered `PrerequisiteDerivation` records and
`prerequisite-extractor-v1`. Each record retains the normal domain
`Prerequisite` and its derivation reason. An `Edges` projection supplies
defensive copies for later compiler passes.

Inputs are sorted by dependent Concept ID, required Concept ID, kind and reason,
and Claim references are sorted by bundle/Claim ID. Visual phases, modules,
lessons, topics and their ordering are intentionally absent from the request,
so presentation order cannot become an implicit prerequisite.

This pass derives only edges whose two Concepts already exist. Discovery or
insertion of missing prerequisite Concepts belongs to Prerequisite Expansion
v1; full graph analysis belongs to the Knowledge Graph Compiler.
