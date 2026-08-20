# Mastery threshold policy

Step 7 defines how strict Kelyro is before progression may consider calculated
mastery sufficient. A threshold is not an assessment grade and it does not
calculate mastery. It is compared only with a `MasteryScore` that another,
later evidence policy has already calculated.

## Versioned rule

Policy `threshold-v1` uses one inclusive comparison:

```text
may satisfy progression = calculated mastery >= required threshold
```

Equality satisfies the threshold. The policy returns only whether the numeric
requirement is met; it does not unlock curriculum, bypass prerequisites, score
an assessment, or mutate concept state.

## Presets and custom range

| Mode | Threshold |
|---|---:|
| Relaxed | 70% |
| Standard | 80% |
| Strict | 85% |
| Mastery | 90% |
| Custom | any other value from 50% through 99% |

Both endpoints of the custom policy range are inclusive. The application and
domain APIs accept finite decimal values from `0.50` through `0.99`; the CLI
accepts whole percentages from `50` through `99`. Exact preset values receive
their named mode, while every other valid value is reported as `Custom`.

The generic `MasteryThreshold` value object continues to represent `[0,1]` so
published historical data remains readable. New progression settings and new
goals pass through `MasteryRequirement`, which enforces the narrower v1 range.

## Precedence

Resolution is deterministic and uses the highest available layer:

```text
future bounded Learning Pack override
                >
workspace override
                >
student default
```

- The student default begins at Standard (80%). Onboarding confirmation stores
  the selected strictness in this layer.
- A workspace override changes progression for the current workspace without
  erasing the learner's default.
- A future Learning Pack may provide an override only together with inclusive
  minimum and maximum limits. The override is rejected when its value is
  outside those declared limits or the global 50–99% policy range.

Step 7 defines and tests the pack boundary but does not load or compile Learning
Packs. That remains outside this implementation step.

## Persistence and compatibility

Forward-only migration v8 creates one `mastery_threshold_settings` row per
student with:

- policy version;
- student default;
- optional workspace override;
- last update timestamp.

When upgrading an existing workspace, v8 uses the active goal's threshold as
the student default if it lies in the v1 range. This carries forward the choice
confirmed by Step 6 onboarding. Missing or legacy out-of-range values safely
fall back to Standard. New onboarding confirmations also write the student
default explicitly through `MasteryPolicyService`.

## CLI

```text
kelyro mastery threshold
kelyro mastery threshold set PERCENT
kelyro mastery threshold set-default PERCENT
kelyro mastery threshold reset
```

`set` writes the workspace override. `set-default` changes the learner default
while preserving an existing workspace override. `reset` removes only the
workspace override. Output identifies the effective percentage, named mode,
winning source, policy version, and reminds the learner that the number is not
an assessment grade.

The resolver is an application/domain boundary consumed by future progression
logic. SQLite and CLI handlers contain no advancement decisions.
