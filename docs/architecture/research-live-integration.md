# Controlled live Research integration checks

Step 47 adds a deliberately small public-web smoke suite for the real Research
HTTP, fetch, normalization, snapshot, trust, and privacy boundaries. It is a
diagnostic complement to the deterministic fixture from Step 46, not a CI
source of truth.

## Opt-in boundary

The suite performs no network operation unless the environment variable is
exactly enabled:

```text
KELYRO_LIVE_RESEARCH_TESTS=1
```

Run it explicitly from the repository root:

```sh
KELYRO_LIVE_RESEARCH_TESTS=1 go test ./tests/live -count=1 -v
```

`go test ./...` compiles the suite but skips it before constructing an HTTP
client when the variable is absent. The live environment variable is not set
by the default CI or release workflows, and this suite is not part of the E2E
gate.

## Source inventory

The basic suite uses two stable, unauthenticated primary sources:

- “How to Write Go Code” at `https://go.dev/doc/code`;
- RFC 2606 reserved DNS names from
  `https://www.rfc-editor.org/rfc/rfc2606.txt`.

Both are public official sources, require no paid API, token, account, search
provider, or browser, and exercise independent hosts. Adding a source requires
the same properties and should remain exceptional so the suite stays bounded.

## Assertions and limits

Each source receives a 15-second caller deadline. The HTTP client uses an
8-second request timeout, one attempt, at most three redirects, and a 2 MiB
decoded response limit.

The checks deliberately avoid exact prose, page layout, ETag presence, dates,
or byte-for-byte content expectations. They verify only:

- the source is reachable through the hardened production HTTP/fetch adapters;
- Trust Policy v1 classifies the declared primary source kind as expected;
- HTTP status, content type, bounded content length, and normalized structural
  metadata are present;
- the snapshot contains a canonical hash matching the transient fetched body
  and is readable from the repository;
- `privacy.allow_network=false` blocks before the live fetcher is invoked.

Failures can indicate an adapter regression, a source outage, DNS/TLS policy
change, or an upstream representation change. They must be investigated but do
not block default CI, releases, or the deterministic Step 46 suite by
themselves.

The suite never stores complete public pages in SQLite or fixtures. Its memory
repository retains snapshot metadata and hashes only; fetched bytes exist only
long enough to exercise normalization.

## Live SearchProvider smoke

I-03C adds a separate authenticated smoke for the production Brave Web Search
adapter. It performs no network request unless this exact opt-in is enabled:

```text
KELYRO_LIVE_RESEARCH_SEARCH_TESTS=1
```

The API key remains in Foundation Secrets under
`research.search.brave.api_key`. For environment-only execution, the standard
secret adapter resolves it from:

```text
KELYRO_SECRET_RESEARCH_SEARCH_BRAVE_API_KEY
```

Run only the SearchProvider smoke with:

```sh
KELYRO_LIVE_RESEARCH_SEARCH_TESTS=1 go test ./tests/live -run '^TestLiveResearchSearchProvider$' -count=1 -v
```

The smoke uses the hardened fixed-endpoint transport, Foundation Secrets,
privacy gate, and the real Brave adapter. It submits a bounded query and
requires one to three valid, unique result URLs returned dynamically by the
provider. It does not assert an exact URL, title, ranking, snippet, or result
count, and it never renders the credential.
