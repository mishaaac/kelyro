package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type CoverageEngineV1 struct{}

func NewCoverageEngineV1() CoverageEngineV1 {
	return CoverageEngineV1{}
}

type coverageIndexes struct {
	goal         curriculum.LearningGoalSpec
	curriculumID curriculum.CurriculumID
	competencies map[curriculum.ID]curriculum.Competency
	concepts     map[curriculum.ConceptID]curriculum.Concept
	byOutcome    map[curriculum.ID][]curriculum.Competency
	supports     map[curriculum.ID][]curriculum.CoverageSupport
}

func (CoverageEngineV1) Analyze(ctx context.Context, request CoverageAnalysisRequest) (curriculum.CoverageReport, error) {
	const operation = "analyze curriculum coverage"
	if err := ctx.Err(); err != nil {
		return curriculum.CoverageReport{}, ExternalError(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.CoverageReport{}, Invalid(operation, err)
	}
	indexes, err := indexCoverageRequest(request, knownEvidence)
	if err != nil {
		return curriculum.CoverageReport{}, Invalid(operation, err)
	}
	requirementsByDimension := make(map[curriculum.CoverageDimension][]curriculum.CoverageRequirement)
	seenRequirements := make(map[curriculum.ID]struct{}, len(request.Requirements))
	for _, requirement := range request.Requirements {
		if err := requirement.Validate(); err != nil {
			return curriculum.CoverageReport{}, Invalid(operation, err)
		}
		if _, exists := seenRequirements[requirement.ID]; exists {
			return curriculum.CoverageReport{}, Invalid(operation, fmt.Errorf("duplicate coverage requirement %q", requirement.ID))
		}
		seenRequirements[requirement.ID] = struct{}{}
		if err := requireKnownEvidence(requirement.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.CoverageReport{}, Invalid(operation, fmt.Errorf("coverage requirement %q: %w", requirement.ID, err))
		}
		if err := validateCoverageTarget(requirement, indexes); err != nil {
			return curriculum.CoverageReport{}, Invalid(operation, err)
		}
		requirementsByDimension[requirement.Dimension] = append(requirementsByDimension[requirement.Dimension], requirement)
	}
	for requirementID := range indexes.supports {
		if _, exists := seenRequirements[requirementID]; !exists {
			return curriculum.CoverageReport{}, Invalid(operation, fmt.Errorf("coverage support references missing requirement %q", requirementID))
		}
	}

	report := curriculum.CoverageReport{AlgorithmVersion: curriculum.CoverageEngineVersionV1}
	for _, dimension := range curriculum.AllCoverageDimensions() {
		if err := ctx.Err(); err != nil {
			return curriculum.CoverageReport{}, ExternalError(operation, err)
		}
		requirements := requirementsByDimension[dimension]
		sort.Slice(requirements, func(i, j int) bool { return requirements[i].ID.String() < requirements[j].ID.String() })
		dimensionReport := curriculum.CoverageDimensionReport{Dimension: dimension}
		for _, requirement := range requirements {
			status, reasons := assessCoverageRequirement(requirement, indexes)
			result := curriculum.CoverageResult{RequirementID: requirement.ID, Status: status, Reasons: reasons}
			dimensionReport.Requirements = append(dimensionReport.Requirements, result)
			switch status {
			case curriculum.CoverageMissing:
				dimensionReport.MissingRequirementIDs = append(dimensionReport.MissingRequirementIDs, requirement.ID)
			case curriculum.CoveragePartial:
				dimensionReport.PartialRequirementIDs = append(dimensionReport.PartialRequirementIDs, requirement.ID)
			case curriculum.CoverageCovered:
				dimensionReport.CoveredRequirementIDs = append(dimensionReport.CoveredRequirementIDs, requirement.ID)
			}
		}
		dimensionReport.Status = aggregateDimensionStatus(dimensionReport.Requirements)
		if len(requirements) == 0 {
			dimensionReport.Reasons = []string{"no_requirements_declared"}
		} else {
			dimensionReport.Reasons = []string{fmt.Sprintf("requirements:covered=%d:partial=%d:missing=%d", len(dimensionReport.CoveredRequirementIDs), len(dimensionReport.PartialRequirementIDs), len(dimensionReport.MissingRequirementIDs))}
		}
		report.Dimensions = append(report.Dimensions, dimensionReport)
	}
	if err := report.Validate(); err != nil {
		return curriculum.CoverageReport{}, Invalid(operation, err)
	}
	return report, nil
}

func indexCoverageRequest(request CoverageAnalysisRequest, knownEvidence map[evidenceKey]struct{}) (coverageIndexes, error) {
	if err := request.CurriculumID.Validate(); err != nil {
		return coverageIndexes{}, err
	}
	if err := request.Goal.Validate(); err != nil {
		return coverageIndexes{}, err
	}
	if err := request.Competencies.Validate(); err != nil {
		return coverageIndexes{}, err
	}
	if request.Competencies.GoalID != request.Goal.ID {
		return coverageIndexes{}, fmt.Errorf("coverage competency matrix belongs to another goal")
	}
	indexes := coverageIndexes{
		goal: request.Goal, curriculumID: request.CurriculumID,
		competencies: make(map[curriculum.ID]curriculum.Competency, len(request.Competencies.Competencies)),
		concepts:     make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts)),
		byOutcome:    make(map[curriculum.ID][]curriculum.Competency), supports: make(map[curriculum.ID][]curriculum.CoverageSupport),
	}
	outcomes := make(map[curriculum.ID]struct{}, len(request.Goal.Outcomes))
	for _, outcome := range request.Goal.Outcomes {
		outcomes[outcome.ID] = struct{}{}
		if err := requireKnownEvidence(outcome.EvidenceRefs, knownEvidence); err != nil {
			return coverageIndexes{}, fmt.Errorf("goal outcome %q: %w", outcome.ID, err)
		}
	}
	for _, competency := range request.Competencies.Competencies {
		if _, exists := outcomes[competency.OutcomeID]; !exists {
			return coverageIndexes{}, fmt.Errorf("coverage competency %q references outcome outside goal", competency.ID)
		}
		if err := requireKnownEvidence(competency.EvidenceRefs, knownEvidence); err != nil {
			return coverageIndexes{}, fmt.Errorf("coverage competency %q: %w", competency.ID, err)
		}
		indexes.competencies[competency.ID] = competency
		indexes.byOutcome[competency.OutcomeID] = append(indexes.byOutcome[competency.OutcomeID], competency)
	}
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return coverageIndexes{}, err
		}
		if _, exists := indexes.concepts[concept.ID]; exists {
			return coverageIndexes{}, fmt.Errorf("duplicate coverage concept %q", concept.ID)
		}
		if err := requireKnownEvidence(concept.EvidenceRefs, knownEvidence); err != nil {
			return coverageIndexes{}, fmt.Errorf("coverage concept %q: %w", concept.ID, err)
		}
		indexes.concepts[concept.ID] = concept
	}
	seenSupports := make(map[curriculum.ID]struct{}, len(request.Supports))
	for _, support := range request.Supports {
		if err := support.Validate(); err != nil {
			return coverageIndexes{}, err
		}
		if _, exists := seenSupports[support.ID]; exists {
			return coverageIndexes{}, fmt.Errorf("duplicate coverage support %q", support.ID)
		}
		seenSupports[support.ID] = struct{}{}
		for _, conceptID := range support.ConceptIDs {
			if _, exists := indexes.concepts[conceptID]; !exists {
				return coverageIndexes{}, fmt.Errorf("coverage support %q references missing concept %q", support.ID, conceptID)
			}
		}
		if err := requireKnownEvidence(support.EvidenceRefs, knownEvidence); err != nil {
			return coverageIndexes{}, fmt.Errorf("coverage support %q: %w", support.ID, err)
		}
		indexes.supports[support.RequirementID] = append(indexes.supports[support.RequirementID], support)
	}
	return indexes, nil
}

