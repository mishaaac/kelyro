# Concept Atomizer v1

`atomizer-v1` turns one validated concept candidate into one or more atomic
Concepts. It consumes the candidate, its accepted evidence sets,
`atomic-concept-criteria-v1`, and pack/domain-authored atomization hints. It
does not use network access, AI, domain-specific title lists or lesson content.

## Domain hints

Each hint declares a stable Concept ID, title, definition, version, difficulty,
foundational flag, exact candidate Claim references and a complete atomicity
criteria assessment. Hints are evidence-constrained instructions: they may
only reference Claims already attached to the candidate.

An atomic candidate requires exactly one hint. A `needs_split` candidate
requires at least two. Every child hint must itself assess as `atomic`, and all
candidate Claims must map to at least one emitted Concept. A Claim may support
multiple Concepts when the mapping is explicit; this preserves legitimate
overlapping evidence without merging the Concepts.

Candidates assessed as `unknown` or `too_fragmented` are rejected. A broad
candidate without enough evidence-backed hints is also rejected instead of
causing the atomizer to invent a decomposition.

## Output and determinism

`AtomicConceptSet` records:

- Concepts ordered by stable Concept ID;
- split reasons from the candidate atomicity assessment;
- a complete one-to-many Claim-to-Concept mapping ordered by bundle/Claim ID;
- `atomic-concept-criteria-v1` and `atomizer-v1` versions.

Concept status is inherited from the candidate. A version-bound candidate
requires matching Concept versions. Claim references and concept mappings are
sorted, validated bidirectionally and copied defensively. Therefore hint and
Claim input order cannot change IDs or output order.

The atomizer does not derive prerequisites, merge concepts for presentation,
construct the knowledge graph or create I-05 lesson/practice/assessment
runtime behavior.
