# Research Engine controlled E2E fixture

Step 46 verifies the Research & Source Intelligence pipeline without contacting
the public Internet. The test in `tests/e2e/research_engine_test.go` runs the
production application services and SQLite repositories against one controlled
HTTP server.

## Network boundary

The server is an in-process `httptest.Server`. Each source uses a distinct
`*.fixture.test` hostname so authority organizations remain independent even
though every connection terminates on the same loopback listener.

Production builds continue to reject loopback and private addresses. A
build-tagged constructor in `internal/infra/researchhttp/e2e_fixture.go` exists
only with `-tags=e2e`; it resolves an explicit allowlist of `*.fixture.test`
names to loopback and rejects every other hostname or address. The normal HTTP
client still owns request validation, bounded responses, conditional headers,
retry policy, concurrency, and rate limiting.

No environment proxy, public DNS response, public URL, browser, or live search
provider is involved.

## Endpoint inventory

The fixture serves official documentation, release notes, a version-bound
historical page, a community article, contradictory guidance, an explicit
deprecation notice, a mutable page with ETag/304 behavior, and an endpoint that
returns HTTP 429 before succeeding. The test fails if any endpoint is not
exercised.

## Pipeline and scenarios

The persisted path is:

```text
request → query plan → fake discovery candidates → trust classification
→ live fixture fetch → normalization → Evidence → Claims → verification
→ Source Bundle → SQLite → CLI inspection
```

The same run proves primary-source sufficiency, a production recommendation
that still needs corroboration, an explicit conflict, current versus historical
scope, new-release ingestion, evidence-backed deprecation, fresh and stale
filesystem-cache reads under the production TTL policy, privacy denial before
live adapters, conditional 304 revalidation, source-content change, and durable
drift reporting.

Search output remains candidate data until selected sources are explicitly
registered. Raw fixture pages are never written to SQLite; snapshots retain
metadata and hashes while Evidence retains bounded normalized excerpts.

## CI contract

`go run ./tools/quality e2e` executes the fixture on the Linux, Windows, and
macOS CI matrix. The test uses only Go networking, cross-platform workspace
paths, the Kelyro CLI binary, and the workspace-local SQLite database.

Live-web tests remain outside this contract and are not enabled by Step 46.
