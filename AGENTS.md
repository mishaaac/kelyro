# Kelyro — Repository Instructions

## Project boundaries

- Kelyro is written in Go.
- Work only on the explicitly authorized implementation step. Do not implement future steps.
- Never store secrets in the repository.
- Do not add external dependencies without documenting why they are necessary.

## Before changing code

1. Identify the implementation or maintenance scope explicitly authorized by the user.
2. For I-02 regression work, read `docs/implementation/I-02-student-learning-core/PLAN.md` and `PROGRESS.md`.
3. Review `git status` and the latest relevant commits.
4. Inspect only the files required for the requested scope.

## Implementation status

- I-01 Foundation and I-02 Student & Learning Core are complete.
- I-02 is complete in source but is not yet part of a release newer than `v0.1.0-alpha.2`.
- Do not begin I-03 or any later implementation without its own specification and explicit authorization.

## I-02 compatibility boundaries

- Do not implement the Research Engine, Curriculum Compiler, AI providers, plugins, or the full Exercise Engine.
- Use deterministic, versioned fixtures for educational tests.
- Keep educational logic out of TUI and CLI handlers.
- Encapsulate and test every mastery, retention, streak, or scheduling algorithm; give formulas and policies explicit versions or configuration where appropriate.
- Never modify published migrations; add forward-only migrations.
- Keep the learning domain general and do not assume a single language, programming ecosystem, or subject area.

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
