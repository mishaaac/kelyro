# Kelyro

Kelyro is a local-first, cross-platform learning workspace. Its I-01 Foundation
is complete: the repository provides the executable, local workspace lifecycle,
layered configuration, secure secret references, and workspace-local structured
persistence needed to build the product incrementally.

## Status

I-03 Research & Source Intelligence is formally complete after controlled E2E,
opt-in live checks, real-source dogfooding, and hosted Linux/macOS/Windows CI
with Linux race coverage. The latest published prerelease remains
`v0.1.0-alpha.3`; it delivers the completed and manually accepted I-02 Student
& Learning Core on top of I-01 and does not contain I-03.

The Research layer now provides transport-neutral requests and runs,
topic-aware authority and trust, privacy-gated discovery/fetch/release ports,
bounded snapshots and evidence, provenance and citations, freshness,
verification and explicit conflicts, release/deprecation intelligence, source
bundles, offline cache, update/drift/impact reports, audit metadata, and
Research/Sources CLI and TUI transparency. It intentionally has no configured
public search provider, production Curriculum Compiler, production Learning
Pack, AI Research Reviewer, or automatic curriculum/student-state migration.
See the [I-03 progress record](docs/implementation/I-03-research-source-intelligence/PROGRESS.md)
and [dogfooding report](docs/implementation/I-03-research-source-intelligence/DOGFOOD.md).

The executable also provides workspace
initialization, global/project configuration, secret management, safe opening
of Foundation documents, portability and recovery operations, and an
interactive TUI with resumable onboarding, a learning goal, mastery policy, an
optional deterministic diagnostic, and fixture-backed initial curriculum
state, persistent mistake/session lifecycles, and `history`/`time` inspection.
The core includes retention, review scheduling, warm-ups, non-punitive streaks,
achievements, explainable analytics, deterministic adaptive daily planning, and
a coherent height-aware dashboard presented through Home, Today, Progress,
Concept, Roadmap, Reviews, History, Goal, and Profile views. Exercise work and
generated exercise content remain pending. The same read model powers the
human-readable Student Core CLI and explicit `kelyro progress export` Markdown
snapshot. Advanced maintenance can preview or apply a backup-protected rebuild
of derived learning state with `kelyro maintenance recalculate --dry-run` or
`kelyro maintenance recalculate`. Student Core persistence performs
cross-aggregate integrity checks and is regression-tested with a 2,000-concept,
6,000-evidence deterministic fixture. The completion record and known
limitations live in
[I-02 PROGRESS](docs/implementation/I-02-student-learning-core/PROGRESS.md),
with published scope and limitations in the
[`v0.1.0-alpha.3` release notes](docs/releases/v0.1.0-alpha.3.md).

The canonical Go module path is `github.com/mishaaac/kelyro`.

## Target platforms

Kelyro release archives target Linux, macOS, and Windows on `amd64` and
`arm64`. See [docs/releases.md](docs/releases.md) for manual installation,
checksum verification, reproducible local packaging, and the maintainer release
procedure.

## Requirements

- Go 1.24 or newer

## Build

```sh
go build ./cmd/kelyro
```

Run the resulting binary with `./kelyro` on Linux or macOS, or
`kelyro.exe` on Windows.

Run `kelyro help` to list the available commands. Invoking `kelyro` without a
command opens the TUI from a Kelyro workspace. An incomplete learner setup opens
automatically and resumes its last durable checkpoint. After setup, Home shows
the active goal, completion, today's next work, reviews, mastery requirement,
streak, and weekly study time. Visible keyboard shortcuts open the Student Core
views; `q` exits and `Esc` returns Home. Set `NO_COLOR` or pass `--no-color` to
disable color.

Configuration is available through `kelyro config show`, `path`, `get`, and
`set`. Use `--global` or `--project` to choose a scope explicitly. Kelyro stores
ordinary settings only; API keys and other secrets do not belong in these TOML
files.

Kelyro is local-first and Foundation works offline. SQLite, Markdown,
configuration, logs, backups, and exports stay on the local machine; there is
no automatic telemetry or hidden HTTP request. Network use is opt-in through
`privacy.allow_network`, which defaults to `false`. Future AI content transfer
and usage telemetry additionally require `privacy.allow_ai_content` and
`privacy.allow_usage_telemetry`, both also `false` by default. Every future
network-capable component, including plugins and update checks, must use the
same deny-by-default privacy gate.

