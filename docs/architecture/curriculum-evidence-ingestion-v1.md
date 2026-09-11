# Curriculum evidence ingestion v1

## Purpose and boundary

`source-bundle-ingestion-v1` converts one exact, durable I-03 Source Bundle
into learner-neutral `CurriculumEvidenceSet` input. The application service
depends only on `ResearchBundleProvider.GetBundle`, `GetClaim` and
`GetConflict`. Those are read-only lookups: discovery, fetch, refresh, release
lookup, queueing and every other network-capable Research operation are absent
from the port.

`internal/infra/curriculumresearch.Provider` is the production composition
bridge: it adapts the existing I-03 bundle, Claim and conflict `Get` services
to that narrow port. Its public surface cannot request live work.

The ingestor does not decompose a goal, extract concepts, compile curriculum,
write a Learning Pack, mutate student state or reinterpret external content as
instructions.

## Output

The evidence set preserves:

- the exact `(bundle_id, content_hash, algorithm_version, verified_at)` tuple;
- structured Claim ID, statement, scope, confidence, source IDs, version scope
  and current/preview/experimental/legacy/historical status;
- bundle target, Claim and source version scopes as a sorted unique set;
- frozen primary/supporting/historical source roles and temporal scopes;
- conservative bundle freshness state, score, oldest verification and
  algorithm;
- complete referenced conflict identities, affected Claims, resolution state,
  reason and algorithm;
- original bundle issue codes as caveats;
- eligibility and the ingestion algorithm version.

No raw web body, search candidate or current mutable trust assessment is added.
Claim and conflict lookups must match the identities and topic frozen in the
bundle. Missing, corrupt or inconsistent durable records are errors rather than
empty evidence.

## Eligibility policy

The mapping is conservative and never upgrades I-03 state:

| I-03 bundle state | Curriculum eligibility | Accepted |
| --- | --- | --- |
| `ready` | `ready_for_compile` | yes |
| `ready_with_caveats` | `ready_with_caveats` | yes, caveats retained |
| `incomplete` | `not_ready` | no |
| `conflicted` | `not_ready` | no |

Callers may identify Claim IDs as critical for the intended curriculum use.
Stale or unknown bundle freshness then changes the result to `not_ready` with
an explicit reason. An unresolved conflict that affects a critical Claim also
adds an explicit blocking reason. A critical ID outside the exact bundle is
invalid input.

Acceptance of a caveated evidence set does not silently acknowledge those
caveats for compilation. A future compiler policy must surface and record its
decision. This step only preserves the eligible I-03 hand-off.

## Determinism and errors

The exact same provider records and request produce equal output. Claims and
conflicts follow the canonical Source Bundle order; source IDs, version scopes
and rejection reasons are sorted deterministically.

Provider `not_found` remains `not_found`; cancellation/deadline remains
`unavailable`; other adapter failures remain external failures. Policy
rejection is an explainable result with `Accepted=false`, `not_ready` evidence
and stable reason text, not a storage error.
