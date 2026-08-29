# Research Update Scan v1

Step 37 defines the read-only update scan identified by `update-scan-v1`. It
compares durable Research metadata with an optional adapter-provided view of
current change signals. It reports possible updates; it never changes a Source
Bundle, curriculum, lesson, mastery, or student state.

## Inputs and local analysis

Every scan records its UTC timestamp and inventories known technology IDs,
known releases, tracked sources, and freshness records currently due. The
local pass remains available without network access and reports:

- a new release when a technology has prior release history and its latest
  verified record is current;
- a changed source when the two newest comparable stored content hashes differ,
  or the latest stored fetch explicitly reports HTTP 404/410;
- stale evidence from the persisted refresh schedule due at the scan time;
- the latest durable deprecation determination for each subject; and
- every unresolved persisted conflict.

HTTP 304 snapshots and other snapshots without a content hash are not treated
as proof of change. Absence of snapshots or releases is not converted into a
claim that a source or release does not exist.

Signals use a closed type, stable reference, bounded detail, observation time,
and origin (`stored_metadata` or `current_lookup`). The report deduplicates a
type/reference pair and orders types as new release, changed source, stale
evidence, deprecated subject, and unresolved conflict, followed by reference.

## Live adapter and privacy

`UpdateSignalProvider` is the only port for current external change signals.
It receives bounded source/release metadata, never stored page bodies or
evidence excerpts. Before the provider can run, application code authorizes
`research.update_scan` through Foundation's network gate.

When `privacy.allow_network` is false, the provider is not called and the
local report is returned as `incomplete (network_disabled)`. If networking is
allowed but no provider is configured, the reason is `provider_unavailable`;
an adapter failure produces `provider_failed`. Cancellation and deadlines are
returned as errors rather than hidden inside a partial report.

The current production wiring deliberately has no live update provider, so
`kelyro research update-scan` is complete for stored metadata and explicit
about why its current-source comparison is incomplete.

## Boundaries

- External content remains untrusted adapter data and cannot become evidence
  merely because it appears in a scan.
- The scan does not fetch sources itself or write unbounded content to SQLite.
- A finding is a change signal, not proof of semantic drift; Step 38 owns that
  comparison.
- No curriculum compiler, impact analysis, scheduler, automatic migration, or
  Student Core mutation is part of this policy.
