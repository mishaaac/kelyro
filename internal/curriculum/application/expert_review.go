package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type ExpertCoverageReviewerV1 struct {
	advisor ExpertCoverageAdvisor
}

func NewExpertCoverageReviewerV1(advisor ExpertCoverageAdvisor) ExpertCoverageReviewerV1 {
	return ExpertCoverageReviewerV1{advisor: advisor}
}

func (reviewer ExpertCoverageReviewerV1) Review(ctx context.Context, request ExpertCoverageReviewRequest) (curriculum.ExpertCoverageReviewResult, error) {
	const operation = "review expert curriculum coverage"
	if err := ctx.Err(); err != nil {
		return curriculum.ExpertCoverageReviewResult{}, ExternalError(operation, err)
	}
	if err := request.Goal.Validate(); err != nil {
		return curriculum.ExpertCoverageReviewResult{}, Invalid(operation, err)
	}
	if err := request.Competencies.Validate(); err != nil {
		return curriculum.ExpertCoverageReviewResult{}, Invalid(operation, err)
	}
	if request.Competencies.GoalID != request.Goal.ID {
		return curriculum.ExpertCoverageReviewResult{}, Invalid(operation, fmt.Errorf("competency matrix belongs to another goal"))
	}
	if err := request.Coverage.Validate(); err != nil {
		return curriculum.ExpertCoverageReviewResult{}, Invalid(operation, err)
	}

	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.ExpertCoverageReviewResult{}, Invalid(operation, err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.ExpertCoverageReviewResult{}, Invalid(operation, fmt.Errorf("expert review concepts contain duplicate %q", concept.ID))
		}
		concepts[concept.ID] = concept
	}
	competenciesByOutcome := make(map[curriculum.ID][]curriculum.Competency)
	for _, competency := range request.Competencies.Competencies {
		for _, conceptID := range competency.ConceptRefs {
			if _, exists := concepts[conceptID]; !exists {
				return curriculum.ExpertCoverageReviewResult{}, Invalid(operation, fmt.Errorf("competency %q references unknown concept %q", competency.ID, conceptID))
			}
		}
		competenciesByOutcome[competency.OutcomeID] = append(competenciesByOutcome[competency.OutcomeID], competency)
	}

	result := curriculum.ExpertCoverageReviewResult{
		Professional:     request.Goal.Role != nil,
		AlgorithmVersion: curriculum.ExpertCoverageReviewVersionV1,
	}
	for _, outcome := range request.Goal.Outcomes {
		result.ReviewedOutcomeIDs = append(result.ReviewedOutcomeIDs, outcome.ID)
		competencies := competenciesByOutcome[outcome.ID]
		if outcome.Capability != "" && (len(competencies) == 0 || !outcomeHasRequiredLevel(outcome, competencies)) {
			result.Findings = append(result.Findings, curriculum.ExpertCoverageFinding{
				Kind: curriculum.ExpertMissingAdvancedCompetency, TargetID: outcome.ID,
				Reason: "no competency reaches the level required for professional outcome capability " + string(outcome.Capability),
			})
		}
	}
	for _, competency := range request.Competencies.Competencies {
		result.ReviewedCompetencyIDs = append(result.ReviewedCompetencyIDs, competency.ID)
		requiredDifficulty := competencyDifficulty(competency.ExpectedLevel)
		deepEnough := false
		for _, conceptID := range competency.ConceptRefs {
			if concepts[conceptID].Difficulty >= requiredDifficulty {
				deepEnough = true
				break
			}
		}
		if !deepEnough {
			result.Findings = append(result.Findings, curriculum.ExpertCoverageFinding{
				Kind: curriculum.ExpertInsufficientDepth, TargetID: competency.ID,
				Reason: fmt.Sprintf("competency level %s requires at least one concept at difficulty %d", competency.ExpectedLevel, requiredDifficulty),
			})
		}
	}
	if result.Professional {
		for _, dimension := range []curriculum.CoverageDimension{curriculum.CoverageProduction, curriculum.CoverageSecurity, curriculum.CoverageToolchain} {
			status := coverageDimensionStatus(request.Coverage, dimension)
			if status != curriculum.CoverageCovered {
				result.Findings = append(result.Findings, curriculum.ExpertCoverageFinding{
					Kind: curriculum.ExpertMissingProductionCapability, TargetID: request.Goal.ID, Dimension: dimension,
					Reason: fmt.Sprintf("professional route has %s %s coverage", status, dimension),
				})
			}
		}
	}
	sort.Slice(result.Findings, func(i, j int) bool {
		if result.Findings[i].Kind != result.Findings[j].Kind {
			return result.Findings[i].Kind < result.Findings[j].Kind
		}
		if result.Findings[i].TargetID != result.Findings[j].TargetID {
			return result.Findings[i].TargetID.String() < result.Findings[j].TargetID.String()
		}
		return result.Findings[i].Dimension < result.Findings[j].Dimension
	})
	sort.Slice(result.ReviewedOutcomeIDs, func(i, j int) bool {
		return result.ReviewedOutcomeIDs[i].String() < result.ReviewedOutcomeIDs[j].String()
	})
	sort.Slice(result.ReviewedCompetencyIDs, func(i, j int) bool {
		return result.ReviewedCompetencyIDs[i].String() < result.ReviewedCompetencyIDs[j].String()
	})
	result.Passed = len(result.Findings) == 0
	if err := result.Validate(); err != nil {
		return curriculum.ExpertCoverageReviewResult{}, Invalid(operation, err)
	}
	if reviewer.advisor != nil {
		notes, err := reviewer.advisor.Advise(ctx, request, result)
		if err != nil {
			result.AdvisorNotes = []string{"advisor unavailable: " + err.Error()}
		} else {
			for _, note := range notes {
				if normalized := strings.TrimSpace(note); normalized != "" {
					result.AdvisorNotes = append(result.AdvisorNotes, normalized)
				}
			}
			sort.Strings(result.AdvisorNotes)
		}
	}
	if err := result.Validate(); err != nil {
		return curriculum.ExpertCoverageReviewResult{}, Invalid(operation, err)
	}
	return result, nil
}

