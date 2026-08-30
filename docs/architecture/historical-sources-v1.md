# Historical Sources v1

Step 22 preserves old sources without presenting them as current guidance. The
domain contract is implemented by `SourceTemporalScope` and the pure
`internal/research/temporal` policy. Its immutable algorithm identifier is
`source-temporal-policy-v1`; it performs no I/O, trust scoring, conflict
resolution, or version ordering.

## Temporal scopes

Temporal scope is independent from source kind, registry status, authority,
trust, and freshness:

| Scope | Meaning |
| --- | --- |
| `current` | Eligible for current guidance, subject to the separate trust and freshness policies. |
| `historical` | Retained as historical context and never silently presented as current guidance. |
| `version_bound` | Applies to one required opaque source version. |
| `archived` | Retained for audit, historical, or version-specific use after normal current use ended. |

`version_bound` requires `Source.Version`. Version identifiers remain opaque and
are compared only for exact equality; v1 does not infer compatibility ranges or
order unrelated version schemes.

Every non-current scope has a deterministic warning. `current` has no temporal
warning. The warning is an applicability guard, not a downgrade of the source's
authority: archived official documentation or old release notes can still be
the primary authority for behavior of the exact version they document.

## Guidance assessment

`AssessV1` combines a validated source, research purpose, and optional target
version and emits one closed role:

| Condition | Role |
| --- | --- |
| Current source with no known version mismatch | `current_guidance` |
| Non-current source, `version_behavior` purpose, and exact source/target version match | `version_authority` |
| Non-current source without that exact historical-version use | `historical_context` |
| Any source with known source/target version mismatch | `not_applicable` |

A role describes temporal applicability only. It does not accept a candidate as
evidence, calculate trust or freshness, decide between conflicting Claims, or
authorize curriculum compilation.

## Citations and bundles

New `citation-v1` records copy the source's scope and version at generation
time, add the deterministic warning, and record
`source-temporal-policy-v1`. Relationship validation rejects a citation whose
scope, version, or warning does not agree with the source used to generate it.
This makes historical applicability visible even if the source is reclassified
later.

`SourceBundleSource` similarly captures source ID, scope, optional version, and
warning at assembly time. A `version_bound` member must exactly match the
bundle's target version. A bundle for `current_usage` that includes any
non-current source cannot be `ready`; it must remain caveated or conflicted.
This preserves the distinction between current and historical material without
implementing the Conflict Resolver reserved for Step 23 or full Source Bundle
assembly reserved for Step 25.

## Persistence and compatibility

Migration v32 adds checked temporal scope to sources and citations, plus the
citation warning and temporal algorithm version. Existing sources and citations
are conservatively readable as `current`; legacy citations carry the explicit
`source-temporal-legacy-current` marker and cannot be appended as new records.
Existing source bundle source items are backfilled as `current`, while claim
items keep a null temporal scope. Triggers require the correct source/claim
shape for future bundle items.

Source reclassification updates only the source's current classification.
Immutable snapshots, evidence, citations, and already captured bundle metadata
are not rewritten. No raw external content or secrets are added by this step.

## Boundaries

Step 22 adds no network access, parsing, source discovery, conflict resolution,
multi-source verification, drift detection, curriculum compilation, or Student
Core mutation. External content remains untrusted data and search candidates
remain distinct from evidence.
