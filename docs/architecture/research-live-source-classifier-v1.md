# Live Source Classifier v1

`source-classifier-v1` closes the boundary between an untrusted discovery
candidate and the Source kind used by trust policy. Search rank, provider title,
snippet, and publication hint are not inputs. A Source is classified only after
its body has been fetched, snapshotted, and successfully normalized.

The classifier is deterministic and performs no I/O. Its inputs are the
registered Source identity plus the matching `NormalizedSource`; its output is
`SourceID`, `SourceKind`, and algorithm version. Existing reviewed kinds are
preserved. Only `SourceOther` may receive an automatic kind.

## Rules

Canonical host and path provide the primary signal:

- `go.dev`, `golang.org`, and `tip.golang.org` distinguish specifications,
  release notes, official blog, official tutorial, source code, and other
  official documentation by stable path families;
- `pkg.go.dev` is a package reference;
- explicit repository issue/pull paths are issue trackers and blob/tree paths
  are source code;
- known forum, video, and paper hosts receive their corresponding kind;
- other successfully normalized HTML, XHTML, plain text, or Markdown is a
  community article;
- unsupported binary content remains `other`.

Classification does not grant trust. The existing trust, freshness,
verification, conflict, and bundle policies still make their own decisions.
The current v1 host list is deliberately closed; extending it requires a new
reviewed rule and deterministic tests.

Redirects whose final locator differs from the registered discovery identity
are retained as partial normalization failures. V1 neither rewrites Source
identity nor invents an alias; a future alias workflow may reconcile them.

The live normalization stage persists each changed kind through
`SourceService.ClassifyKind` and carries the same classified Sources into all
downstream stages. SQLite updates only the current Source row; immutable
snapshots, Evidence, citations, and previous bundles are not rewritten.