func outcomeHasRequiredLevel(outcome curriculum.GoalOutcome, competencies []curriculum.Competency) bool {
	for _, competency := range competencies {
		if levelMeetsCapability(competency.ExpectedLevel, outcome.Capability) {
			return true
		}
	}
	return false
}

func levelMeetsCapability(level curriculum.CompetencyLevel, capability curriculum.OutcomeCapability) bool {
	switch capability {
	case curriculum.OutcomeCapabilityBuild:
		return level == curriculum.CompetencyApply || level == curriculum.CompetencyAnalyze || level == curriculum.CompetencyDesign || level == curriculum.CompetencyOperate
	case curriculum.OutcomeCapabilityDebug, curriculum.OutcomeCapabilityMaintain:
		return level == curriculum.CompetencyAnalyze || level == curriculum.CompetencyDesign || level == curriculum.CompetencyOperate
	case curriculum.OutcomeCapabilityOperate:
		return level == curriculum.CompetencyOperate
	default:
		return level != curriculum.CompetencyAwareness
	}
}

func competencyDifficulty(level curriculum.CompetencyLevel) curriculum.Difficulty {
	switch level {
	case curriculum.CompetencyAwareness:
		return curriculum.DifficultyIntroductory
	case curriculum.CompetencyUnderstand:
		return curriculum.DifficultyFoundational
	case curriculum.CompetencyApply:
		return curriculum.DifficultyIntermediate
	default:
		return curriculum.DifficultyAdvanced
	}
}

func coverageDimensionStatus(report curriculum.CoverageReport, target curriculum.CoverageDimension) curriculum.CoverageStatus {
	for _, dimension := range report.Dimensions {
		if dimension.Dimension == target {
			return dimension.Status
		}
	}
	return curriculum.CoverageMissing
}

var _ ExpertCoverageReviewService = ExpertCoverageReviewerV1{}
