# Secure Learning Pack loader v1

## Boundary

`internal/infra/learningpack.Validator` is the read-only adapter behind
`curriculum/application.PackValidationService`. It accepts a directory or ZIP
path and returns a `PackValidationResult`. Invalid untrusted content is reported
as structured validation issues; cancellation remains an operational error.

The validator never installs or activates a pack, resolves dependencies,
performs research, follows links, executes content, or mutates learner state.

## Validation pipeline

The v1 pipeline is deterministic:

1. classify the root with `Lstat` and reject root links/special files;
2. enumerate a directory without following links, or enumerate one ZIP;
3. validate canonical portable names and reject links, special files,
   executable modes, active/script-like extensions, controls, Windows device
   names and platform-invalid path characters;
4. enforce 1,024 filesystem/archive entries, 1,024 bytes per path, 16 MiB per
   file, 32 MiB total uncompressed and an exact 100:1 per-entry ZIP
   compression-ratio ceiling;
5. require valid UTF-8, `pack.yaml`, and `checksums.txt`;
6. verify a complete, sorted, duplicate-free SHA-256 inventory;
7. strictly decode the manifest, curriculum, evidence report, build info and optional
   environment document;
8. validate domain aggregates and all cross-document identities/references.

The directory adapter counts files and directories, resolves each discovered
file inside the root, compares the opened regular file with the enumerated path
and rechecks file/root identity and size. The ZIP adapter rejects traversal
names and duplicate entries before opening content. Resource bounds are checked
from metadata and again while reading. Builder-owned in-memory entries pass
through the same entry-count, per-file, and total-byte checks before an archive
is returned.

All accepted text is terminal-control-safe. Markdown outside fenced code rejects
raw HTML, active non-HTTP schemes and embedded images. JSON duplicate-key
inspection is iterative so deeply nested untrusted input does not consume the
goroutine stack.

## Cross-document invariants

- manifest `curriculum_id` equals the curriculum entry ID;
- manifest entry paths exist and are distinct;
- evidence report bundle refs equal the curriculum's ordered, immutable bundle
  refs, including hash, algorithm and verification time;
- every Claim ref used by goals, competencies, concepts, prerequisites,
  coverage requirements or environment tools appears in the evidence report;
- evidence report goal/counts and compiler/pass versions match the curriculum
  and build-info entry;
- canonical `EVIDENCE.md`, citation URL/excerpt bounds, the asset license
  ledger, and known forbidden source-retention paths/fields are checked before
  a portable snapshot is accepted;
- environment concept and bundle refs resolve in the curriculum;
- the complete `LearningPack` passes domain validation.

Dependency constraints are syntactically validated here. Availability,
version selection and cycle resolution belong to later install/resolution
steps. Non-current pack status is a warning rather than a format error because
preview, experimental, legacy, historical and deprecated are explicit valid
v1 states.

The complete threat review, compiler non-execution boundary and distinction
between checksum integrity and future publisher authenticity are recorded in
[Learning Pack and Curriculum Compiler threat model](../security/learning-pack-threat-model.md).

## CLI

`kelyro packs validate <path>` invokes only this read-only service. A valid
pack exits zero and prints stable pack/curriculum identities. Validation issues
exit non-zero with code, path and reason. `--quiet` suppresses successful output
but never suppresses errors.
