# Multi-source verification v1

`multi-source-verification-v1` deterministically decides whether the complete
stored source set of one Claim satisfies the corroboration required by that
Claim type. It reads validated domain records only: source content remains
untrusted data and the policy performs no search, fetch, parsing, or semantic
claim invention.

## Inputs and requirements

The application service loads the Claim, every declared Source, each latest
available Trust Decision, reviewed Source Registry ownership, and append-only
conflict history visible at the verification time. Missing trust or registry
ownership remains explicit; it is never replaced with an inferred tier or
organization.

| Claim type | Requirement | Full verification rule |
| --- | --- | --- |
| `definition`, `requirement` | `normative_primary` | One strong primary source, or two strong independent organizations. |
| `recommendation` | `production_recommendation` | Two strong sources owned by independent organizations. |
| `security` | `security_authority` | At least one accepted tier-A specification, standard, official document, or source-code record. |
| `example` | `community_corroboration` | At least two accepted sources from independent organizations. |
| other Claim types | `general_support` | One strong source; accepted weaker support is retained with a caveat. |

An accepted source has a latest `accepted` or `accepted_as_supplement` Trust
Decision and is not blocked or deprecated by its matched Registry entry. A
strong source additionally has an `accepted` tier-A or tier-B decision.
Specifications, standards, official documentation, package references, and
source code are primary kinds. Security authority intentionally uses the
narrower set in the table.

## Independence and metrics

Organization identity comes only from a reviewed Source Registry match and is
normalized for case and whitespace. Repeated pages, aliases, or mirrors owned
by that same organization count once. An unknown organization counts as zero,
not as an automatically independent source. Registry authors therefore own the
canonical organization name used across entries.

Every result stores these audit metrics:

- total Claim source count;
- independent accepted, scope-consistent organization count;
- tier-A through tier-E plus unknown authority distribution for all sources;
- whether all sources are consistent with the Claim's temporal/version scope.

The authority counts must total the source count. The policy evaluates every
declared Claim source and sorts identities before producing its immutable
result.

## Temporal scope and conflicts

Current sources are applicable unless their explicit version contradicts the
Claim. Version-bound sources require an exact Claim/source version match.
Historical and archived sources support only historical Claims or an exact
version match. Scope-inconsistent observations remain visible in metrics but
do not provide support; a result that would otherwise be fully verified is
downgraded to `verified_with_caveat`.

Conflict history is grouped by canonical Claim pair and only the latest record
for each pair is effective. Any current unresolved conflict produces
`conflicted`. A resolved conflict won by the other Claim produces `rejected`.
The verifier does not rerun or overwrite `conflict-resolver-v1`.

## Outcomes and confidence

The closed statuses are `verified`, `verified_with_caveat`,
`insufficient_evidence`, `conflicted`, and `rejected`. Versioned reason codes
record the rule that fired, including primary or independent support, missing
security authority, missing corroboration, same/unknown organization,
scope inconsistency, conflict disposition, and rejected sources.

Confidence is the Claim confidence capped by outcome: `0.95` for verified,
`0.75` with caveat, `0.40` for insufficient evidence, `0.30` for conflicted,
and `0.10` for rejected. It is not a truth probability and does not replace
Trust Decisions, freshness, or conflict confidence.

## Application and persistence

`VerificationService.Verify` owns orchestration and appends exactly one result
through `VerificationRepository`; `Get` and `Latest` remain offline reads.
Memory and SQLite adapters require the result source set to equal the stored
Claim source set.

Forward-only SQLite migration v34 adds the requirement, four metrics, reason
codes, and algorithm version. Existing rows remain readable as
`verification-unversioned-legacy` with `legacy_unclassified` markers and zero
metrics; migration does not invent corroboration facts. New records require
`multi-source-verification-v1` and checked JSON/count consistency.

This step does not create Source Bundles, compile curriculum, mutate Student
Core/mastery, discover conflict candidates, or perform network access.

## Source Diversity v1 composition

Step 30 leaves this immutable algorithm and its v34 representation unchanged.
`source-diversity-v1` is an adjacent assessment that extends organization-only
corroboration analysis with reviewed upstream dependency components,
source-kind, perspective, and implementation/reference dimensions. It can
therefore reveal cross-organization mirrors without retroactively changing a
stored Verification Result. Its unique normative-source rule agrees with this
policy's normative-primary requirement. The complete contract is documented in
[source-diversity-v1.md](source-diversity-v1.md).
