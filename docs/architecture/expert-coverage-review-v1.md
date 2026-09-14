# Expert Coverage Review v1

`expert-coverage-review-v1` checks that a compiled route reaches its declared
professional destination instead of stopping at introductory material. It is
deterministic and consumes only the frozen goal, competency matrix, Concepts,
and compiled coverage report.

## Capability policy

Every professional outcome must have a competency whose declared level can
perform that capability:

- explain requires at least understanding;
- build requires apply, analyze, design, or operate;
- debug and maintain require analyze, design, or operate;
- operate requires the explicit operate level.

The mapping is capability-aware rather than a generic numeric ordering: for
example, design does not silently imply operational capability.

Each competency must also reference at least one Concept deep enough for its
declared level. Awareness maps to introductory, understanding to foundational,
application to intermediate, and analyze/design/operate/explain to advanced.
The policy requires evidence of depth; it does not impose a count or curriculum
size limit.

For goals with a `ProfessionalRole`, production, security, and toolchain
coverage must each be `covered`. Missing or partial status produces a
`missing_production_capability` finding identified by its exact dimension.

## Output and extension boundary

The result contains canonically ordered reviewed outcome/competency IDs and
findings for missing advanced competency, insufficient depth, and missing
production capability. The compiler runs it after Beginner Simulation and the
Curriculum Reviewer treats every finding as blocking in `expert_coverage`.

`ExpertCoverageAdvisor` is an optional future AI or human adapter. It may add
sorted notes after the core decision but cannot alter findings or `Passed`.
The core has no network, AI, UI, runtime, or student-state dependency.
