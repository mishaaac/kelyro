# Curriculum Compiler Pipeline v1

`curriculum-compiler-v1` is the deterministic application-layer orchestrator
that turns frozen I-03 evidence plus pack-authored domain policy into one
learner-neutral `CurriculumDefinition`. It has no network, filesystem, UI, AI,
or student-state dependency.

## Input boundary

`CurriculumCompileRequest` separates:

- `CompilationInput`: goal, exact Source Bundle IDs/hashes/verification times,
  and request time;
- `CompilationConfig`: the exact compiler version and evidence policy;
- immutable output identity metadata;
- already-ingested `CurriculumEvidenceSet` values;
- pack-authored domain profile, competencies, atomization plans, prerequisite
  semantics, vocabulary declarations, coverage contracts, learner baseline,
  hierarchy hints, and temporal context.

The validation pass requires a one-to-one identity match between frozen Source
Bundle references and the evidence sets. The compiler cannot discover, fetch,
refresh, or otherwise perform live research.

## Pass order

The v1 pipeline records these passes:

1. input validation;
2. goal decomposition;
3. competency matrix;
4. Concept candidate extraction;
5. atomization and granularity guard;
6. prerequisite extraction and recursive expansion;
7. vocabulary and knowledge graph compilation;
8. visible hierarchy building;
9. multidimensional coverage;
10. definition-before-use and zero-assumption audits;
11. temporal and current-vs-historical guidance classification;
12. gap scanning;
13. final structural review;
14. compiled artifact assembly.

Gap scanning runs after temporal classification because the existing scanner
consumes current-guidance findings. This is a data dependency, not a change in
the gap policy. The final structural review validates the complete domain
aggregate; the publication decision belongs to Curriculum Reviewer v1.

## Trace and determinism

Every pass emits its name, algorithm version, SHA-256 input/output hashes,
structured warning/error strings, and elapsed duration. A failed pass is
included in the partial result before the causal application error is returned.
Durations are observational and excluded from content hashes.

Stable ordering in each constituent pass plus content-only hashing means the
same request/configuration produces the same pass hashes and compiled artifact.
The compiler does not apply concept/module/lesson limits and does not modify a
published pack or learner mastery.

`CompilationDiagnostics` retains the graph, hierarchy, coverage, gaps, audits,
and temporal/guidance artifacts needed by the deterministic publication review.
