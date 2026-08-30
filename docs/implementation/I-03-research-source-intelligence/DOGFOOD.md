# I-03 Research & Source Intelligence — Dogfooding

Date: 2026-08-29

Status: passed

## Exit result

Eight real technology profiles were reviewed. All eight candidate claims were
supported by bounded evidence at the cited source location. No invented claim,
hidden conflict, privacy bypass, unreproducible snapshot, corrupt cache entry,
historical/current mix-up, or systematic release-state error was observed.

The session did not write public page bodies to SQLite. Live bodies existed
only as bounded transient normalization input and in the workspace-scoped
offline cache; durable snapshots retained fetch metadata and canonical hashes.

## Method

The live pass used the production privacy gate, hardened HTTP client, source
fetch adapter, snapshot capture service, HTML/text normalizer, Atom release
provider, and filesystem cache. Network use was explicitly enabled for the
session. Each fetched body was then read through `ResearchModeOffline` and its
hash compared with the durable snapshot hash.

The one-off harness was removed after use. It was intentionally not added to
the stable live suite because several sources are mutable and their wording is
not an appropriate CI contract. The durable evidence of the session is this
record, while the repeatable product contracts remain covered by:

- `tests/live/research_live_test.go` for opt-in public-network behavior and the
  privacy denial path;
- `tests/e2e/research_engine_test.go` for the persisted evidence pipeline,
  bundles, CLI, offline cache, conflict, release, deprecation, update and drift;
- `internal/tui/research_test.go` for source, freshness, conflict and provenance
  transparency.

No public search provider is configured by I-03. Discovery review therefore
covered the provider-neutral `query-planner-v1` output and manual candidate
selection. Candidates were never treated as evidence before trust, fetch,
normalization and extraction.

## Manual claim-to-source comparison

