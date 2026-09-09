# I-03C — Formal closure audit

Date: 2026-09-08

Audited implementation: `e76638b`

Architecture documentation: `7018b1f`

Verdict: PASS; eligible for the Step 52 formal closure gates

## Audit basis

The audit reconciles four independent evidence layers:

1. the production composition root in `cmd/kelyro/main.go` and
   `internal/app/research_topic_execution.go`;
2. the application services and infrastructure adapters used by that root;
3. the deterministic unit, integration, and E2E suites; and
4. the final public-network binary dogfooding recorded in `DOGFOODING.md`.

The following current-tree checks passed during this audit:

```text
go test ./... -count=1
go vet ./...
go test -tags=e2e ./tests/e2e
go test ./tests/live -count=1 -v  # all public tests skipped before network
```

The final command confirms the opt-in boundary, not public reachability. Public
reachability and the production CLI pipeline were already exercised on
`e76638b` through five isolated workspaces in the completed Step 48 dogfooding.
Four representative topics produced durable `ready_with_caveats` bundles; the
noisy topic produced no supported Claim and failed safely.

Race and explicit live opt-in reruns belong to the Step 52 gate and were not
advanced by this audit.

## Capability matrix

| Capability | Verdict | Production evidence | Verification evidence |
| --- | --- | --- | --- |
| Production SearchProvider | YES | `internal/infra/researchsearch` implements the Brave adapter, hardened fixed-endpoint transport, Foundation Secrets lookup, readiness probe, and provider factory wired by `cmd/kelyro/main.go`. | Adapter matrix, factory/credential/transport tests, SearchProvider conformance suite, and public dogfooding. |
| URL discovery | YES | The CLI search stage executes the bounded `query-planner-v1` plan through `DiscoveryService`, maps provider results to candidates, canonicalizes/deduplicates them, and records durable Sources/discoveries during registration. | Candidate contract tests, application search-stage tests, E2E query-to-bundle, and 14–27 Sources discovered per public run. |
| HTTP fetch | YES | Registered Sources pass through the existing hardened `researchhttp`/`researchfetch` adapters, privacy gate, cache, cost reservation, and `live-source-fetch-v2` allocation. | Fetch/privacy/cost tests, partial-failure E2E, and 10–12 public snapshots in the representative final runs. |
| Normalization | YES | Snapshot-scoped transient bodies pass through the existing HTML/Markdown/JSON/text normalizer; only successful outputs reach `source-classifier-v1`. | Normalization/classification tests, E2E, and classified public Source kinds independent of provider rank. |
| Evidence extraction | YES | `evidence-extractor-v2` emits bounded literal, snapshot-scoped candidates and persists them idempotently. | Extractor contract/unit tests, E2E, and 39–175 public Evidence records per final scenario. |
| Claim extraction | YES | `claim-extractor-v2` derives only supported closed-family Claims, persists exact Evidence links and citations, and rejects ambiguous prose. | Extractor contract/unit tests, E2E, and public release/deprecation/definition Claims with literal support. |
| Trust/verification | YES | The extraction stage applies the existing trust and temporal/freshness services; the verification stage applies `multi-source-verification-v2` plus diversity without converting provider rank into authority. | Trust/temporal/verification tests and public `verified_with_caveat` results with explicit `authoritative_source_requires_verification` and `organization_unknown` reasons. |
| Source Bundle | YES | `live-bundle-claim-selection-v1` selects only verified, caveated, or conflicted Claims; the existing Source Bundle service persists the bundle and the finalization stage reloads and validates it. | Bundle/finalization/provenance tests, E2E roundtrip, and four durable public `ready_with_caveats` bundles. |
| CLI query-to-bundle | YES | The shipped `kelyro` composition root synchronously consumes the existing queue and executes search through terminal audit/finalization within the two-minute command context. | `TestResearchTopicQueryToBundleEndToEnd`, retry/partial-failure E2E, and the five production-binary dogfooding invocations. |
| Offline mode | YES | Durable Sources, snapshots, Evidence, Claims, audit, and bundles remain readable without live dependencies; the filesystem fetch cache remains available through its explicit boundary. | Network/cache unit tests and E2E workspace reopen with provider/fetcher removed, plus a retry fixture exercising the optional search-cache port with network disabled. |
| Privacy gate | YES | Provider configuration never grants network authority; every live search and fetch call passes through `privacy.NetworkGate`. | Provider/fetch denial tests and `TestResearchTopicPrivacyDisabledEndToEnd`, all asserting zero live calls. |
| Cost control | YES | Search calls, provider API calls, fetch requests, bytes, model calls, and cache savings use the existing durable per-run budget and atomic reservations. | Cost-control tests, terminal audit tests, E2E, and public runs bounded to four searches, twelve fetch reservations, 8,388,600 bytes, and zero model calls. |
| Live query smoke | YES | The real production binary used Brave plus public HTTP fetch in fresh workspaces and traversed the durable CLI pipeline. | Final Step 48 dogfooding: official documentation, release, deprecation, and multi-source topics completed with bundles; noisy results failed conservatively. |

## Acceptance and safety conclusions

- Search results remain candidates. They become usable support only after URL
  validation, Source registration, fetch, snapshot, normalization, literal
  Evidence/Claim extraction, trust, freshness, and verification.
- A completed run requires a durable matching Source Bundle. Queue execution,
  run status, bundle reference, terminal audit, and cost metadata settle through
  the existing transactional finalizer.
- A public query is not guaranteed to produce a bundle. No results, unavailable
  support, unsafe content, insufficient verification, cancellation, or budget
  exhaustion produce a terminal safe failure rather than fabricated evidence.
- Fresh workspaces can legitimately produce `ready_with_caveats`: source
  classification does not invent reviewed organization ownership or silently
  convert `requires_verification` into accepted trust.
- Public smokes remain explicitly opt-in. Ordinary tests and E2E are
  deterministic and do not depend on the public Internet.
- No AI dependency, second queue, crawler, browser, background daemon,
  Curriculum Compiler, or Student Core mutation was introduced.

## Final question

Can Kelyro currently search the public Internet from a user/research request?

**YES, when configured and network is allowed.** Concretely, the reference
provider must be selected, its credential must be available through Foundation
Secrets, and `privacy.allow_network` must permit the live operation. Otherwise
a new production discovery attempt fails closed without making a network
request; previously stored Research artifacts remain available offline.

At the time of this audit, Step 51 passed but did not formally close I-03C by
itself. Step 52 subsequently executed the complete test/race/E2E/live gates,
closed every checklist item, and selected `v0.2.0-alpha.2` as the completion
release.
