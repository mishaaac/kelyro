package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type PracticeCoverageV1 struct{}

func NewPracticeCoverageV1() PracticeCoverageV1 {
	return PracticeCoverageV1{}
}

func (PracticeCoverageV1) Analyze(ctx context.Context, request PracticeCoverageRequest) (curriculum.PracticeCoverageReport, error) {
	const operation = "analyze practice coverage"
	if err := ctx.Err(); err != nil {
		return curriculum.PracticeCoverageReport{}, ExternalError(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.PracticeCoverageReport{}, Invalid(operation, err)
	}
	if err := request.Competencies.Validate(); err != nil {
		return curriculum.PracticeCoverageReport{}, Invalid(operation, err)
	}
	competencies := make(map[curriculum.ID]curriculum.Competency, len(request.Competencies.Competencies))
	for _, competency := range request.Competencies.Competencies {
		if err := requireKnownEvidence(competency.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("practice competency %q: %w", competency.ID, err))
		}
		competencies[competency.ID] = competency
	}
	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate practice concept %q", concept.ID))
		}
		if err := requireKnownEvidence(concept.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("practice concept %q: %w", concept.ID, err))
		}
		concepts[concept.ID] = concept
	}
	important := make(map[curriculum.ConceptID]struct{}, len(request.ImportantConceptIDs))
	for _, conceptID := range request.ImportantConceptIDs {
		if err := conceptID.Validate(); err != nil {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := concepts[conceptID]; !exists {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("important practice concept %q is absent", conceptID))
		}
		if _, exists := important[conceptID]; exists {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate important practice concept %q", conceptID))
		}
		important[conceptID] = struct{}{}
	}
	if len(important) == 0 {
		return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("practice coverage has no important concepts"))
	}
	requirements := make(map[curriculum.ID]curriculum.PracticeRequirement, len(request.Requirements))
	byConcept := make(map[curriculum.ConceptID]curriculum.PracticeRequirement, len(request.Requirements))
	for _, requirement := range request.Requirements {
		if err := requirement.Validate(); err != nil {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := important[requirement.ConceptID]; !exists {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("practice requirement %q targets a concept not declared important", requirement.ID))
		}
		competency, exists := competencies[requirement.CompetencyID]
		if !exists {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("practice requirement %q references missing competency", requirement.ID))
		}
		if !containsConceptID(competency.ConceptRefs, requirement.ConceptID) {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("practice concept %q is not mapped to competency %q", requirement.ConceptID, competency.ID))
		}
		if _, exists := requirements[requirement.ID]; exists {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate practice requirement %q", requirement.ID))
		}
		if _, exists := byConcept[requirement.ConceptID]; exists {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("multiple practice requirements target concept %q", requirement.ConceptID))
		}
		if err := requireKnownEvidence(requirement.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("practice requirement %q: %w", requirement.ID, err))
		}
		requirements[requirement.ID] = requirement
		byConcept[requirement.ConceptID] = requirement
	}
	if len(byConcept) != len(important) {
		return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("every important concept must have exactly one practice requirement"))
	}
	expectations := make(map[curriculum.ID][]curriculum.PracticeExpectation)
	seenExpectations := make(map[curriculum.ID]struct{}, len(request.Expectations))
	for _, expectation := range request.Expectations {
		if err := expectation.Validate(); err != nil {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, err)
		}
		if _, exists := seenExpectations[expectation.ID]; exists {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("duplicate practice expectation %q", expectation.ID))
		}
		seenExpectations[expectation.ID] = struct{}{}
		if _, exists := requirements[expectation.RequirementID]; !exists {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("practice expectation %q references missing requirement", expectation.ID))
		}
		if err := requireKnownEvidence(expectation.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.PracticeCoverageReport{}, Invalid(operation, fmt.Errorf("practice expectation %q: %w", expectation.ID, err))
		}
		expectations[expectation.RequirementID] = append(expectations[expectation.RequirementID], expectation)
	}

	report := curriculum.PracticeCoverageReport{
		CompatibilityVersion: curriculum.PracticeCompatibilityVersionV1,
		AlgorithmVersion:     curriculum.PracticeCoverageVersionV1,
	}
	requirementIDs := make([]curriculum.ID, 0, len(requirements))
	for id := range requirements {
		requirementIDs = append(requirementIDs, id)
	}
	sort.Slice(requirementIDs, func(i, j int) bool { return requirementIDs[i].String() < requirementIDs[j].String() })
	for _, requirementID := range requirementIDs {
		if err := ctx.Err(); err != nil {
			return curriculum.PracticeCoverageReport{}, ExternalError(operation, err)
		}
		requirement := requirements[requirementID]
		competency := competencies[requirement.CompetencyID]
		coverageID := practiceCoverageRequirementID(requirement.ID)
		result := curriculum.PracticeCoverageResult{
			RequirementID: requirement.ID, ConceptID: requirement.ConceptID,
			CompetencyID: requirement.CompetencyID, ExpectedLevel: competency.ExpectedLevel,
			Status: curriculum.CoverageMissing,
		}
		values := append([]curriculum.PracticeExpectation(nil), expectations[requirement.ID]...)
		sort.Slice(values, func(i, j int) bool { return values[i].ID.String() < values[j].ID.String() })
		for _, expectation := range values {
			if practiceCompatible(competency.ExpectedLevel, expectation.Kind) {
				result.CompatibleExpectationIDs = append(result.CompatibleExpectationIDs, expectation.ID)
				report.CoverageSupports = append(report.CoverageSupports, curriculum.CoverageSupport{
					ID: expectation.ID, RequirementID: coverageID,
					ConceptIDs:   []curriculum.ConceptID{requirement.ConceptID},
					EvidenceRefs: sortedEvidenceCopy(expectation.EvidenceRefs), ArtifactRefs: []curriculum.ID{expectation.ID},
					Reason: expectation.Reason,
				})
			} else {
				result.IncompatibleExpectationIDs = append(result.IncompatibleExpectationIDs, expectation.ID)
			}
		}
		if len(result.CompatibleExpectationIDs) > 0 {
			result.Status = curriculum.CoverageCovered
			result.Reasons = []string{"compatible_practice_expectation_declared"}
		} else if len(values) == 0 {
			result.Reasons = []string{"no_practice_expectation_declared"}
		} else {
			result.Reasons = []string{"practice_expectations_incompatible_with_expected_competency"}
		}
		report.Results = append(report.Results, result)
		targetID, _ := curriculum.NewID(requirement.ConceptID.String())
		report.CoverageRequirements = append(report.CoverageRequirements, curriculum.CoverageRequirement{
			ID: coverageID, Dimension: curriculum.CoveragePractice, TargetKind: curriculum.CoverageTargetConcept,
			TargetID: targetID, Description: "Provide at least one practice expectation compatible with " + string(competency.ExpectedLevel) + ".",
			EvidenceRefs: sortedEvidenceCopy(requirement.EvidenceRefs),
		})
	}
	sort.Slice(report.CoverageSupports, func(i, j int) bool {
		return report.CoverageSupports[i].ID.String() < report.CoverageSupports[j].ID.String()
	})
	if err := report.Validate(); err != nil {
		return curriculum.PracticeCoverageReport{}, Invalid(operation, err)
	}
	return report, nil
}

