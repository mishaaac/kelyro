package curriculum

import "fmt"

const CoverageEngineVersionV1 = "coverage-v1"

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

type CoverageSupport struct {
	ID            ID
	RequirementID ID
	ConceptIDs    []ConceptID
	EvidenceRefs  []EvidenceRef
	ArtifactRefs  []ID
	Reason        string
}

func (support CoverageSupport) Validate() error {
	if err := support.ID.Validate(); err != nil {
		return fmt.Errorf("coverage support: %w", err)
	}
	if err := support.RequirementID.Validate(); err != nil {
		return fmt.Errorf("coverage support requirement: %w", err)
	}
	if err := validateConceptIDs("coverage support concepts", support.ConceptIDs); err != nil {
		return err
	}
	if err := validateEvidenceRefs("coverage support evidence", support.EvidenceRefs); err != nil {
		return err
	}
	if err := validateIDs("coverage support artifacts", support.ArtifactRefs); err != nil {
		return err
	}
	if len(support.ConceptIDs) == 0 && len(support.EvidenceRefs) == 0 && len(support.ArtifactRefs) == 0 {
		return fmt.Errorf("coverage support %q has no supporting references", support.ID)
	}
	return requireText("coverage support reason", support.Reason)
}

type CoverageDimensionReport struct {
	Dimension             CoverageDimension
	Status                CoverageStatus
	Requirements          []CoverageResult
	MissingRequirementIDs []ID
	PartialRequirementIDs []ID
	CoveredRequirementIDs []ID
	Reasons               []string
}

func (report CoverageDimensionReport) Validate() error {
	if err := report.Dimension.Validate(); err != nil {
		return err
	}
	if err := report.Status.Validate(); err != nil {
		return err
	}
	if err := validateIDs("missing coverage requirements", report.MissingRequirementIDs); err != nil {
		return err
	}
	if err := validateIDs("partial coverage requirements", report.PartialRequirementIDs); err != nil {
		return err
	}
	if err := validateIDs("covered coverage requirements", report.CoveredRequirementIDs); err != nil {
		return err
	}
	if err := validateTexts("coverage dimension reasons", report.Reasons); err != nil {
		return err
	}
	if len(report.Reasons) == 0 {
		return fmt.Errorf("coverage dimension %q has no reasons", report.Dimension)
	}
	seen := make(map[ID]CoverageStatus, len(report.Requirements))
	for _, result := range report.Requirements {
		if err := result.Validate(); err != nil {
			return err
		}
		if _, exists := seen[result.RequirementID]; exists {
			return fmt.Errorf("coverage dimension %q contains duplicate requirement %q", report.Dimension, result.RequirementID)
		}
		seen[result.RequirementID] = result.Status
	}
	for status, ids := range map[CoverageStatus][]ID{
		CoverageMissing: report.MissingRequirementIDs,
		CoveragePartial: report.PartialRequirementIDs,
		CoverageCovered: report.CoveredRequirementIDs,
	} {
		for _, id := range ids {
			if actual, exists := seen[id]; !exists || actual != status {
				return fmt.Errorf("coverage dimension %q %s partition does not match requirement %q", report.Dimension, status, id)
			}
		}
	}
	if len(report.MissingRequirementIDs)+len(report.PartialRequirementIDs)+len(report.CoveredRequirementIDs) != len(report.Requirements) {
		return fmt.Errorf("coverage dimension %q partitions do not cover every requirement", report.Dimension)
	}
	expected := aggregateCoverageStatus(report.Requirements)
	if report.Status != expected {
		return fmt.Errorf("coverage dimension %q status %q does not match aggregate %q", report.Dimension, report.Status, expected)
	}
	return nil
}

type CoverageReport struct {
	Dimensions       []CoverageDimensionReport
	AlgorithmVersion string
}

func (report CoverageReport) Validate() error {
	if report.AlgorithmVersion != CoverageEngineVersionV1 {
		return fmt.Errorf("unsupported coverage engine version %q", report.AlgorithmVersion)
	}
	if len(report.Dimensions) != len(AllCoverageDimensions()) {
		return fmt.Errorf("coverage report must contain all dimensions")
	}
	seen := make(map[CoverageDimension]struct{}, len(report.Dimensions))
	for _, dimension := range report.Dimensions {
		if err := dimension.Validate(); err != nil {
			return err
		}
		if _, exists := seen[dimension.Dimension]; exists {
			return fmt.Errorf("coverage report contains duplicate dimension %q", dimension.Dimension)
		}
		seen[dimension.Dimension] = struct{}{}
	}
	return nil
}

func AllCoverageDimensions() []CoverageDimension {
	return []CoverageDimension{
		CoverageCompetency, CoverageConcept, CoverageEvidence, CoverageTheory,
		CoveragePractice, CoverageProduction, CoverageSecurity, CoverageToolchain,
	}
}

func aggregateCoverageStatus(results []CoverageResult) CoverageStatus {
	if len(results) == 0 {
		return CoverageMissing
	}
	covered := 0
	missing := 0
	for _, result := range results {
		switch result.Status {
		case CoverageCovered:
			covered++
		case CoverageMissing:
			missing++
		}
	}
	if covered == len(results) {
		return CoverageCovered
	}
	if missing == len(results) {
		return CoverageMissing
	}
	return CoveragePartial
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