| Profile | Claim reviewed | Primary evidence and scope | Authority / lifecycle | Result |
| --- | --- | --- | --- | --- |
| Stable concept | RFC 2606 reserves `.test`, `.example`, `.invalid` and `.localhost` for their documented special uses. | [RFC 2606, section 2](https://www.rfc-editor.org/rfc/rfc2606.txt), lines 64–83. | Standard, tier A; old publication date does not make the still-applicable BCP historical guidance. | Supported exactly. |
| API/version behavior | In Go 1.22, `net/http.ServeMux` patterns gained method and wildcard matching, and `Request.PathValue` exposes a matched wildcard segment. | [Go 1.22 release notes](https://go.dev/doc/go1.22), “Enhanced routing patterns”. Snapshot used the official website source at commit `0423504e5e369aa38fc87f551465ba0074b90dda`. | Official release notes, tier B; version-bound to Go 1.22. | Supported exactly; not generalized to earlier releases. |
| Recent release | The official golangci-lint feed exposed `v2.13.2` as the newest stable candidate at verification time. | [golangci-lint releases](https://github.com/golangci/golangci-lint/releases), release `v2.13.2`; parsed from the repository Atom feed. | Official project release notes, tier B; current and highly time-sensitive. | Atom provider returned `2.13.2`, channel `stable`; manual page check agreed. |
| Experimental feature | The `Arenas` GOEXPERIMENT flag makes the `arena` package visible. | [Go experiment flags at the reviewed commit](https://github.com/golang/go/blob/603439a1c6f2d37c7f02e246342847056ed04c21/src/internal/goexperiment/flags.go), `Arenas` field comment. | Official source code, tier B; experimental, so it requires verification and must not be presented as stable guidance. | Supported with the experimental caveat preserved. |
| Deprecated API | Package `io/ioutil` is deprecated as of Go 1.16; new code should prefer corresponding functionality in `io` or `os`. | [io/ioutil package reference](https://pkg.go.dev/io/ioutil) and [reviewed package source](https://github.com/golang/go/blob/603439a1c6f2d37c7f02e246342847056ed04c21/src/io/ioutil/ioutil.go). | Official package/source evidence, tier A for package API; legacy/deprecated. | Supported exactly; replacement stayed function-specific. |
| Security guidance | `govulncheck` reports known vulnerabilities relevant to functions actually reached by the analyzed code, helping prioritize remediation. | [Go security guidance](https://go.dev/doc/security/), “Scan code for vulnerabilities with govulncheck”. | Official security guidance, tier A; current and short-TTL. | Supported; no stronger guarantee such as absence of all vulnerabilities was claimed. |
| Historical behavior | Before Go 1.22, loop variables were created once and updated per iteration; Go 1.22 creates new variables for each iteration. | [Go 1.22 release notes](https://go.dev/doc/go1.22), “Changes to the language” and “References to loop variables”; same pinned website commit as above. | Release notes, tier A for historical behavior; explicit Go 1.22 boundary. | Supported exactly; historical and current semantics remained separate. |
| Community-heavy topic | `golang-standards/project-layout` is a community collection, not an official Go standard; official guidance describes useful layouts such as `internal` and `cmd` contextually. | [Community project-layout README at the reviewed commit](https://github.com/golang-standards/project-layout/blob/a9d6fae7015527b10550ebb6d6b71a71b1ef5ea7/README.md), contrasted with [official module layout guidance](https://go.dev/doc/modules/layout). | Community source tier D and supplementary; official guide tier B and primary. | Supported. The difference is a scope/authority nuance, not a factual conflict. |

Manual comparison result: 8/8 supported, 0 invented, 0 contradicted, and 0
claims with a missing version or lifecycle qualifier.

## Live snapshot inventory

The same inventory was fetched twice during the session. Commit-addressable
GitHub sources and RFC 2606 produced identical bodies by construction; mutable
sources also remained unchanged across the two immediate passes. Every entry
was reproduced from the offline cache with the same canonical hash.

| Profile/source | HTTP bytes | `canonical-content-hash-v1` |
| --- | ---: | --- |
| RFC 2606 | 8,008 | `sha256:b6869c8984701701bc2e6973b6ffc750d497f845cc1a65a106e9301590a13ab0` |
| Go 1.22 release-note Markdown, used by API and historical profiles | 38,493 | `sha256:e968bd94b820bf7b85d97cfac0f06d5dc33ae4861c9979c73c631aa68aabcf42` |
| golangci-lint Atom feed | 159,263 | `sha256:aa860966942b2f5752ab95953a354cf0f6dfd4e6ce9508a170944a6cdb08206b` |
| Go experiment flags | 4,720 | `sha256:5932ecc49c11e13c5a182a29152ee2715ffe49fdd02eb76ff8aefad8ef7a7970` |
| `io/ioutil` package source | 3,362 | `sha256:166c114baec89fd662376a1f08682589e97f595023df450a9f17a42ae419af39` |
| Go security guidance | 32,335 | `sha256:02dc95198cf89d985c7d9c167baf99c610da1b134dd08bf1e15603504fe89829` |
| Community project-layout README | 16,205 | `sha256:521f05a1955ae80c1e4080e5810aa8ec882fe333be7675272ab79fc84a1b32aa` |
| Official Go module-layout guidance | 36,100 | `sha256:e788e8159c6c15fccdbbc4ddd283b46f41f9fc54644e6c8848f03b0fcee32d1c` |

The Atom feed is intentionally mutable: its recorded hash identifies this
observation, while a future different hash is an update/drift input rather than
a reproducibility failure.

## Product-surface review

| Surface | Result | Observation |
| --- | --- | --- |
| Discovery quality | Pass with declared limitation | `query-planner-v1` produced primary-first, purpose-specific intentions for all profiles. Candidate execution was manual because I-03 has no configured public search provider. |
| Authority ranking | Pass | Normative/security/package/historical use cases reached the appropriate A tier; official supporting material stayed B; community material stayed D/supplementary. Experimental state required verification independently of authority. |
| Primary source selection | Pass | Every claim used a normative or official source where one existed. Community material supplemented rather than displaced official guidance. |
| Source snapshots | Pass | Nine successful live captures represented eight profiles. Durable metadata included exact locator, fetch time, representation, byte count and canonical hash. |
| Citations and provenance | Pass | Each reviewed claim has a bounded section or code locator. Citation/provenance services and their CLI/TUI presentation passed their deterministic suites; no discovery snippet was cited. |
| Freshness | Pass | Current release, security, experimental and community observations are short-lived; stable RFC guidance remains applicable; Go 1.22 behavior is version-bound instead of mislabeled current. |
| Conflicts | Pass | No contradiction was found in the real claims. The community/official layout difference remained an authority/scope caveat. The explicit-conflict E2E and TUI paths kept unresolved conflicts visible. |
| Release state | Pass | The production Atom provider parsed `2.13.2` as stable and the official release page marked `v2.13.2` latest at the observation time. |
| Source bundle | Pass | Bounded claims, source identities, snapshot hashes, trust, freshness and temporal caveats satisfied the bundle contract. Persistent assembly and ready-with-caveats behavior passed the controlled E2E. |
| CLI/TUI transparency | Pass | CLI E2E exposed run state, bundle caveats, source identity and snapshot hash. TUI tests exposed authority, freshness, historical scope, conflicts and claim provenance without doing I/O during render. |
| Offline access | Pass | All live fetched representations were cached and read back in offline mode with byte-derived hashes matching their snapshots. The controlled E2E separately covered fresh/stale warnings and zero live calls. |
| Update scan | Pass | Mutable release/guidance sources have snapshot/hash inputs suitable for later scans; the deterministic service suite covered changed source, release, deprecation, stale evidence, conflict, provider and offline-incomplete signals. |

## Findings and disposition

1. The rendered `https://go.dev/doc/go1.22` page exceeded the bounded
   normalizer output and returned `output_limit`. This is the intended safety
   behavior, not silent truncation. The session selected the smaller official
   Markdown representation and pinned it to a website commit; the resulting
   snapshot normalized successfully.
2. Mutable branch URLs are poor deep links for source-code evidence. Go source,
   website source and the community README were therefore pinned to the exact
   commits reviewed. Mutable human-facing pages remain citations, while hashes
   identify the observed representations.
3. A search candidate can have a good title and still be the wrong
   representation. Discovery output must continue to expose candidates, not
   auto-promote them to Evidence.

No code defect required a regression fix. The first finding exercised an
existing bound and its safe fallback; subsequent targeted and full suites
passed without modification.

## Blocker audit

- Network privacy bypass: not observed; denied mode stopped before the adapter.
- Evidence without provenance: not observed in the persisted E2E or TUI trace.
- Invented claims: none in the eight manual comparisons.
- Hidden conflict: none; real scope nuance and fixture contradictions remained
  visible.
- Non-reproducible snapshots: not observed; repeat and offline hashes matched.
- Corruptible source cache: not observed; offline decode and hash checks passed.
- Historical/current mixing: not observed; Go 1.22 boundaries were explicit.
- Systematically wrong release state: not observed; Atom and manual official
  release state agreed.

Paso 48 exit decision: pass. Paso 49 may perform the formal I-03 close when it
is separately authorized. This record does not start I-04.
