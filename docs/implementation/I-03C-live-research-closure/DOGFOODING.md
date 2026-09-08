# I-03C Live Research Dogfooding

Date: 2026-09-08  
Branch: `feat/i03c-live-research-closure`  
Tested baseline: `a9afbc2`  
Provider: `brave` / `brave-web-search-v1`

## Environment

The production binary was built from the tested baseline and run against five
fresh Kelyro workspaces. Global configuration selected Brave with the default
four queries and eight results per query. `privacy.allow_network=true`, Doctor
reported the provider configured and the credential available, and the secret
value was never printed, logged, copied into a workspace, or written to this
repository.

The runs used the real CLI composition root and public network:

```text
kelyro research topic "<topic>"
```

The scenarios were isolated so that Sources, cost and audit from one topic did
not affect another topic. A preliminary probe with the over-qualified subject
`Go interfaces official documentation` also failed at Evidence extraction and
is not included in the five-scenario comparison below.

## Results

| Scenario | Topic | Run outcome | Sources | Snapshots | Evidence | Claims | Trust | Verification | Conflicts | Bundles |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Official documentation | `Go interfaces` | `failed / invalid_state` | 24 | 3 | 2 | 0 | 0 | 0 | 0 | 0 |
| Release/version | `Go 1.27 release notes` | `failed / invalid_state` | 15 | 4 | 20 | 0 | 0 | 0 | 0 | 0 |
| Deprecation | `Go io/ioutil deprecated` | `failed / invalid_state` | 18 | 4 | 32 | 0 | 0 | 0 | 0 | 0 |
| Multi-source | `Go context cancellation` | `failed / invalid_state` | 18 | 4 | 0 | 0 | 0 | 0 | 0 | 0 |
| Noisy/irrelevant results | `Go channels` | `failed / invalid_state` | 25 | 4 | 2 | 0 | 0 | 0 | 0 | 0 |

No scenario satisfied the mandatory query-to-bundle acceptance contract. Three
runs stopped with `Evidence contains no unambiguous claim statement`; the
official multi-source scenario stopped with `normalized sources contain no
relevant evidence`. Every run finalized durably as failed rather than reporting
false success or leaving work running.

## Inspection

### Queries and discovery

`query-planner-v1` produced the expected four-query family for every subject:

```text
<topic> official documentation current usage
<topic> API reference
<topic> tutorial
<topic> source code
```

Brave made four provider API calls per run. Result counts after canonical URL
deduplication ranged from 15 to 25 Sources. Search results remained discovery
candidates and were not persisted as Evidence.

Discovery included relevant primary URLs such as `go.dev/blog/go1.27`,
`go.dev/doc/devel/release`, `pkg.go.dev/io/ioutil` and Go source, alongside
community articles, issue trackers, package references and noisy SDK results.
The noisy `Go channels` case visibly retained results about unrelated channel
APIs, so provider rank alone correctly did not become trust or authority.

### Sources and authority

Every newly registered search result was stored as `SourceOther`. No production
stage classified official documentation, release notes, package references,
source code or community material after fetch. Fresh workspaces also contained
no matching Source Registry entries. Since execution failed before the trust
stage, all five runs persisted zero trust decisions; an unclassified
`SourceOther` would otherwise start at Tier E in `trust-policy-v1`.

This prevents the live CLI from satisfying the required authority and trust
portion of query-to-bundle even if Claim extraction is made to admit a literal
statement.

### Evidence and Claims

Four scenarios persisted Evidence with exact Source and snapshot provenance.
Examples observed directly in stored excerpts included:

```text
Today the Go team is pleased to release Go 1.27.
Deprecated: As of Go 1.16, the same functionality is now provided by package io or package os.
```

Despite those literal statements, `claim-extractor-v1` emitted no candidates.
Its topic anchor requires the complete subject phrase or an independently
populated `Topic.Technology`; the CLI stores the entire user input as Subject
and leaves Domain and Technology empty. Natural source prose commonly contains
only part of subjects such as `Go 1.27 release notes` or
`Go io/ioutil deprecated`. The extractor therefore rejects otherwise explicit
release and deprecation statements. In the multi-source case the Evidence
extractor rejected every normalized passage before Claim extraction.

The behavior is conservative and did not invent Claims, but it is not
interoperable with ordinary CLI topic text.

### Verification, conflicts and bundle

No Claims meant that trust evaluation, multi-source verification and bundle
assembly were not reached. Each workspace contained zero verification results,
zero conflicts and zero Source Bundles. The absence of conflicts is therefore
not evidence of agreement; it means conflict analysis had no Claims to assess.

### Cost and audit

Each run durably recorded:

```text
search requests:       4
provider API calls:    4
fetch reservations:    4
reserved fetch bytes:  8 MiB
cache savings:          0
model calls:            0
```

