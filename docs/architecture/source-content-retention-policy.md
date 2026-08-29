# External source content retention policy v1

Step 43 defines `source-content-retention-v1`, the copyright, licensing, and
storage boundary for Research. Kelyro is an evidence system, not a mirror of
external websites, repositories, books, or videos.

## Retention matrix

| Data | Durable Research store | Disposable cache | Export by default |
| --- | --- | --- | --- |
| Stable source ID, locator, title, publisher, dates | yes | optional | yes |
| Snapshot locator, fetch metadata, bounded length, content hash | yes | optional | yes |
| Evidence excerpt and location | yes, at most 8 KiB per Evidence | optional | only the minimum cited excerpt |
| Raw response body | never in SQLite or SourceSnapshot | at most 1 MiB and 7 days by default | never |
| Normalized source representation | not as a raw page archive | bounded and 7 days by default | never as a full source body |
| Search snippet | no | bounded and 24 hours by default | no |
| Source Bundle | yes, claims/source IDs and bounded annotations | bounded and 30 days by default | yes |
| Video transcript or media | never | never | never |

Discovery candidates and snippets are not Evidence. A durable claim must point
to separately validated Evidence and provenance. Source Bundle JSON contains
identities, claims, citations through their durable relationships, hashes, and
bounded annotations; it does not embed fetched pages or full excerpts. Any
future human-readable bundle exporter must load only the cited Evidence and
include the smallest excerpt needed to understand each claim.

## Fetch and `no-store`

The hardened HTTP boundary parses all `Cache-Control` field values and carries
an explicit, case-insensitive `no-store` directive on the transient
`FetchedSource`. The raw response may still be hashed, used to append bounded
snapshot metadata, and normalized in memory. It must not become a bounded-body
or normalized-source cache entry.

`SnapshotCapture` reports `CacheSuppressed` when a caller requested
`bounded_cached_body` but the response prohibited storage. The filesystem
offline adapter independently rejects attempts to cache either the fetched body
or a normalized representation. This is defense in depth: callers cannot turn
an explicit response prohibition into offline content accidentally.

Durable source/snapshot metadata and a reviewed minimal Evidence excerpt are
research records, not an HTTP response cache. Teams must still remove an
excerpt when applicable law, source terms, or a rights-holder request requires
it; `no-store` alone does not erase already reviewed durable records.

## Cache bounds and eviction

`research-cache-v1` remains disposable and workspace-local. Default hard
ceilings are 512 records and 64 MiB of decoded payload. `ResearchCacheLimits`
and `researchcachefs.Factory.WithLimits` allow a workspace integration to
choose lower item/byte eviction thresholds. Values above the hard ceilings are
rejected. TTLs and per-record limits remain fixed by cache layer so custom
eviction cannot extend retention silently.

Eviction removes expired records first, then the deterministic oldest record
until both configured thresholds hold. `kelyro research cache clear` removes
only disposable Research cache records; it never removes SourceSnapshots,
Evidence, claims, bundles, or Student Core data.

## Licensing

License metadata is descriptive evidence, never a compatibility judgment.
For reviewed source-code Evidence, `SourceCodeLicense` may preserve a supplied
identifier, human-readable name, and license locator. The fields are optional:
Kelyro must leave them absent when a source or adapter did not provide a
reviewed license. It must never infer a license from repository visibility,
host, filename, language, or popularity, and must never fabricate an SPDX ID.

A license record does not authorize copying an entire file. Source-code
Evidence remains pinned to an immutable commit, clean relative path, bounded
line range, permalink, version scope, and an Evidence excerpt capped at 8 KiB.

## Video and transcript boundary

Video is supplemental navigation metadata only. Kelyro stores the source
locator, provider/channel metadata, duration, language, captions availability,
quality assessment, and citations to bounded Evidence when available. It does
not download video/audio, retain captions, or store/export full transcripts.

## Failure policy

- Oversized bodies/excerpts/cache records fail explicitly; they are not
  truncated into misleading Evidence.
- Missing license metadata remains unknown.
- `no-store` cache rejection is explicit and occurs before any cache layer is
  written.
- External bodies never enter Research audit checkpoints, Source Bundles, or
  workspace exports by default.
- These rules do not grant permission to redistribute source content and do
  not replace legal review for a particular deployment or jurisdiction.
