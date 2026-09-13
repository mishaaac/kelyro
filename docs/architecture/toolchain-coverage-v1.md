# Toolchain Coverage v1

`toolchain-coverage-v1` verifies that the real tools declared necessary for a
goal are supplied by exact Environment Pack versions with complete curriculum
metadata.

## Declaration and resolution

`ToolchainCoverageRequirement` identifies a tool ID, purpose through the
existing Environment Pack `ToolRequirement`, minimum required/recommended/
optional level, exact Environment Pack ID/version, reason, and evidence. Core
does not contain a built-in inventory of Git, runtimes, editors, containers,
databases, debuggers, formatters, linters, or any other tool.

Resolution uses the exact `EnvironmentPackReference`; another installed or
available version is not silently substituted. The resolved `ToolRequirement`
must provide:

- a non-empty purpose;
- a level at least as strong as the declared minimum;
- a `when introduced` Concept present in the curriculum;
- one or more platform-specific notes through the portable `Platforms` field;
- evidence refs resolving to accepted I-03-derived Claims.

Required is stronger than recommended, which is stronger than optional. A
stronger Environment Pack declaration may satisfy a weaker minimum, never the
reverse.

## Results and Coverage bridge

A missing pack or missing tool is `missing`. A resolved tool with incomplete
metadata is `partial`, with stable field codes such as `when_introduced`,
`introduction_concept`, `requirement_level`, `platform_notes`, and `evidence`.
Only a complete tool is `covered` and emits a generic `toolchain` Coverage
support. The result preserves the exact environment pack reference and a
defensive copy of the resolved tool metadata.

Requirements, tools, platforms, evidence, results, and supports are ordered
deterministically. The pass does not inspect the host machine, install tools or
packs, execute commands, access the network, store secrets, generate lessons,
or mutate student state.
