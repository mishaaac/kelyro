# Professional outcome model v1

## Purpose

The outcome model prevents a professional curriculum from being considered
complete when it only promises syntax recall or passive knowledge.
`GoalOutcome` carries two explicit classifications; validators never infer them
from prose.

## Categories

Every outcome has exactly one closed category:

```text
knowledge
application
debugging
design
tool_usage
production
security
communication_documentation
maintenance
```

Categories describe the kind of professional coverage. They remain distinct
from competency depth and from future lesson/practice contracts.

## Capabilities and professional completeness

An outcome may name one closed capability:

```text
explain | build | debug | operate | maintain
```

Capability is optional for a narrow, non-professional goal. When
`LearningGoalSpec.Role` is present, all five capabilities are mandatory across
the goal's outcomes. More than one outcome may share a capability and an
outcome remains evidence-backed independently.

This rule checks declared contracts, not keywords. “Learn production Go” does
not count as `operate` unless the outcome explicitly declares that capability.
Likewise, category and capability are separate: a security outcome may support
explain, build, operate or maintain depending on its actual contract.

The model does not generate outcome prose, exercises, assessments, projects or
learner state. Packs/goals author the outcomes; domain validation enforces
their completeness.
