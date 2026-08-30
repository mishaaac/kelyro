# Release Intelligence Model

Step 19 defines `research.TechnologyRelease` and `research.VersionIdentifier`
as Kelyro's persistence- and provider-independent vocabulary for technology
versions and release lifecycle. This step represents already established
facts; it does not discover releases or ingest release notes.

## Version identity

`VersionIdentifier` preserves the exact, non-empty text supplied by a reviewed
source. It is an alias of the existing `SourceVersion` value object so source
scopes, research target versions, citations, release records, and the v23
SQLite text column retain one compatible identity contract.

Classification is deterministic and ordered:

1. strict SemVer 2.0.0;
2. a supported zero-padded date form;
3. opaque fallback.

The closed `VersionScheme` values are `semantic`, `date_based`, and `opaque`.
Classification never rewrites or rejects a valid opaque identifier merely
because it is not SemVer.

### Semantic versions

Semantic recognition requires `major.minor.patch`. Numeric core identifiers
and numeric prerelease identifiers reject leading zeroes except for the value
`0`. Prerelease and build identifiers accept ASCII alphanumeric characters and
hyphens separated by dots. Core numbers must fit unsigned 64-bit integers.

`Semantic()` returns major, minor, patch, prerelease, and build metadata.
`NewSemanticVersionIdentifier` is the strict constructor for callers that know
SemVer applies; the generic constructor keeps a non-matching value opaque.
Build metadata remains part of identity even though future SemVer precedence
logic may ignore it. Step 19 does not implement ordering or upgrade decisions.

### Date-based versions

The accepted calendar forms are:

```text
YYYY-MM-DD
YYYY.MM.DD
YYYYMMDD
YYYY-MM
YYYY.MM
```

Dates must be real Gregorian calendar dates. Month-precision values expose day
one together with explicit `month` precision, so they cannot be confused with
an explicit first-of-month day version. `NewDateVersionIdentifier` rejects
invalid dates; the generic constructor retains them as opaque rather than
inventing a correction.

Other calendar/version schemes such as quarters, epochs, editions, distro
names, `go1.25`, or `R2026a` remain valid opaque identifiers. Kelyro does not
impose SemVer or guess provider-specific ordering.

## TechnologyRelease

`TechnologyRelease` contains:

```text
id
technology_id
version
released_at
channel
status
source_ids
verified_at
```

`ReleaseRecord` remains a type alias for source compatibility with the Step 02
repository/application ports. New code can use the explicit entity name
without a parallel persistence model.

Channels are `stable`, `preview`, `beta`, `rc`, `experimental`, `nightly`, and
`unknown`. Lifecycle statuses are `current`, `superseded`, `legacy`, `eol`, and
`unknown`. Channel and lifecycle are independent: preview is a distribution
channel, while legacy/EOL describe lifecycle.

A release requires stable release and technology IDs, a valid version, at
least one evidence-bearing source identity, a valid channel/status, and a real
UTC `verified_at`. `released_at` remains optional when a source does not
establish it, but when present it cannot follow verification. Absence of a
release date is not replaced with discovery, snapshot, or verification time.

## Persistence and application compatibility

No migration is needed. Migration v23 already stores the exact version text,
channel, status, source IDs, optional release date, and verification date.
Semantic/date/opaque classification is reconstructed deterministically from
the unchanged text, so SQLite and memory round trips preserve identity and
scheme without provider metadata or JSON duplication.

The existing narrow `ReleaseRepository`, `ReleaseIntelligenceService`,
privacy-gated lookup port, and memory/SQLite adapters continue to carry
`ReleaseRecord`, which is now the alias of `TechnologyRelease`. Lookup results
remain candidates until evidence and verification establish a durable fact.

## Deferred work

Step 19 does not implement version precedence, current-stable selection,
duplicate-release policy, provider adapters, GitHub assumptions, release-note
snapshots, change claims, auto-upgrades, curriculum mutations, or any part of
Student Core. Release Discovery and Release Notes ingestion belong to Step 20.
