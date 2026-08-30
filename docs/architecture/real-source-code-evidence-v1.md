# Real Source Code Evidence v1

Step 31 makes implementation evidence reproducible instead of treating a
mutable repository page as a stable citation. The contract is identified by
`source-code-evidence-v1` and is part of a bounded `Evidence` record.

## Contract

`SourceCodeLocator` records reviewed, host-neutral metadata:

```text
repository locator
commit-pinned permalink
7–64 hexadecimal commit
clean repository-relative path
bounded inclusive line range
optional symbol
required opaque version scope
optional reviewed license identifier/name/locator
algorithm version
```

The repository is the canonical `Source.Locator`. A forge adapter supplies the
permalink because hosts do not share URL construction or line-fragment syntax.
The domain requires the same host, a distinct permalink containing the exact
commit, and rejects symbolic revisions such as `main`. It does not hard-code
GitHub, GitLab, or another forge.

Paths use repository `/` semantics, not operating-system paths. Absolute,
backslash, traversal, and non-clean paths are rejected. Lines are positive,
ordered, and limited to 200 per evidence record. The existing Evidence excerpt
limit remains 8 KiB, so neither metadata nor excerpts become a source mirror.

The version scope is always explicit even when no release tag exists; adapters
may use an ecosystem version, edition, or reviewed commit-oriented scope
because `SourceVersion` is intentionally opaque. When the Source already has a
version, both values must agree.

License metadata is optional because absence cannot be invented. When present,
its identifier is required and may be an SPDX identifier or another reviewed,
host-neutral label. This metadata records attribution context; it is not a
license compatibility decision.

## Relationships and authority

New Evidence for a `source_code` Source must carry a valid SourceCodeLocator.
Other source kinds cannot carry it. Legacy evidence without the v1 record
remains readable but cannot be presented as reproducible source-code evidence.

Implementation evidence supports observed behavior. It does not replace a
normative specification or standard. Trust Policy v1 keeps Specification and
Standard at tier A and Source Code at tier B for language-specification use
cases. Conflicts remain visible through the existing resolver and verification
contracts.

Citation generation reads the persisted SourceCodeLocator and emits the exact
adapter-supplied permalink with `source_permalink`. It no longer accepts a
second ephemeral permalink that could disagree with the Evidence record.

## Persistence

Forward-only SQLite migration v38 adds bounded
`evidence.source_code_locator_json`. Canonical JSON is capped at 8 KiB;
non-empty metadata is allowed only when the referenced Source has physical kind
`source_code`. Repository writes also load the Source and validate the full
identity, repository, kind, and version relationship before append.

Existing rows receive an empty value and no fabricated commit, version, path,
line, symbol, or license. Corrupt or non-canonical stored metadata crosses the
adapter as `persistence_failure`. Memory and SQLite reads use defensive copies.

## Boundaries

Step 31 adds no forge network adapter, repository clone, Git command execution,
license inference, whole-file retention, AI extraction, curriculum compilation,
or Student Core mutation. Future adapters may target GitHub first, but their
consumer contract remains `SourceCodeLocator` rather than a provider SDK type.
