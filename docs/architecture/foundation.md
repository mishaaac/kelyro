# Foundation architecture

Kelyro's Foundation is the smallest cross-platform base on which later product
subsystems can depend. It defines vocabulary and boundaries; it does not make a
presentation framework, database, external service, or operating system part of
the core.

I-01 Foundation is complete as of `v0.1.0-alpha.1`. Its contracts are the stable
base for I-02 Student & Learning Core. Because Kelyro remains in the `0.x`
pre-release series, intentional contract changes still follow SemVer and must be
documented rather than assumed to be backward-compatible.

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
| `internal/tui` | Bubble Tea terminal presentation adapter: model, messages, commands, views/components, styles, and terminal lifecycle. |
| `internal/platform` | OS-dependent contract plus native directory and path normalization helpers. |
| `internal/workspace` | Workspace identity, discovery, initialization, and validation contract. |
| `internal/infra/workspacefs` | Local filesystem adapter for the workspace lifecycle contract. |
| `internal/config` | Format-independent schema, validation, precedence, and global/project persistence contract. |
| `internal/infra/configfs` | Strict TOML and atomic filesystem adapter for configuration. |
| `internal/storage` | Opaque state and secret persistence contracts. |
| `internal/infra/secretstore` | Environment and native OS-keychain adapters for secret references. |
| `internal/storage/sqlite` | Workspace-local SQLite adapter, migrations, transactions, and Foundation repositories. |
| `internal/session` | Versioned session state, resume defaults, crash detection, and recovery policy. |
| `internal/infra/sessiondb` | Transactional SQLite adapter for workspace session lifecycle and recovery audit. |
| `internal/artifacts` | Ownership, hashing, and integrity metadata contracts for workspace files. |
| `internal/artifacts/markdown` | Pure rendering of stable human-readable Foundation documents. |
| `internal/infra/artifactfs` | Ownership-aware atomic writes and workspace path sandbox. |
| `internal/audit` | Typed critical-event recording and chronological audit reading contracts. |
| `internal/infra/auditsqlite` | Workspace-scoped durable audit store over Foundation SQLite. |
| `internal/editor` | Neutral editor discovery and file-opening contract. |
| `internal/infra/editoros` | Native executable discovery and safe process-launch adapter. |
| `internal/doctor` | Typed diagnostics, tool registry, contextual requirements, and failure policy. |
| `internal/infra/doctoros` | Native writable-path, executable-resolution, and bounded version probes. |
| `internal/infra/doctorsqlite` | SQLite health, migration-version, and artifact-index diagnostic adapter. |
| `internal/logging` | Structured diagnostic levels, entries, redaction, and workspace logger contracts. |
| `internal/infra/logfs` | Bounded workspace-local JSONL logging with restrictive permissions and rotation. |
| `internal/backup` | Neutral backup manifests, summaries, validation, and lifecycle contracts. |
| `internal/infra/backupfs` | Atomic allowlisted workspace backups, retention, integrity validation, and rollback-safe restore. |
| `internal/privacy` | Local-first policy and deny-by-default network authorization contract. |
| `internal/update` | SemVer comparison, release-provider/cache contracts, channels, and update-check policy. |
| `internal/infra/updatecache` | Versioned atomic update metadata cache in the native user cache directory. |
| `internal/infra/updategithub` | Optional bounded GitHub REST release-metadata adapter. |
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

Bubble Tea and Lip Gloss remain isolated in `internal/tui`; SQLite remains
behind storage and artifact contracts. None of these implementation details is
visible to core or application callers.

### Foundation terminal interface

Invoking Kelyro without an explicit command makes the CLI delegate terminal
lifecycle to `internal/tui`. The model starts in a loading state and obtains a
typed `app.FoundationSnapshot` through an asynchronous command. Workspace
discovery, layered configuration, and the database integrity probe are
coordinated by the application service; Bubble Tea's `Update` handles only UI
messages, screen state, keyboard navigation, and resize events.

