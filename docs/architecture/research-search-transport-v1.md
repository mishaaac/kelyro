# Research Search Transport v1

Status: implemented by I-03C Step 7.

`brave-search-transport-v1` is the dedicated HTTP boundary for the reference
production `SearchProvider`. It is intentionally narrower than the reusable
Research fetch transport: it can call one search API endpoint and cannot fetch
any URL returned by that API.

## Two separate network boundaries

```text
SearchQuery
  -> SecureHTTPClient
  -> GET https://api.search.brave.com/res/v1/web/search
  -> transient JSON candidates
  -> SearchResult.Locator

SearchResult.Locator selected for ingestion
  -> SourceFetcher + researchhttp.Client
  -> independently privacy-gated and SSRF-checked result URL
  -> bounded source content
```

The first URL is a fixed, trusted service endpoint. A result URL is untrusted
provider data. The search credential is sent only to the fixed endpoint and is
never copied to a result URL, redirect, fetch request, Source, SQLite row,
observer event, or error string.

## Transport invariants

| Concern | v1 policy |
|---|---|
| Endpoint | Exact scheme, origin and path from `EndpointURL`; only `q`, `count` and `offset` are accepted. |
| Method | `GET` without a request body. |
| TLS | HTTPS production endpoint, platform root verification, minimum TLS 1.2 and bounded handshake timeout. |
| Proxy | Environment proxies are not inherited. |
| Redirects | Never followed. A 3xx response is mapped as a provider response error, so `X-Subscription-Token` cannot cross an origin or path boundary. |
| Headers | Caller may provide only `Accept: application/json` and the subscription token. The transport owns `User-Agent` and `Accept-Encoding: identity`. |
| Timeouts | 15 s total, 5 s dial, 5 s TLS handshake and 10 s response headers by default; every configured timeout is positive and at most two minutes. |
| Connection pool | 8 idle connections total and 4 for the only host by default; configuration is bounded. |
| Response headers | At most 64 KiB by default and never more than 256 KiB. |
| Response body | Exactly `application/json`, identity/no content encoding, declared and actual body at most 1 MiB. |
| JSON/results | One JSON value, at most 20 results per page, offsets 0–9 and at most 100 returned candidates. |
| Cancellation | The request context is propagated to DNS/dial/TLS/request/body reads; explicit cancellation is returned unchanged. |
| Status | 401/403 authentication, 429 rate limited, 400/422 invalid request, 408/5xx unavailable, other non-200 response error. |
| Rate limits | The four documented `X-RateLimit-*` values are retained only as opaque metadata bounded to 256 bytes. No automatic retry hides an extra paid API call. |
| Diagnostics | Errors and page observations exclude credential, query, request URL, response body and result URLs. No logger exists inside the adapter or transport. |

`DefaultTransportConfig` is the only production baseline. Callers may tighten
its limits, but invalid or unbounded settings are rejected. `CloseIdleConnections`
is available for lifecycle shutdown.

## Ownership boundaries

- `DiscoveryService` and the privacy policy decide whether a live request may
  run before this transport is invoked.
- Foundation Secrets will resolve the subscription token in I-03C Step 8; the
  transport only holds the in-memory header value for the duration of a call.
- Cost/audit wiring will count each observed page as one provider API call in a
  later step. The transport does not persist counters or response metadata.
- Result locators remain candidates, not Sources, Evidence, Claims, trust
  decisions or authority signals.

Unit and race tests use injected transports and `httptest`; they do not call the
public Internet.
