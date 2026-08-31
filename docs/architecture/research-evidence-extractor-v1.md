# Deterministic Evidence Extractor v1

I-03C Step 23 defines `evidence-extractor-v1`, the conservative boundary
between a fetched, normalized document and durable Evidence. It is a pure,
domain-general selector: it performs no network or repository I/O, uses no LLM,
does not assign authority/trust, and does not derive Claims.

The extractor consumes exactly one validated `NormalizedSource`, its matching
immutable `SourceSnapshot`, a `ResearchTopic`, `ResearchPurpose`, and optional
target version. A mismatched Source ID or final locator is invalid input. Search
titles/snippets and provider rank are not inputs and can never become Evidence.

## Output contract

The transient output is `EvidenceCandidate`:

```text
source_id + snapshot_id
kind
location
exact bounded excerpt + canonical hash
optional bounded context_before/context_after
integer score + closed signal set
extractor_version
```

Candidate kinds are `heading`, `passage`, and `structured_metadata`. Locations
are deterministic snapshot-local locators:

```text
heading[0000]
text[0000]
metadata/version[0000]
metadata/published_at
metadata/updated_at
```

The normalizer currently preserves ordered headings and ordered text segments,
but not a heading-to-paragraph edge. V1 therefore never labels a paragraph with
a heading path it cannot prove. A heading locator may include its own validated
path as a human hint, while a passage retains its exact `text[n]` identity.
These locators are suitable for `Evidence.Location`; they are not fabricated
URL fragments or a promise that the remote page still has the same layout.

## Candidate sources

V1 may inspect only normalized, bounded values:

- headings and their explicit paths;
- ordered text segments and adjacent bounded context;
- structured publication/update timestamps and version hints;
- explicit lexical release/version facts;
- explicit lexical deprecation markers;
- passages that overlap the requested topic, technology, domain, or target
  version.

Code blocks and links are excluded from generic v1 extraction. Source-code
Evidence requires the existing reviewed source-code locator contract, while a
link destination proves neither the linked content nor a fact on the current
page.

## Stable scoring and admission

Scoring is integer-only, additive, capped at 100, and independent of discovery
rank and source authority:

| Signal | Weight |
| --- | ---: |
| exact subject phrase | 55 |
| subject term overlap | 12 per distinct term, max 36 |
| document title/heading anchors the subject | 20 |
| exact technology | 15 |
| exact domain | 10 |
| exact target version | 25 |
| heading representation | 5 |
| structured metadata representation | 10 |
| explicit version fact | 20 |
| explicit release fact | 20 |
| explicit deprecation marker | 35 |
| fact matches the request purpose | 20 |

At least one topical anchor is mandatory: subject phrase/terms, technology,
domain, target version, or a document-level subject match. A marker such as
"deprecated" on an unrelated page is therefore not admitted by itself. The
minimum score is 50. Signals are recorded once even if repeated text triggers
them multiple times.

The purpose bonus applies only to release/version facts for
`release_status`/`version_behavior`, and to explicit deprecation markers for
`deprecation_check`. It does not turn ambiguous prose into a fact.

Candidates sort by descending score, then kind, location, and excerpt hash.
Exact duplicate excerpts within a snapshot keep the highest-ranked earliest
candidate. V1 returns at most 24 candidates per source and the live stage may
retain at most 1,000 per run.

## Bounds and literal selection

An excerpt is at most 2 KiB, below the 8 KiB domain ceiling. Context before and
after are independently capped at 512 bytes, below the 2 KiB domain ceilings.
Selection uses UTF-8-safe word boundaries around the admitted signal and must
retain that signal in the excerpt. It never silently truncates a fact into a
different assertion. Metadata observations use a stable field label plus the
canonical normalized value.

No full page, raw body, search snippet, hidden markup, or unbounded section is
persisted. Candidate hashes use `CanonicalEvidenceExcerptHashV1` over the exact
excerpt bytes.

## Persistence and live-stage implementation

I-03C Step 24 implements the selector and converts admitted candidates into immutable
`Evidence` with stable IDs derived from extractor version, source, snapshot,
location, and excerpt hash. It persists Evidence before exposing it to the Claim
stage and makes retries idempotent by reusing byte-identical existing records.
A stable-ID collision with different content is invalid state. A run with
normalized sources but no relevant candidate fails without inventing fallback
Evidence.

`LiveEvidenceExtractionService` consumes only the Source, snapshot, and
`NormalizedSource` artifacts already produced in the run. It validates the
exact Source ID + final-locator chain before extraction and rejects a clock
earlier than the snapshot. Candidate and Evidence collections are copied
defensively into `LiveResearchArtifacts`; raw bodies are never reloaded.

Generic v1 intentionally skips a Source already classified as `source_code`.
That Source kind requires the existing reviewed commit/path/line/permalink
locator, which cannot be reconstructed from generic normalized prose. Live
web candidates are initially registered as `other`, so this guard preserves
the specialized contract without blocking ordinary docs, specifications,
release notes, or community sources.

V1's explicit release/deprecation lexicon is deliberately small and stable.
It recognizes direct English/Spanish markers (for example `released`,
`deprecated`, `no longer supported`, `lanzamiento`, `deprecado`, and
`en desuso`) only when the candidate also has the required topic anchor.
Versions remain opaque: an explicit structured version or target such as
`2026`, `1.22.0`, or an edition label is not forced into SemVer.

The workspace composition root constructs the pure extractor locally and uses
the existing SQLite-backed `EvidenceRepository`; there is no new dependency,
schema, network path, or provider.

Steps 23–24 do not implement Claim extraction, trust/freshness, verification,
bundle assembly, I-04 behavior, or AI review.
