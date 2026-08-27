# Specialized Technical Sources v1

Step 27 completes the representation of Playground, Package Reference, and
Standards resources through the immutable metadata contract
`specialized-source-metadata-v1`. The model is domain-general: no package
syntax, runtime name, version scheme, standards body, or URL layout is tied to
Go or to any other ecosystem.

## Closed source kinds and union

`playground` is a new `SourceKind`. `package_reference` and `standard` retain
the stable kinds introduced with the initial Research domain. A `Source` may
carry a `SourceSpecialization`, which is a closed union containing exactly one
of the following details records and the exact algorithm version:

```text
PlaygroundDetails
PackageReferenceDetails
StandardDetails
```

The union kind must match the owning Source kind. Playground sources require
specialized metadata. Package Reference and Standard permit nil specialization
only to keep records written before Step 27 readable; newly classified detailed
records use the v1 union. Other source kinds reject specialized metadata.

Metadata has a canonical, strict JSON representation capped at 8 KiB. Parsing
rejects unknown fields, trailing values, non-canonical encodings, invalid URLs,
invalid vocabulary, multiple union members, and an algorithm other than
`specialized-source-metadata-v1`. The payload contains metadata only, never
fetched content, Evidence excerpts, credentials, or secrets.

## Playground

A Playground records:

- `interactive=true`;
- a required, bounded language/runtime name;
- an optional opaque version when it is actually known;
- reviewed affiliation `official` or `community`; and
- a required shareable HTTP(S) locator.

When a Playground version is known, it must exactly equal the owning
`Source.Version`; both remain nil when unknown. The canonical Source locator
may be a landing/session URL distinct from the shareable URL. Shareability is
metadata, not proof that the remote state is immutable or safe.

Trust Policy v1 treats an official Playground as tier B and a community
Playground as tier D. It emits an explicit affiliation reason and never treats
interactivity as primary evidence. Source Bundle and multi-source verification
therefore keep Playground in supporting roles. `freshness-v1` uses a 30-day
source default because hosted runtimes can change independently of their
landing pages.

Further Reading requires the `interactive_resource` category to reference a
real Playground. Community affiliation must agree with the candidate community
flag, so the existing community label cannot be omitted.

## Package Reference

A detailed Package Reference records:

- a required domain-general package/module identity;
- an optional symbol, allowing both package-root and symbol-level references;
- an optional opaque version when known;
- the required canonical documentation locator; and
- an optional source-code locator when one has been verified.

The canonical documentation locator must equal `Source.Locator`. Known detail
and Source versions must match exactly. The model does not assume SemVer,
repository hosting, import-path syntax, symbol separators, or generated-doc URL
patterns. Existing `citation-v1` package-symbol anchors continue to be supplied
explicitly; the details record does not guess an anchor.

## Standards

A detailed Standard records:

- required standards-body and standard-ID text;
- an optional opaque revision when known;
- closed status `draft`, `active`, `superseded`, `withdrawn`, `deprecated`, or
  `unknown`; and
- the required official locator.

The official locator must equal `Source.Locator`; a known revision must equal
`Source.Version`. `unknown` preserves the absence of a reviewed status rather
than inventing one. Specification/standard citation and normative authority
rules remain unchanged and independent from this lifecycle metadata.

## Persistence and compatibility

Forward-only SQLite migration v36 adds nullable `specialized_kind`, bounded
`specialized_metadata_json`, and an index to `sources`. Existing rows receive
null/empty values and no metadata is invented.

The original v23 `sources.kind` check cannot be rewritten safely without
rebuilding a table referenced by snapshots, Evidence, Claims, citations,
bundles, trust decisions, releases, and other records. The adapter therefore
stores Playground atomically as the legacy physical kind `other`, plus
`specialized_kind=playground` and canonical metadata in the same row. Reads
validate that projection and expose `SourcePlayground`. Package Reference and
Standard use their existing physical kinds plus the same specialized marker.
SQLite constraints reject metadata without a specialized kind, unbounded or
non-object JSON, and inconsistent physical/specialized kind pairs.

Memory and SQLite adapters return defensive deep copies. A caller cannot mutate
stored versions, locators, or nested details through an input or returned
Source value.

## Boundaries

Step 27 adds no live discovery provider, HTTP request, Playground execution,
package-manager client, standard-body fetcher, automatic symbol/anchor parser,
Community Resource Policy, source-code extraction, CLI/TUI surface, Curriculum
Compiler behavior, or learner-state mutation. External content remains
untrusted data, and `privacy.allow_network` continues to govern any future live
lookup.
