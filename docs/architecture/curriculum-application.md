# Curriculum Compiler application boundaries

## Purpose

`internal/curriculum/application` separates I-04 use cases from SQLite,
filesystem/archive formats, I-03 persistence, signatures, CLI, and TUI. It
depends on the curriculum domain and, only at the read-only Research adapter
boundary, on I-03 domain records.

Step 2 defines contracts and deterministic fakes. It does not implement the
compiler, pack parsing/validation, installation, upgrades, coverage algorithms,
audits, persistence, or presentation handlers.

## Repository ports

The five ports are split by aggregate and use case:

- `CurriculumRepository` adds immutable definitions and reads exact
  `(CurriculumID, CurriculumVersion)` identities.
- `PackRepository` adds immutable pack versions and stores the explicit active
  pack selection separately.
- `PackCatalogRepository` replaces and reads discovery manifests only. Catalog
  presence is not trust, validation, installation, or activation.
- `EnvironmentPackRepository` stores exact immutable environment-pack versions.
- `CompilationRepository` appends compilation records and reads them by stable
  record or curriculum identity.

`CompilationRecord` binds validated input, config, result, and UTC creation
time. Its input goal must match the compiled curriculum goal.

Published-version immutability is represented through `Add`/`Append` rather
than an update operation. A duplicate exact identity is a conflict.

## Service contracts

The application surface names the future use cases explicitly:

```text
CurriculumCompilerService
PackService
PackValidationService
PackInstallService
PackUpgradeService
CoverageService
CurriculumAuditService
```

Requests and results are transport-neutral. Validation issues carry stable
codes plus bounded path/message fields. Upgrade and installation shapes do not
authorize writes on their own; their behavior, safety gates, backups, and
migration policies remain in later steps.

No service contract contains learner mastery or a direct Student Core write.

## External adapter contracts

`ResearchBundleProvider` reads exact durable `SourceBundle` and `Claim` records
from I-03. Implementations must not discover, fetch, refresh, or otherwise use
the network. Step 6 will own verified-bundle ingestion and eligibility mapping.

`Clock` returns a domain UTC timestamp. `Filesystem` exposes readers and
metadata without importing OS APIs into the domain. `PackArchiveReader` exposes
bounded entry metadata and readers without selecting ZIP/TAR here. Security
limits, safe paths, duplicate handling, and archive-bomb defenses belong to the
pack-loader steps.

`SignatureVerifier` is an optional future hook over a payload hash, signature,
and key identity. No signing ecosystem or trust conclusion is required by Step
2, and the signature bytes are not credentials.

## Error contract

Application errors preserve causes and classify failures as:

```text
not_found
conflict
invalid_state
unavailable
persistence_failure
external_failure
```

Context cancellation and deadlines map to `unavailable`. Repository and
external boundary helpers preserve an existing classification rather than
flattening it. Presentation code can branch with `errors.Is` or `KindOf`
without parsing messages.

## In-memory fakes

`internal/curriculum/application/memory` supplies narrow wrappers over one
mutex-protected store for all five repository ports. The fake:

- validates writes;
- rejects duplicate immutable versions/records;
- returns `not_found` distinctly;
- honors cancelled contexts;
- sorts list results deterministically by stable identity/version;
- stores and returns defensive deep copies, including nested evidence,
  hierarchy, environment, pass, gap, and warning slices;
- never performs filesystem, network, SQLite, UI, AI, or Student Core work.

The fake is a test utility, not a production persistence implementation.
Forward-only SQLite schema and adapters begin in Step 3.
