# Curriculum Gap Scanner v1

`gap-scanner-v1` converts incomplete multidimensional coverage and explicit
upstream findings into stable, prioritized, actionable curriculum gaps.

## Inputs and mappings

Every `missing` or `partial` atomic Coverage result maps by dimension:

| Coverage dimension | Gap kind |
| --- | --- |
| competency | `missing_competency` |
| concept | `missing_concept` |
| evidence | `missing_evidence` |
| theory | `missing_theory` |
| practice contract | `missing_practice_contract` |
| production | `missing_production_reality` |
| security | `missing_security` |
| toolchain | `missing_toolchain` |

If a dimension has no requirements, the scanner emits one goal-targeted gap
whose action is to declare and satisfy that dimension. Covered requirements do
not emit gaps. Requirement descriptions, result reasons, targets, and evidence
are retained in the diagnostic.

Prerequisite expansion findings become `missing_prerequisite`. Findings from a
future temporal review become `missing_current_guidance`. Step 20 accepts the
latter through `CurrentGuidanceFinding`; it does not perform the current,
experimental, legacy, or historical classification assigned to Steps 28–29.

## Severity policy v1

Missing competency, concept, prerequisite, evidence, and security coverage is
`blocking`. Missing theory, practice contract, production reality, toolchain,
and current guidance is `important`. A partial atomic result lowers blocking to
important and important to recommended. Informational remains a valid domain
severity for later advisory policies but is not assigned by this scanner.

The policy prioritizes structural and evidence integrity while keeping every
dimension visible; it does not combine severities into a score.

## Identity and determinism

Gap IDs are derived with SHA-256 from kind, target, and the complete stable
source identity (requirement, prerequisite finding, or current-guidance
finding). Exact duplicates coalesce. Gaps sort by severity, kind, target, and
ID. Evidence refs are sorted defensively, so equivalent input order produces
the same report.

The scanner validates that the Coverage Report and supplied requirements are
an exact match. It has no network, persistence, hierarchy mutation, I-05, or
student-state behavior.
