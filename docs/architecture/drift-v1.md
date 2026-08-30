# Evidence Drift Detection v1

Step 38 implements the deterministic policy identified by `drift-v1`. It
compares an immutable old Source Bundle with current structured claims,
snapshots, an optional new Source Bundle, and release observations. It emits
candidate `DriftReport` values; it does not perform network access, infer new
claims from raw content, analyze curriculum impact, or modify learning state.

## Identity and inputs

Claims are matched by stable `ClaimID`, whose domain contract is independent
from wording revisions. The old bundle must have a validated old Claim for
every bundled ID. When a new bundle is supplied, it must describe the same
research topic/purpose and all its Claim IDs must have validated new Claims.

Snapshot and release observations explicitly list affected old Claim IDs.
Snapshot pairs must belong to a source frozen in the old bundle and move
forward in time. A release observation is already-verified structured release
data; discovery candidates and Update Scan signals are not accepted as proof.

## Drift classification

The v1 policy emits these closed types:

| Type | Deterministic condition | Severity | Base confidence |
| --- | --- | --- | --- |
| `source_changed` | comparable content hashes differ | minor | 0.50 |
| `source_changed` | current snapshot is HTTP 404/410 | important | 0.75 |
| `claim_invalidated` | Claim type or normalized statement changes | important | 0.80–0.85 |
| `version_superseded` | Claim version changes or a mapped current release differs | important | 0.95 |
| `recommendation_changed` | recommendation statement changes | important | 0.85 |
| `deprecation_introduced` | a previously non-deprecation Claim becomes deprecation | critical | 0.95 |
| `scope_changed` | normalized scope or maturity/status scope changes | important | 0.90 |

Statement and scope normalization lowercases Unicode text, treats punctuation
and whitespace as token separators, and collapses separators. Thus casing,
spacing, or punctuation alone does not create semantic drift. V1 deliberately
does not claim synonym or paraphrase understanding: any other statement change
is conservatively visible for review rather than silently declared equivalent.

Findings of the same type are grouped with stable sorted Claim/Evidence IDs.
The group takes the strongest severity and the lowest confidence of its
members. Report order is source, invalidation, version, recommendation,
deprecation, then scope. Application IDs are deterministic over the compared
bundle IDs, finding contents, confidence, and detection time.

## Missing current evidence

Missing is not invalidated. If a stable old Claim has no current Claim, or a
snapshot observation has no current snapshot, the Claim appears in
`UnresolvedClaims` and no unsupported drift report is invented. A confirmed
404/410 is different: it is positive stored evidence that the source is gone,
so v1 emits `source_changed` while keeping `NewEvidence` empty when none exists.

## Persistence and compatibility

Detection is pure and does not write. `DriftService.Record` is a separate
explicit append operation, so callers can review a complete detection result
before persistence and cannot leave a partially stored multi-report run.

Forward-only migration v41 adds `confidence` and `algorithm_version` to
`drift_reports`, plus a bundle/type history index. Rows written before Step 38
are read as `drift-unversioned-legacy` with zero, explicitly unknown
confidence. New memory and SQLite writes accept only `drift-v1`; legacy data
remains readable but cannot masquerade as a new v1 conclusion.

## Boundaries

- Source body text and discovery candidates never enter the policy.
- A changed hash reports source drift, not automatic Claim invalidation.
- Update Scan supplies candidates; callers must construct validated structured
  observations before Drift v1 can classify them.
- Step 39 owns impact analysis. Step 38 does not identify lessons/concepts,
  compile curriculum, migrate content, or mutate Student Core/mastery.

## I-03 closure status

Step 49 reconfirmed `drift-v1` as the shipped conservative comparison contract.
A changed representation is not automatically a changed Claim, missing current
evidence is not invalidation, and all curriculum/learner actions remain outside
the policy. The hosted closure matrix passed on Linux, macOS and Windows,
including Linux race coverage.
