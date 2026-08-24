# Source normalization pipeline v1

Step 10 introduces `internal/infra/researchnormalize`, the deterministic
`SourceNormalizer` adapter for bounded fetched documents. It transforms
untrusted HTML, plain text, JSON, and direct Markdown into application-owned
`NormalizedSource` values without executing, rendering, persisting, or treating
source content as instructions.

The immutable algorithm identifier is `source-normalization-v1`. Normalization
is derived data: the Step 09 snapshot and its canonical fetched-content hash
remain the historical source of truth.

## Output contract

`NormalizedSource` preserves:

- stable source ID, fetched locator, media type, and normalization version;
- title, declared/fallback canonical HTTP(S) locator, and normalized language;
- ordered headings with level and explicit ancestor path;
- bounded, whitespace-normalized text segments;
- bounded code blocks with optional language metadata and preserved line
  structure;
- canonicalized HTTP(S) links resolved against the fetched locator;
- optional published/updated UTC timestamps;
- ordered, deduplicated version hints.

The contract rejects invalid heading paths/levels, invalid locators or
timestamps, reversed publication chronology, empty researchable output, and
missing algorithm version. Per-record and collection limits prevent a bounded
input from amplifying into unbounded derived output:

| Output | Limit |
| --- | ---: |
| headings | 512 |
| text segments | 4,096 |
| code blocks | 256 |
| links | 2,048 |
| version hints | 64 |
| heading/text segment | 16 KiB |
| code block | 128 KiB |

Long prose is split at UTF-8-safe word boundaries. Oversized structural output
fails explicitly as `output_limit`; it is not silently persisted.

## Format behavior

### HTML and XHTML

The HTML extractor is a bounded, non-rendering lexical pipeline. It handles
quoted attributes, comments, entity decoding, common block boundaries,
headings, `<title>`, metadata, canonical links, anchors, `<time>`, and
`<pre>/<code>`. Script, style, noscript, template, SVG, canvas, navigation,
footer, form, and aside subtrees are removed using exact closing-tag matching.

This is deliberately not a browser DOM, HTML5 renderer, or reusable output
sanitizer. It never emits HTML and no trust decision is based on markup shape.
`golang.org/x/net/html` was evaluated from its
[official package documentation](https://pkg.go.dev/golang.org/x/net/html), but
no dependency was added: the reviewed
[v0.58.0 module](https://raw.githubusercontent.com/golang/net/v0.58.0/go.mod)
declares Go 1.25 while Kelyro's compatibility remains Go 1.24. The extraction
scanner keeps this step dependency-free and its smaller semantics are tested
against adversarial tag-prefix input.

### Plain text

CRLF/CR is normalized to LF, blank lines delimit segments, and prose whitespace
is collapsed. No markup or structure is inferred.

### JSON

The standard JSON decoder uses exact `json.Number` values and rejects trailing
values. Object keys are traversed in lexical order, arrays in source order, and
scalar leaves become JSON-pointer-like text segments. Recognized top-level
metadata fields populate title, language, canonical locator, dates, and version;
URL-shaped fields become links and explicit code/source/snippet fields become
code blocks. Traversal depth is capped at 64.

### Direct Markdown

The parser recognizes simple scalar front matter, ATX headings, fenced code
blocks, paragraphs, and inline links. Raw executable/noise HTML blocks are
discarded and remaining inline HTML contributes text only. Unclosed code fences
are invalid documents. Step 10 does not attempt a full CommonMark renderer.

## Canonicalization and metadata

Relative links resolve against the final fetched locator. Only validated
HTTP(S) locators without credentials survive; `javascript:`, `data:`, malformed,
and other unsupported schemes are ignored. Declared canonical locators also
drop fragments. Without a declaration, the final fetched locator is the
canonical fallback.

Dates accept RFC3339/RFC3339Nano, common HTTP date layouts, and ISO calendar
dates (UTC midnight). Language labels are lowercased and `_` becomes `-`.
Explicit version metadata takes precedence in ordering; conservative text
matching recognizes `version N.N` and `vN.N` forms. No release status or trust
meaning is inferred from a version hint.

## Error and security boundary

Stable adapter errors distinguish `unsupported_content_type`,
`invalid_document`, and `output_limit`; context cancellation remains directly
categorizable. Inputs must be valid UTF-8, body-bearing `2xx` fetches with
matching length and Step 09 canonical SHA-256 metadata. `204`/`304`, PDF, XML,
and unsupported media types are not normalized by v1.

Golden fixtures cover all four formats. Additional tests cover script/style
removal, exact discarded-tag closure, unsafe links, malformed JSON, invalid
UTF-8, bodyless responses, cancellation, output limits, hierarchy, dates, code,
and deterministic ordering.

This step does not implement discovery, evidence/claims, metadata persistence,
PDF parsing, cache writes, trust decisions, release ingestion, curriculum
compilation, or Student Core mutation.
