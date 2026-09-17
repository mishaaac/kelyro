# Learning Pack format v1

## Purpose and boundaries

`learning-pack/v1` is Kelyro's portable, immutable curriculum container. It
contains compiled curriculum structure and exact I-03 evidence references. It
does not contain executable lessons, arbitrary scripts, plugins, credentials,
learner state, or live-research instructions.

The format, secure loader, installer, activation, catalog and safe-upgrade
workflows are complete in Kelyro `v0.3.0-alpha.1`.

The same pack ID may have many immutable SemVer versions. `schema_version`
versions this file contract independently from `version`, which versions pack
content. A published `(id, version)` is never changed in place.

## Container

A v1 pack is either a directory or a ZIP archive with this logical root:

```text
pack.yaml
checksums.txt
curriculum/curriculum.yaml
sources/evidence-report.json
build/build-info.json
EVIDENCE.md
assets/licenses.json
assets/...                       # optional Kelyro-authored assets
environment/environment.yaml   # optional
README.md                       # optional
LICENSE                         # optional
```

All names use UTF-8 and canonical `/`-separated relative paths of at most 1,024
bytes. Absolute paths, empty segments, `.`, `..`, backslashes, controls,
Windows-invalid characters/device names, trailing dots/spaces, links, duplicate
entries and paths that escape the logical root are invalid. Implementations
must not execute any pack file. V1 rejects executable, script-like, package,
shared-library, WebAssembly and active SVG entries.

## Manifest

`pack.yaml` is one strict YAML document with these fields:

```yaml
id: go.backend
name: Go Backend
description: A production-oriented backend curriculum.
version: 1.2.0
schema_version: learning-pack/v1
domain: software-engineering
target: Backend Engineer with Go
authors: [Kelyro Curriculum Team]
maintainers: [Kelyro Curriculum Team]
license: CC-BY-4.0
created_at: 2026-09-11T15:00:00Z
minimum_kelyro_version: 0.2.0
dependencies:
  - id: computing.foundations
    constraint: ">=1.0.0 <2.0.0"
environment_pack: environment/environment.yaml
curriculum_entry: curriculum/curriculum.yaml
source_evidence_entry: sources/evidence-report.json
build_info_entry: build/build-info.json
status: current
curriculum_id: curriculum.go-backend
```

IDs use lowercase ASCII components separated by `.`, `_`, or `-`. `version`
and `minimum_kelyro_version` use strict SemVer 2.0 without a leading `v`.
Dependency constraints are a whitespace-separated AND set of exact SemVer
values with optional `=`, `<`, `<=`, `>`, or `>=` comparators. Resolution and
installation are enforced by the I-04 pack application services.

`created_at` is RFC 3339 in UTC `Z`. `authors` and `maintainers` are non-empty.
`license` is a declared license identifier or expression; validation does not
grant rights or infer compatibility. `environment_pack` is optional. The two
other entry paths are required and distinct.

Status uses the closed curriculum temporal vocabulary: `current`, `preview`,
`experimental`, `legacy`, `historical`, or `deprecated`. Old or experimental
material must not be represented as current guidance.

## Checksums

`checksums.txt` contains one line per regular pack file except itself:

```text
<64 lowercase hex SHA-256><two spaces><canonical relative path>
```

Entries are sorted bytewise by path. Missing, extra, duplicate, malformed or
mismatched records invalidate the pack. Checksums cover `pack.yaml`, all
required entries and optional documentation/license files.

`EVIDENCE.md` is the byte-exact canonical Markdown projection of the evidence
JSON. Citation excerpts are optional, UTF-8, SHA-256-bound, and limited to 512
bytes. Packs with Source Bundles include absolute HTTP(S) citation URLs bounded
to 8 KiB and without userinfo or credential-like query/fragment parameters.

Every file below `assets/` except `assets/licenses.json` is Kelyro-authored
original content and has one sorted ledger entry with path, authorship marker,
declared license, copyright holder, and exact SHA-256. The builder never accepts undeclared
external assets.

## Resource limits and validation

Loaders apply bounded per-file, total-uncompressed, filesystem/archive-entry,
path-length and exact compression-ratio limits before decoding. YAML/JSON/text
entries must be valid UTF-8 and terminal-control-safe.
Unknown schema fields, duplicate YAML/JSON keys and trailing documents/data are
invalid. Validation is entirely local and must not fetch evidence, resolve a
marketplace, execute code, install the pack, or modify learner state.

Markdown outside fenced code must not contain raw HTML, active `javascript:`,
`vbscript:`, `data:` or `file:` destinations, or embedded images that could
trigger an external request. Checksums establish byte integrity only; they do
not authenticate a publisher. Signature verification is an optional future
application port and is not required by v1.

Known source-retention patterns are invalid: cache/raw/body/snapshot/transcript
paths, full-document formats, transcript/full-article filenames, unlicensed
assets, and structured fields that retain raw/cached/full bodies. These checks
are conservative detectable-pattern gates, not a general copyright oracle.

The curriculum entry is the complete learner-neutral I-04 definition. The
evidence entry follows `curriculum-evidence-report/v1`: it contains goal and
competency summaries, exact immutable Source Bundle/Claim references, primary
coverage, freshness, conflicts/caveats, temporal content, gaps, and compiler
versions. It never embeds Claim statements or Evidence excerpts. Pack builds
also include its human-readable `EVIDENCE.md` projection. The required
build-info entry follows `curriculum-build-info/v1`
and freezes the compiler/pass/config/source recipe plus input/output hashes.
The optional environment entry follows
[`environment-pack/v1`](environment-pack-v1.md), contains declarative tool
requirements only, and never grants permission to store credentials, execute
commands, or install software.

Each serialized goal outcome includes required `id`, `statement`, `category`
and `evidence_refs`, plus optional `capability`. Categories and capabilities
use the closed vocabulary in `professional-outcomes-v1`; a professional goal
must declare all five required capabilities.

Each serialized competency includes required `id`, `area_id`, `area`,
`outcome_id`, `expected_level` and `evidence_refs`. Optional `dimensions` hold
`id`/`expected_level` pairs; `parent_id` declares same-area hierarchy and
`concept_refs` may remain empty until concept extraction. Matrices produced by
I-04 use `version: competency-matrix-v1`.
