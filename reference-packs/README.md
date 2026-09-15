# Kelyro development reference packs

These artifacts exercise the I-04 compiler and Learning Pack lifecycle. They
are development fixtures, not production-complete curricula.

`backend-go-reference.zip` is generated deterministically from
`internal/infra/referencepack`. Its frozen evidence set is intentionally
marked with a caveat and must be replaced by production I-03 Source Bundles
before anyone claims professional coverage.

Regenerate it from the repository root:

```bash
go run ./internal/infra/referencepack/cmd reference-packs/backend-go-reference.zip
```

Validate it through the public lifecycle:

```bash
go run ./cmd/kelyro packs validate reference-packs/backend-go-reference.zip
```