func validateCoverageTarget(requirement curriculum.CoverageRequirement, indexes coverageIndexes) error {
	switch requirement.TargetKind {
	case curriculum.CoverageTargetCurriculum:
		if requirement.TargetID.String() != indexes.curriculumID.String() {
			return fmt.Errorf("coverage requirement %q references another curriculum", requirement.ID)
		}
	case curriculum.CoverageTargetGoal:
		if requirement.TargetID != indexes.goal.ID {
			return fmt.Errorf("coverage requirement %q references another goal", requirement.ID)
		}
	case curriculum.CoverageTargetCompetency, curriculum.CoverageTargetConcept:
		// Missing target entities are valid analysis input and produce missing coverage.
	}
	return nil
}

func assessCoverageRequirement(requirement curriculum.CoverageRequirement, indexes coverageIndexes) (curriculum.CoverageStatus, []string) {
	if !coverageTargetExists(requirement, indexes) {
		return curriculum.CoverageMissing, []string{"coverage_target_missing"}
	}
	switch requirement.Dimension {
	case curriculum.CoverageCompetency:
		return assessCompetencyCoverage(requirement, indexes)
	case curriculum.CoverageConcept:
		return assessConceptCoverage(requirement, indexes)
	case curriculum.CoverageEvidence:
		return assessEvidenceCoverage(requirement, indexes)
	default:
		if len(indexes.supports[requirement.ID]) == 0 {
			return curriculum.CoverageMissing, []string{"no_explicit_coverage_support"}
		}
		return curriculum.CoverageCovered, []string{"explicit_coverage_support"}
	}
}

