# Curriculum-aware Doctor v1

Step 38 connects a validated `environment-pack/v1` to the existing Foundation
Doctor without allowing Learning Packs to execute code. The integration has two
deterministic boundaries:

1. `EnvironmentDoctorPlannerV1` projects a compiled curriculum position and an
   Environment Pack into an `EnvironmentDoctorPlan`.
2. The application adapter maps that plan to `doctor.Context`; Doctor probes
   only commands already present in its trusted registry.

The planner is learner-state neutral. It reads the current Concept supplied by
the caller, but it never reads or mutates mastery, scheduling, retention, or
other I-02 state.

## Timing policy

`environment-doctor-planner-v1` uses the compiled knowledge graph's canonical
topological order. The hierarchy supplies only human-readable phase and module
locations; it never changes prerequisite order.

For each tool supported by the current platform:

- `not_needed_yet`: the current Concept precedes `introduced_at`;
- `future`: the tool has been introduced, but the current Concept precedes
  `when_needed`;
- `current`: the current Concept is at or after `when_needed`.

`when_needed` before `introduced_at` is invalid. This prevents a curriculum
from requiring a tool before teaching its role. Output is sorted by tool ID so
input ordering cannot change the plan.

Every diagnostic carries its declared requirement level, minimum version,
current and needed phase/module, an explanation, and the selected official
installation guidance for the active platform.

## Doctor behavior

Current tools are resolved and version-probed through Doctor's maintained
registry. A detected version below the Environment Pack minimum fails that
check. If a minimum version cannot be verified, the current check also fails;
Doctor does not silently claim compatibility.

Future and not-needed-yet tools are displayed as `deferred`. They are not
resolved, version-probed, or treated as blocking, even if their eventual level
is `required`. This makes upcoming environment needs visible without imposing
them early.

An unregistered current tool produces a `diagnostic unavailable` failure and
is never executed. An unregistered deferred tool remains visible as deferred.
Supporting a new executable therefore requires trusted Kelyro code, not merely
pack-authored data.

## Security and scope boundaries

Environment Packs cannot define command candidates, version arguments,
package-manager commands, scripts, credentials, or environment mutations.
Official URLs and installation instructions remain inert display metadata.
Doctor never opens a URL or installs a tool.

Pack installation, workspace activation, automatic selection of an active
pack, and tool installation belong to later steps. Step 38 accepts an explicit
validated plan from the application command boundary so Step 39 can connect
activation without changing Doctor's trust model.
