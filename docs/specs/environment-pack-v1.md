# Environment Pack Format v1

## Purpose

`environment-pack/v1` separates tool and platform requirements from the
conceptual curriculum. It is declarative metadata inside an optional Learning
Pack entry; it is not an installer, shell script, plugin, credential store, or
permission to execute pack-authored commands.

The format and its phase-aware Doctor integration are complete in Kelyro
`v0.3.0-alpha.1`.

## Document

`environment/environment.yaml` is one strict UTF-8 YAML document:

```yaml
schema_version: environment-pack/v1
id: environment.go-backend
version: 1.0.0
platform_support: [linux, darwin, windows]
install_guidance:
  - id: install.go.linux
    platform: linux
    source_name: Go project
    official_url: https://go.dev/doc/install
    instructions: Follow the official archive or package guidance.
    evidence_refs:
      - bundle_id: bundle.go
        claim_id: claim.go-install
tools:
  - id: go
    display_name: Go
    purpose: Compile and test Go programs.
    minimum_version: 1.24.0
    level: required
    introduced_at: concept.go-toolchain
    when_needed: concept.go-first-program
    platforms: [linux]
    install_guidance_refs: [install.go.linux]
    evidence_refs:
      - bundle_id: bundle.go
        claim_id: claim.go-toolchain
```

Unknown fields, duplicate YAML keys, multiple documents, invalid UTF-8, and
content beyond the Learning Pack loader limits are rejected.

## Platforms and tools

The closed platform vocabulary is `linux`, `darwin`, and `windows`. Every tool
platform must be included in `platform_support`. Tool levels are `required`,
`recommended`, and `optional`.

`minimum_version` uses strict SemVer syntax without `v`. `introduced_at` is the
Concept that teaches the tool or its underlying workflow. `when_needed` is the
first Concept where the tool becomes relevant. Both must resolve within the
containing curriculum. Doctor integration rejects a requirement timeline where
the tool is needed before it is introduced.

Tool IDs are declarative references. Doctor may probe only tools already known
to its trusted executable registry; pack content cannot add executable names or
version commands.

## Official installation guidance

Each supported tool platform has at least one referenced guidance record.
Guidance provides a source name, explanatory instructions, evidence refs, and
a query-free HTTPS official URL without credentials or fragments. URLs are
metadata for display and are never opened automatically.

Declaring a source as official remains untrusted pack data until pack evidence
and review validate it. Environment Packs contain no tokens, passwords,
environment-variable values, inline commands, package-manager invocations, or
automatic-install instructions.

## Validation boundaries

The domain exposes basic validation separately from strict portable validation.
This lets Toolchain Coverage report missing metadata from compiler inputs while
the Learning Pack loader rejects the same omissions in a publishable file.

Environment versions are independent from Kelyro and Learning Pack versions.
The entry remains checksum-covered and immutable with its containing pack.