func coverageTargetExists(requirement curriculum.CoverageRequirement, indexes coverageIndexes) bool {
	switch requirement.TargetKind {
	case curriculum.CoverageTargetCurriculum, curriculum.CoverageTargetGoal:
		return true
	case curriculum.CoverageTargetCompetency:
		_, exists := indexes.competencies[requirement.TargetID]
		return exists
	case curriculum.CoverageTargetConcept:
		id, err := curriculum.NewConceptID(requirement.TargetID.String())
		if err != nil {
			return false
		}
		_, exists := indexes.concepts[id]
		return exists
	default:
		return false
	}
}

func assessCompetencyCoverage(requirement curriculum.CoverageRequirement, indexes coverageIndexes) (curriculum.CoverageStatus, []string) {
	if requirement.TargetKind == curriculum.CoverageTargetCompetency {
		if _, exists := indexes.competencies[requirement.TargetID]; exists {
			return curriculum.CoverageCovered, []string{"target_competency_exists"}
		}
		return curriculum.CoverageMissing, []string{"target_competency_missing"}
	}
	if requirement.TargetKind != curriculum.CoverageTargetGoal && requirement.TargetKind != curriculum.CoverageTargetCurriculum {
		return curriculum.CoverageMissing, []string{"competency_coverage_target_is_not_goal_curriculum_or_competency"}
	}
	covered := 0
	for _, outcome := range indexes.goal.Outcomes {
		if len(indexes.byOutcome[outcome.ID]) > 0 {
			covered++
		}
	}
	return cardinalityCoverageStatus(covered, len(indexes.goal.Outcomes), "goal_outcomes_with_competencies")
}

