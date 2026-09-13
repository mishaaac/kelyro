# Current vs Historical Guidance v1

`guidance-classifier-v1` turns temporal classifications into an explicit
teaching-intent vocabulary without allowing “how it used to work” to masquerade
as “what should be done now.”

## Guidance mapping

Every classification preserves exact Claim refs and maps deterministically:

| Temporal evidence | Guidance type |
| --- | --- |
| current with an explicit recommendation Claim | `recommended_current` |
| current without a recommendation Claim | `acceptable_current` |
| legacy | `legacy_maintenance` |
| historical | `historical_context` |
| deprecated | `avoid` |
| preview or experimental | `experimental` |

This keeps acceptable behavior distinct from evidence-backed best practice.
Experimental guidance remains separated. Legacy material is available for an
explicit maintenance context, while historical material is available for an
explicit historical context. Deprecated guidance is retained only to explain
what to avoid.

## Missing-current findings

A deprecated target always emits a `CurrentGuidanceFinding`. Legacy or
historical targets emit one when they were not explicitly declared contextual.
Those findings can feed the existing Gap Scanner as `missing_current_guidance`.
Explicit contextual material does not create a false requirement that every
history or maintenance lesson have a current replacement in the same target.

## Evidence and boundaries

Every Guidance classification has one or more Claim refs resolving to usable
I-03-derived evidence. Recommendation is detected only from the structured
`recommendation` Claim kind, never keyword matching.

Targets, evidence, and findings have stable ordering. The classifier performs
no live research, lesson generation, hierarchy construction, UI rendering,
curriculum mutation, or student-state mutation.
