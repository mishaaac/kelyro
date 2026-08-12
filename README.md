# Kelyro

Kelyro is a local-first, cross-platform learning workspace. This repository is
currently in its foundation phase: it provides the executable, local workspace
lifecycle, layered configuration, secure secret references, and workspace-local
structured persistence needed to build the product incrementally.

## Status

Early pre-release (`v0.1.0-alpha.1` development line). The current executable
provides workspace initialization, global/project configuration, secret
management, and safe opening of Foundation documents; other Foundation commands
remain explicit placeholders.
It does not include an interactive TUI or learning features yet.

The canonical Go module path is `github.com/mishaaac/kelyro`.

## Target platforms

Kelyro targets Linux, macOS, and Windows on architectures supported by Go.

## Requirements

- Go 1.24 or newer

## Build

```sh
go build ./cmd/kelyro
```

Run the resulting binary with `./kelyro` on Linux or macOS, or
`kelyro.exe` on Windows.

Run `kelyro help` to list the available Foundation commands. Invoking `kelyro`
without a command enters the current TUI bootstrap placeholder.

Configuration is available through `kelyro config show`, `path`, `get`, and
`set`. Use `--global` or `--project` to choose a scope explicitly. Kelyro stores
ordinary settings only; API keys and other secrets do not belong in these TOML
files.

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

Build metadata can be injected without changing source code:

```sh
go build -ldflags "-X github.com/mishaaac/kelyro/internal/version.Version=v0.1.0-alpha.1 -X github.com/mishaaac/kelyro/internal/version.Commit=<commit> -X github.com/mishaaac/kelyro/internal/version.Date=<date>" ./cmd/kelyro
```

## Test

```sh
go test ./...
go vet ./...
```

Kelyro uses `modernc.org/sqlite` as its one direct external dependency. It
provides a `database/sql` SQLite driver implemented in pure Go, which keeps the
workspace database local without requiring CGO toolchains for cross-platform
builds. Database details remain isolated in `internal/storage/sqlite`.
