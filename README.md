# Kelyro

Kelyro is a local-first, cross-platform learning workspace. This repository is
currently in its foundation phase: it provides the executable, local workspace
lifecycle, layered configuration, secure secret references, and workspace-local
structured persistence needed to build the product incrementally.

## Status

Early pre-release (`v0.1.0-alpha.1` development line). The current executable
provides workspace initialization, global/project configuration, secret
management, safe opening of Foundation documents, and an interactive Foundation
TUI. Learning features are not implemented yet.

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

Run `kelyro help` to list the available Foundation commands. Invoking `kelyro`
without a command opens the Foundation TUI from a Kelyro workspace. Its Home,
Doctor, Config, and Roadmap screens use visible keyboard shortcuts; `q` exits and
`Esc` returns Home. Set `NO_COLOR` or pass `--no-color` to disable color.

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

The TUI and the `status`, `roadmap`, `doctor`, `config`, `backup`, and `export`
commands remain local. `kelyro roadmap` prints the location of the generated
local `ROADMAP.md`; documentation links shown by Doctor are never fetched or
opened automatically. Privacy denials are written to the workspace-local
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
the isolated Foundation lifecycle E2E suite, `vet`, race detection, a CLI
build, and `--version`/`--help` smoke tests. Individual gates are also available
as `test`, `e2e`, `vet`, `race`, and `build-smoke`. The E2E gate builds the real
CLI with deterministic fake secret and release-provider adapters, so it never
contacts a host keychain or the network.

GitHub Actions runs tests, Foundation E2E, `vet`, the build, and smoke tests on
Linux, macOS, and Windows. The required race gate runs on Linux because Go's
race detector requires CGO and a supported C toolchain; contributors on other
platforms can run `go run ./tools/quality race` when their local toolchain
supports it.

Kelyro uses three direct external dependencies. `modernc.org/sqlite` provides a
pure-Go `database/sql` SQLite driver, keeping the workspace database local
without requiring CGO toolchains. Bubble Tea drives the interactive terminal
lifecycle and message loop, while Lip Gloss provides terminal-safe styling and
display-width measurement. Database details remain isolated in
`internal/storage/sqlite`; both presentation dependencies remain isolated in
`internal/tui`.
