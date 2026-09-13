package curriculum

import "fmt"

const (
	PracticeCoverageVersionV1      = "practice-coverage-v1"
	PracticeCompatibilityVersionV1 = "practice-compatibility-v1"
)

type PracticeExpectationKind string

const (
	PracticeRecall    PracticeExpectationKind = "recall"
	PracticeRecognize PracticeExpectationKind = "recognize"
	PracticeApply     PracticeExpectationKind = "apply"
	PracticeDebug     PracticeExpectationKind = "debug"
	PracticeDesign    PracticeExpectationKind = "design"
	PracticeBuild     PracticeExpectationKind = "build"
	PracticeCompare   PracticeExpectationKind = "compare"
	PracticeExplain   PracticeExpectationKind = "explain"
)

func (kind PracticeExpectationKind) Validate() error {
	switch kind {
	case PracticeRecall, PracticeRecognize, PracticeApply, PracticeDebug,
		PracticeDesign, PracticeBuild, PracticeCompare, PracticeExplain:
		return nil
	default:
		return fmt.Errorf("invalid practice expectation %q", kind)
	}
}

type PracticeRequirement struct {
	ID           ID
	ConceptID    ConceptID
	CompetencyID ID
	Reason       string
	EvidenceRefs []EvidenceRef
}

func (requirement PracticeRequirement) Validate() error {
	if err := requirement.ID.Validate(); err != nil {
		return fmt.Errorf("practice requirement: %w", err)
	}
	if err := requirement.ConceptID.Validate(); err != nil {
		return fmt.Errorf("practice requirement concept: %w", err)
	}
	if err := requirement.CompetencyID.Validate(); err != nil {
		return fmt.Errorf("practice requirement competency: %w", err)
	}
	if err := requireText("practice requirement reason", requirement.Reason); err != nil {
		return err
	}
	if err := validateEvidenceRefs("practice requirement evidence", requirement.EvidenceRefs); err != nil {
		return err
	}
	if len(requirement.EvidenceRefs) == 0 {
		return fmt.Errorf("practice requirement %q has no evidence", requirement.ID)
	}
	return nil
}

// PracticeExpectation describes what I-05 must make practicable or assessable.
// It is not an exercise, prompt, solution, score, or runtime definition.
type PracticeExpectation struct {
	ID            ID
	RequirementID ID
	Kind          PracticeExpectationKind
	Reason        string
	EvidenceRefs  []EvidenceRef
}

func (expectation PracticeExpectation) Validate() error {
	if err := expectation.ID.Validate(); err != nil {
		return fmt.Errorf("practice expectation: %w", err)
	}
	if err := expectation.RequirementID.Validate(); err != nil {
		return fmt.Errorf("practice expectation requirement: %w", err)
	}
	if err := expectation.Kind.Validate(); err != nil {
		return err
	}
	if err := requireText("practice expectation reason", expectation.Reason); err != nil {
		return err
	}
	if err := validateEvidenceRefs("practice expectation evidence", expectation.EvidenceRefs); err != nil {
		return err
	}
	if len(expectation.EvidenceRefs) == 0 {
		return fmt.Errorf("practice expectation %q has no evidence", expectation.ID)
	}
	return nil
}

type PracticeCoverageResult struct {
	RequirementID              ID
	ConceptID                  ConceptID
	CompetencyID               ID
	ExpectedLevel              CompetencyLevel
	Status                     CoverageStatus
	CompatibleExpectationIDs   []ID
	IncompatibleExpectationIDs []ID
	Reasons                    []string
}

func (result PracticeCoverageResult) Validate() error {
	if err := result.RequirementID.Validate(); err != nil {
		return fmt.Errorf("practice coverage requirement: %w", err)
	}
	if err := result.ConceptID.Validate(); err != nil {
		return fmt.Errorf("practice coverage concept: %w", err)
	}
	if err := result.CompetencyID.Validate(); err != nil {
		return fmt.Errorf("practice coverage competency: %w", err)
	}
	if err := result.ExpectedLevel.Validate(); err != nil {
		return err
	}
	if result.Status != CoverageMissing && result.Status != CoverageCovered {
		return fmt.Errorf("practice coverage status must be missing or covered")
	}
	if err := validateIDs("compatible practice expectations", result.CompatibleExpectationIDs); err != nil {
		return err
	}
	if err := validateIDs("incompatible practice expectations", result.IncompatibleExpectationIDs); err != nil {
		return err
	}
	if err := validateTexts("practice coverage reasons", result.Reasons); err != nil {
		return err
	}
	if len(result.Reasons) == 0 {
		return fmt.Errorf("practice coverage result has no reasons")
	}
	if result.Status == CoverageCovered && len(result.CompatibleExpectationIDs) == 0 {
		return fmt.Errorf("covered practice requirement has no compatible expectation")
	}
	if result.Status == CoverageMissing && len(result.CompatibleExpectationIDs) != 0 {
		return fmt.Errorf("missing practice requirement has compatible expectations")
	}
	return nil
}

type PracticeCoverageReport struct {
	Results              []PracticeCoverageResult
	CoverageRequirements []CoverageRequirement
	CoverageSupports     []CoverageSupport
	CompatibilityVersion string
	AlgorithmVersion     string
}

func (report PracticeCoverageReport) Validate() error {
	if len(report.Results) == 0 {
		return fmt.Errorf("practice coverage report has no results")
	}
	results := make(map[ID]PracticeCoverageResult, len(report.Results))
	for _, result := range report.Results {
		if err := result.Validate(); err != nil {
			return err
		}
		if _, exists := results[result.RequirementID]; exists {
			return fmt.Errorf("duplicate practice coverage result %q", result.RequirementID)
		}
		results[result.RequirementID] = result
	}
	requirements := make(map[ID]struct{}, len(report.CoverageRequirements))
	for _, requirement := range report.CoverageRequirements {
		if err := requirement.Validate(); err != nil {
			return err
		}
		if requirement.Dimension != CoveragePractice || requirement.TargetKind != CoverageTargetConcept {
			return fmt.Errorf("practice report contains invalid coverage requirement %q", requirement.ID)
		}
		if _, exists := requirements[requirement.ID]; exists {
			return fmt.Errorf("duplicate practice coverage requirement %q", requirement.ID)
		}
		requirements[requirement.ID] = struct{}{}
	}
	if len(requirements) != len(results) {
		return fmt.Errorf("practice report requirements do not match results")
	}
	seenSupports := make(map[ID]struct{}, len(report.CoverageSupports))
	for _, support := range report.CoverageSupports {
		if err := support.Validate(); err != nil {
			return err
		}
		if _, exists := requirements[support.RequirementID]; !exists {
			return fmt.Errorf("practice support %q references missing coverage requirement", support.ID)
		}
		if _, exists := seenSupports[support.ID]; exists {
			return fmt.Errorf("duplicate practice coverage support %q", support.ID)
		}
		seenSupports[support.ID] = struct{}{}
	}
	if report.CompatibilityVersion != PracticeCompatibilityVersionV1 {
		return fmt.Errorf("unsupported practice compatibility version %q", report.CompatibilityVersion)
	}
	if report.AlgorithmVersion != PracticeCoverageVersionV1 {
		return fmt.Errorf("unsupported practice coverage version %q", report.AlgorithmVersion)
	}
	return nil
}
