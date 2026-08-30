# Research Impact Analysis v1

Step 39 implements the deterministic policy identified by
`impact-analysis-v1`. It translates one persisted, versioned `DriftReport` into
an immutable `ImpactReport` describing what may be affected and what a later
consumer should consider doing. It does not read curriculum, compile content,
perform network access, or mutate Student Core.

## Input and identity

`ImpactService.Assess` accepts a persisted `DriftReport` ID, an assessment
timestamp, and optional explicit `ClaimImpactReference` values. The service
loads the drift through `DriftRepository`; legacy/unversioned drift is rejected
because v1 cannot safely derive a current conclusion from unknown semantics.

Each reference maps one affected stable `ClaimID` to zero or more:

- future concept IDs;
- future lesson IDs;
- technology ID and version pairs.

References for Claims outside the drift are invalid. Duplicate mappings for a
Claim are invalid. The policy never guesses relationships from topic text,
technology names, Claim wording, or curriculum state.

The generated report ID is deterministic over the drift, complete normalized
output, and assessment time. `Assess` is pure with respect to persistence;
`Record` is a separate append operation, matching the review-before-write
boundary of Drift v1.

## Affected relationships

The report contains sorted, duplicate-free identities:

| Output | Source |
| --- | --- |
| affected evidence | union of old and new Evidence IDs on the drift |
| affected bundles | old bundle and optional distinct new bundle |
| affected claims | stable Claim IDs carried by the drift |
| future concept/lesson refs | caller-supplied mappings for affected Claims |
| technology version refs | caller-supplied exact technology/version pairs |

The report severity is exactly the drift severity; impact analysis does not
downgrade evidence change.

## Recommended-action policy

Action precedence is closed and deterministic:

| Condition | Action |
| --- | --- |
| informational severity | `no_action` |
| critical severity | `manual_review` |
| non-critical source change | `reverify` |
| non-critical version superseded | `recompile_future` |
| non-critical Claim invalidation, recommendation, deprecation, or scope change | `review_curriculum` |

Actions are recommendations for a future owner. `recompile_future` does not
invoke I-04, and no action authorizes automatic curriculum or student-state
mutation.

## Persistence and compatibility

Forward-only migration v42 adds affected Evidence, future concept, future
lesson, technology-version references, and `algorithm_version` to
`impact_reports`. Rows written before Step 39 load as
`impact-unversioned-legacy` with all newly introduced relationships empty.
New memory and SQLite writes accept only `impact-analysis-v1`; legacy records
remain readable but cannot be written as new conclusions.

## Boundaries

- I-03 owns evidence change and possible downstream impact only.
- Future concept/lesson IDs are opaque contracts, not validated against a
  curriculum repository.
- I-03 does not decide whether content is breaking or non-breaking, migrate a
  curriculum version, or protect/copy learner state; Step 40 defines that
  hand-off contract for I-04.
- No mastery, retention, streak, schedule, diagnostic, mistake, session, or
  curriculum-instance record is read or written.
