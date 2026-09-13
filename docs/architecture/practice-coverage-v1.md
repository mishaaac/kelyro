# Practice Coverage Contract v1

`practice-coverage-v1` defines what I-05 must make practicable or assessable
without generating exercises, prompts, solutions, scores, or runtime behavior.

## Explicit important Concepts

The compiler input declares important Concept IDs separately. Every important
Concept must have exactly one evidence-backed `PracticeRequirement` linking it
to a competency that already contains that Concept. The competency's expected
level is therefore the compatibility target; importance and target level are
never inferred from names or prose.

A `PracticeExpectation` selects one capability:

`recall`, `recognize`, `apply`, `debug`, `design`, `build`, `compare`, or
`explain`.

An expectation is a pedagogical contract for I-05, not an executable exercise.

## Compatibility policy v1

`practice-compatibility-v1` accepts these exact combinations:

| Expected competency | Compatible expectations |
| --- | --- |
| awareness | recall, recognize |
| understand | compare, explain |
| apply | apply, build |
| analyze | debug, compare |
| design | design, build, compare |
| operate | apply, debug, build |
| teach/explain | explain, compare |

Lower-demand activity does not satisfy a higher target merely because it is
related. A requirement is covered when at least one compatible expectation is
declared. Incompatible expectations remain visible in the diagnostic; they are
not discarded or counted as coverage.

## Coverage bridge and boundaries

Each important Concept produces one `practice_contract` Coverage requirement.
Only compatible expectations produce Coverage supports; their IDs are exposed
as artifact references for eventual I-05 consumption. Requirement and support
evidence must resolve to accepted I-03-derived Claims.

Inputs and outputs are sorted by stable IDs. The pass has no network,
exercise-generation, assessment, execution, scoring, persistence, or learner
mastery behavior.
