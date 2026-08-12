# Kelyro

Kelyro is a local-first, cross-platform learning workspace. This repository is
currently in its foundation phase: it provides only the minimal executable and
project conventions needed to build the product incrementally.

## Status

Early pre-release (`v0.1.0-alpha.1` development line). The current executable
exposes project help and build-version information; it does not include a TUI
or learning features yet.

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

Build metadata can be injected without changing source code:

```sh
go build -ldflags "-X github.com/mishaaac/kelyro/internal/version.Version=v0.1.0-alpha.1 -X github.com/mishaaac/kelyro/internal/version.Commit=<commit> -X github.com/mishaaac/kelyro/internal/version.Date=<date>" ./cmd/kelyro
```

## Test

```sh
go test ./...
go vet ./...
```

Kelyro currently uses only the Go standard library, so `go.sum` is empty until
the project requires an external module.
