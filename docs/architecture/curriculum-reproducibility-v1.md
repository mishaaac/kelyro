# Curriculum build reproducibility metadata v1

`curriculum-build-info/v1` records the exact content-addressed recipe for one
successful `curriculum-compiler-v1` result. The metadata is part of the
immutable compilation record and every portable Learning Pack carries it at
the manifest-declared `build_info_entry`.

## Recorded contract

The record contains:

- compiler version and every ordered pass name/version;
- exact Source Bundle IDs, canonical SHA-256 hashes, ingestion algorithm
  versions, and verification timestamps;
- the complete compilation config, including source policy and target pack
  schema;
- canonical input and final compiled-artifact SHA-256 hashes;
- the deterministic UTC build timestamp supplied by immutable build metadata.

Pass durations remain diagnostic observations and do not enter these hashes.
The same frozen request/config therefore emits the same input/output hashes and
ordered version list even when wall-clock execution time differs.

## Validation and persistence

Compilation records fail closed when build metadata is absent. Validation
cross-checks the first/final pass hashes, every pass version, compilation
config, pack schema, and the ordered Source Bundle set against the compiled
curriculum. The portable loader strictly decodes the JSON document, rejects
unknown or duplicate fields, and repeats the source/schema checks before a pack
can be installed.

`kelyro curriculum build-info` resolves the workspace's active immutable pack
and renders this metadata. It performs no compilation, network access, pack
mutation, or Student Core write.
