# Research Source Transparency TUI v1

Step 36 exposes stored Research & Source Intelligence through six terminal
views: Research, Sources, Source detail, Claim detail, Conflicts, and
Freshness. Bubble Tea remains an incoming adapter. It formats application read
models and dispatches commands; it does not query SQLite or evaluate trust,
freshness, conflicts, or provenance.

## Navigation and read models

Home opens Research with `R`. Research links to Sources (`s`), unresolved
Conflicts (`c`), and records due for reverification (`f`). A selected source
opens Source detail. A selected conflict and claim open the latest persisted
provenance graph as Claim detail.

All workspace reads run in `tea.Cmd` functions and return typed messages to
`Update`. `View` is pure formatting over model state, so repeated renders do
not reopen the workspace database and never trigger discovery, fetch, release
lookup, or any other network operation. Explicit `r` refreshes only the current
stored read model.

The source transparency projection combines:

- stable source identity, kind, temporal scope, version, and bounded metadata;
- the latest persisted Trust Policy decision and authority tier;
- persisted freshness state, score, and last-verification timestamp;
- the latest immutable snapshot for Source detail.

A missing trust decision is rendered as `authority unknown`; it is never
inferred from `official` or another source kind. Missing freshness is rendered
as `freshness unknown`. Historical, archived, and version-bound sources reuse
the domain temporal warning and always show their version scope.

## URL and platform boundary

The terminal displays only a compact `host/path` locator. Query parameters and
fragments are omitted from the visible label, and terminal-width truncation
prevents an external locator from breaking layout. Pressing `o` passes the
complete validated HTTP(S) locator to the I-01 `platform.Platform` boundary.

The production `platformos` adapter selects the native system opener without a
shell (`xdg-open`, `open`, or `rundll32.exe`) and rejects non-HTTP(S) URLs. The
operation runs as a Bubble Tea command, and failures return to Source detail as
a retryable notice.

## Session and scope boundaries

Research views are ephemeral inspection state and do not extend or migrate the
published Student Core session payload. Leaving them returns to the existing
Home session destination. Step 36 adds no search provider, live fetch, update
scan, drift detection, curriculum compilation, mastery mutation, or new schema.
