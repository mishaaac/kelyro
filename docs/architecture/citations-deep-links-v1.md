# Citations and Deep Links v1

Step 15 adds deterministic source citations that take a student or reviewer to
the most specific stable location already known to Research. The implementation
lives in `internal/research/citation`, is identified by `citation-v1`, performs
no I/O, and never guesses an anchor from heading text.

## Citation contract

A generated `Citation` preserves:

- stable source, snapshot, and evidence identities;
- the source title and canonical source locator;
- an optional, more specific `DeepLink` and its selection strategy;
- a required bounded section, heading, path, or line hint;
- the exact snapshot fetch time;
- the source's optional opaque version scope;
- the explicit last-verification time; and
- the immutable `citation-v1` algorithm identifier.

The section hint is required even when a deep link exists and is capped at 2
KiB of valid UTF-8; a deep-link label has the same bound. `last_verified` cannot
predate either the snapshot or the evidence extraction. Relationship validation
also checks the exact source,
snapshot, evidence, title, canonical locator, snapshot date, and version scope.

`snapshot_date`, source publication/update dates, `last_verified`, and future
Freshness outputs are distinct values. Step 15 does not calculate Freshness.

## Strategy selection

`GenerateV1` selects a closed strategy from the validated source kind and
location input:

| Source/location | Strategy | Stable locator |
| --- | --- | --- |
| general page with observed anchor | `url_anchor` | canonical URL plus explicit fragment |
| package reference with observed symbol | `package_symbol` | canonical URL plus explicit symbol fragment |
| specification or standard section | `spec_section` | canonical URL plus explicit section fragment |
| release notes heading | `release_heading` | canonical URL plus explicit release fragment |
| source code | `source_permalink` | same-host file URL pinned to commit and line range |
| no stable deep link | `canonical_fallback` | canonical URL plus section/path hint |

Anchor input must be explicit. The generator validates and encodes it, but does
not slugify a heading because sites use incompatible and changeable anchor
rules. A missing anchor produces the documented fallback rather than a guessed
link.

Source code never accepts an anchor on a mutable branch URL. Its permalink must
name the same host as the canonical source, contain a 7–64 character hexadecimal
commit revision and clean relative file path, and end with the exact
`#Lstart` or `#Lstart-Lend` range declared by the input. The adapter or reviewer
supplies the host-specific URL; v1 verifies the stable components without
hard-coding one source host.

All canonical and deep-link locators use the existing `SourceLocator` contract:
absolute HTTP(S), no credentials, normalized scheme/host, and valid URL syntax.

## Application and persistence

`CitationService.Generate` loads one source/snapshot/evidence chain through
narrow repositories, invokes the pure generator, and appends the result only
after relationship validation. `Get` and `ListForEvidence` expose deterministic
offline reads. The memory adapter uses defensive copies and SQLite uses
forward-only migration v28 plus `CitationRepository`.

Migration v28 adds strategy, bounded section, optional version scope, immutable
algorithm version, and an evidence lookup index to the citation schema. Legacy
rows keep their locator and receive conservative metadata defaults; existing
deep links are classified as generic URL anchors without inventing a more
specific semantic strategy.

## Boundaries

Step 15 adds no network access, anchor discovery, HTML parsing, evidence
extraction, verification algorithm, Freshness calculation, Source Bundle
assembly, CLI/TUI presentation, curriculum compilation, or Student Core
mutation. Discovery metadata remains a candidate and cannot become a citation
without a persisted source, snapshot, and evidence relationship.
