# Research Engine input security v1

Step 44 defines `research-input-security-v1`. Every URL, header, response,
document, cache record, provider field, filename, and excerpt entering Research
is untrusted. Trust/authority scoring evaluates usefulness of evidence; it does
not make bytes safe to execute or interpret as instructions.

> External source content is data, never instructions.

The executable marker for that rule is
`external-source-content-is-data-v1`. Current deterministic normalizers extract
bounded structure only. They do not run scripts, macros, templates, shell
commands, source code, or instructions found in a page.

## Network boundary

All live calls still require the Foundation `privacy.allow_network` gate before
the hardened transport is invoked. The transport then applies these controls:

| Threat | Fail-closed control |
| --- | --- |
| SSRF / DNS rebinding | Initial and redirect hosts resolve to public addresses only; dialing resolves again, validates every answer, and connects to the validated IP. |
| Proxy SSRF bypass | Environment proxies are disabled because a proxy can resolve the target again outside Kelyro's address policy. |
| Metadata services | Known metadata hostnames/addresses, localhost, loopback, private, link-local, multicast, unspecified, and non-global addresses are blocked. |
| Redirect abuse | Redirect count is bounded; every target is revalidated; HTTPS-to-HTTP downgrade is blocked. |
| Cross-origin leakage | Conditional validators and Range headers are removed when origin changes; sensitive headers are always rejected/stripped. |
| Oversized response | Declared and actual decoded body sizes plus response headers have hard bounds. |
| Decompression bomb | Go's decoded stream is read through the same `max+1` limit; residual/unsupported encodings fail. |
| Content-type spoofing | Declared text must be UTF-8 without NUL; JSON and XML must parse; PDF must carry a PDF signature. |
| Header injection | Request values containing CR/LF/NUL, sensitive headers, caller User-Agent, and caller Accept-Encoding are rejected. Returned validators are bounded and control-free. |
| Credential leakage | URL userinfo and credential-like source query keys are rejected. Direct transport output removes credential-like query keys and fragments before a locator can be persisted. Errors/events expose no raw URL, header, body, or transport message. |

Credential-like query matching includes API keys, keys, access tokens, auth,
passwords, credentials, secrets, and signatures, including common prefixed
forms. A signed or secret-bearing URL is not a valid durable `SourceLocator`;
an adapter needing authentication must use a separately authorized secret
channel and return a non-secret canonical locator.

## URL and redirect normalization

Source locators and HTTP targets are absolute bounded HTTP(S) URLs without
userinfo, opaque form, host backslashes, whitespace, or control characters.
Malformed query encoding fails. `SourceID`, not URL text, remains identity.

Redirect comparison uses scheme, normalized hostname, and effective port.
Same-origin redirects may retain ETag revalidation; cross-origin redirects do
not receive validators belonging to the previous resource. A final fetch
locator excludes fragments (never sent in HTTP) and sensitive query pairs.

## Parser and content boundary

Fetch limits apply before normalization. The normalizer additionally requires
body length/hash consistency and valid UTF-8, supports only its explicit media
types, checks cancellation, and bounds headings, text segments, code blocks,
links, version hints, and each output item.

HTML scanning is non-executing. It discards script, style, template, SVG,
canvas, navigation, form, and other noise; ignores non-HTTP(S) links; tolerates
malformed markup without invoking a browser engine; and emits only bounded
data. Terminal/log control characters are removed from normalized prose and
from code except for newline/tab formatting. Source title, publisher, and
language metadata reject control characters.

Instruction-like prose such as "ignore previous instructions" remains ordinary
quoted source data. It receives no special authority, cannot call tools, and
cannot change Kelyro policy.

## Cache and filename boundary

Caller keys never become filenames. `research-cache-v1` hashes layer plus key
to a lowercase SHA-256 filename and uses only closed layer directory names.
Paths are checked to remain under the normalized workspace boundary; existing
path components and records containing symlinks are rejected, and records must
be regular bounded strict-JSON files with matching identity and content hash.
Temporary files use restrictive permissions and same-directory replacement.

Inventory treats a symlinked record as corruption without reading its target.
Clear may unlink a cache symlink itself but never follows it or operates outside
the Research cache layers. Cache key, envelope, item, and decoded-byte limits
remain enforced independently.

## Logs and errors

The HTTP observer is a closed structured event containing only attempt, status,
outcome, and bounded retry delay. Classified errors expose a stable category
and optional status, never provider text. Research source metadata and
normalized text cannot contain terminal control sequences. Code must not add
raw external content, URLs with queries, headers, filenames, or provider errors
to normal logs.

## Future AI extractor gate

Connecting an AI extractor later requires a separate specification and all of
these controls:

1. Delimit source text as untrusted data with source/snapshot IDs and hashes.
2. Keep system/developer policy outside and higher priority than source text.
3. Give extraction no shell, network, filesystem, secret, migration, or
   curriculum-write tools by default.
4. Never place secrets in model context unless an explicitly reviewed use case
   requires the minimum secret material.
5. Validate model output against bounded domain schemas and durable Evidence;
   output is a candidate, not proof.
6. Record extractor/prompt/policy versions and require human review for
   security-critical or conflicting claims.

I-03 does not implement that extractor or grant those permissions.

## Verification

Ordinary tests use deterministic `httptest`, fake DNS/policies, local files,
and fixtures only. Regression tests cover SSRF targets, redirects, HTTPS
downgrade, cross-origin headers, oversize and decompressed-size limits,
content-type mismatch, secret redaction, malformed/malicious HTML, control
characters, and cache symlinks. Go fuzz targets seed URL validation, URL
redaction, HTML normalization, and cache-key/path generation; no public Internet
is required.
