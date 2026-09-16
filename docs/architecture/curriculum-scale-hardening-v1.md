# Curriculum scale hardening v1

## Scope

I-04 keeps compiler behavior independent of curriculum cardinality. The scale
gate is a deterministic, offline regression fixture; it is not a recommended
pack size and it does not impose a product maximum.

The fixture in `internal/infra/curriculumscale` contains:

- 10,000 evidence-backed atomic concepts;
- 20,000 acyclic prerequisite edges;
- 1,000 competencies;
- 100 frozen Source Bundles with 10,000 Claims;
- all eight coverage dimensions.

It runs the real compiler passes, serializes a real `learning-pack/v1`,
validates and installs the archive through the application service, and reads
the I-02 progress dashboard for a 10,000-concept curriculum. It performs no
network access and does not mutate learner mastery.

## Complexity controls

- The knowledge graph stores one path length and predecessor per concept and
  reconstructs only selected critical paths. A linear graph therefore uses an
  O(V) path index instead of retaining O(V²) path prefixes.
- Existing prerequisite cycles are detected once with deterministic Kahn
  traversal in O(V+E). Reachability checks remain limited to newly proposed
  expansion edges.
- Prerequisite expansion uses an explicit stack, so a deep prerequisite chain
  does not consume one Go call frame per concept.
- Frozen evidence is indexed once per compilation and shared by every
  atomization candidate.
- Compiler trace hashes stream canonical JSON into SHA-256 while preserving
  the existing `json.Marshal` digest bytes; they no longer retain a second
  complete encoded buffer for every pass.
- Definition-before-use skips the reverse reachability search when the valid
  introduction path has already been found.

Learning Pack loading remains bounded at 1,024 entries, 16 MiB per entry,
32 MiB total uncompressed, and a 100:1 per-entry compression ratio. In-memory
builder validation applies the same bounds as directory and ZIP loading, so a
builder cannot return an artifact that its paired validator rejects solely due
to divergent limits.

## Determinism and regressions

The scale test compiles the same request twice and compares the curriculum,
diagnostics, and output hash. It also builds the pack twice and compares both
the content hash and complete archive bytes. Separate regression tests cover:

- a cycle already present in extracted prerequisites;
- iterative expansion of a 10,000-concept prerequisite chain;
- builder-side enforcement of loader resource bounds;
- roadmap projection over 10,000 concepts;
- streamed compiler hashes remaining byte-compatible with `json.Marshal`.

## Local measurements

Measurements are observations, not portable pass/fail budgets. On Linux
`amd64` with an Intel i7-12650H, one representative run reported:

| Operation | Observed time |
| --- | ---: |
| Full compiler pipeline | 1.15 s |
| Knowledge graph pass, including trace hashing | 80 ms |
| Coverage pass, including trace hashing | 21 ms |
| Definition-before-use audit | 46 ms |
| Zero-assumption audit | 21 ms |
| Final review | 134 ms |
| Pack serialization | 2.04 s |
| Pack validation and install | 1.64 s |
| I-02 roadmap/dashboard read | 20 ms |

The resulting stored ZIP is 10,677,994 bytes; its largest entry is the
9,038,778-byte curriculum YAML, and total uncompressed content is 10,676,694
bytes. A one-iteration benchmark reports allocations with `-benchmem` so
future work can detect allocation regressions without converting a local
machine measurement into a brittle CI threshold.

Run the gate and measurements with:

```bash
go test ./internal/infra/curriculumscale -run TestLargeCurriculumFixtureCompilesDeterministically -v -count=1
go test ./internal/infra/curriculumscale -run '^$' -bench 'BenchmarkLargeCurriculum' -benchtime=1x -benchmem -count=1
go test ./internal/learning/application -run TestProgressDashboardHandlesTenThousandConcepts -v -count=1
```
