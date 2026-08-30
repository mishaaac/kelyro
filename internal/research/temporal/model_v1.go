package temporal

import (
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

type GuidanceRole string

const (
	RoleCurrentGuidance   GuidanceRole = "current_guidance"
	RoleVersionAuthority  GuidanceRole = "version_authority"
	RoleHistoricalContext GuidanceRole = "historical_context"
	RoleNotApplicable     GuidanceRole = "not_applicable"
)

func (role GuidanceRole) Validate() error {
	switch role {
	case RoleCurrentGuidance, RoleVersionAuthority, RoleHistoricalContext, RoleNotApplicable:
		return nil
	default:
		return fmt.Errorf("invalid source guidance role %q", role)
	}
}

type Input struct {
	Source        research.Source
	Purpose       research.ResearchPurpose
	TargetVersion *research.SourceVersion
}

type Assessment struct {
	SourceID         research.SourceID
	TemporalScope    research.SourceTemporalScope
	SourceVersion    *research.SourceVersion
	TargetVersion    *research.SourceVersion
	Role             GuidanceRole
	Warning          string
	AlgorithmVersion string
}

func (assessment Assessment) Validate() error {
	if err := assessment.SourceID.Validate(); err != nil {
		return err
	}
	if err := assessment.TemporalScope.Validate(); err != nil {
		return err
	}
	if err := assessment.Role.Validate(); err != nil {
		return err
	}
	for _, version := range []*research.SourceVersion{assessment.SourceVersion, assessment.TargetVersion} {
		if version != nil {
			if err := version.Validate(); err != nil {
				return err
			}
		}
	}
	wantWarning, err := assessment.TemporalScope.Warning(assessment.SourceVersion)
	if err != nil {
		return err
	}
	if assessment.Warning != wantWarning {
		return fmt.Errorf("source temporal warning does not match scope")
	}
	if assessment.TemporalScope == research.SourceTemporalCurrent && assessment.Role == RoleHistoricalContext {
		return fmt.Errorf("current source cannot be historical context")
	}
	if assessment.TemporalScope != research.SourceTemporalCurrent && assessment.Role == RoleCurrentGuidance {
		return fmt.Errorf("non-current source cannot be current guidance")
	}
	if assessment.Role == RoleVersionAuthority &&
		(assessment.SourceVersion == nil || assessment.TargetVersion == nil || *assessment.SourceVersion != *assessment.TargetVersion) {
		return fmt.Errorf("version authority requires matching source and target versions")
	}
	if assessment.AlgorithmVersion != research.SourceTemporalPolicyV1 {
		return fmt.Errorf("source temporal algorithm version must be %q", research.SourceTemporalPolicyV1)
	}
	return nil
}

func AssessV1(input Input) (Assessment, error) {
	if err := input.Source.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("source: %w", err)
	}
	if err := input.Purpose.Validate(); err != nil {
		return Assessment{}, err
	}
	if input.TargetVersion != nil {
		if err := input.TargetVersion.Validate(); err != nil {
			return Assessment{}, fmt.Errorf("target version: %w", err)
		}
	}
	warning, err := input.Source.TemporalScope.Warning(input.Source.Version)
	if err != nil {
		return Assessment{}, err
	}
	assessment := Assessment{
		SourceID: input.Source.ID, TemporalScope: input.Source.TemporalScope,
		SourceVersion: cloneVersion(input.Source.Version), TargetVersion: cloneVersion(input.TargetVersion),
		Warning: warning, AlgorithmVersion: research.SourceTemporalPolicyV1,
	}

	matchingVersion := input.Source.Version != nil && input.TargetVersion != nil && *input.Source.Version == *input.TargetVersion
	mismatchedVersion := input.Source.Version != nil && input.TargetVersion != nil && *input.Source.Version != *input.TargetVersion
	switch {
	case input.Source.TemporalScope == research.SourceTemporalCurrent && mismatchedVersion:
		assessment.Role = RoleNotApplicable
	case input.Source.TemporalScope == research.SourceTemporalCurrent:
		assessment.Role = RoleCurrentGuidance
	case input.Purpose == research.PurposeVersionBehavior && matchingVersion:
		assessment.Role = RoleVersionAuthority
	case mismatchedVersion:
		assessment.Role = RoleNotApplicable
	default:
		assessment.Role = RoleHistoricalContext
	}
	if err := assessment.Validate(); err != nil {
		return Assessment{}, err
	}
	return assessment, nil
}

func cloneVersion(version *research.SourceVersion) *research.SourceVersion {
	if version == nil {
		return nil
	}
	clone := *version
	return &clone
}
