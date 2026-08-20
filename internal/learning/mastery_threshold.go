package learning

import (
	"fmt"
	"math"
)

const (
	MasteryThresholdPolicyVersion = "threshold-v1"
	MinimumProgressionThreshold   = 0.50
	MaximumProgressionThreshold   = 0.99
)

type MasteryThresholdMode string

const (
	MasteryModeRelaxed  MasteryThresholdMode = "relaxed"
	MasteryModeStandard MasteryThresholdMode = "standard"
	MasteryModeStrict   MasteryThresholdMode = "strict"
	MasteryModeMastery  MasteryThresholdMode = "mastery"
	MasteryModeCustom   MasteryThresholdMode = "custom"
)

func (mode MasteryThresholdMode) Valid() bool {
	switch mode {
	case MasteryModeRelaxed, MasteryModeStandard, MasteryModeStrict, MasteryModeMastery, MasteryModeCustom:
		return true
	default:
		return false
	}
}

func (mode MasteryThresholdMode) DisplayName() string {
	switch mode {
	case MasteryModeRelaxed:
		return "Relaxed"
	case MasteryModeStandard:
		return "Standard"
	case MasteryModeStrict:
		return "Strict"
	case MasteryModeMastery:
		return "Mastery"
	case MasteryModeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// MasteryRequirement is a progression policy value. It is deliberately not an
// assessment grade: SatisfiedBy compares evidence-derived mastery with the
// minimum needed to advance.
type MasteryRequirement struct {
	Threshold MasteryThreshold
	Mode      MasteryThresholdMode
}

func NewMasteryRequirement(value float64) (MasteryRequirement, error) {
	threshold, err := NewMasteryThreshold(value)
	if err != nil {
		return MasteryRequirement{}, err
	}
	return MasteryRequirementFromThreshold(threshold)
}

func MasteryRequirementFromThreshold(threshold MasteryThreshold) (MasteryRequirement, error) {
	if err := threshold.Validate(); err != nil {
		return MasteryRequirement{}, err
	}
	if threshold.Value() < MinimumProgressionThreshold || threshold.Value() > MaximumProgressionThreshold {
		return MasteryRequirement{}, fmt.Errorf("progression threshold must be between %.0f%% and %.0f%%", MinimumProgressionThreshold*100, MaximumProgressionThreshold*100)
	}
	return MasteryRequirement{Threshold: threshold, Mode: modeForThreshold(threshold.Value())}, nil
}

func (requirement MasteryRequirement) Validate() error {
	derived, err := MasteryRequirementFromThreshold(requirement.Threshold)
	if err != nil {
		return err
	}
	if !requirement.Mode.Valid() || requirement.Mode != derived.Mode {
		return fmt.Errorf("mastery threshold mode %q does not match %.2f", requirement.Mode, requirement.Threshold.Value())
	}
	return nil
}

// SatisfiedBy implements threshold-v1: calculated mastery equal to or greater
// than the requirement may advance. It does not calculate mastery or unlock a
// concept by itself.
func (requirement MasteryRequirement) SatisfiedBy(score MasteryScore) bool {
	return score.Value() >= requirement.Threshold.Value()
}

func modeForThreshold(value float64) MasteryThresholdMode {
	switch {
	case nearlyEqual(value, 0.70):
		return MasteryModeRelaxed
	case nearlyEqual(value, 0.80):
		return MasteryModeStandard
	case nearlyEqual(value, 0.85):
		return MasteryModeStrict
	case nearlyEqual(value, 0.90):
		return MasteryModeMastery
	default:
		return MasteryModeCustom
	}
}

func nearlyEqual(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}

type MasteryThresholdSource string

const (
	MasterySourceStudentDefault    MasteryThresholdSource = "student_default"
	MasterySourceWorkspaceOverride MasteryThresholdSource = "workspace_override"
	MasterySourcePackOverride      MasteryThresholdSource = "pack_override"
)

func (source MasteryThresholdSource) Valid() bool {
	switch source {
	case MasterySourceStudentDefault, MasterySourceWorkspaceOverride, MasterySourcePackOverride:
		return true
	default:
		return false
	}
}

func (source MasteryThresholdSource) DisplayName() string {
	switch source {
	case MasterySourceStudentDefault:
		return "Student default"
	case MasterySourceWorkspaceOverride:
		return "Workspace override"
	case MasterySourcePackOverride:
		return "Learning Pack override"
	default:
		return "Unknown"
	}
}

// MasteryThresholdSettings stores durable student and workspace layers. The
// effective value is resolved separately so future pack input remains
// infrastructure-independent.
type MasteryThresholdSettings struct {
	StudentID         ID
	PolicyVersion     string
	StudentDefault    MasteryThreshold
	WorkspaceOverride *MasteryThreshold
	UpdatedAt         Timestamp
}

func NewMasteryThresholdSettings(studentID ID, updatedAt Timestamp) (MasteryThresholdSettings, error) {
	standard, _ := NewMasteryThreshold(0.80)
	settings := MasteryThresholdSettings{
		StudentID: studentID, PolicyVersion: MasteryThresholdPolicyVersion,
		StudentDefault: standard, UpdatedAt: updatedAt,
	}
	return settings, settings.Validate()
}

func (settings MasteryThresholdSettings) Validate() error {
	if err := settings.StudentID.Validate(); err != nil {
		return fmt.Errorf("mastery settings student: %w", err)
	}
	if settings.PolicyVersion != MasteryThresholdPolicyVersion {
		return fmt.Errorf("unsupported mastery threshold policy version %q", settings.PolicyVersion)
	}
	if _, err := MasteryRequirementFromThreshold(settings.StudentDefault); err != nil {
		return fmt.Errorf("student default: %w", err)
	}
	if settings.WorkspaceOverride != nil {
		if _, err := MasteryRequirementFromThreshold(*settings.WorkspaceOverride); err != nil {
			return fmt.Errorf("workspace override: %w", err)
		}
	}
	if err := settings.UpdatedAt.Validate(); err != nil {
		return fmt.Errorf("mastery settings updated at: %w", err)
	}
	return nil
}

func (settings MasteryThresholdSettings) SetStudentDefault(threshold MasteryThreshold, at Timestamp) (MasteryThresholdSettings, error) {
	if err := settings.transition(threshold, at); err != nil {
		return MasteryThresholdSettings{}, err
	}
	settings.StudentDefault = threshold
	settings.UpdatedAt = at
	return settings, settings.Validate()
}

func (settings MasteryThresholdSettings) SetWorkspaceOverride(threshold MasteryThreshold, at Timestamp) (MasteryThresholdSettings, error) {
	if err := settings.transition(threshold, at); err != nil {
		return MasteryThresholdSettings{}, err
	}
	settings.WorkspaceOverride = &threshold
	settings.UpdatedAt = at
	return settings, settings.Validate()
}

func (settings MasteryThresholdSettings) ClearWorkspaceOverride(at Timestamp) (MasteryThresholdSettings, error) {
	if err := at.Validate(); err != nil {
		return MasteryThresholdSettings{}, fmt.Errorf("mastery settings transition: %w", err)
	}
	if at.Before(settings.UpdatedAt) {
		return MasteryThresholdSettings{}, fmt.Errorf("mastery settings transition precedes prior update")
	}
	settings.WorkspaceOverride = nil
	settings.UpdatedAt = at
	return settings, settings.Validate()
}

func (settings MasteryThresholdSettings) transition(threshold MasteryThreshold, at Timestamp) error {
	if _, err := MasteryRequirementFromThreshold(threshold); err != nil {
		return err
	}
	if err := at.Validate(); err != nil {
		return fmt.Errorf("mastery settings transition: %w", err)
	}
	if at.Before(settings.UpdatedAt) {
		return fmt.Errorf("mastery settings transition precedes prior update")
	}
	return nil
}

// PackMasteryOverride is the future pack boundary. A pack must declare both
// an override and the inclusive bounds within which that override is valid.
type PackMasteryOverride struct {
	Threshold MasteryThreshold
	Minimum   MasteryThreshold
	Maximum   MasteryThreshold
}

func NewPackMasteryOverride(threshold, minimum, maximum float64) (PackMasteryOverride, error) {
	thresholdValue, err := NewMasteryThreshold(threshold)
	if err != nil {
		return PackMasteryOverride{}, err
	}
	minimumValue, err := NewMasteryThreshold(minimum)
	if err != nil {
		return PackMasteryOverride{}, err
	}
	maximumValue, err := NewMasteryThreshold(maximum)
	if err != nil {
		return PackMasteryOverride{}, err
	}
	override := PackMasteryOverride{Threshold: thresholdValue, Minimum: minimumValue, Maximum: maximumValue}
	return override, override.Validate()
}

func (override PackMasteryOverride) Validate() error {
	if _, err := MasteryRequirementFromThreshold(override.Threshold); err != nil {
		return fmt.Errorf("pack mastery threshold: %w", err)
	}
	if _, err := MasteryRequirementFromThreshold(override.Minimum); err != nil {
		return fmt.Errorf("pack mastery minimum: %w", err)
	}
	if _, err := MasteryRequirementFromThreshold(override.Maximum); err != nil {
		return fmt.Errorf("pack mastery maximum: %w", err)
	}
	if override.Minimum.Value() > override.Maximum.Value() {
		return fmt.Errorf("pack mastery minimum exceeds maximum")
	}
	if override.Threshold.Value() < override.Minimum.Value() || override.Threshold.Value() > override.Maximum.Value() {
		return fmt.Errorf("pack mastery threshold is outside declared limits")
	}
	return nil
}

type ResolvedMasteryThreshold struct {
	Requirement   MasteryRequirement
	Source        MasteryThresholdSource
	PolicyVersion string
}

func (resolved ResolvedMasteryThreshold) Validate() error {
	if err := resolved.Requirement.Validate(); err != nil {
		return err
	}
	if !resolved.Source.Valid() {
		return fmt.Errorf("invalid mastery threshold source %q", resolved.Source)
	}
	if resolved.PolicyVersion != MasteryThresholdPolicyVersion {
		return fmt.Errorf("unsupported mastery threshold policy version %q", resolved.PolicyVersion)
	}
	return nil
}

// ResolveMasteryThreshold applies threshold-v1 precedence:
// pack override > workspace override > student default.
func ResolveMasteryThreshold(settings MasteryThresholdSettings, pack *PackMasteryOverride) (ResolvedMasteryThreshold, error) {
	if err := settings.Validate(); err != nil {
		return ResolvedMasteryThreshold{}, err
	}
	threshold := settings.StudentDefault
	source := MasterySourceStudentDefault
	if settings.WorkspaceOverride != nil {
		threshold = *settings.WorkspaceOverride
		source = MasterySourceWorkspaceOverride
	}
	if pack != nil {
		if err := pack.Validate(); err != nil {
			return ResolvedMasteryThreshold{}, err
		}
		threshold = pack.Threshold
		source = MasterySourcePackOverride
	}
	requirement, err := MasteryRequirementFromThreshold(threshold)
	if err != nil {
		return ResolvedMasteryThreshold{}, err
	}
	return ResolvedMasteryThreshold{Requirement: requirement, Source: source, PolicyVersion: MasteryThresholdPolicyVersion}, nil
}
