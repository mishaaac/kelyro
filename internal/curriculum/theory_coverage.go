package curriculum

import "fmt"

const TheoryCoverageVersionV1 = "theory-coverage-v1"

type TheoryFacet string

const (
	TheoryDefinition   TheoryFacet = "definition"
	TheoryMentalModel  TheoryFacet = "mental_model"
	TheoryMechanism    TheoryFacet = "mechanism"
	TheoryTradeoffs    TheoryFacet = "tradeoffs"
	TheoryFailureModes TheoryFacet = "failure_modes"
)

func (facet TheoryFacet) Validate() error {
	switch facet {
	case TheoryDefinition, TheoryMentalModel, TheoryMechanism, TheoryTradeoffs, TheoryFailureModes:
		return nil
	default:
		return fmt.Errorf("invalid theory facet %q", facet)
	}
}

func AllTheoryFacets() []TheoryFacet {
	return []TheoryFacet{TheoryDefinition, TheoryMentalModel, TheoryMechanism, TheoryTradeoffs, TheoryFailureModes}
}

// TheoryContract declares the conceptual knowledge required for one important
// competency. It does not contain generated lesson prose.
type TheoryContract struct {
	ID             ID
	CompetencyID   ID
	RequiredFacets []TheoryFacet
	Reason         string
	EvidenceRefs   []EvidenceRef
}

func (contract TheoryContract) Validate() error {
	if err := contract.ID.Validate(); err != nil {
		return fmt.Errorf("theory contract: %w", err)
	}
	if err := contract.CompetencyID.Validate(); err != nil {
		return fmt.Errorf("theory contract competency: %w", err)
	}
	if len(contract.RequiredFacets) == 0 {
		return fmt.Errorf("theory contract %q has no required facets", contract.ID)
	}
	seen := make(map[TheoryFacet]struct{}, len(contract.RequiredFacets))
	for _, facet := range contract.RequiredFacets {
		if err := facet.Validate(); err != nil {
			return err
		}
		if _, exists := seen[facet]; exists {
			return fmt.Errorf("theory contract %q contains duplicate facet %q", contract.ID, facet)
		}
		seen[facet] = struct{}{}
	}
	if err := requireText("theory contract reason", contract.Reason); err != nil {
		return err
	}
	if err := validateEvidenceRefs("theory contract evidence", contract.EvidenceRefs); err != nil {
		return err
	}
	if len(contract.EvidenceRefs) == 0 {
		return fmt.Errorf("theory contract %q has no evidence", contract.ID)
	}
	return nil
}

// TheoryFacetSupport links one required facet to curriculum Concepts and
// verified evidence. Content generation remains an I-05 concern.
type TheoryFacetSupport struct {
	ID           ID
	ContractID   ID
	Facet        TheoryFacet
	ConceptIDs   []ConceptID
	EvidenceRefs []EvidenceRef
	Reason       string
}

func (support TheoryFacetSupport) Validate() error {
	if err := support.ID.Validate(); err != nil {
		return fmt.Errorf("theory support: %w", err)
	}
	if err := support.ContractID.Validate(); err != nil {
		return fmt.Errorf("theory support contract: %w", err)
	}
	if err := support.Facet.Validate(); err != nil {
		return err
	}
	if err := validateConceptIDs("theory support concepts", support.ConceptIDs); err != nil {
		return err
	}
	if len(support.ConceptIDs) == 0 {
		return fmt.Errorf("theory support %q has no concepts", support.ID)
	}
	if err := validateEvidenceRefs("theory support evidence", support.EvidenceRefs); err != nil {
		return err
	}
	if len(support.EvidenceRefs) == 0 {
		return fmt.Errorf("theory support %q has no evidence", support.ID)
	}
	return requireText("theory support reason", support.Reason)
}

type TheoryFacetCoverage struct {
	Facet      TheoryFacet
	Status     CoverageStatus
	SupportIDs []ID
	ConceptIDs []ConceptID
	Reasons    []string
}

func (coverage TheoryFacetCoverage) Validate() error {
	if err := coverage.Facet.Validate(); err != nil {
		return err
	}
	if coverage.Status != CoverageMissing && coverage.Status != CoverageCovered {
		return fmt.Errorf("theory facet %q must be missing or covered", coverage.Facet)
	}
	if err := validateIDs("theory facet supports", coverage.SupportIDs); err != nil {
		return err
	}
	if err := validateConceptIDs("theory facet concepts", coverage.ConceptIDs); err != nil {
		return err
	}
	if err := validateTexts("theory facet reasons", coverage.Reasons); err != nil {
		return err
	}
	if len(coverage.Reasons) == 0 {
		return fmt.Errorf("theory facet %q has no reasons", coverage.Facet)
	}
	if coverage.Status == CoverageCovered && (len(coverage.SupportIDs) == 0 || len(coverage.ConceptIDs) == 0) {
		return fmt.Errorf("covered theory facet %q has no support", coverage.Facet)
	}
	if coverage.Status == CoverageMissing && (len(coverage.SupportIDs) != 0 || len(coverage.ConceptIDs) != 0) {
		return fmt.Errorf("missing theory facet %q unexpectedly has support", coverage.Facet)
	}
	return nil
}