Home reports workspace, database, and configuration health without relying on
color alone. Doctor groups its typed report by Platform, Kelyro, Development,
and Optional, using explicit markers as well as color. Config
reads every resolved setting and provides a deliberately small wizard for
cycling `ui.color` and toggling common booleans; other scalar values remain
available through `kelyro config set`. Roadmap renders the intentional empty
state and delegates opening `ROADMAP.md` to the existing application/editor
service.

Views wrap shortcuts and diagnostic text according to the latest terminal
width. They use ordinary Unicode rather than icon fonts, and all actions expose
visible keys. `NO_COLOR`, `--no-color`, and `ui.color = "never"` disable styling.
The runner uses Bubble Tea's alternate-screen lifecycle and adds a recovery
boundary so a panic unwinds Bubble Tea's terminal cleanup before becoming a
normal CLI error.

### Environment diagnostics and tool registry

`internal/doctor` owns presentation-neutral checks, requirements, reports, and
the registry metadata for Go, Git, VS Code, Neovim, Docker, and lazygit. Tool
metadata includes command candidates, supported platforms, requirement level,
purpose, maintained educational guidance, platform notes, official
documentation URL, and safe version arguments. Guidance separates what a tool
is, why it may help, and which underlying fundamentals Kelyro teaches first. It
is static local metadata: Doctor never asks an LLM to produce it. A future
curriculum phase can supply a `doctor.Context` that selects only relevant tools
and can strengthen a registry entry—for example, making Docker required for one
module with a module-specific explanation.

The application supplies resolved configuration and workspace paths, while the
native adapters probe write access and SQLite health. Executables are resolved
without a shell. Version commands use only registry-owned arguments, bounded
output, and a per-command timeout; failure to obtain a version does not erase a
successful executable detection. SQLite checks independently cover integrity,
the latest applied migration, and readability of the artifact index.

Both `kelyro doctor` and the Doctor TUI screen render the same typed report.
Required failures produce an unsuccessful CLI exit, while missing recommended
or optional tools remain informative and always include their purpose and an
installation link when absent. `kelyro doctor --explain <tool>` reads the richer
guidance directly from the registry and selects the note for the current
platform without requiring a workspace. Official links are printed for the user
to inspect; Kelyro does not open them or start an installation automatically.

### Session persistence and recovery

Each TUI startup transactionally loads the single versioned Foundation session
payload and immediately marks it active. A clean `q` or Ctrl+C exit writes the
final resumable payload and clears that marker. If Kelyro exits before this
happens, the next startup preserves safe context while identifying the previous
session as incomplete and recording `session.recovered` in the workspace audit
trail. Invalid, unsupported, or explicitly unsafe secondary state is replaced
with defaults and never prevents the Foundation interface from opening.

The durable payload contains only the last view, opened artifact, significant
command, setup flags, session timestamp, and safe-resume marker. The TUI queues
and serializes checkpoints after meaningful transitions such as changing views,
saving configuration, or opening the roadmap. Ordinary keypresses, resize
events, loading animation, and reconstructable snapshots do not write SQLite.
`internal/session` owns payload version migration without knowing SQLite;
`sessiondb` binds each resume, checkpoint, and completion to the existing
workspace-local transaction runner and state/audit repositories.

Bubble Tea is necessary for the cross-platform terminal event loop, raw-mode
cleanup, keyboard input, and resize delivery. Lip Gloss is necessary for style
composition and terminal cell-width measurement. Both dependencies are confined
to this presentation adapter; Bubbles is not included because the small current
screens do not need a reusable interactive component.

## Stable Foundation contracts

`platform.Platform` owns OS-specific discovery and actions, including standard
user directories, command lookup, and opening paths or URLs.

