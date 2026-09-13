# Curriculum Reviewer v1

`curriculum-reviewer-v1` is the deterministic pre-publication gate for a
compiled curriculum. It consumes `CompilationResult`, its typed diagnostics,
and the exact frozen evidence sets. It does not compile, repair, fetch, publish,
or mutate the curriculum.

## Decisions

The closed result vocabulary is:

- `approved`: no error or warning findings;
- `approved_with_warnings`: at least one warning and no error;
- `rejected`: at least one error.

Every finding has a stable dimension, severity, code, target, and reason. The
review always emits all eleven dimensions in canonical order:

1. coverage;
2. granularity;
3. prerequisites;
4. definition-before-use;
5. zero-assumption;
6. source readiness;
7. freshness;
8. security;
9. production;
10. toolchain;
11. temporal status.

Missing or partial declared coverage is blocking. Non-atomic forced splits,
unreachable Concepts, missing prerequisites, error-level audits, not-ready
sources, unresolved evidence conflicts, stale/unknown evidence, deprecated
guidance, and uncontextualized legacy/historical material are also blocking.
Granularity diagnostics, caveated-ready/aging sources, and explicitly separated
preview/experimental material are warnings.

The reviewer trusts the versioned outputs of the constituent passes. It does
not infer requirements from domain keywords or re-run I-03 trust/freshness
algorithms.

## Optional advisor boundary

`CurriculumReviewAdvisor` is an optional application port for future AI or
human-assistance adapters. It receives the completed deterministic core result
and may return notes only. Notes are sorted and stored separately; they cannot
add/remove findings or change the decision. Advisor absence or failure does not
block the core review, so no AI dependency is required.

The core reviewer has no network, filesystem, UI, pack-publication, or learner
state capability.
