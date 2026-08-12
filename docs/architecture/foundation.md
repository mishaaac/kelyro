# Foundation architecture

Kelyro's Foundation is the smallest cross-platform base on which later product
subsystems can depend. It defines vocabulary and boundaries; it does not make a
presentation framework, database, external service, or operating system part of
the core.

## Layers and dependency direction

Dependencies point inward:

```text
cmd / cli / tui
       |
       v
application services
       |
       v
domain types and contracts
       ^
       |
infrastructure adapters
```

- **Domain types and contracts** express Foundation concepts and the operations
  the application needs. They contain no framework or infrastructure imports.
- **Application services** coordinate contracts and policy. They do not know
  whether a caller is the CLI or TUI, or whether persistence uses files,
  SQLite, or another local mechanism.
- **Infrastructure adapters** implement contracts using the filesystem, the
  operating system, databases, credential stores, editors, or network clients.
- **UI adapters** translate CLI or TUI input into application-service calls and
  render results. They contain no educational or persistence policy.

Import cycles are forbidden. In particular, the core never imports presentation
or infrastructure adapters merely to perform work.

## Package responsibilities

| Package | Responsibility |
| --- | --- |
| `internal/app` | Application-service orchestration. |
| `internal/cli` | Command-line input and output adapter. |
| `internal/tui` | Terminal presentation adapter. |
| `internal/platform` | OS-dependent contract plus native directory and path normalization helpers. |
| `internal/workspace` | Workspace identity, discovery, initialization, and validation contract. |
| `internal/infra/workspacefs` | Local filesystem adapter for the workspace lifecycle contract. |
| `internal/config` | Format-independent global and project configuration contract. |
| `internal/storage` | Opaque state and secret persistence contracts. |
| `internal/artifacts` | Ownership classification for generated and human-authored files. |
| `internal/audit` | Boundary for recording critical actions, without an event bus. |
| `internal/editor` | Future editor integration adapters. |
| `internal/doctor` | Future Foundation diagnostics. |
| `internal/logging` | Future structured logging infrastructure. |
| `internal/backup` | Future backup and restore services. |
| `internal/privacy` | Future privacy inspection services. |
| `internal/update` | Future update-check services. |
| `internal/version` | Build metadata independent of Git at runtime. |

Reserved packages document ownership only. Their functionality is introduced by
later implementation steps.

## Allowed and prohibited dependencies

Core contracts may depend on the Go standard library and other inward-facing
domain types. Application services may depend on those contracts.
Infrastructure and UI packages may depend on application services and contracts.

The following dependencies are prohibited from the core:

- Bubble Tea, Cobra, or another presentation/command framework;
- a SQLite driver or database-specific types;
- GitHub clients, AI-provider SDKs, or other external services;
- direct operating-system operations outside platform or infrastructure
  adapters.

If the CLI later adopts Cobra, it remains isolated in `internal/cli`. If state
later uses SQLite, the driver and queries remain behind `storage.StateStore`.
Neither implementation detail is visible to application callers.

## Stable Foundation contracts

`platform.Platform` owns OS-specific discovery and actions, including standard
user directories, command lookup, and opening paths or URLs.

### Path conventions

All path construction and normalization goes through `internal/platform` and
Go's `path/filepath` package. `NormalizePath` cleans a path and resolves relative
input against the process working directory without requiring the target to
exist. It preserves the spelling and case supplied by the operating system; no
caller may infer case sensitivity from a path string.

Kelyro resolves home, configuration, cache, and temporary directories with the
standard `os` APIs. Consequently, global data follows native conventions:

- Windows uses the directories represented by `USERPROFILE`, `AppData`,
  `LocalAppData`, and the system temporary-directory configuration. Drive
  letters and UNC-compatible separators retain `filepath` semantics.
- macOS uses the user's home plus the standard Application Support and Caches
  locations returned by Go.
- Unix-like systems use the XDG configuration and cache locations when set,
  with Go's home-based fallbacks otherwise.

The global configuration and cache directories append `kelyro` to their native
bases. Workspace data never moves into those global directories:
`WorkspaceInternalDir` resolves `<workspace>/.kelyro`, the database is
`learning.db`, metadata is `workspace.json`, state is under `state`, disposable
cache is under `cache`, backups are under `backups`, and logs are under `logs`.
`WorkspaceLearningPath` resolves the visible `LEARNING.md` document. These
helpers accept relative roots and paths containing spaces. They do not create
directories or validate workspace identity; those lifecycle operations belong
to the filesystem adapter behind the workspace contract. The database path is
reserved for a later step and workspace initialization does not create it.

`workspace.Service` discovers, initializes, and validates a workspace while
returning a neutral `workspace.Workspace`. Paths cross boundaries as complete
values; adapters must use Go's cross-platform path APIs when manipulating them.

### Workspace lifecycle and ownership

A workspace is identified by a valid `.kelyro/workspace.json` with a stable
random ID, the current schema version, its UTC creation time, and the Kelyro
version that created it. Discovery starts at an existing directory and walks
upward until it finds and validates `.kelyro`. An invalid marker is reported
rather than silently skipped or repaired.

Initialization builds all machine internals in a temporary directory before
renaming them into place. If a later operation reports failure, it removes the
new internals and any `LEARNING.md` it created. Repeating initialization loads
the existing identity without changing it. Creating a workspace below another
workspace is rejected unless the caller explicitly sets the nested-workspace
option.

Ownership rules are intentionally narrow:

- `.kelyro/` and all of its contents are machine-owned. Kelyro may create,
  validate, migrate, replace, or remove those internals according to the active
  workspace schema.
- `LEARNING.md` is student-owned. Initialization may create it only when the
  path is absent; Kelyro never overwrites existing content automatically.
- Every other visible file is student-owned by default. Kelyro requires an
  explicit, operation-specific confirmation before modifying or deleting it.
- Workspace internals contain no secrets. Credential storage remains behind
  `storage.SecretStore` and outside the workspace tree.

`config.Store` loads and saves format-independent settings at global and project
scope. `storage.StateStore` persists opaque application state without revealing
a database engine. `storage.SecretStore` similarly hides the credential backend,
so callers never encode storage policy or place secrets in repository files.

`artifacts.Ownership` distinguishes:

- `machine-owned` opaque state managed by Kelyro;
- `system-generated-human-readable` generated output people may inspect;
- `student-owned` human-authored material that must not be overwritten without
  explicit confirmation.

`audit.Recorder` records critical actions through a small event contract. It is
not a general event bus; timestamps, serialization, and destinations belong to
its adapter.

## Why UI and persistence stay outside the core

The TUI is one way to operate Kelyro, alongside the CLI and possible future
interfaces. Keeping educational and application policy outside it makes the same
behavior testable without starting a terminal and reusable by every interface.

Likewise, exposing SQLite connections, rows, or query types would force every
caller to understand the chosen storage engine. The state contract instead
expresses only what callers need and lets storage evolve independently.

## Local-first

Local-first means the user's workspace and local state are the primary working
copy. Core workflows must remain useful without a network connection; remote
services are optional adapters and synchronization must not become a hidden
prerequisite. Local-first does not mean every file is freely writable: artifact
ownership and explicit confirmation still protect student-authored work.
