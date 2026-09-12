# Curriculum Coverage Engine v1

`coverage-v1` measures curriculum coverage against an immutable Curriculum ID,
Learning Goal, Competency Matrix, Concepts, verified I-03 evidence, atomic
coverage requirements, and explicit coverage supports.

## Multidimensional report

The report always contains these independent dimensions in stable order:

1. competency;
2. concept;
3. evidence;
4. theory;
5. practice contract;
6. production;
7. security;
8. toolchain.

Each atomic requirement is `missing`, `partial`, or `covered`. A dimension is
covered only when every declared requirement is covered, missing when every
declared requirement is missing, and partial otherwise. A dimension with no
declared requirements is missing with `no_requirements_declared`. The engine
does not calculate a global score or percentage, so strength in one dimension
cannot conceal a failure in another.

## Structural measurement

The first three dimensions are measured directly:

- competency coverage checks goal-outcome mappings or a targeted Competency;
- concept coverage checks targeted Concepts and Competency-to-Concept refs;
- evidence coverage checks the relevant goal outcomes, competencies, or
  Concepts for references to Claims present in accepted evidence sets.

Missing target entities are coverage results, not parser failures. References
to unrelated goals/curricula or unavailable evidence are invalid input.

## Explicit supports

Theory, practice, production, security, and toolchain requirements use
`CoverageSupport`: a stable support ID, target requirement, reason, and one or
more Concept, verified Claim, or artifact refs. These supports are an explicit
compiler hand-off, not inference from titles or prose. A later specialized pass
can emit artifact refs after validating its own detailed contract.

Step 19 intentionally does not define the specialized theory, practice,
production, security, or toolchain rules assigned to Steps 23–27. It only
provides their common aggregation and traceability mechanism.

## Determinism and boundaries

Requirements and results are ordered by stable ID; dimensions use a closed
order. Equivalent input ordering produces the same `CoverageReport`. The pass
has no network, persistence, UI, I-05 runtime, or student-state behavior.
