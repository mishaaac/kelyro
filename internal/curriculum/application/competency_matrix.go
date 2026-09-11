package application

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type CompetencyMatrixBuilderV1 struct{}

func NewCompetencyMatrixBuilderV1() CompetencyMatrixBuilderV1 {
	return CompetencyMatrixBuilderV1{}
}

func (CompetencyMatrixBuilderV1) Build(ctx context.Context, request CompetencyMatrixRequest) (curriculum.CompetencyMatrix, error) {
	const operation = "build competency matrix"
	if err := ctx.Err(); err != nil {
		return curriculum.CompetencyMatrix{}, ExternalError(operation, err)
	}
	if err := request.Decomposition.Validate(); err != nil {
		return curriculum.CompetencyMatrix{}, Invalid(operation, err)
	}
	evidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.CompetencyMatrix{}, Invalid(operation, err)
	}
	if len(request.Competencies) == 0 {
		return curriculum.CompetencyMatrix{}, Invalid(operation, fmt.Errorf("competency specification is empty"))
	}

	outcomes := make(map[curriculum.ID]struct{}, len(request.Decomposition.Outcomes))
	for _, outcome := range request.Decomposition.Outcomes {
		outcomes[outcome.ID] = struct{}{}
	}
	areas := make(map[curriculum.ID]curriculum.CompetencyArea, len(request.Decomposition.CompetencyAreas))
	for _, area := range request.Decomposition.CompetencyAreas {
		areas[area.ID] = area
	}
	seen := make(map[curriculum.ID]struct{}, len(request.Competencies))
	coveredOutcomes := make(map[curriculum.ID]struct{}, len(outcomes))
	coveredAreas := make(map[curriculum.ID]struct{}, len(areas))
	competencies := cloneCompetencies(request.Competencies)
	for _, competency := range competencies {
		if err := competency.Validate(); err != nil {
			return curriculum.CompetencyMatrix{}, Invalid(operation, err)
		}
		if _, exists := seen[competency.ID]; exists {
			return curriculum.CompetencyMatrix{}, Invalid(operation, fmt.Errorf("duplicate competency %q", competency.ID))
		}
		seen[competency.ID] = struct{}{}
		area, exists := areas[competency.AreaID]
		if !exists {
			return curriculum.CompetencyMatrix{}, Invalid(operation, fmt.Errorf("competency %q references unsupported area %q", competency.ID, competency.AreaID))
		}
		if competency.Area != area.Name {
			return curriculum.CompetencyMatrix{}, Invalid(operation, fmt.Errorf("competency %q area name does not match decomposition", competency.ID))
		}
		if _, exists := outcomes[competency.OutcomeID]; !exists || !areaCoversOutcome(area, competency.OutcomeID) {
			return curriculum.CompetencyMatrix{}, Invalid(operation, fmt.Errorf("competency %q references unsupported outcome %q for area %q", competency.ID, competency.OutcomeID, competency.AreaID))
		}
		if len(competency.EvidenceRefs) == 0 {
			return curriculum.CompetencyMatrix{}, Invalid(operation, fmt.Errorf("competency %q has no evidence", competency.ID))
		}
		if err := requireKnownEvidence(competency.EvidenceRefs, evidence); err != nil {
			return curriculum.CompetencyMatrix{}, Invalid(operation, fmt.Errorf("competency %q: %w", competency.ID, err))
		}
		coveredOutcomes[competency.OutcomeID] = struct{}{}
		coveredAreas[competency.AreaID] = struct{}{}
	}
	for _, outcome := range request.Decomposition.Outcomes {
		if _, exists := coveredOutcomes[outcome.ID]; !exists {
			return curriculum.CompetencyMatrix{}, Invalid(operation, fmt.Errorf("goal outcome %q is not covered by a competency", outcome.ID))
		}
	}
	for _, area := range request.Decomposition.CompetencyAreas {
		if _, exists := coveredAreas[area.ID]; !exists {
			return curriculum.CompetencyMatrix{}, Invalid(operation, fmt.Errorf("competency area %q is not covered by a competency", area.ID))
		}
	}
	result := curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: request.Decomposition.GoalID, Competencies: competencies}
	if err := result.Validate(); err != nil {
		return curriculum.CompetencyMatrix{}, Invalid(operation, err)
	}
	return result, nil
}

func areaCoversOutcome(area curriculum.CompetencyArea, outcomeID curriculum.ID) bool {
	for _, candidate := range area.OutcomeIDs {
		if candidate == outcomeID {
			return true
		}
	}
	return false
}

func cloneCompetencies(values []curriculum.Competency) []curriculum.Competency {
	result := append([]curriculum.Competency(nil), values...)
	for index := range result {
		result[index].Dimensions = append([]curriculum.CompetencyDimension(nil), result[index].Dimensions...)
		result[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), result[index].EvidenceRefs...)
		result[index].ConceptRefs = append([]curriculum.ConceptID(nil), result[index].ConceptRefs...)
		if result[index].ParentID != nil {
			parent := *result[index].ParentID
			result[index].ParentID = &parent
		}
	}
	return result
}

var _ CompetencyMatrixBuilderService = CompetencyMatrixBuilderV1{}
