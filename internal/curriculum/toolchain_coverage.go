package curriculum

import "fmt"

const ToolchainCoverageVersionV1 = "toolchain-coverage-v1"

type EnvironmentPackReference struct {
	ID      ID
	Version PackVersion
}

func (reference EnvironmentPackReference) Validate() error {
	if err := reference.ID.Validate(); err != nil {
		return fmt.Errorf("environment pack reference: %w", err)
	}
	if err := reference.Version.Validate(); err != nil {
		return fmt.Errorf("environment pack reference: %w", err)
	}
	return nil
}

// ToolchainCoverageRequirement declares a tool needed by the goal. The tool's
// operational metadata remains owned by the referenced Environment Pack.
type ToolchainCoverageRequirement struct {
	ID              ID
	ToolID          ID
	EnvironmentPack EnvironmentPackReference
	MinimumLevel    ToolRequirementLevel
	Reason          string
	EvidenceRefs    []EvidenceRef
}

func (requirement ToolchainCoverageRequirement) Validate() error {
	if err := requirement.ID.Validate(); err != nil {
		return fmt.Errorf("toolchain coverage requirement: %w", err)
	}
	if err := requirement.ToolID.Validate(); err != nil {
		return fmt.Errorf("toolchain tool: %w", err)
	}
	if err := requirement.EnvironmentPack.Validate(); err != nil {
		return err
	}
	if err := requirement.MinimumLevel.Validate(); err != nil {
		return err
	}
	if err := requireText("toolchain requirement reason", requirement.Reason); err != nil {
		return err
	}
	if err := validateEvidenceRefs("toolchain requirement evidence", requirement.EvidenceRefs); err != nil {
		return err
	}
	if len(requirement.EvidenceRefs) == 0 {
		return fmt.Errorf("toolchain requirement %q has no evidence", requirement.ID)
	}
	return nil
}

type ToolchainCoverageResult struct {
	RequirementID   ID
	ToolID          ID
	EnvironmentPack EnvironmentPackReference
	Status          CoverageStatus
	ResolvedTool    *ToolRequirement
	MissingFields   []string
	Reasons         []string
}

func (result ToolchainCoverageResult) Validate() error {
	if err := result.RequirementID.Validate(); err != nil {
		return fmt.Errorf("toolchain result requirement: %w", err)
	}
	if err := result.ToolID.Validate(); err != nil {
		return fmt.Errorf("toolchain result tool: %w", err)
	}
	if err := result.EnvironmentPack.Validate(); err != nil {
		return err
	}
	if err := result.Status.Validate(); err != nil {
		return err
	}
	if err := validateUniqueTexts("toolchain missing fields", result.MissingFields); err != nil {
		return err
	}
	if err := validateTexts("toolchain coverage reasons", result.Reasons); err != nil {
		return err
	}
	if len(result.Reasons) == 0 {
		return fmt.Errorf("toolchain coverage result has no reasons")
	}
	if result.ResolvedTool == nil {
		if result.Status != CoverageMissing || len(result.MissingFields) == 0 {
			return fmt.Errorf("unresolved toolchain result must be missing with diagnostics")
		}
		return nil
	}
	if err := result.ResolvedTool.Validate(); err != nil {
		return err
	}
	if result.ResolvedTool.ID != result.ToolID {
		return fmt.Errorf("resolved toolchain tool identity does not match result")
	}
	if result.Status == CoverageCovered && len(result.MissingFields) != 0 {
		return fmt.Errorf("covered toolchain result has missing fields")
	}
	if result.Status == CoveragePartial && len(result.MissingFields) == 0 {
		return fmt.Errorf("partial toolchain result has no missing fields")
	}
	if result.Status == CoverageMissing {
		return fmt.Errorf("resolved toolchain result cannot be missing")
	}
	return nil
}

type ToolchainCoverageReport struct {
	Results              []ToolchainCoverageResult
	CoverageRequirements []CoverageRequirement
	CoverageSupports     []CoverageSupport
	AlgorithmVersion     string
}

func (report ToolchainCoverageReport) Validate() error {
	if len(report.Results) == 0 {
		return fmt.Errorf("toolchain coverage report has no results")
	}
	requirements := make(map[ID]struct{}, len(report.CoverageRequirements))
	for _, requirement := range report.CoverageRequirements {
		if err := requirement.Validate(); err != nil {
			return err
		}
		if requirement.Dimension != CoverageToolchain || requirement.TargetKind != CoverageTargetGoal {
			return fmt.Errorf("toolchain report contains invalid requirement %q", requirement.ID)
		}
		if _, exists := requirements[requirement.ID]; exists {
			return fmt.Errorf("duplicate toolchain coverage requirement %q", requirement.ID)
		}
		requirements[requirement.ID] = struct{}{}
	}
	seenResults := make(map[ID]struct{}, len(report.Results))
	for _, result := range report.Results {
		if err := result.Validate(); err != nil {
			return err
		}
		if _, exists := requirements[result.RequirementID]; !exists {
			return fmt.Errorf("toolchain result references missing requirement %q", result.RequirementID)
		}
		if _, exists := seenResults[result.RequirementID]; exists {
			return fmt.Errorf("duplicate toolchain result %q", result.RequirementID)
		}
		seenResults[result.RequirementID] = struct{}{}
	}
	if len(seenResults) != len(requirements) {
		return fmt.Errorf("toolchain results do not cover every requirement")
	}
	seenSupports := make(map[ID]struct{}, len(report.CoverageSupports))
	for _, support := range report.CoverageSupports {
		if err := support.Validate(); err != nil {
			return err
		}
		if _, exists := requirements[support.RequirementID]; !exists {
			return fmt.Errorf("toolchain support %q references missing requirement", support.ID)
		}
		if _, exists := seenSupports[support.ID]; exists {
			return fmt.Errorf("duplicate toolchain coverage support %q", support.ID)
		}
		seenSupports[support.ID] = struct{}{}
	}
	if report.AlgorithmVersion != ToolchainCoverageVersionV1 {
		return fmt.Errorf("unsupported toolchain coverage version %q", report.AlgorithmVersion)
	}
	return nil
}