func assessConceptCoverage(requirement curriculum.CoverageRequirement, indexes coverageIndexes) (curriculum.CoverageStatus, []string) {
	if requirement.TargetKind == curriculum.CoverageTargetConcept {
		id, err := curriculum.NewConceptID(requirement.TargetID.String())
		if err == nil {
			if _, exists := indexes.concepts[id]; exists {
				return curriculum.CoverageCovered, []string{"target_concept_exists"}
			}
		}
		return curriculum.CoverageMissing, []string{"target_concept_missing"}
	}
	if requirement.TargetKind == curriculum.CoverageTargetCompetency {
		competency, exists := indexes.competencies[requirement.TargetID]
		if !exists || len(competency.ConceptRefs) == 0 {
			return curriculum.CoverageMissing, []string{"target_competency_has_no_concepts"}
		}
		covered := 0
		for _, id := range competency.ConceptRefs {
			if _, exists := indexes.concepts[id]; exists {
				covered++
			}
		}
		return cardinalityCoverageStatus(covered, len(competency.ConceptRefs), "competency_concept_refs_present")
	}
	if requirement.TargetKind != curriculum.CoverageTargetGoal && requirement.TargetKind != curriculum.CoverageTargetCurriculum {
		return curriculum.CoverageMissing, []string{"concept_coverage_target_is_unsupported"}
	}
	covered := 0
	for _, competency := range indexes.competencies {
		if competencyConceptsComplete(competency, indexes.concepts) {
			covered++
		}
	}
	return cardinalityCoverageStatus(covered, len(indexes.competencies), "competencies_with_complete_concept_refs")
}

func assessEvidenceCoverage(requirement curriculum.CoverageRequirement, indexes coverageIndexes) (curriculum.CoverageStatus, []string) {
	if len(indexes.supports[requirement.ID]) > 0 {
		for _, support := range indexes.supports[requirement.ID] {
			if len(support.EvidenceRefs) > 0 {
				return curriculum.CoverageCovered, []string{"explicit_verified_evidence_support"}
			}
		}
	}
	withEvidence, total := 0, 0
	switch requirement.TargetKind {
	case curriculum.CoverageTargetGoal:
		for _, outcome := range indexes.goal.Outcomes {
			total++
			if len(outcome.EvidenceRefs) > 0 {
				withEvidence++
			}
		}
	case curriculum.CoverageTargetCompetency:
		total = 1
		if competency, exists := indexes.competencies[requirement.TargetID]; exists && len(competency.EvidenceRefs) > 0 {
			withEvidence = 1
		}
	case curriculum.CoverageTargetConcept:
		total = 1
		if id, err := curriculum.NewConceptID(requirement.TargetID.String()); err == nil {
			if concept, exists := indexes.concepts[id]; exists && len(concept.EvidenceRefs) > 0 {
				withEvidence = 1
			}
		}
	case curriculum.CoverageTargetCurriculum:
		for _, outcome := range indexes.goal.Outcomes {
			total++
			if len(outcome.EvidenceRefs) > 0 {
				withEvidence++
			}
		}
		for _, competency := range indexes.competencies {
			total++
			if len(competency.EvidenceRefs) > 0 {
				withEvidence++
			}
		}
		for _, concept := range indexes.concepts {
			total++
			if len(concept.EvidenceRefs) > 0 {
				withEvidence++
			}
		}
	}
	return cardinalityCoverageStatus(withEvidence, total, "targets_with_verified_evidence")
}

func cardinalityCoverageStatus(covered, total int, reason string) (curriculum.CoverageStatus, []string) {
	detail := fmt.Sprintf("%s:%d/%d", reason, covered, total)
	if total == 0 || covered == 0 {
		return curriculum.CoverageMissing, []string{detail}
	}
	if covered == total {
		return curriculum.CoverageCovered, []string{detail}
	}
	return curriculum.CoveragePartial, []string{detail}
}

func competencyConceptsComplete(competency curriculum.Competency, concepts map[curriculum.ConceptID]curriculum.Concept) bool {
	if len(competency.ConceptRefs) == 0 {
		return false
	}
	for _, id := range competency.ConceptRefs {
		if _, exists := concepts[id]; !exists {
			return false
		}
	}
	return true
}

func aggregateDimensionStatus(results []curriculum.CoverageResult) curriculum.CoverageStatus {
	if len(results) == 0 {
		return curriculum.CoverageMissing
	}
	covered, missing := 0, 0
	for _, result := range results {
		if result.Status == curriculum.CoverageCovered {
			covered++
		}
		if result.Status == curriculum.CoverageMissing {
			missing++
		}
	}
	if covered == len(results) {
		return curriculum.CoverageCovered
	}
	if missing == len(results) {
		return curriculum.CoverageMissing
	}
	return curriculum.CoveragePartial
}
