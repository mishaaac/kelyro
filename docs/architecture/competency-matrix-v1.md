# Competency matrix v1

## Contract

`competency-matrix-v1` converts a validated `GoalDecomposition`, pack-authored
competency declarations and accepted evidence sets into a deterministic
`CompetencyMatrix`.

Each competency contains:

```text
stable competency ID
stable area ID + display name
one goal outcome ID
expected level
zero or more named depth dimensions
one or more exact evidence refs
future concept refs (optional)
parent competency ID (optional)
```

The expected-level vocabulary is `awareness`, `understand`, `apply`, `analyze`,
`design`, `operate`, and `teach_explain`. A dimension has a pack-defined stable
ID and its own expected level, so profiles can express multiple axes without
hardcoding subject-specific dimensions in core.

## Builder invariants

- every competency ID is unique and valid;
- every area ID exists in the decomposition and its display name matches;
- every competency outcome exists and is assigned to that area;
- every competency has at least one Source Bundle/Claim ref present in an
  accepted evidence set;
- every decomposed goal outcome is covered by at least one competency;
- every selected competency area contains at least one competency;
- optional parent IDs exist, remain within the same area and form no cycle;
- dimension IDs are unique within their competency and levels are closed;
- concept refs may be empty before concept extraction and are preserved when
  supplied, but final curriculum validation still owns concept existence.

The builder preserves declaration order and deep-copies slices/pointers. The
same request produces an equal matrix. Unknown areas/outcomes, missing evidence,
duplicates, incomplete coverage and invalid hierarchy are `invalid_state`
errors with reasons.

## Boundaries

Competencies and dimensions are declared by the pack; v1 does not infer them
from prose, hardcode a technology taxonomy, extract concepts, atomize content,
build lessons, perform research or modify student state. Fixture-only domain
objects may remain unevidenced under the existing explicit fixture policy, but
the production v1 builder never emits an unevidenced competency.
