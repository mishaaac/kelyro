# Student learning Markdown artifacts

Step 28 exposes the Student Core's current state as inspectable workspace
documents while preserving SQLite as the only source of truth. The export is
explicit:

```text
kelyro progress export
```

The command discovers the workspace, builds one coherent
`progress-dashboard/v1` read model, renders all documents in memory, and then
writes them through Foundation's ownership-aware artifact store. Rendering has
no database or filesystem access and the Markdown is never parsed back into the
learning domain.

## Documents

| Path | Human-facing content |
| --- | --- |
| `LEARNING.md` | active goal, current curriculum location, today's plan, effective mastery requirement, and due-review count |
| `00-roadmap/ROADMAP.md` | ordered curriculum hierarchy, concept status, known mastery, and human lock reasons |
| `00-roadmap/PROGRESS.md` | study-time windows, concept counts, completion, reviews due, streak metrics, and the latest milestone |

An incomplete setup produces concise actionable empty states rather than
inventing a curriculum or progress. Titles and explanations are normalized and
escaped for Markdown. Stable student, goal, curriculum-instance, concept, plan,
and milestone IDs are deliberately omitted. Profile details, goal descriptions,
mistake contents, evidence, diagnostic answers, secrets, and machine metadata
are also absent.

## Ownership and regeneration

All three paths are `system-generated-human-readable`. The specific
`00-roadmap/PROGRESS.md` path is classified explicitly; other files named
`PROGRESS.md`, including implementation logs, remain student-owned by default.

Each successful write records its content hash, creator, generation time, and
template version in the existing workspace artifact index. Regeneration is
allowed only if the current file still matches the last generated hash. An
untracked pre-existing file or a human edit returns a conflict and is not
overwritten. Writes remain atomic per file and workspace-sandboxed through the
Foundation adapter.

Templates are independently versioned as `student-learning-learning/v1`,
`student-learning-roadmap/v1`, and `student-learning-progress/v1`. They consume
only `progress-dashboard/v1`; an unknown read-model version fails closed. The
command runs only when requested and is not attached to TUI keypresses or other
read commands.

## Boundaries

The export does not recalculate or migrate learning algorithms, add a schema
migration, generate curriculum or exercises, contact a network service, or
introduce an editable Markdown configuration format. Steps 29 and later retain
ownership of algorithm compatibility, hardening, and final end-to-end work.