The narrower `editor.Service` is the application-facing boundary for selecting
and launching editors. `editoros` resolves executable names with the native
process lookup, passes the artifact path as a distinct argument, and never
evaluates a shell command string. An explicit `editor.command` must be one
executable name or path; if it is absent, detection checks `code`, `nvim`,
`vim`, `zed`, and `cursor` in that order before using the platform's default
file opener. A configured but missing executable fails visibly instead of
silently selecting something else.

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
bases; the global configuration file is `config.toml` in that Kelyro directory.
Workspace data never moves into those global directories:
`WorkspaceInternalDir` resolves `<workspace>/.kelyro`, the database is
`learning.db`, metadata is `workspace.json`, state is under `state`, disposable
cache is under `cache`, backups are under `backups`, and logs are under `logs`.
`WorkspaceConfigPath` resolves `.kelyro/config.toml`, and
`WorkspaceLearningPath` resolves the visible `LEARNING.md` document and
`WorkspaceRoadmapPath` resolves `00-roadmap/ROADMAP.md`. These
helpers accept relative roots and paths containing spaces. They do not create
directories or validate workspace identity; those lifecycle operations belong
to the filesystem adapter behind the workspace contract. The application opens
the SQLite adapter after structural initialization because generated documents
need their integrity metadata recorded from the first write.

`workspace.Service` discovers, initializes, and validates a workspace while
returning a neutral `workspace.Workspace`. Paths cross boundaries as complete
values; adapters must use Go's cross-platform path APIs when manipulating them.

### Workspace lifecycle and ownership

A workspace is identified by a valid `.kelyro/workspace.json` with a stable
random ID, the current schema version, its UTC creation time, and the Kelyro
version that created it. Discovery starts at an existing directory and walks
upward until it finds and validates `.kelyro`. An invalid marker is reported
rather than silently skipped or repaired.

Structural initialization builds all machine internals in a temporary directory
before renaming them into place. It does not write visible files. The
application then renders and persists the Foundation Markdown documents through
the ownership-aware artifact store. Repeating initialization loads the existing
identity and safely regenerates only documents whose recorded hashes still
match. Creating a workspace below another workspace is rejected unless the
caller explicitly sets the nested-workspace option.

Ownership rules are intentionally narrow:

- `.kelyro/` and its contents are machine-owned. Kelyro may create, validate,
  migrate, replace, or remove those internals according to the active workspace
  schema. `.kelyro/config.toml` is the explicit exception intended for advanced
  manual editing; Kelyro validates it strictly and preserves comments during
  single-key CLI updates.
- `LEARNING.md` and `00-roadmap/ROADMAP.md` are system-generated,
  human-readable artifacts. They are indexed on creation and may be regenerated
  only while their current hashes match the last generated content.
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

Known generated Markdown names receive the human-readable classification, but
classification alone never grants overwrite permission. `artifactfs.Store`
requires a matching artifact-index entry and compares the current SHA-256 hash
with the hash from the last successful generation. An absent index entry or a
different hash returns a conflict without changing the file or metadata;
student-owned paths are never written by this adapter. Machine-owned writes are
restricted to `.kelyro/` by the same path classification check.

Successful writes use a temporary file in the destination directory, sync it,
and atomically replace the destination. The index stores the workspace-relative
path, ownership category, creator, content hash, creation and last-generation
times, and optional expected template/version. Windows uses its native
replace-existing operation so regeneration has the same atomic contract as
Unix-like systems.

`artifacts/markdown` renders the initial `LEARNING.md` and roadmap placeholder
from a small human-facing model. Rendering has no filesystem or database access,
uses UTF-8 with LF-terminated lines, and is covered by golden files. The visible
documents contain no serialized internal state or decorative frontmatter, so
workspace schema changes do not become a compatibility constraint for Markdown.
The application supplies each template version and creator to `artifactfs`,
which binds filesystem writes to the workspace-local SQLite artifact index.

`artifactfs.Sandbox` accepts relative paths only. It rejects traversal
components, native and foreign absolute-path forms, and any existing symlink
chain that resolves outside the configured workspace root. Missing descendant
directories are allowed only after their nearest existing ancestor is proven
to remain inside the root. This is confinement infrastructure for later
exercise workflows; it does not create exercises in Foundation.

`audit.Recorder` records critical actions through a small event contract. It is
not a general event bus; timestamps, serialization, and destinations belong to
its adapter. `audit.Reader` returns the same durable trail in chronological
order without exposing SQLite details.

### Structured logging and audit trail

Diagnostic logs are JSON Lines records under
`.kelyro/logs/kelyro.jsonl`. Every record has a timestamp, one of `debug`,
`info`, `warn`, or `error`, plus operation, workspace, component, and an error
category for failures. Normal commands write only to the file; successful logs
never appear in the terminal. `--verbose` enables an additional debug entry
and safe structured fields. `kelyro logs path` reports the current file without
opening it automatically.