Run `kelyro update check` for an explicit version-aware release check. Checks
use `updates.channel = "stable"` by default; set it to `"prerelease"` to opt
into prerelease metadata, or set `updates.check = false` to disable checks.
Results are cached in the native user cache for 24 hours. A refresh occurs only
when network access is allowed, and offline or temporarily unavailable metadata
is reported without breaking other commands. Kelyro never checks automatically
as a side effect of ordinary commands, and the test suite uses fake providers
without external requests. Kelyro never downloads or installs an update
silently. `kelyro update` remains intentionally unsupported until release
artifacts can be verified by checksum/signature and installed with explicit
consent.

The TUI and the `status`, `progress`, `roadmap`, `today`, `doctor`, `config`,
`backup`, and `export` commands remain local. Stored Research evidence,
snapshots, bundles, conflicts, freshness and cache inventory also remain
available offline. Live Research discovery, fetch, release lookup and update
providers require the same explicit network permission. `kelyro roadmap` renders the
active curriculum, while `kelyro progress export` safely regenerates
`LEARNING.md`, `00-roadmap/ROADMAP.md`, and `00-roadmap/PROGRESS.md` from local
state. Documentation links shown by Doctor are never fetched or opened
automatically. Privacy denials are written to the workspace-local
diagnostic log; their authorization metadata contains no URL, user path,
student content, prompt, or secret.

Secrets are managed with `kelyro secrets status`, `set <name>`, and
`delete <name>`. Manual values are read without terminal echo and are stored in
the operating-system credential service: Secret Service/libsecret on Linux,
Keychain on macOS, or Credential Manager on Windows. Linux requires
`secret-tool` and an active Secret Service session. In headless environments,
set `KELYRO_SECRET_<NAME>` instead; for example, the name `provider.token` maps
to `KELYRO_SECRET_PROVIDER_TOKEN`. Environment variables take precedence and
are never copied into Kelyro files. Status and configuration output show only
`configured` or `not configured` plus the reference name.

Use `kelyro open` to open `LEARNING.md` or `kelyro open roadmap` to open the
Foundation roadmap. Kelyro honors `editor.command`, detects `code`, `nvim`,
`vim`, `zed`, or `cursor`, and finally tries the operating system's default
file opener. Configure an executable name or complete executable path, not a
shell command; for example, `kelyro config set editor.command code`. The
`editor.prompt` boolean controls whether the future interactive TUI offers an
open-after-generation prompt; it defaults to `true` and does not affect the
explicit CLI command.

Create a readable Markdown archive with `kelyro export`, or add the allowlisted
workspace database, project configuration, identity, and session state with
`kelyro export --full`. Use `--output <file>` to choose the archive path. Before
importing, `kelyro import <file> --dry-run` validates the entire archive and
reports conflicts without changing the destination. Imports fail on different
existing files by default; choose `--conflict keep` to preserve them or
`--conflict overwrite` to explicitly authorize replacement. Secrets, logs,
caches, and nested backups are excluded from portable exports.

Build metadata can be injected without changing source code:

```sh
go build -ldflags "-X github.com/mishaaac/kelyro/internal/version.Version=v0.1.0-alpha.1 -X github.com/mishaaac/kelyro/internal/version.Commit=<commit> -X github.com/mishaaac/kelyro/internal/version.Date=<date>" ./cmd/kelyro
```

For distributable archives, use `go run ./tools/release build` as documented in
`docs/releases.md`. It builds all six supported OS/architecture targets with
CGO disabled, embeds canonical release metadata, and writes `SHA256SUMS`.

## Test

```sh
go run ./tools/quality all
```

The Go-native quality runner requires no Make installation and works from the
repository root on Linux, macOS, and Windows. The `all` gate runs unit tests,
the isolated Foundation, Student Core, and controlled Research Engine E2E
suites, `vet`, race detection, a CLI build, and `--version`/`--help` smoke
tests. Individual gates are also available as `test`, `e2e`, `vet`, `race`,
and `build-smoke`. The E2E gate builds the real CLI with deterministic fake
secret/release adapters and a loopback-only Research fixture, so it never
contacts a host keychain or the public Internet. Public-source adapter checks
remain a separate explicit opt-in suite.

GitHub Actions runs tests, Foundation/Student Core/Research E2E, `vet`, the
build, and smoke tests on Linux, macOS, and Windows. The required race gate runs
on Linux because Go's race detector requires CGO and a supported C toolchain;
contributors on other platforms can run `go run ./tools/quality race` when
their local toolchain supports it.

Kelyro uses three direct external dependencies. `modernc.org/sqlite` provides a
pure-Go `database/sql` SQLite driver, keeping the workspace database local
without requiring CGO toolchains. Bubble Tea drives the interactive terminal
lifecycle and message loop, while Lip Gloss provides terminal-safe styling and
display-width measurement. Database details remain isolated in
`internal/storage/sqlite`; both presentation dependencies remain isolated in
`internal/tui`.
