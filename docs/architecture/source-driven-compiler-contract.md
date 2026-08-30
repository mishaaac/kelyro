# Source-driven Compiler contract v1

## Purpose and boundary

`source-driven-compiler/v1` is the transport-neutral read contract through
which the future I-04 Curriculum Compiler consumes durable I-03 research. It
turns stored Source Bundles, Claims, source roles, conflicts, freshness, and
verification state into an explicit compile-eligibility decision.

The contract does not compile curriculum, generate lessons, choose pedagogy,
run network research, mutate a Source Bundle, or read/write learner state.
I-03 owns evidence truth and eligibility facts; I-04 owns curriculum
criticality, compilation policy, immutable curriculum versions, and any future
student-safe migration proposal.

Consumers MUST reject unsupported contract versions instead of guessing their
meaning.

## Query context and immutable result

Every operation receives the same normalized context:

```text
CompilerResearchQuery
  contract_version
  topic
  target_version optional
  purpose
  as_of
  criticality            critical | non_critical
```

`topic` is the complete validated `ResearchTopic`, not a display-title match.
When `target_version` is present, exact version scope is required for current
guidance; when it is absent, a consumer that needs version-bound guidance must
treat `version_unknown` as unresolved. `as_of` is a UTC evaluation boundary,
not permission to rewrite historical records.

Responses form one immutable compiler view:

```text
CompilerResearchView
  contract_version
  evaluated_at
  query
  bundle_refs[]
  claim_ids[]
  primary_source_ids[]
  conflict_ids[]
  freshness[]
  verification_requirements[]
  eligibility
  reasons[]
  caveats[]
```

A bundle ref contains the complete version identity
`(bundle_id, content_hash, algorithm_version, verified_at)`. All referenced
Claims, Sources, conflicts, freshness records, and verification results must be
those observed while producing the view. Collections are duplicate-free and
deterministically ordered. A consumer MUST fail closed if any referenced
record is missing or inconsistent; it must not combine independently read
"latest" values into a result that never existed.

Step 41 defines this logical API only. A future implementation may expose one
snapshot read or compose the named operations below behind a consistent
transaction, but transport and persistence details are not part of the
contract.

## Required I-04 operations

```text
GetReadyBundles(query) -> bundle refs + eligibility + reasons + caveats
GetClaims(query) -> Claims belonging to the returned bundle refs
GetPrimarySources(query) -> frozen primary Source refs per bundle and Claim
GetConflicts(query) -> latest conflict records represented by those bundles
GetFreshness(query) -> bundle aggregate plus Claim freshness records
RequireVerification(query) -> verification requirements, not a verification result
```

The operations have the following semantics:

- `GetReadyBundles` returns only exact-topic bundles from completed research
  runs and preserves their immutable identity. It may return
  `ready_with_caveats` views, but never labels an `incomplete`, `conflicted`,
  legacy, corrupt, or version-mismatched bundle as ready.
- `GetClaims` returns durable structured Claims and their Evidence identities;
  discovery candidates and search snippets are never Claims.
- `GetPrimarySources` returns sources frozen with `primary` role in each exact
  bundle. Current registry/trust data may add a warning but cannot rewrite the
  historical role or hash.
- `GetConflicts` includes resolved and unresolved conflict records referenced
  by the bundle. Absence from this response means "no represented conflict was
  found", not proof that sources agree globally.
- `GetFreshness` returns the conservative bundle aggregate and the underlying
  Claim records, including algorithm versions, `last_verified_at`, and missing
  freshness explicitly.
- `RequireVerification` derives work still required for compile eligibility.
  It can identify Claims, reason codes, priority, and an existing queue item;
  it MUST NOT claim that verification occurred, perform network I/O, or make a
  `not_ready` view ready. Enqueueing future work remains an explicit application
  action behind the privacy and cost gates.

`not_found`, inconsistent/corrupt data, unsupported version, unavailable
storage, and incomplete research are distinct outcomes. An empty result MUST
not be presented as successful verification.

## Compiler eligibility

Eligibility is closed and is evaluated by `compiler-eligibility-v1`:

| Eligibility | Meaning | I-04 behavior |
| --- | --- | --- |
| `ready_for_compile` | exact, current, verified bundle with no unresolved issue | compilation may proceed under I-04 policy |
| `ready_with_caveats` | usable evidence has bounded, explicit warnings | I-04 must surface and acknowledge every caveat before compiling |
| `not_ready` | evidence or scope cannot support safe compilation | compilation is blocked |