type TheoryCompetencyCoverage struct {
	CompetencyID ID
	ContractID   *ID
	Status       CoverageStatus
	Facets       []TheoryFacetCoverage
	Reasons      []string
}

func (coverage TheoryCompetencyCoverage) Validate() error {
	if err := coverage.CompetencyID.Validate(); err != nil {
		return fmt.Errorf("theory coverage competency: %w", err)
	}
	if err := coverage.Status.Validate(); err != nil {
		return err
	}
	if err := validateTexts("theory competency reasons", coverage.Reasons); err != nil {
		return err
	}
	if len(coverage.Reasons) == 0 {
		return fmt.Errorf("theory competency %q has no reasons", coverage.CompetencyID)
	}
	if coverage.ContractID == nil {
		if coverage.Status != CoverageMissing || len(coverage.Facets) != 0 {
			return fmt.Errorf("competency %q without theory contract must be missing", coverage.CompetencyID)
		}
		return nil
	}
	if err := coverage.ContractID.Validate(); err != nil {
		return fmt.Errorf("theory coverage contract: %w", err)
	}
	if len(coverage.Facets) == 0 {
		return fmt.Errorf("theory competency %q has no facet results", coverage.CompetencyID)
	}
	seen := make(map[TheoryFacet]struct{}, len(coverage.Facets))
	covered := 0
	for _, facet := range coverage.Facets {
		if err := facet.Validate(); err != nil {
			return err
		}
		if _, exists := seen[facet.Facet]; exists {
			return fmt.Errorf("theory competency %q contains duplicate facet %q", coverage.CompetencyID, facet.Facet)
		}
		seen[facet.Facet] = struct{}{}
		if facet.Status == CoverageCovered {
			covered++
		}
	}
	expected := CoveragePartial
	if covered == 0 {
		expected = CoverageMissing
	} else if covered == len(coverage.Facets) {
		expected = CoverageCovered
	}
	if coverage.Status != expected {
		return fmt.Errorf("theory competency %q status does not match its facets", coverage.CompetencyID)
	}
	return nil
}

type TheoryCoverageReport struct {
	Competencies         []TheoryCompetencyCoverage
	CoverageRequirements []CoverageRequirement
	CoverageSupports     []CoverageSupport
	AlgorithmVersion     string
}

func (report TheoryCoverageReport) Validate() error {
	if len(report.Competencies) == 0 {
		return fmt.Errorf("theory coverage report has no competencies")
	}
	seenCompetencies := make(map[ID]struct{}, len(report.Competencies))
	for _, competency := range report.Competencies {
		if err := competency.Validate(); err != nil {
			return err
		}
		if _, exists := seenCompetencies[competency.CompetencyID]; exists {
			return fmt.Errorf("duplicate theory coverage competency %q", competency.CompetencyID)
		}
		seenCompetencies[competency.CompetencyID] = struct{}{}
	}
	requirements := make(map[ID]struct{}, len(report.CoverageRequirements))
	for _, requirement := range report.CoverageRequirements {
		if err := requirement.Validate(); err != nil {
			return err
		}
		if requirement.Dimension != CoverageTheory || requirement.TargetKind != CoverageTargetCompetency {
			return fmt.Errorf("theory report contains non-theory coverage requirement %q", requirement.ID)
		}
		if _, exists := seenCompetencies[requirement.TargetID]; !exists {
			return fmt.Errorf("theory requirement %q targets an unreported competency", requirement.ID)
		}
		if _, exists := requirements[requirement.ID]; exists {
			return fmt.Errorf("duplicate theory coverage requirement %q", requirement.ID)
		}
		requirements[requirement.ID] = struct{}{}
	}
	if len(report.CoverageRequirements) == 0 {
		return fmt.Errorf("theory coverage report has no coverage requirements")
	}
	seenSupports := make(map[ID]struct{}, len(report.CoverageSupports))
	for _, support := range report.CoverageSupports {
		if err := support.Validate(); err != nil {
			return err
		}
		if _, exists := requirements[support.RequirementID]; !exists {
			return fmt.Errorf("theory coverage support %q references missing requirement", support.ID)
		}
		if _, exists := seenSupports[support.ID]; exists {
			return fmt.Errorf("duplicate theory coverage support %q", support.ID)
		}
		seenSupports[support.ID] = struct{}{}
	}
	if report.AlgorithmVersion != TheoryCoverageVersionV1 {
		return fmt.Errorf("unsupported theory coverage version %q", report.AlgorithmVersion)
	}
	return nil
}
