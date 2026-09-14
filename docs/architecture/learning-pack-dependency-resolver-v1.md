# Learning Pack Dependency Resolver v1

`pack-dependency-resolver-v1` resolves the required Learning Pack graph from a
fixed root manifest and locally available manifests. It never queries a
catalog, installs files, activates packs, performs network access, or changes
student state.

## Constraints and selection

V1 dependencies are required. Each dependency uses the `learning-pack/v1`
AND-only constraint grammar: exact SemVer values or whitespace-separated `=`,
`<`, `<=`, `>`, and `>=` clauses. Optional dependencies are reserved for a
future schema and are never inferred from missing packs.

The resolver:

1. validates every manifest and constraint;
2. expands transitive dependencies in stable pack-ID order;
3. tries compatible versions in descending SemVer precedence;
4. backtracks when later shared constraints or cycles invalidate a choice;
5. returns the highest deterministic solution it can prove valid.

Build metadata does not affect SemVer precedence. If malformed external state
contains equal-precedence build variants, the complete version text provides a
stable tie-breaker; the resolver still never mutates either artifact.

## Results

A successful result contains exactly one manifest per pack ID and a
dependency-first install order ending in the requested root. A failed result
contains no partial selection and one structured issue:

- `missing`: no version of a required pack is locally available;
- `incompatible`: available versions cannot satisfy all accumulated clauses;
- `cycle`: the selected dependency graph contains a cycle, with a closed path.

These are pack-level relationships. They never create, remove, or reinterpret
Concept prerequisite edges. Installation and activation consume this artifact
in later steps and remain separate side effects.