The filesystem adapter restricts log files to owner read/write permissions and
rotates before the active file exceeds one MiB. Foundation retains at most
three files, including the active file. An individually oversized entry is
replaced by a bounded truncation marker. Secret-like field names are redacted,
explicit sensitive values are removed from every serialized string, and fields
that could contain a prompt, answer, submission, or complete student document
are omitted. Application operations log identifiers and state transitions, not
document bodies or secret values. Failure to open or write a diagnostic log is
best effort and cannot replace the result of the user's requested operation.

The audit trail is distinct from diagnostic logging. It persists significant
state changes in `.kelyro/learning.db`, including workspace initialization,
configuration-key changes, applied migrations, generated artifacts, blocked
artifact regeneration, and session recovery or migration. Each entry stores a
UTC timestamp, stable event name, `system`, `user`, or future `plugin` actor,
safe metadata, subject, and app version. Configuration values and generated
content are not audit metadata. `kelyro audit` reads events in timestamp and
insertion order, and ordinary keypresses never create events.

### Layered configuration

Configuration resolves in this order: safe defaults, the global file, the
discovered workspace file, and explicit CLI overrides. Later layers win. The
initial CLI override is `--no-color`, which resolves `ui.color` to `never`
without persisting it. Global files accept UI, editor, privacy, and update
settings. Project files may also define those keys as overrides and add
`workspace.name` and `learning.mastery_threshold`; the threshold is schema only
and has no educational behavior yet.

Editor configuration includes the executable-only `editor.command` and the
`editor.prompt` boolean. The latter defaults to enabled and reserves the user's
choice for an optional TUI open-after-generation prompt; explicit `kelyro open`
commands always open immediately.

Update configuration includes `updates.check`, enabled by default, and
`updates.channel`, which accepts `stable` (the default) or `prerelease`.
Enabling checks does not grant network permission; `privacy.allow_network`
remains the independent opt-in boundary.

Both files carry an optional `schema_version`; newly written files use version
1, and unsupported versions fail before any values are used. The filesystem
adapter accepts the strict scalar TOML subset required by the known schema:
tables, quoted strings, booleans, and numbers. Unknown tables or keys, duplicate
definitions, incorrect scalar types, and invalid values produce errors naming
the offending key. This bounded parser avoids an external dependency while the
schema is small; adding TOML features outside the supported schema requires an
explicit parser decision.

Writes use a same-directory temporary file, restrictive permissions, file
synchronization, and rename replacement. A failed commit leaves the prior file
unchanged and removes staging files. Single-key `config set` updates retain
existing comments, including an inline comment on that key. The lower-level
bulk save methods intentionally emit canonical TOML and therefore do not retain
comments; they are intended for generated new files and migrations, not normal
CLI edits. Crash-level rename guarantees ultimately follow the host operating
system's filesystem semantics.

`kelyro config show`, `path`, `get`, and `set` operate on this model. `--global`
or `--project` selects a scope explicitly. Without a scope, reads include the
discovered project when present, and writes target that project or fall back to
global configuration outside a workspace. The schema exposes metadata for a
future common-settings wizard, while advanced editing remains available through
the paths reported by `config path`. Configuration files never accept API keys
or other secrets.

### Local-first privacy and offline boundary

Foundation persists its database, Markdown, configuration, logs, backups, and
portable exports on the local machine. It performs no hidden HTTP requests and
emits no automatic usage telemetry. The TUI, `status`, `roadmap`, `doctor`,
`config`, `backup`, and `export` workflows operate without Internet access;
Doctor's maintained documentation links are rendered as text and are not
opened or fetched automatically.

The resolved privacy policy is deny-by-default:

```toml
[privacy]
allow_network = false
allow_ai_content = false
allow_usage_telemetry = false
```

`internal/privacy.NetworkGate` is the mandatory boundary for future external
resources, update checks, plugins, and AI-provider integrations. General
external access requires `allow_network`. Sending AI content or usage telemetry
also requires its dedicated opt-in; enabling either dedicated setting without
general network access does not bypass offline mode. The `updates.check`
preference expresses interest in checks, but it cannot bypass the privacy gate.

