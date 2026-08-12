# Kelyro — Repository Instructions

## Project boundaries

- Kelyro is written in Go.
- Work only on the explicitly authorized implementation step. Do not implement future steps.
- Never store secrets in the repository.
- Do not add external dependencies without documenting why they are necessary.

## Before changing code

1. Read the requested step in `docs/implementation/I-01-foundation/PLAN.md`.
2. Read `docs/implementation/I-01-foundation/PROGRESS.md`.
3. Review `git status` and the latest relevant commits.
4. Inspect only the files required for the requested step.

## SDD workflow

For each step: understand the specification, propose a concrete plan, implement only that step, verify it, update the implementation documents, and commit the result.

- Do not mark a step complete while any applicable test, build, lint, verification, or acceptance criterion fails.
- Add tests for functional changes when reasonable.
- Do not overwrite human-owned code or files without explicit design confirmation.
- Use cross-platform path APIs; never concatenate paths manually with `/` or `\`.
- Keep the core independent of Bubble Tea, SQLite, GitHub, AI providers, and the operating system.

## Completing a step

- Mark its checkbox in `PLAN.md` and update `PROGRESS.md`.
- Leave the working tree clean with at least one coherent commit.
- Split clearly independent changes instead of creating giant commits.
- Use Conventional Commits.
