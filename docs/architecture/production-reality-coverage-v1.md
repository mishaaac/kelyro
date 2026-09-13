# Production Reality Coverage v1

`production-coverage-v1` prevents a curriculum from treating tutorial-level
knowledge as sufficient evidence of professional operational readiness.

## Domain-declared categories

The available vocabulary is `failure_modes`, `performance`, `observability`,
`deployment`, `configuration`, `maintenance`, `debugging`, `reliability`,
`tradeoffs`, and `operational_concerns`. A domain declares only the categories
relevant to its goal as atomic `ProductionRequirement` records. Core does not
force every category onto every subject and does not infer categories from
Concept titles or prose.

Each requirement targets an existing curriculum, goal, competency, or Concept.
`ProductionSupport` connects it to explicit curriculum Concepts and exact Claim
references. One adequate support covers the atomic requirement; absent or
inadequate support leaves it missing.

## Production evidence policy v1

`production-evidence-v1` requires every referenced Claim to come from a usable
I-03-derived evidence set and at least one `primary` or `supporting` source with
`current` or `version_bound` authority. Claims must express requirements,
behavior, version changes, deprecations, recommendations, warnings,
compatibility, or security. Definition-only, example-only, historical-source,
legacy, historical, and deprecated evidence does not count as current
production coverage.

Known but inadequate evidence is retained as a rejected support diagnostic
rather than becoming a validation error or silently counting as coverage.
Unknown Claim references remain invalid input.

## Coverage bridge and boundaries

Production requirements and only their adequate supports are emitted directly
to the generic Coverage Engine under the `production` dimension. Categories
remain independent, and no aggregate percentage lets one compensate for
another.

Ordering is stable by IDs and the policy is versioned. The pass performs no
live research, deployment, environment inspection, hierarchy mutation, lesson
generation, or student-state mutation.