Authorization requests and denial-specific metadata contain only a bounded
stable operation identifier and a declared purpose. URLs, filesystem paths,
student content, prompts, and credentials are deliberately absent, preventing
accidental disclosure through the boundary. Denials are returned as typed
errors and recorded best-effort in the workspace-local structured log with the
operation, purpose, and `privacy` category. The ordinary local log record still
identifies its workspace as described above. A logging failure never permits
network access or replaces the denial.

### Version-aware update checks

`internal/update` parses and compares SemVer 2.0 versions, including numeric and
alphanumeric prerelease precedence. A conventional leading `v` is accepted and
build metadata does not affect precedence. Stable checks reject prereleases;
the explicitly configured prerelease channel considers both stable and
prerelease versions. A published version equal to or older than the embedded
build never produces an update or downgrade offer.

`update.ReleaseProvider` exposes only provider-neutral release metadata. The
optional production adapter lists public releases from the fixed Kelyro GitHub
repository using the versioned REST API, a ten-second HTTP timeout, required
GitHub headers, a two-MiB response limit, and no credential. Drafts and
malformed tags are ignored. No GitHub or HTTP type crosses into the core.

`kelyro update check` first honors `updates.check`, then uses a fresh result from
the native global cache when available. Otherwise it consults the privacy gate
before calling the provider. Successful checks, including an empty release
list, are cached per channel for 24 hours in a bounded, schema-versioned JSON
file written atomically with restrictive permissions. Corrupt or stale cache
data is disposable and triggers a refresh; cache failures do not replace a
valid provider result.

Offline privacy denials and temporary provider failures are successful,
informative check results so they cannot prevent the rest of Foundation from
running. Malformed current or selected release versions remain explicit errors.
Checks show metadata as text only: they never open a release URL, download an
artifact, or change the executable. `kelyro update` fails safely with an
explanation until release artifacts have checksum/signature verification and a
separate explicit-consent installation design.

### Secure secret storage

`storage.SecretStore` is the only secret-value boundary visible to application
code. It supports named reads, writes, deletion, reference-only status, and a
backend availability check. The contract exposes `ErrSecretNotFound` and
`ErrSecretStoreUnavailable` without revealing native credential APIs. A fake or
another future adapter can replace the production store without changing the
core.

The production adapter resolves `KELYRO_SECRET_<NAME>` first, normalizing dots
and hyphens to underscores, then uses the native credential service. Linux uses
the Secret Service through `secret-tool`, macOS uses Keychain through
`security`, and Windows calls Credential Manager directly. A small sorted index
of reference names is stored inside the same keychain so status can enumerate
entries; the index contains no values. No credential data or index is written
under `.kelyro`, into global or project TOML, or into a repository file.

`kelyro secrets set <name>` obtains the value through the CLI's terminal-input
adapter with echo disabled. The value is not accepted as an argument. Status,
success messages, and `config show` render only `configured`,
`not configured`, and a reference such as `KELYRO_SECRET_PROVIDER_TOKEN` or
`keychain:kelyro/provider.token`. Backend errors are actionable and values are
redacted defensively before they cross into CLI output.

When a native service is absent or inaccessible, the adapter remains usable for
environment-backed reads and reports the exact `KELYRO_SECRET_*` alternative.

### Workspace-local structured persistence

`internal/storage/sqlite` owns `.kelyro/learning.db` through `database/sql` and
the pure-Go `modernc.org/sqlite` driver. This storage dependency avoids CGO
requirements on Linux, macOS, and Windows while the core continues to depend
only on neutral repository interfaces. Callers create
an explicit database instance per workspace and close it; there is no global
connection singleton.

Opening a database uses a bounded operation timeout, enables SQLite foreign
keys on every connection, restricts the database file permissions, and runs
`PRAGMA quick_check` before migrations. New databases migrate from version zero
to the latest embedded schema. The initial migration creates only
`schema_migrations`, `workspace_meta`, `app_state`, `artifact_index`, and
`audit_events`; a second non-destructive migration enriches `artifact_index`
with integrity and generation metadata. A third non-destructive migration adds
the actor and app-version identity required by the durable audit trail.
Educational tables remain outside Foundation.