The existing Source Bundle lifecycle maps conservatively:

| Source Bundle state | Baseline compiler eligibility |
| --- | --- |
| `ready` | `ready_for_compile` |
| `ready_with_caveats` | `ready_with_caveats` |
| `incomplete` | `not_ready` |
| `conflicted` | `not_ready` |
| legacy/unsupported/corrupt | `not_ready` |

The mapping can only become more conservative after topic, target-version,
criticality, and current metadata checks. It never upgrades the bundle's
persisted state.

## Required reason codes

Every non-ready result and every caveat has at least one stable reason code,
affected IDs, a bounded explanation, and the algorithm version that produced
it. The minimum v1 vocabulary is:

| Reason | Trigger | Baseline result |
| --- | --- | --- |
| `missing_primary_source` | no qualifying primary source supports a required Claim | `not_ready` |
| `stale` | bundle or required Claim freshness is stale | caveat; `not_ready` for critical content |
| `conflicted` | an unresolved conflict or conflicted verification affects a required Claim | `not_ready` |
| `insufficient_corroboration` | verification is absent, insufficient, rejected, legacy, or below its explicit corroboration rule | `not_ready` |
| `version_unknown` | version-bound guidance lacks an exact target or the bundle does not prove that target | `not_ready` |

Existing Source Bundle issue codes remain available as detail. For example,
missing freshness is not silently converted to `stale`: it remains unknown and
blocks eligibility through `insufficient_corroboration` with the original
`missing_freshness` issue attached. Aging, resolved-conflict, historical-source,
and verified-with-caveat warnings normally produce `ready_with_caveats` unless
I-04 applies a stricter documented policy.

A source is primary only according to the frozen Source Bundle v1 role policy:
accepted trust decision, tier A/B, eligible primary kind, and compatible
temporal/version scope. Popularity, provider rank, domain resemblance, or a
current registry entry cannot fill `missing_primary_source`.

## Critical-content gate

I-04 declares criticality because I-03 does not know the future curriculum's
pedagogical or safety impact. Criticality is an input to eligibility, never a
trust shortcut.

I-04 MUST NOT silently compile a critical concept from `not_ready`. It also
MUST NOT downgrade criticality, drop the affected Claim, substitute a search
candidate, use an older bundle without an explicit version-bound decision, or
reinterpret a missing record as approval.

For critical content:

- `stale`, unknown freshness, unknown target version, unresolved conflict, no
  primary source, and insufficient corroboration all produce `not_ready`;
- `ready_with_caveats` requires an explicit reviewed acceptance naming the
  caveats, exact bundle ref, actor/policy, and time before compilation;
- a reviewed override may request more conservative handling but cannot turn
  missing evidence or an unresolved conflict into verified evidence.

The compiler must return an explainable blocked result and may call
`RequireVerification`. That call is a request for research work, not authority
to compile while the work remains outstanding.

## Version, freshness, and replacement behavior

An exact target version takes precedence over generic current guidance.
Historical/archived sources may explain prior behavior but cannot satisfy a
current primary-source requirement. A version-bound source for another version
is historical relative to the query.

Eligibility is a point-in-time view. New freshness, verification, conflict,
trust, release, deprecation, or drift information creates a new evaluation and,
when evidence changes, a new immutable Source Bundle. It never rewrites the
old bundle or makes an earlier compilation appear to have used later facts.

If multiple eligible bundles exist, I-03 returns them in deterministic order;
it does not silently merge them. I-04 must select exact refs and record them in
the future compiled definition's provenance. A newer timestamp alone does not
prove higher authority or semantic correctness.

## Compile audit hand-off

For every compile attempt, including a blocked one, I-04 should retain:

```text
contract_version
compiler_policy_version
query and criticality
evaluated_at
selected bundle refs
eligibility and reason/caveat codes
Claim, Source, conflict, freshness, and verification refs
reviewed caveat acceptance or block outcome
```

This is compiler-side provenance. Step 42 separately records how the Research
Run produced its inputs. Neither record authorizes learner-state changes.

## Explicit non-goals

Step 41 does not implement the API in Go, a new repository/index, a compiler,
Learning Pack generation, curriculum selection, AI synthesis, network
research, automatic re-verification, selective migration, or Student Core
mutation. I-04 must receive its own specification and explicit authorization
before any of those responsibilities are implemented.
