# Deprecation & Legacy Intelligence v1

Step 21 implements deterministic, evidence-linked conclusions that a practice,
API, or version is `deprecated`, `removed`, `legacy`, `historical_only`, or
`superseded`. The immutable algorithm identifier is
`deprecation-intelligence-v1`.

This step consumes already persisted Claims and Evidence. It does not fetch,
search, parse natural language, classify historical sources, resolve conflicts,
verify claims generally, update release lifecycle state, or mutate curriculum
or learner data.

## Structured input

`DeprecationAssessmentRequest` contains one subject and between 1 and 32
structured signals. Each `DeprecationSignal` carries:

- `explicit_statement` or `strong_inference`;
- one `ClaimID`, `EvidenceID`, and `SourceID`;
- the concluded deprecation status;
- optional introduced, deprecated, and removed version identities;
- an optional replacement.

The signal producer is responsible for interpreting source content as
untrusted data and attaching the structured conclusion to the literal bounded
Evidence. The v1 service does not inspect keywords or guess semantics from a
Claim statement. In particular, there is no valid “absent from docs” signal:
missing text is neither an explicit statement nor evidence that a subject no
longer exists.

All signals in one assessment must use the same signal kind and agree exactly
on status, known version identities, and replacement. Disagreement is rejected
as `invalid_state`; Step 21 does not create or hide a conflict because general
conflict detection belongs to Step 23.

## Evidence admission policy

| Determination | Admission rule | Persisted marker |
| --- | --- | --- |
| Explicit evidence | At least one `explicit_statement` signal backed by a deprecation Claim and its Evidence. | `explicit_evidence` |
| Strong inference | Only `strong_inference` signals, at least two distinct sources, and Claim confidence `>= 0.8` for every signal. | `multi_source_strong_inference` |

For every signal, the service loads the Claim and Evidence and requires:

- Claim type `deprecation`;
- the Claim declares both the selected source and Evidence ID;
- the Evidence belongs to that source;
- neither Claim nor Evidence is dated after the injected assessment clock.

The confidence threshold and distinct-source rule are a narrow v1 admission
policy. They do not produce a `VerificationResult` and are not a substitute for
the general `multi-source-verification-v1` Claim algorithm.

## Output and history

The service sorts source and Evidence identities, derives a stable record ID
from the complete conclusion and assessment time, records `verified_at`, and
appends a `DeprecationRecord`. Replacement remains optional; no placeholder is
invented when a source supplies none.

Records are never updated in place. If an API is first deprecated and later
removed, both records remain available through exact-subject chronological
history. This preserves version-bound historical guidance without implementing
the general historical-source scopes reserved for Step 22.

Deprecation conclusions do not modify `TechnologyRelease` status and never
upgrade, rewrite, or compile curriculum.

## Persistence compatibility

Forward-only migration v31 adds `determination`, `algorithm_version`, and an
exact-subject history index to the v23 deprecation table. Existing rows cannot
be proven to have passed v1, so migration marks them
`legacy_unclassified`/`deprecation-unversioned-legacy`. This legacy pair is
readable but cannot be written through the new repository.

The SQLite and deterministic memory adapters enforce append-only identity,
source existence, Evidence existence, Evidence/source ownership, and at least
one Evidence item per declared source. SQLite additionally checks the closed
marker/version combinations and the minimum source count for persisted
multi-source inference.