Migrations are consecutive, immutable records with a name and SHA-256 checksum.
The runner validates existing history, skips already applied versions, and
applies each pending migration in its own transaction. Errors identify the
migration and statement, and both DDL and its history record roll back together.
A migration marked destructive fails closed unless an injected backup callback
succeeds before its SQL begins. Every production SQLite entry point receives
the filesystem backup callback, so the same preflight protects artifact,
session, audit, and Doctor-triggered database opens.

State, workspace metadata, artifact ownership, and audit recording are exposed
through their core interfaces. Independent operations are atomic SQL statements;
callers that must update more than one repository use `WithTransaction` so all
writes commit or roll back together. Values remain opaque at this boundary and
no secret is stored in SQLite.
Environment values take precedence and are not copied into native storage.
Deletion affects the keychain only and explicitly leaves environment variables
unchanged. Logs, audit events, backups, and exports have no secret-value path;
they may carry reference names or configuration state only, never the result of
`SecretStore.Get`.

### Backup and restore boundary

`backup.Service` is independent of filesystem and SQLite types. The
`backupfs` adapter publishes each backup by renaming a fully written staging
directory beneath `.kelyro/backups`. Its JSON manifest records format, app,
workspace, and database schema versions, a UTC timestamp, operation reason,
and a SHA-256 plus size for every copied file. The allowlist is limited to
`learning.db`, project `config.toml`, `workspace.json`, and files below
`state`; logs, caches, nested backups, arbitrary internal files, global config,
and native/environment secrets are never traversed.

`backup.retention` defaults to five and accepts integer values from one through
one hundred at global or project scope. Retention runs only after a new backup
has been atomically published. Listing verifies every recorded file. Database
snapshots are additionally opened read-only for `quick_check` and immutable
migration-history validation.

Restore requires presentation-level confirmation and application-level proof
of that confirmation. Before touching live state, the adapter verifies the
manifest and hashes, copies every file to staging, validates the staged
database read-only, and checks workspace identity. It then swaps the four
managed components (`learning.db`, project config, metadata, and the complete
state directory) on the same filesystem. Any failed swap rolls back already
replaced components; if rollback itself cannot finish, the preserved originals
remain in a reported recovery directory rather than being deleted.

### Portable export and import boundary

`portability.Service` keeps archive policy independent of tar, gzip, SQLite,
and the operating system. The default human export selects visible Markdown
documents below the workspace root, including Foundation documents and
student-authored Markdown notes. Hidden paths, symlinks, non-Markdown files,
and all `.kelyro` internals are excluded. A full export contains the same
readable selection plus only `.kelyro/learning.db`, project `config.toml`,
`workspace.json`, and regular files below `.kelyro/state`. Global settings,
credential providers, environment values, logs, caches, backups, and unknown
machine internals have no path into either mode.

Exports are gzip-compressed tar archives with a JSON manifest as their first
entry. The manifest records format and schema versions, mode, app and workspace
identity, UTC creation time, ownership, relative POSIX path, size, and SHA-256
for every file. Export rejects paths that cannot round-trip safely on supported
platforms, including traversal, backslashes, drive syntax, Windows reserved
names, and case-insensitive collisions. Publication uses a same-directory
temporary file followed by rename and never intentionally replaces an existing
archive.

Import extracts only declared regular files into temporary staging, verifies
the complete manifest, entry set, sizes, hashes, workspace metadata, and any
SQLite database before creating or changing the destination. It rejects
absolute/traversing paths, duplicate entries, foreign separators, symlinked
destination components, undeclared files, and non-allowlisted machine state.
The explicit conflict strategies are `fail` (default), `keep`, and `overwrite`;
only `overwrite` authorizes replacement of different student-owned files. A
dry-run performs the same validation and conflict preflight without writing to
the destination. Real imports stage each replacement and retain originals for
rollback until the complete commit succeeds. Full imports recreate empty
managed directories required by workspace validation, and successful imports
into a valid workspace append `import.completed` to the audit trail.

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
