package curriculum

import (
	"fmt"
	"net/url"
	"strings"
)

const EnvironmentPackSchemaVersionV1 = "environment-pack/v1"

const (
	EnvironmentPlatformLinux   = "linux"
	EnvironmentPlatformDarwin  = "darwin"
	EnvironmentPlatformWindows = "windows"
)

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
	ID                  ID
	DisplayName         string
	Purpose             string
	MinimumVersion      string
	Level               ToolRequirementLevel
	IntroducedAt        *ConceptID
	WhenNeeded          *ConceptID
	Platforms           []string
	InstallGuidanceRefs []ID
	EvidenceRefs        []EvidenceRef
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
	if requirement.DisplayName != "" {
		if err := requireText("tool display name", requirement.DisplayName); err != nil {
			return err
		}
	}
	if requirement.MinimumVersion != "" {
		if _, err := NewPackVersion(requirement.MinimumVersion); err != nil {
			return fmt.Errorf("tool minimum version: %w", err)
		}
	}
	if requirement.IntroducedAt != nil {
		if err := requirement.IntroducedAt.Validate(); err != nil {
			return fmt.Errorf("tool introduction: %w", err)
		}
	}
	if requirement.WhenNeeded != nil {
		if err := requirement.WhenNeeded.Validate(); err != nil {
			return fmt.Errorf("tool when needed: %w", err)
		}
	}
	if err := validateEnvironmentPlatforms("tool platforms", requirement.Platforms); err != nil {
		return err
	}
	if err := validateIDs("tool install guidance refs", requirement.InstallGuidanceRefs); err != nil {
		return err
	}
	return validateEvidenceRefs("tool evidence", requirement.EvidenceRefs)
}

type ToolInstallGuidance struct {
	ID           ID
	Platform     string
	SourceName   string
	OfficialURL  string
	Instructions string
	EvidenceRefs []EvidenceRef
}

func (guidance ToolInstallGuidance) Validate() error {
	if err := guidance.ID.Validate(); err != nil {
		return fmt.Errorf("tool install guidance: %w", err)
	}
	if !validEnvironmentPlatform(guidance.Platform) {
		return fmt.Errorf("invalid install guidance platform %q", guidance.Platform)
	}
	if err := requireText("install guidance source name", guidance.SourceName); err != nil {
		return err
	}
	if err := requireText("install guidance instructions", guidance.Instructions); err != nil {
		return err
	}
	parsed, err := url.Parse(guidance.OfficialURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("install guidance official URL must be a query-free HTTPS URL without credentials or fragment")
	}
	return validateEvidenceRefs("install guidance evidence", guidance.EvidenceRefs)
}

type EnvironmentPack struct {
	ID                 ID
	Version            PackVersion
	SchemaVersion      string
	SupportedPlatforms []string
	Tools              []ToolRequirement
	InstallGuidance    []ToolInstallGuidance
}

func (environment EnvironmentPack) Validate() error {
	if err := environment.ID.Validate(); err != nil {
		return fmt.Errorf("environment pack: %w", err)
	}
	if err := environment.Version.Validate(); err != nil {
		return err
	}
	if environment.SchemaVersion != EnvironmentPackSchemaVersionV1 {
		return fmt.Errorf("unsupported environment pack schema %q", environment.SchemaVersion)
	}
	if err := validateEnvironmentPlatforms("environment pack platforms", environment.SupportedPlatforms); err != nil {
		return err
	}
	if len(environment.SupportedPlatforms) == 0 {
		return fmt.Errorf("environment pack has no supported platforms")
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
	guidanceByID := make(map[ID]ToolInstallGuidance, len(environment.InstallGuidance))
	for _, guidance := range environment.InstallGuidance {
		if err := guidance.Validate(); err != nil {
			return err
		}
		if _, exists := guidanceByID[guidance.ID]; exists {
			return fmt.Errorf("environment pack contains duplicate install guidance %q", guidance.ID)
		}
		guidanceByID[guidance.ID] = guidance
	}
	platforms := make(map[string]struct{}, len(environment.SupportedPlatforms))
	for _, platform := range environment.SupportedPlatforms {
		platforms[platform] = struct{}{}
	}
	for _, guidance := range environment.InstallGuidance {
		if _, exists := platforms[guidance.Platform]; !exists {
			return fmt.Errorf("install guidance %q targets unsupported platform %q", guidance.ID, guidance.Platform)
		}
	}
	for _, tool := range environment.Tools {
		for _, platform := range tool.Platforms {
			if _, exists := platforms[platform]; !exists {
				return fmt.Errorf("tool %q targets unsupported platform %q", tool.ID, platform)
			}
		}
		for _, reference := range tool.InstallGuidanceRefs {
			if _, exists := guidanceByID[reference]; !exists {
				return fmt.Errorf("tool %q references missing install guidance %q", tool.ID, reference)
			}
		}
	}
	return nil
}

// ValidatePortableV1 enforces the complete on-disk environment-pack/v1
// contract. Validate intentionally remains suitable for coverage diagnostics,
// where incomplete tool metadata must be reported rather than hidden by a
// parsing failure.
func (environment EnvironmentPack) ValidatePortableV1() error {
	if err := environment.Validate(); err != nil {
		return err
	}
	guidanceByID := make(map[ID]ToolInstallGuidance, len(environment.InstallGuidance))
	for _, guidance := range environment.InstallGuidance {
		guidanceByID[guidance.ID] = guidance
		if len(guidance.EvidenceRefs) == 0 {
			return fmt.Errorf("install guidance %q has no evidence", guidance.ID)
		}
	}
	for _, tool := range environment.Tools {
		for _, field := range []struct{ name, value string }{
			{"display name", tool.DisplayName}, {"minimum version", tool.MinimumVersion},
		} {
			if err := requireText("tool "+field.name, field.value); err != nil {
				return fmt.Errorf("tool %q: %w", tool.ID, err)
			}
		}
		if tool.IntroducedAt == nil || tool.WhenNeeded == nil {
			return fmt.Errorf("tool %q must declare introduced_at and when_needed", tool.ID)
		}
		if len(tool.Platforms) == 0 || len(tool.InstallGuidanceRefs) == 0 || len(tool.EvidenceRefs) == 0 {
			return fmt.Errorf("tool %q must declare platforms, install guidance refs, and evidence", tool.ID)
		}
		coveredPlatforms := make(map[string]struct{}, len(tool.InstallGuidanceRefs))
		for _, reference := range tool.InstallGuidanceRefs {
			coveredPlatforms[guidanceByID[reference].Platform] = struct{}{}
		}
		for _, platform := range tool.Platforms {
			if _, exists := coveredPlatforms[platform]; !exists {
				return fmt.Errorf("tool %q has no official install guidance for platform %q", tool.ID, platform)
			}
		}
	}
	return nil
}

func validateEnvironmentPlatforms(name string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != strings.TrimSpace(value) || !validEnvironmentPlatform(value) {
			return fmt.Errorf("%s contains invalid platform %q", name, value)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%s contains duplicate platform %q", name, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validEnvironmentPlatform(value string) bool {
	return value == EnvironmentPlatformLinux || value == EnvironmentPlatformDarwin || value == EnvironmentPlatformWindows
}
