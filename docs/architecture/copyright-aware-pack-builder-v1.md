# Copyright-aware Learning Pack Builder v1

The v1 builder turns one validated compiler result and its frozen evidence into
a deterministic, distributable Learning Pack ZIP. It is an in-memory adapter:
it performs no network access, source fetching, installation, execution, or
Student Core writes.

## Emitted layout

The builder serializes only validated Kelyro curriculum/environment metadata,
`curriculum-build-info/v1`, `curriculum-evidence-report/v1`, the canonical
`EVIDENCE.md` projection, a minimal README/license declaration, optional
Kelyro-authored assets, and their license ledger. It then creates complete
sorted SHA-256 checksums and a byte-stable ZIP with normalized modes/times.

Every source represented by a frozen evidence set requires one absolute HTTP(S)
citation URL. Citations may omit excerpts; an included excerpt is valid UTF-8,
limited to 512 bytes, and bound to its exact SHA-256. Claim statements and
Research Evidence excerpts are not copied implicitly.

Optional assets must be explicitly declared Kelyro-authored original content,
live below `assets/`, be valid UTF-8/non-executable, and provide a license plus
copyright holder. `assets/licenses.json` records each path, license, holder,
authorship marker, and content hash in sorted order.

## Validation hardening

The ordinary pack validator enforces the builder's portable boundary even for
third-party archives:

- `EVIDENCE.md` and `assets/licenses.json` are required;
- `EVIDENCE.md` must byte-match the machine report's canonical projection;
- packs with Source Bundles require citation URLs;
- every asset requires an exact hash-bound license record, with no extra ledger
  entries;
- cache/raw/body/snapshot/transcript path segments, transcript/full-article
  filenames, retained-document extensions, and known structured fields such as
  `raw_body`, `cached_content`, `full_text`, or `transcript` are rejected.

These checks identify known retention patterns; they do not claim to determine
copyright ownership from arbitrary prose. The builder therefore also requires
the explicit authorship/license declarations at its input boundary.
