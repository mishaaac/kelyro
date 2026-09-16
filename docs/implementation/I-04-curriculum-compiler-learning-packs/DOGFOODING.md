# I-04 Curriculum Compiler & Learning Packs — Dogfooding

Date: 2026-09-16

Status: passed after fixes

Tested commits: `9a8d4a1`, `9166b40`

Artifact: `backend-go-reference@0.2.0`

Curriculum: `curriculum.backend-go-reference@2026.09.16-dev.2`

Checksum: `sha256:3609f07446adfcbbe2f5f24ad97130177fcb272910fec31b22ec642c46c27b23`

## Method

The production CLI was built and exercised with isolated XDG config, data,
cache and state directories plus fresh workspaces under `/tmp`. Network access
remained disabled. The run covered validation, installation, activation,
compiled-artifact inspection, Student Core roadmap, Environment Doctor,
upgrade preview, offline catalog behavior and the 10,000-concept scale fixture.

The reference pack has only nine concepts. Therefore “first 20” and “random
20” both resolve to the complete population; no duplicate or invented sample
was used to make either set appear larger. Every concept, the complete advanced
phase, the security module and the production module were read manually.

## Manual review

| Dimension | Result | Observation |
| --- | --- | --- |
| Goal decomposition | Pass | Five explicit outcomes cover explain, build, debug, operate and maintain; scope excludes production completeness and I-05 runtimes. |
| Competency matrix | Pass | Five competencies across foundations, service construction/verification, security and operations reference all nine concepts. |
| Concept atomicity | Pass | Each concept has one independently definable and assessable boundary; no compound lesson or project was encoded as a concept. |
| Prerequisites | Pass after fix | One foundational root and eight evidence-backed edges form an acyclic reachable DAG. Observability is a hard prerequisite of deployment. |
| Hierarchy | Pass after fix | Four phases, seven modules, seven lessons and nine topics preserve the graph's topological introduction order through `curriculum-hierarchy-builder-v2`. |
| Roadmap | Pass after fix | Initial activation creates an I-02 pack Curriculum Instance; the real CLI shows all nodes, current/locked states and prerequisite explanations. |
| Zero-assumption | Pass | Terminal process execution is explicit and foundational before module/toolchain work. The retained compiler audit has no violation. |
| Definition-before-use | Pass | `module` is introduced before handlers/tests and `handler` before tests/threats/observability. |
| Vocabulary | Pass | The two domain terms have canonical concepts, explicit introduction points and bounded uses. |
| Production coverage | Pass within fixture scope | Operations includes observability and repeatable deployment. The pack remains visibly non-production-complete. |
| Toolchain | Pass | Go toolchain 1.24 metadata is portable across Linux, macOS and Windows with official installation guidance. |
| Security | Pass | Service trust boundary is an advanced, current, evidence-referenced concept; pack security hardening rejects active/untrusted content. |
| Current / experimental / legacy | Pass | Eight concepts are current, GOPATH workspace mode is legacy, and no concept is falsely marked experimental. |
| Source evidence | Pass with mandatory caveat | Every concept references one of nine fixture Claims from one frozen bundle; Evidence Report shows 9/9 primary-reference coverage and explicitly disclaims production completeness. |
| Pack installation | Pass | Validate/install/list/show succeeded and reproduced the committed checksum. |
| Activation | Pass after fix | Activation is idempotent, requires an active goal, creates the Student Core instance and rejects direct version transitions. |
| Upgrade dry-run | Pass | `0.1.0 -> 0.2.0` preserved nine stable concepts, introduced no unknown state, required no review and performed no writes. |
| Environment Doctor | Pass after fix | At the terminal root, Doctor reports Go as `not_needed_yet`, minimum 1.24.0, explains when it becomes necessary and shows the official URL. |
| Performance | Pass | 10,000-concept compiler: ~1.27 s, 872,480,680 B/op; serialization: ~2.10 s, 1,425,290,832 B/op on Linux amd64 i7-12650H. |
| Offline behavior | Pass | `privacy.allow_network=false`; compiler and pack lifecycle completed locally, and catalog returned `Mode: offline cache` without entries. |

## Concept and evidence sample

| Concept | Status | Direct prerequisite | Manual result |
| --- | --- | --- | --- |
| Terminal process execution | current, foundational | none | Explicit zero-assumption root; matches its fixture definition Claim. |
| Go module identity | current | terminal | Defines module identity/toolchain boundary before service work. |
| GOPATH workspace mode | legacy | terminal | Historical context is not presented as current guidance. |
| HTTP handler boundary | current | modules | One protocol/application boundary, not a complete HTTP lesson. |
| Database transaction boundary | current | modules | One atomic commit/rollback boundary. |
| Deterministic service test | current | handlers | Explicitly excludes dependency on the public Internet. |
| Service trust boundary | current | handlers | Security validation/authorization boundary is explicit. |
| Service observability signal | current | handlers | Logs, metrics and traces are scoped to operational evidence. |
| Repeatable service deployment | current | observability | Health checks follow observable signals in both graph and roadmap. |

The comparison validates internal Claim-to-concept traceability for the
development fixture. It does not assert that the broad Go documentation link
independently proves all nine statements, and it does not promote the pack to
production-ready evidence.

## Defects found and disposition

1. Sibling topics were alphabetically ordered, putting “Deployment and
   operations” before its hard prerequisite “Production observability”. A
   regression test now forces the opposite alphabetical case and v2 orders
   hierarchy groups by earliest topological position.
2. `packs activate` only selected a pack artifact; it did not create the I-02
   Curriculum Instance, so `kelyro roadmap` remained empty. Initial activation
   now performs an idempotent application-service hand-off for the active goal.
3. Direct activation of another installed version bypassed the classified,
   backed-up migration workflow. It now fails before Student Core writes;
   only the upgrade executor can authorize the final version transition.
4. Environment Pack diagnostics were implemented but not wired into the real
   CLI composition. Doctor now derives inert requirements from the active pack
   and current I-02 concept while retaining trusted tool detection.

The first attempted patch release was also rejected by the existing SemVer
policy because a visible hierarchy change is minor impact. The artifact was
correctly published as `0.2.0`; the failed `0.1.1` candidate was never committed.

## Gates

```text
go test ./... -count=1
go vet ./...
go test -race ./internal/curriculum/... ./internal/infra/learningmigration ./internal/cli ./cmd/kelyro -count=1
go test -tags=e2e ./tests/e2e -run TestCurriculumCompilerAndPackLifecycleEndToEnd -count=1
git diff --check
```

All passed. No missing foundation, cycle, evidence-less concept,
historical/current mix-up, mastery bypass, nondeterministic artifact,
validation bypass, or serious security/copyright issue remains known from this
dogfooding pass. Step 54 may perform formal I-04 closure; this step does not
start I-05.
