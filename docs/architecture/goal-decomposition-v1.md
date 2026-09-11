# Goal decomposition v1

## Contract

`goal-decomposer-v1` turns a learner-neutral `LearningGoalSpec`, one or more
accepted `CurriculumEvidenceSet` values, and a pack-authored `DomainProfile`
into a deterministic `GoalDecomposition`.

The output contains the goal's evidence-backed outcomes, selected competency
areas, scope, exclusions, profile identity/version and algorithm version. It
does not create concepts, competencies, lessons or learner paths.

## Declarative domain profile

Core contains no catalog such as “backend implies HTTP and databases”. Each
pack supplies a versioned profile with:

- its domain and supported scope vocabulary;
- stable competency-area IDs and display descriptions;
- the goal outcomes each area covers;
- explicit Source Bundle/Claim references;
- optional scope selectors;
- an explicit `required_for_professional` marker.

For a narrow goal, v1 selects non-professional areas whose declared scopes
intersect the goal scope; areas with no selector apply generally. For a goal
with a `ProfessionalRole`, it additionally selects every area explicitly marked
professional-required. Profile order is preserved for reproducible output.

## Safety and failure rules

- goal and profile domains must match exactly;
- every requested scope must be supported by the profile;
- evidence sets must validate and must not be `not_ready`;
- every goal outcome and selected area must cite a Claim present in an exact
  supplied evidence set;
- selected areas may reference only outcomes in the goal;
- every goal outcome must be covered by at least one selected area;
- an empty evidence set, empty selection or unsupported scope is an
  `invalid_state` error with a reason.

`ready_with_caveats` evidence is retained as eligible input; the decomposer
does not erase or acknowledge its caveats. It performs no network I/O and does
not infer domain knowledge from titles, prose, popularity or hardcoded lists.
