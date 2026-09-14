package curriculum

import "fmt"

const EnvironmentDoctorPlannerVersionV1 = "environment-doctor-planner-v1"

type EnvironmentToolTiming string

const (
	EnvironmentToolCurrent      EnvironmentToolTiming = "current"
	EnvironmentToolFuture       EnvironmentToolTiming = "future"
	EnvironmentToolNotNeededYet EnvironmentToolTiming = "not_needed_yet"
)

func (timing EnvironmentToolTiming) Validate() error {
	switch timing {
	case EnvironmentToolCurrent, EnvironmentToolFuture, EnvironmentToolNotNeededYet:
		return nil
	default:
		return fmt.Errorf("invalid environment tool timing %q", timing)
	}
}

type EnvironmentToolDiagnostic struct {
	ToolID              ID
	DisplayName         string
	Level               ToolRequirementLevel
	MinimumVersion      string
	Timing              EnvironmentToolTiming
	WhyNeeded           string
	CurrentPhase        string
	CurrentModule       string
	NeededPhase         string
	NeededModule        string
	OfficialSourceName  string
	OfficialURL         string
	InstallInstructions string
}

func (diagnostic EnvironmentToolDiagnostic) Validate() error {
	if err := diagnostic.ToolID.Validate(); err != nil {
		return fmt.Errorf("environment diagnostic tool: %w", err)
	}
	if err := diagnostic.Level.Validate(); err != nil {
		return err
	}
	if _, err := NewPackVersion(diagnostic.MinimumVersion); err != nil {
		return fmt.Errorf("environment diagnostic minimum version: %w", err)
	}
	if err := diagnostic.Timing.Validate(); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{
		{"environment diagnostic display name", diagnostic.DisplayName},
		{"environment diagnostic reason", diagnostic.WhyNeeded},
		{"environment diagnostic current phase", diagnostic.CurrentPhase},
		{"environment diagnostic current module", diagnostic.CurrentModule},
		{"environment diagnostic needed phase", diagnostic.NeededPhase},
		{"environment diagnostic needed module", diagnostic.NeededModule},
		{"environment diagnostic official source", diagnostic.OfficialSourceName},
		{"environment diagnostic official URL", diagnostic.OfficialURL},
		{"environment diagnostic install instructions", diagnostic.InstallInstructions},
	} {
		if err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	if err := validateOfficialInstallURL(diagnostic.OfficialURL); err != nil {
		return fmt.Errorf("environment diagnostic: %w", err)
	}
	return nil
}

type EnvironmentDoctorPlan struct {
	EnvironmentPack  EnvironmentPackReference
	Platform         string
	CurrentConceptID ConceptID
	Tools            []EnvironmentToolDiagnostic
	AlgorithmVersion string
}

func (plan EnvironmentDoctorPlan) Validate() error {
	if err := plan.EnvironmentPack.Validate(); err != nil {
		return err
	}
	if !validEnvironmentPlatform(plan.Platform) {
		return fmt.Errorf("invalid environment doctor platform %q", plan.Platform)
	}
	if err := plan.CurrentConceptID.Validate(); err != nil {
		return fmt.Errorf("environment doctor current concept: %w", err)
	}
	seen := make(map[ID]struct{}, len(plan.Tools))
	for _, tool := range plan.Tools {
		if err := tool.Validate(); err != nil {
			return err
		}
		if _, exists := seen[tool.ToolID]; exists {
			return fmt.Errorf("environment doctor plan repeats tool %q", tool.ToolID)
		}
		seen[tool.ToolID] = struct{}{}
	}
	if plan.AlgorithmVersion != EnvironmentDoctorPlannerVersionV1 {
		return fmt.Errorf("unsupported environment doctor planner version %q", plan.AlgorithmVersion)
	}
	return nil
}
