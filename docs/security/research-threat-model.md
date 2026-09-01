# Live Research Discovery Threat Model

Status: reviewed for I-03C Step 44  
Review date: 2026-09-01  
Scope: `research topic` live discovery through the Brave Search adapter and
handoff of untrusted candidates to the existing fetch pipeline.

This model complements `docs/architecture/research-security-hardening-v1.md`.
External search results and fetched content are untrusted data. Provider rank,
title, snippet, URL, and publication hints never establish authority and never
become Evidence by themselves.

## Assets and trust boundaries

Protected assets are the provider credential, Foundation privacy decision,
network and cost budgets, workspace records, terminal/log integrity, and the
identity chain from query to Source Bundle.

The relevant boundaries are:

1. Foundation Secrets to the provider factory.
2. Privacy and durable cost authorization to each provider API call.
3. Fixed search transport to the external provider response.
4. Provider JSON to provider-neutral `SearchResult` candidates.
5. Candidate URLs to the separately privacy-gated hardened fetch transport.
6. Bounded fetched bytes to deterministic normalization and extraction.

## Reviewed threats and controls

| Threat | Control and owner | Verification |
| --- | --- | --- |
| Secret leakage | The API key is read only from Foundation Secrets, held only by the adapter, sent only in `X-Subscription-Token` to the pinned endpoint, omitted from `String`/`GoString`, audit, errors, and observations. Search errors discard provider bodies and transport text. | Credentials, transport-redaction, diagnostic-formatting, and factory tests. |
| Provider redirect abuse | The search client accepts only the fixed HTTPS method/path/query/header shape and disables redirect following. A 3xx is a classified response error, and the credential is not forwarded. Discovered URLs use the independent fetch client, which revalidates every redirect and blocks HTTPS downgrade and non-public targets. | Search redirect/credential test and hardened fetch redirect/SSRF tests. |
| Malformed results | JSON must be bounded, valid, singular, correctly typed, and contain no more results than requested. Every result then passes the neutral contract: bounded valid UTF-8 text without control characters, a bounded HTTP(S) locator without credentials, and valid rank/timestamp metadata. The whole provider response fails closed if any item is invalid. | Brave response matrix and `SearchProvider` contract tests. |
| `javascript:`, `data:`, or `file:` URLs | `SourceLocator` accepts only absolute HTTP(S) locators. The candidate mapper canonicalizes and revalidates before registration; no result URL is executed by a browser. | `SourceLocator`, provider-response, and candidate-mapping tests. |
| Oversized URLs | `SourceLocator` rejects locators above 8 KiB before candidate registration or fetch. Fragments are removed and credential-like query or fragment keys are rejected. | Value-object and discovery contract tests. |
| Query or log injection | Discovery normalizes whitespace; the search boundary rejects invalid UTF-8, control characters, and queries above 8 KiB. Brave applies the stricter 400-rune/50-word request limit. Result title, snippet, and provider fields reject controls and use their persistent 8 KiB/16 KiB/1 KiB limits. Network observations and terminal audit contain counters and stable categories, never query text, result URLs, response bodies, headers, or provider messages. | Search contract, adapter-input, observer, audit-view, and redaction tests. |
| Rate-limit or quota abuse | Queries/run and results/query are configuration-bounded. Each physical provider request requires a durable cost reservation immediately before the call; pagination cannot bypass it. HTTP 429 is terminal for that search attempt, is not retried by the adapter, and exposes only bounded quota headers. | Cost-controlled search, pagination authorization, 429, and orchestration tests. |
| Response or decompression bombs | Search response headers, timeouts, connections, content type, content encoding, declared size, and actual body are bounded. Automatic compression is disabled and only identity responses are accepted; JSON bodies are capped at 1 MiB. Content fetch separately bounds decoded response bytes and normalization output. | Search transport/body tests and hardened fetch decompression/size tests. |

## Fail-closed behavior

An invalid search request performs no provider call. A malformed provider
response yields no partial candidate set from that page. A blocked privacy or
cost decision performs no live call. Failures are mapped to stable error kinds;
raw external values are not copied into terminal audit or ordinary logs.

Search success still produces only candidates. Registration, fetch, snapshot,
normalization, Evidence, Claims, trust, verification, and bundle construction
remain separate stages with their existing validation and provenance checks.

## Residual risks and non-goals

- The external provider observes the submitted query when network use is
  explicitly allowed. Kelyro cannot control the provider's retention policy.
- A public result may later redirect or resolve differently. The fetch
  transport therefore rechecks every live target and redirect; discovery alone
  does not authorize reachability.
- Homograph domains and misleading prose remain possible. Authority comes from
  the reviewed registry and evidence pipeline, never search ranking.
- I-03C does not render pages, execute scripts, crawl recursively, follow page
  instructions, or introduce an AI/LLM boundary.

## Review outcome

The eight required threat classes have explicit fail-closed controls. Step 44
also closes one reproducible gap found during review: provider-neutral search
metadata now applies its persistent byte limits and rejects terminal/log
control characters at the `SearchResult` boundary, before candidate mapping or
registration.