func practiceCompatible(level curriculum.CompetencyLevel, kind curriculum.PracticeExpectationKind) bool {
	switch level {
	case curriculum.CompetencyAwareness:
		return kind == curriculum.PracticeRecall || kind == curriculum.PracticeRecognize
	case curriculum.CompetencyUnderstand:
		return kind == curriculum.PracticeCompare || kind == curriculum.PracticeExplain
	case curriculum.CompetencyApply:
		return kind == curriculum.PracticeApply || kind == curriculum.PracticeBuild
	case curriculum.CompetencyAnalyze:
		return kind == curriculum.PracticeDebug || kind == curriculum.PracticeCompare
	case curriculum.CompetencyDesign:
		return kind == curriculum.PracticeDesign || kind == curriculum.PracticeBuild || kind == curriculum.PracticeCompare
	case curriculum.CompetencyOperate:
		return kind == curriculum.PracticeApply || kind == curriculum.PracticeDebug || kind == curriculum.PracticeBuild
	case curriculum.CompetencyExplain:
		return kind == curriculum.PracticeExplain || kind == curriculum.PracticeCompare
	default:
		return false
	}
}

func practiceCoverageRequirementID(requirementID curriculum.ID) curriculum.ID {
	digest := sha256.Sum256([]byte(requirementID.String()))
	id, err := curriculum.NewID("practice." + hex.EncodeToString(digest[:16]))
	if err != nil {
		panic(err)
	}
	return id
}
