package curriculum

import "fmt"

type ToolRequirementLevel string

const (
	ToolRequired    ToolRequirementLevel = "required"
	ToolRecommended ToolRequirementLevel = "recommended"
	ToolOptional    ToolRequirementLevel = "optional"
)

func (level ToolRequirementLevel) Validate() error {
	switch level {
	case ToolRequired, ToolRecommended, ToolOptional:
		return nil
	default:
		return fmt.Errorf("invalid tool requirement level %q", level)
	}
}

type ToolRequirement struct {
	ID           ID
	Purpose      string
	Level        ToolRequirementLevel
	IntroducedAt *ConceptID
	Platforms    []string
	EvidenceRefs []EvidenceRef
}

func (requirement ToolRequirement) Validate() error {
	if err := requirement.ID.Validate(); err != nil {
		return fmt.Errorf("tool requirement: %w", err)
	}
	if err := requireText("tool purpose", requirement.Purpose); err != nil {
		return err
	}
	if err := requirement.Level.Validate(); err != nil {
		return err
	}
	if requirement.IntroducedAt != nil {
		if err := requirement.IntroducedAt.Validate(); err != nil {
			return fmt.Errorf("tool introduction: %w", err)
		}
	}
	if err := validateTexts("tool platforms", requirement.Platforms); err != nil {
		return err
	}
	return validateEvidenceRefs("tool evidence", requirement.EvidenceRefs)
}

type EnvironmentPack struct {
	ID      ID
	Version PackVersion
	Tools   []ToolRequirement
}

func (environment EnvironmentPack) Validate() error {
	if err := environment.ID.Validate(); err != nil {
		return fmt.Errorf("environment pack: %w", err)
	}
	if err := environment.Version.Validate(); err != nil {
		return err
	}
	if len(environment.Tools) == 0 {
		return fmt.Errorf("environment pack has no tool requirements")
	}
	seen := make(map[ID]struct{}, len(environment.Tools))
	for _, requirement := range environment.Tools {
		if err := requirement.Validate(); err != nil {
			return err
		}
		if _, exists := seen[requirement.ID]; exists {
			return fmt.Errorf("environment pack contains duplicate tool %q", requirement.ID)
		}
		seen[requirement.ID] = struct{}{}
	}
	return nil
}
