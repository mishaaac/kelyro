# Learning Pack and Curriculum Compiler threat model

Step 52 defines the untrusted-input boundary for `learning-pack/v1`. A pack can
be supplied by an unknown author and its filename, archive metadata, manifest,
curriculum, evidence, environment metadata, assets and human-readable files
must therefore be treated as data. Validation never executes pack content,
performs research, installs tools, resolves links in a renderer, or grants a
pack access to learner state.

## Threats and controls

| Threat | Fail-closed control |
| --- | --- |
| Archive traversal and path injection | Entry names are bounded canonical relative UTF-8 paths. Absolute/traversal paths, controls, Windows-invalid characters, trailing dots/spaces and device names are rejected. |
| Symlink or file-swap escape | Root and entry symlinks are rejected. Directory entries are resolved inside the root, opened as regular files, compared by file identity before reading, checked again for size changes, and the root identity is rechecked after traversal. |
| Executable or active content | Executable modes and script, binary-package, shared-library, WebAssembly and active SVG extensions are rejected. Pack files are never executed. |
| Malicious YAML/JSON | Documents are byte-bounded, UTF-8, strict-schema, single-document/value inputs. Unknown fields, duplicate keys and trailing data fail. Duplicate-key JSON scanning uses an explicit stack rather than recursive descent. |
| Checksum ambiguity or spoofing | `checksums.txt` is line-bounded, entry-count-bounded, canonical, sorted and duplicate-free. Every non-inventory file has exactly one lowercase SHA-256 and no unlisted file is accepted. SHA-256 provides integrity, not publisher identity. |
| ZIP bomb or filesystem exhaustion | Entry count, path length, per-file bytes, aggregate bytes and exact compression ratio are bounded. Directory entries, including empty directories, consume the same entry budget. Declared and actually read directory bytes are both checked. |
| Terminal/control injection | Decoded curriculum identifiers and display text reject Unicode controls. Raw pack text rejects controls other than LF/TAB formatting before parsing or rendering. |
| Unsafe Markdown/HTML | Markdown outside fenced code rejects raw HTML, active `javascript:`, `vbscript:`, `data:` and `file:` destinations, and embedded images that could trigger external loads. Canonical evidence Markdown still escapes source-derived display text. |
| Secret-bearing evidence links | Citation URLs are length-bounded HTTP(S), reject userinfo and credential-like query/fragment parameters, and retain safe deep links without fetching them. |
| Dependency confusion/cycles | Manifest IDs and SemVer constraints are strict. The deterministic dependency resolver selects only available compatible versions and rejects missing, incompatible and cyclic graphs before installation. |
| Compiler instruction injection | The compiler has no AI, shell, plugin or network capability. Source/pack strings remain validated data and cannot alter compiler passes or policy. |

## Integrity and future authenticity

The checksum inventory binds all accepted bytes and the resulting content hash
identifies that immutable inventory. It does not prove who authored the pack.
`curriculum/application.SignatureVerifier` is the optional authenticity seam:
it accepts a payload hash, signature and key ID and returns a structured
verification result. Learning Pack v1 remains safe when no signing ecosystem is
configured; marketplace signing, trust roots and key distribution are future
work and must not be inferred from a matching checksum.

## Resource policy

The limits protect the file boundary, not the curriculum domain. Kelyro still
imposes no maximum number of concepts, lessons, modules or prerequisite edges.
A larger curriculum can use a future pack schema or adjusted reviewed file
budgets; it is never silently truncated.

Current v1 limits are 1,024 filesystem/archive entries, 1,024 bytes per path,
16 MiB per file, 32 MiB total uncompressed content and a 100:1 per-entry ZIP
compression ratio.

## Verification

Deterministic regressions cover traversal, symlinks, Windows device/drive paths,
oversized paths and manifests, executable/active extensions, compression bombs,
directory-entry exhaustion, checksum mismatch, malformed/duplicate JSON,
unknown YAML, decoded terminal controls, secret-bearing citations, raw HTML,
active Markdown links and embedded images. Fuzz targets exercise the manifest
and duplicate-key JSON parsers without network access.
