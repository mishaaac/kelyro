package application

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type GoalDecomposerV1 struct{}

func NewGoalDecomposerV1() GoalDecomposerV1 { return GoalDecomposerV1{} }

func (GoalDecomposerV1) Decompose(ctx context.Context, request GoalDecompositionRequest) (curriculum.GoalDecomposition, error) {
	const operation = "decompose goal"
	if err := ctx.Err(); err != nil {
		return curriculum.GoalDecomposition{}, ExternalError(operation, err)
	}
	if err := request.Goal.Validate(); err != nil {
		return curriculum.GoalDecomposition{}, Invalid(operation, err)
	}
	if err := request.Profile.Validate(); err != nil {
		return curriculum.GoalDecomposition{}, Invalid(operation, err)
	}
	if request.Profile.Domain != request.Goal.Domain {
		return curriculum.GoalDecomposition{}, Invalid(operation, fmt.Errorf("domain profile %q does not support goal domain %q", request.Profile.Domain, request.Goal.Domain))
	}
	evidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.GoalDecomposition{}, Invalid(operation, err)
	}
	for _, scope := range request.Goal.Scope {
		if !containsString(request.Profile.SupportedScopes, scope) {
			return curriculum.GoalDecomposition{}, Invalid(operation, fmt.Errorf("unsupported goal scope %q", scope))
		}
	}
	outcomes := make(map[curriculum.ID]struct{}, len(request.Goal.Outcomes))
	for _, outcome := range request.Goal.Outcomes {
		outcomes[outcome.ID] = struct{}{}
		if len(outcome.EvidenceRefs) == 0 {
			return curriculum.GoalDecomposition{}, Invalid(operation, fmt.Errorf("goal outcome %q has no evidence", outcome.ID))
		}
		if err := requireKnownEvidence(outcome.EvidenceRefs, evidence); err != nil {
			return curriculum.GoalDecomposition{}, Invalid(operation, fmt.Errorf("goal outcome %q: %w", outcome.ID, err))
		}
	}
	result := curriculum.GoalDecomposition{
		GoalID: request.Goal.ID, Outcomes: cloneOutcomes(request.Goal.Outcomes),
		Scope: append([]string(nil), request.Goal.Scope...), Exclusions: append([]string(nil), request.Goal.Exclusions...),
		ProfileID: request.Profile.ID, ProfileVersion: request.Profile.Version,
		AlgorithmVersion: curriculum.GoalDecomposerVersionV1,
	}
	covered := make(map[curriculum.ID]struct{}, len(outcomes))
	for _, candidate := range request.Profile.Areas {
		if !areaApplies(candidate, request.Goal) {
			continue
		}
		for _, outcomeID := range candidate.OutcomeIDs {
			if _, exists := outcomes[outcomeID]; !exists {
				return curriculum.GoalDecomposition{}, Invalid(operation, fmt.Errorf("selected competency area %q references unknown goal outcome %q", candidate.ID, outcomeID))
			}
			covered[outcomeID] = struct{}{}
		}
		if err := requireKnownEvidence(candidate.EvidenceRefs, evidence); err != nil {
			return curriculum.GoalDecomposition{}, Invalid(operation, fmt.Errorf("competency area %q: %w", candidate.ID, err))
		}
		result.CompetencyAreas = append(result.CompetencyAreas, curriculum.CompetencyArea{
			ID: candidate.ID, Name: candidate.Name, Description: candidate.Description,
			OutcomeIDs:   append([]curriculum.ID(nil), candidate.OutcomeIDs...),
			EvidenceRefs: append([]curriculum.EvidenceRef(nil), candidate.EvidenceRefs...),
		})
	}
	if len(result.CompetencyAreas) == 0 {
		return curriculum.GoalDecomposition{}, Invalid(operation, fmt.Errorf("goal scope selects no competency areas"))
	}
	for outcomeID := range outcomes {
		if _, exists := covered[outcomeID]; !exists {
			return curriculum.GoalDecomposition{}, Invalid(operation, fmt.Errorf("goal outcome %q is not covered by selected competency areas", outcomeID))
		}
	}
	if err := result.Validate(); err != nil {
		return curriculum.GoalDecomposition{}, Invalid(operation, err)
	}
	return result, nil
}

type evidenceKey struct{ bundle, claim string }

func indexUsableEvidence(sets []curriculum.CurriculumEvidenceSet) (map[evidenceKey]struct{}, error) {
	if len(sets) == 0 {
		return nil, fmt.Errorf("goal decomposition requires evidence")
	}
	result := make(map[evidenceKey]struct{})
	for index, set := range sets {
		if err := set.Validate(); err != nil {
			return nil, fmt.Errorf("evidence set %d: %w", index, err)
		}
		if set.Eligibility == curriculum.EvidenceNotReady {
			return nil, fmt.Errorf("evidence set %q is not ready", set.Bundle.ID)
		}
		for _, claim := range set.Claims {
			result[evidenceKey{bundle: set.Bundle.ID.String(), claim: claim.ID.String()}] = struct{}{}
		}
	}
	return result, nil
}

func requireKnownEvidence(references []curriculum.EvidenceRef, evidence map[evidenceKey]struct{}) error {
	for _, reference := range references {
		if _, exists := evidence[evidenceKey{bundle: reference.BundleID.String(), claim: reference.ClaimID.String()}]; !exists {
			return fmt.Errorf("references unavailable evidence %s/%s", reference.BundleID, reference.ClaimID)
		}
	}
	return nil
}

func areaApplies(area curriculum.CompetencyAreaSpec, goal curriculum.LearningGoalSpec) bool {
	if area.RequiredForProfessional {
		return goal.Role != nil
	}
	if len(area.Scopes) == 0 || len(goal.Scope) == 0 {
		return true
	}
	for _, scope := range area.Scopes {
		if containsString(goal.Scope, scope) {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneOutcomes(values []curriculum.GoalOutcome) []curriculum.GoalOutcome {
	result := append([]curriculum.GoalOutcome(nil), values...)
	for index := range result {
		result[index].EvidenceRefs = append([]curriculum.EvidenceRef(nil), result[index].EvidenceRefs...)
	}
	return result
}

var _ GoalDecomposerService = GoalDecomposerV1{}
