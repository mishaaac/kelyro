package curriculum

import "fmt"

type CoverageDimension string

const (
	CoverageCompetency CoverageDimension = "competency"
	CoverageConcept    CoverageDimension = "concept"
	CoverageEvidence   CoverageDimension = "evidence"
	CoverageTheory     CoverageDimension = "theory"
	CoveragePractice   CoverageDimension = "practice_contract"
	CoverageProduction CoverageDimension = "production"
	CoverageSecurity   CoverageDimension = "security"
	CoverageToolchain  CoverageDimension = "toolchain"
)

func (dimension CoverageDimension) Validate() error {
	switch dimension {
	case CoverageCompetency, CoverageConcept, CoverageEvidence, CoverageTheory,
		CoveragePractice, CoverageProduction, CoverageSecurity, CoverageToolchain:
		return nil
	default:
		return fmt.Errorf("invalid coverage dimension %q", dimension)
	}
}

type CoverageTargetKind string

const (
	CoverageTargetCurriculum CoverageTargetKind = "curriculum"
	CoverageTargetGoal       CoverageTargetKind = "goal"
	CoverageTargetCompetency CoverageTargetKind = "competency"
	CoverageTargetConcept    CoverageTargetKind = "concept"
)

func (kind CoverageTargetKind) Validate() error {
	switch kind {
	case CoverageTargetCurriculum, CoverageTargetGoal, CoverageTargetCompetency, CoverageTargetConcept:
		return nil
	default:
		return fmt.Errorf("invalid coverage target kind %q", kind)
	}
}

type CoverageRequirement struct {
	ID           ID
	Dimension    CoverageDimension
	TargetKind   CoverageTargetKind
	TargetID     ID
	Description  string
	EvidenceRefs []EvidenceRef
}

func (requirement CoverageRequirement) Validate() error {
	if err := requirement.ID.Validate(); err != nil {
		return fmt.Errorf("coverage requirement: %w", err)
	}
	if err := requirement.Dimension.Validate(); err != nil {
		return err
	}
	if err := requirement.TargetKind.Validate(); err != nil {
		return err
	}
	if err := requirement.TargetID.Validate(); err != nil {
		return fmt.Errorf("coverage target: %w", err)
	}
	if err := requireText("coverage requirement description", requirement.Description); err != nil {
		return err
	}
	return validateEvidenceRefs("coverage requirement evidence", requirement.EvidenceRefs)
}

type CoverageStatus string

const (
	CoverageMissing CoverageStatus = "missing"
	CoveragePartial CoverageStatus = "partial"
	CoverageCovered CoverageStatus = "covered"
)

func (status CoverageStatus) Validate() error {
	switch status {
	case CoverageMissing, CoveragePartial, CoverageCovered:
		return nil
	default:
		return fmt.Errorf("invalid coverage status %q", status)
	}
}

type CoverageResult struct {
	RequirementID ID
	Status        CoverageStatus
	Reasons       []string
}

func (result CoverageResult) Validate() error {
	if err := result.RequirementID.Validate(); err != nil {
		return fmt.Errorf("coverage result requirement: %w", err)
	}
	if err := result.Status.Validate(); err != nil {
		return err
	}
	return validateTexts("coverage reasons", result.Reasons)
}

type GapKind string

const (
	GapMissingCompetency      GapKind = "missing_competency"
	GapMissingConcept         GapKind = "missing_concept"
	GapMissingPrerequisite    GapKind = "missing_prerequisite"
	GapMissingEvidence        GapKind = "missing_evidence"
	GapMissingTheory          GapKind = "missing_theory"
	GapMissingPractice        GapKind = "missing_practice_contract"
	GapMissingProduction      GapKind = "missing_production_reality"
	GapMissingToolchain       GapKind = "missing_toolchain"
	GapMissingSecurity        GapKind = "missing_security"
	GapMissingCurrentGuidance GapKind = "missing_current_guidance"
)

func (kind GapKind) Validate() error {
	switch kind {
	case GapMissingCompetency, GapMissingConcept, GapMissingPrerequisite,
		GapMissingEvidence, GapMissingTheory, GapMissingPractice,
		GapMissingProduction, GapMissingToolchain, GapMissingSecurity,
		GapMissingCurrentGuidance:
		return nil
	default:
		return fmt.Errorf("invalid gap kind %q", kind)
	}
}

type GapSeverity string

const (
	GapBlocking      GapSeverity = "blocking"
	GapImportant     GapSeverity = "important"
	GapRecommended   GapSeverity = "recommended"
	GapInformational GapSeverity = "informational"
)

func (severity GapSeverity) Validate() error {
	switch severity {
	case GapBlocking, GapImportant, GapRecommended, GapInformational:
		return nil
	default:
		return fmt.Errorf("invalid gap severity %q", severity)
	}
}

type Gap struct {
	ID           ID
	Kind         GapKind
	Severity     GapSeverity
	TargetID     ID
	Reason       string
	EvidenceRefs []EvidenceRef
}

func (gap Gap) Validate() error {
	if err := gap.ID.Validate(); err != nil {
		return fmt.Errorf("gap: %w", err)
	}
	if err := gap.Kind.Validate(); err != nil {
		return err
	}
	if err := gap.Severity.Validate(); err != nil {
		return err
	}
	if err := gap.TargetID.Validate(); err != nil {
		return fmt.Errorf("gap target: %w", err)
	}
	if err := requireText("gap reason", gap.Reason); err != nil {
		return err
	}
	return validateEvidenceRefs("gap evidence", gap.EvidenceRefs)
}
