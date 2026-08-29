# Hardened Research HTTP client

Step 08 introduces `internal/infra/researchhttp`, the single reusable HTTP
boundary for future live Research adapters. It is infrastructure, not a source
fetcher: it returns bounded transport output and does not create sources,
snapshots, evidence, cache records, or release facts.

The client does not authorize network access. `DiscoveryService`,
`FetchService`, and `ReleaseLookupService` must complete the Foundation privacy
gate defined in [research-network-privacy.md](research-network-privacy.md)
before an adapter invokes this package.

## Configuration and transport

`DefaultConfig` supplies conservative non-zero limits that callers may narrow:

| Control | Default |
| --- | --- |
| complete per-attempt HTTP timeout | 20 seconds |
| dial timeout | 5 seconds |
| TLS handshake timeout | 5 seconds |
| response-header timeout | 10 seconds |
| idle connection timeout | 60 seconds |
| decoded response body | 4 MiB |
| response headers | 1 MiB |
| redirects | 5 |
| attempts | 3 |
| retry backoff | 100 ms, capped at 2 seconds |

Configuration validation also places hard ceilings on response size,
redirects, attempts, timeouts, connection pools, backoff, User-Agent length,
and content-type patterns. The User-Agent must be a bounded `Kelyro/...`
identifier. An individual request may lower `MaxResponseBytes`, but cannot
raise the configured decoded-body ceiling.

The reusable `http.Transport`:

- disables environment proxies so target resolution cannot escape Kelyro's
  SSRF policy;
- performs context-aware, address-pinned dialing;
- requires TLS 1.2 or newer while retaining Go's default certificate and
  cipher behavior;
- enables HTTP/2 attempts and safe keep-alive pooling;
- retains automatic gzip negotiation/decompression;
- applies the decoded-body limit after decompression;
- rejects unsupported residual content encodings.

`http.Client.Timeout` bounds connection setup, redirects, headers, and body
reading for each attempt. The bounded attempt count and backoff cap bound the
whole retry sequence; request contexts provide independent caller cancellation.

## SSRF policy

Every initial URL and redirect must be absolute HTTP(S), have no user info, and
resolve only to allowed public addresses. The client rejects:

- localhost names and loopback addresses;
- RFC1918/unique-local private addresses;
- link-local, multicast, unspecified, and other non-global addresses;
- common cloud metadata hostnames and metadata addresses;
- malformed or out-of-range ports.

Direct dialing resolves again, validates every returned address, and connects
to the validated IP rather than asking a second resolver implicitly. A mixed
public/private DNS result rejects the entire target. Redirect targets repeat
the same validation before the client follows them.

Research uses direct address-pinned dialing. A future proxy adapter would need
its own explicit trust configuration and must preserve target validation; proxy
environment variables are not an implicit authorization channel.

## Responses, retries, and hooks

Only `2xx` and `304 Not Modified` are successful. Response bodies are bounded
both by declared `Content-Length` and an actual `max+1` decoded read. Successful
bodies require a configured media type except for `204` and `304`.

Retries are limited to idempotent GET attempts and these transient outcomes:

- transport errors implementing a temporary/timeout network contract;
- `408`, `429`, `500`, `502`, `503`, and `504`.

Other `4xx` responses, invalid content, oversize bodies, unsafe redirects, and
SSRF denials are never retried. Exponential backoff and valid `Retry-After`
values are always capped by `MaxBackoff`; cancellation interrupts a pending
backoff.

`RateLimiter.Wait` runs before every attempt with only a normalized hostname.
`Observer.Observe` receives attempt number, status, outcome, and bounded retry
delay. Its event type cannot contain URLs, request/response headers, content,
credentials, or raw transport error strings.

## Sensitive-data boundary

The request rejects `Authorization`, proxy authorization, cookies,
authentication challenges, caller-supplied User-Agent, and caller-supplied
`Accept-Encoding`. Redirect handling strips sensitive headers defensively.

The returned `Response` exposes only status, validated content type, bounded
ETag/Last-Modified values, final locator, and bounded body. It never returns
authentication or cookie headers. Classified error strings contain only a
stable category and optional status code, so URLs, queries, credentials, and
provider error text are not emitted when errors are logged normally.

Step 44 additionally blocks HTTPS-to-HTTP redirect downgrade, removes all
conditional/Range headers across origins, strips fragments and credential-like
query pairs from returned locators, and verifies that text/JSON/XML/PDF bodies
match their declared representation. Full threat coverage is documented in
[research-security-hardening-v1.md](research-security-hardening-v1.md).

## Step 09 consumer

Step 09's `internal/infra/researchfetch` adapter now consumes this transport,
including the request-specific response limit. Conditional headers, snapshot
hashing/persistence, and body disposition are documented in
[source-fetch-snapshots-v1.md](source-fetch-snapshots-v1.md). Cache writes,
HTML/PDF parsing, discovery, and release ingestion remain assigned to later
I-03 steps.