The fetch budget stopped additional network work after four 2 MiB reservations,
even though discovery registered more Sources. Every run contains two sealed
audit checkpoints (`planned`, `failed`) with the exact queries, provider ID,
network mode, network authorization, fetched Source/snapshot provenance,
observed byte count, terminal failure kind and versioned algorithms.

## Readiness regression found before dogfooding

A fresh Linux Secret Service collection returned exit code 1 with empty output
when Kelyro looked up its missing private reference index. The adapter treated
that normal not-found result as keychain unavailability, preventing the first
secret from being stored. Commit `a9afbc2` maps only an empty exit-1 lookup or
clear to `ErrSecretNotFound`, preserves diagnostic backend failures, and adds
Linux regression coverage. The full test and vet suites pass after the fix.

## Closure decision

Step 48 remains incomplete. The live production path can search and fetch the
public Internet when configured, but it cannot currently complete an ordinary
CLI topic as a trusted, verified Source Bundle. Step 49 must not claim that the
original I-03 administrative gap is resolved until a separately authorized,
tested corrective change closes both observed boundaries:

1. deterministic Evidence/Claim extraction must interoperate with normal CLI
   topic subjects without inventing claims; and
2. fetched Sources need an evidence-based, provider-rank-independent
   classification/registry path before trust evaluation.

## Corrective rerun

Date: 2026-09-08

Tested commit: `b3ba758`

The authorized correction introduced versioned natural-topic extractors and a
post-normalization Source classifier. The full offline suite, vet, and E2E suite
passed before five new isolated public-network runs were executed. The final
matrix was:

| Scenario | Sources | Classified fetched Sources | Snapshots | Evidence | Claims | Trust | Verification | Conflicts | Bundles |
| --- | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Official documentation | 24 | 1 community article | 3 | 1 | 0 | 0 | 0 | 0 | 0 |
| Release/version | 14 | 1 release notes, 1 official blog, 2 community articles | 4 | 71 | 7 | 3 | 7 insufficient | 0 | 1 incomplete |
| Deprecation | 18 | 1 source code, 1 issue tracker, 2 community articles | 4 | 14 | 0 | 0 | 0 | 0 | 0 |
| Multi-source | 18 | 1 package reference, 1 official documentation, 2 community articles | 4 | 10 | 1 | 1 | 1 insufficient | 0 | 1 incomplete |
| Noisy/irrelevant | 26 | 3 community articles | 4 | 18 | 0 | 0 | 0 | 0 | 0 |

All five runs again stopped safely as `failed / invalid_state`. Release and
multi-source now traversed the entire pipeline and persisted an `incomplete`
bundle instead of stopping at Evidence/Claim extraction. The multi-source Claim
was the literal package-reference statement:

```text
Package context defines the Context type, which carries deadlines, cancellation signals, and other request-scoped values across API boundaries and between processes.
```

Release extraction preserved literal official statements including:

```text
Today the Go team is pleased to release Go 1.27.
The latest Go release, version 1.27, arrives in August 2026, six months after Go 1.26.
```

The classifier persisted kinds only for successfully normalized content. It did
not use Brave rank/title/snippet, did not upgrade unfetched candidates, and did
not rewrite redirected identities. A real `medium.com` redirect was isolated as
a partial normalization failure rather than being silently relabeled.

The extraction correction is effective but deliberately conservative. Which
four Sources receive the fixed 8 MiB fetch budget varies with concurrent
reservations. In an intermediate clean run where `pkg.go.dev/io/ioutil` was
fetched, the deprecation scenario produced 42 Evidence and 8 literal Claims;
the final matrix fetched Go source plus community pages instead, and correctly
invented no deprecation Claim. Noisy results likewise produced no unsupported
Claim.

### Newly exposed trust/verification boundary

Every evaluated Source in the corrective rerun received
`TrustRequiresVerification`; every resulting verification was
`insufficient_evidence`. A fresh workspace has no applicable reviewed Source
Registry entry, and natural literal statements normally have `status_scope=all`,
which `live-trust-evaluation-v1` maps to unknown stability. The verification
policy counts only `TrustAccepted` or `TrustAcceptedSupplement` as support, so
even classified `release_notes`, `official_blog`, and `package_reference`
Sources contribute zero accepted support and force an incomplete bundle.

This is distinct from the two corrected boundaries. Resolving it requires an
explicitly scoped decision about built-in reviewed registry data and/or how
trust and verification treat a classified primary Source that still requires
verification. The dogfooding step must not silently relax those policies.

Every final run retained the same cost and audit bounds as the first attempt:
four searches, four fetch reservations, 8 MiB reserved, four provider API calls,
zero model calls, and two sealed audit checkpoints. Step 48 therefore remains
blocked and Step 49 remains pending.
