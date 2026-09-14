# Learning Pack installation and activation v1

## Boundary

`pack-installation-policy-v1` installs only artifacts accepted by the existing
`learning-pack/v1` validator. Installation never fetches research, executes a
pack entry, installs tools, mutates learner state, or treats catalog metadata
as trusted content.

The validator produces a canonical ZIP snapshot from the validated logical
entries. Its content identity is `sha256(checksums.txt)`: the checksum document
is strict, sorted, and covers every other regular entry. The installer passes
that snapshot and identity through the application repository port so the
source cannot change between validation and persistence.

## Global immutable store

Installed versions live once per user below the Foundation global configuration
directory:

```text
<global-config>/kelyro/packs/<pack-id>/<version>/
├── pack.zip
└── installation.json
```

The repository writes a private staging directory, validates the stored ZIP
again, and atomically renames it into place. Existing `(pack-id, version)`
directories are never overwritten. Reinstalling the exact same content is an
idempotent application result; the same identity with another content hash is
a conflict. Reads revalidate the archive and compare its ID, version, and hash
to `installation.json`, so corruption is reported rather than silently used.

Before an install or activation, `pack-dependency-resolver-v1` must resolve the
complete required dependency graph from versions already installed locally.
V1 does not download a missing dependency and does not support optional
dependencies.

## Workspace activation

Activation stores only this machine-owned reference:

```text
<workspace>/.kelyro/state/active-pack.json
```

The reference includes pack ID, version, validated content hash, and activation
time. The global archive is not copied into the workspace. Each workspace can
therefore select its own active version while sharing immutable pack bytes.
Activation requires a structurally valid Foundation workspace and an installed,
dependency-complete pack. It does not change mastery, evidence, or any other
I-02 learner record.

## CLI

The supported commands are:

```text
kelyro packs validate <path>
kelyro packs install <path>
kelyro packs list
kelyro packs show <id>
kelyro packs activate <id>@<version>
```

`--workspace` controls activation discovery; otherwise discovery starts at the
current directory. List and show read the global store and remain independent
of workspace selection.
