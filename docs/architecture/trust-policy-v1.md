# Trust Policy v1

`internal/research/trust.PolicyV1` is Kelyro's first deterministic decision
policy for whether one registered source is appropriate to sustain knowledge in
a stated context. Its immutable version identifier is `trust-policy-v1`.

The policy is pure domain logic. It performs no discovery, network access,
fetching, persistence, authority-profile selection, external lookup,
verification, or conflict resolution. Callers supply validated facts; the
policy returns a validated `research.TrustDecision` with a state, contextual
authority tier, ordered reason codes, policy version, and evaluation time.

## Input dimensions

Authority is calculated from source kind plus use case. The remaining five
dimensions are explicit input and are never collapsed into a numeric score:

| Dimension | Values | Meaning |
| --- | --- | --- |
| authority | A–E | Contextual ability of the source kind to speak for the claim. |
| freshness | fresh, aging, stale, unknown | Current verification state, independent from authority. |
| relevance | exact, strong, partial, unknown, unrelated | Fit to the requested topic. |
| directness | primary, supporting, indirect, unknown | Distance between the source and the asserted fact. |
| stability | stable, preview, experimental, legacy, unknown | Durability of the guidance. |
| corroboration | independent, single_source, none, conflicted, unknown | Independent support or explicit disagreement. |

The input also includes the validated `Source`, research topic and purpose, one
use case, an optional already-matched `SourceRegistryEntry`, an optional
validated `community-resource-policy-v1` assessment, and the UTC evaluation
timestamp. Supported use cases are `general`,
`language_specification`, `security_advisory`, `package_api`, and
`historical_behavior`.

## Contextual authority

The base authority order is:

- A: specifications and standards;
- B: official documentation, release notes, official blogs, package
  references, official tutorials, and source code;
- C: issue trackers, papers, and book references;
- D: community articles/forums and videos;
- E: unclassified or other sources.

Step 27 extends this mapping for the new Playground kind using its reviewed
specialized affiliation: official Playground is B and community Playground is
D. The policy emits `authority.playground_official` or
`authority.playground_community`. A Playground remains supporting material;
interactivity never promotes it to normative or primary evidence.

Use cases refine that order:

- language specifications rank specifications/standards A and official
  documentation/reference/code/tutorial material B;
- security advisories rank official vendor/normative advisories A and direct
  package reference/source evidence B, but acceptance also requires independent
  corroboration;
- package API research ranks official package references and source code A,
  ahead of official documentation/specifications/tutorials at B;
- historical behavior ranks release notes A, ahead of current official
  documentation and source code at B; strong historical secondary material may
  be C.

These rules classify kinds before optional registry context. A registry hint
may make the resulting tier more conservative but never elevate the baseline.
`blocked` rejects; `conditional` and `deprecated` require verification; and
`historical` requires verification outside the historical use case. Registry
matching itself lives in `internal/research/registry`.

Step 28 adds an explicit community override before registry context. A
Community Resource assessment supplies tier D by default or at most tier C
when a matching Authority Profile recognizes both kind and organization/domain.
This prevents a `repository_example` from inheriting the generic source-code
tier B. Comments and stale/unknown community assessments force verification;
even a recognized community resource remains supplementary.

## Metadata requirement

Every accepted source needs publisher metadata. Time-sensitive use cases and
purposes additionally need either `published_at` or `updated_at`. Missing
metadata never silently lowers authority: the tier remains contextual, while
the decision becomes `requires_verification`.

## Decision precedence

Policy v1 applies this explicit order:

1. Reject a matched registry entry with `blocked` status.
2. Reject unrelated or tier-E sources.
3. Reject community sources when partial/unknown relevance, indirect/unknown
   directness, and absent/unknown corroboration combine into low quality.
4. Require verification for incomplete metadata; stale/unknown freshness;
   partial/unknown relevance; unknown directness; preview, experimental, or
   unknown stability; non-historical legacy material; absent, unknown, or
   conflicted corroboration; and uncorroborated community-only evidence.
5. Security guidance additionally requires tier A/B, primary/supporting
   directness, and independent corroboration.
6. Otherwise, tier C/D or non-primary sources are accepted only as supplements.
7. Remaining tier A/B primary evidence is accepted.

An `aging` source is not automatically stale. A `single_source` normative
source may be accepted outside security guidance. A conflict always produces
`requires_verification`; contextual historical precedence is recorded but does
not hide or resolve the conflict.

## Reason codes

Every decision emits one reason per dimension, one metadata reason, optional
contextual reasons, and one terminal decision reason. Stable code families are:

```text
authority.tier_a ... authority.tier_e
freshness.<value>
relevance.<value>
directness.<value>
stability.<value>
corroboration.<value>
metadata.complete | metadata.incomplete
authority.historical_primary
authority.playground_official | authority.playground_community
community.supplementary | community.recognized_supplementary | community.context_only
security.independent_corroboration_required
registry.<status>
registry.authority_hint
decision.accepted
decision.accepted_as_supplement
decision.requires_verification
decision.rejected_low_quality
decision.rejected_registry_blocked
```

Reason ordering is deterministic, and each reason includes a concise detail for
audit and future presentation. The policy never returns a bare `trusted`
boolean.

## Boundaries

Policy v1 evaluates a registered source, not a discovery candidate, and does
not invent missing facts. It does not persist its output; existing
`TrustRegistryRepository.SaveDecision` adapters own persistence. It does not
perform multi-source verification, calculate freshness, resolve conflicts,
score resource quality, fetch sources, or modify curriculum/student state.
