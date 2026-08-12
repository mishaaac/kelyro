# Kelyro releases

Kelyro publishes versioned archives for Linux, macOS, and Windows on `amd64`
and `arm64`. Release builds disable CGO and embed the version, source commit,
and reproducible build date. `SHA256SUMS` detects accidental corruption; it is
not a cryptographic signature or proof of publisher identity.

Release-specific scope and limitations are recorded under [`docs/releases/`](releases/).
The current Foundation pre-release is
[`v0.1.0-alpha.2`](releases/v0.1.0-alpha.2.md); the initial Foundation release
remains documented as [`v0.1.0-alpha.1`](releases/v0.1.0-alpha.1.md).

## Manual installation

1. Download the archive for the operating system and architecture plus
   `SHA256SUMS` from the same GitHub release.
2. Verify the archive from the directory containing both files. On Linux,
   select its line from the checksum manifest:

   ```sh
   grep '  kelyro_0.1.0_linux_amd64.tar.gz$' SHA256SUMS | sha256sum --check
   ```

   Replace the example filename with the downloaded archive. On macOS, use the
   same filter with `shasum -a 256 --check`. On Windows PowerShell, compare
   `(Get-FileHash .\kelyro_<version>_windows_<arch>.zip -Algorithm SHA256).Hash`
   with the matching line in `SHA256SUMS`.
3. Extract `kelyro` (`kelyro.exe` on Windows) and move it into a directory on
   `PATH`. No installer or CGO runtime is required.
4. Run `kelyro --version` and confirm that its version and commit match the
   release page.

The archive names are stable:

```text
kelyro_<version>_linux_<arch>.tar.gz
kelyro_<version>_darwin_<arch>.tar.gz
kelyro_<version>_windows_<arch>.zip
```

## Reproducing artifacts locally

Use the tagged commit date as the fixed build date and an empty output
directory:

```sh
tag=v0.1.0
commit=$(git rev-list -n 1 "$tag")
build_date=$(git show -s --format=%cI "$commit")
go run ./tools/release build \
  --version "$tag" \
  --commit "$commit" \
  --date "$build_date" \
  --output dist
```

The Go-native builder uses `-trimpath`, disables VCS auto-stamping and CGO,
clears the linker build ID, fixes archive timestamps, and sorts checksum output.
Given the same source tree, Go toolchain, dependency graph, and inputs, it emits
byte-identical archives. Pinning the Go toolchain remains necessary when
comparing artifacts made on different machines.

## Maintainer release procedure

Kelyro follows SemVer during `0.x` and Conventional Commits. A release must be
made from a clean commit reachable from `main`, with all quality checks green.

1. Select a canonical SemVer version such as `v0.2.0` and review the commits
   included since the previous release, calling out every breaking change.
2. From a clean, current `main`, create and push an annotated tag:

   ```sh
   git status --short
   git tag -a v0.2.0 -m "Kelyro v0.2.0"
   git push origin v0.2.0
   ```

3. The release workflow independently verifies that the tag is annotated, its
   target is reachable from `origin/main`, the checkout is clean, the version
   is valid, and the full Linux/macOS/Windows quality matrix passes.
4. The workflow creates a GitHub **draft** release with generated notes,
   archives, and checksums. A maintainer must review and edit those notes,
   verify breaking changes and artifacts, and explicitly publish the draft.

The workflow refuses to overwrite a published release. Release publication is
therefore never an automatic consequence of pushing a branch or tag.
