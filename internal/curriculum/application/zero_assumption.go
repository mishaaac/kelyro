package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type ZeroAssumptionAuditV1 struct{}

func NewZeroAssumptionAuditV1() ZeroAssumptionAuditV1 {
	return ZeroAssumptionAuditV1{}
}

func (ZeroAssumptionAuditV1) Audit(ctx context.Context, request ZeroAssumptionAuditRequest) (curriculum.ZeroAssumptionAuditResult, error) {
	const operation = "audit zero assumptions"
	if err := ctx.Err(); err != nil {
		return curriculum.ZeroAssumptionAuditResult{}, ExternalError(operation, err)
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, err)
	}
	if err := request.DomainProfile.Validate(); err != nil {
		return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, err)
	}
	if err := request.Baseline.Validate(); err != nil {
		return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, err)
	}
	if err := request.Competencies.Validate(); err != nil {
		return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, err)
	}
	if err := request.Graph.Validate(); err != nil {
		return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, err)
	}
	if request.Baseline.DomainProfileID != request.DomainProfile.ID || request.Baseline.DomainProfileVersion != request.DomainProfile.Version {
		return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, fmt.Errorf("assumption baseline does not match domain profile"))
	}

	profileAreas := make(map[curriculum.ID]struct{}, len(request.DomainProfile.Areas))
	for _, area := range request.DomainProfile.Areas {
		profileAreas[area.ID] = struct{}{}
		if err := requireKnownEvidence(area.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, fmt.Errorf("domain profile area %q: %w", area.ID, err))
		}
	}
	competencies := make(map[curriculum.ID]curriculum.Competency, len(request.Competencies.Competencies))
	for _, competency := range request.Competencies.Competencies {
		if _, exists := profileAreas[competency.AreaID]; !exists {
			return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, fmt.Errorf("competency %q is outside the domain profile", competency.ID))
		}
		competencies[competency.ID] = competency
	}
	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, fmt.Errorf("duplicate zero-assumption concept %q", concept.ID))
		}
		if err := requireKnownEvidence(concept.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, fmt.Errorf("zero-assumption concept %q: %w", concept.ID, err))
		}
		concepts[concept.ID] = concept
	}
	if err := requireExactAuditGraphConcepts(request.Graph.ConceptIDs, concepts); err != nil {
		return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, err)
	}
	assumed := make(map[curriculum.ConceptID]struct{}, len(request.Baseline.AssumedConceptIDs))
	for _, id := range request.Baseline.AssumedConceptIDs {
		assumed[id] = struct{}{}
	}
	dependencies := make(map[curriculum.ConceptID][]curriculum.ConceptID)
	for _, prerequisite := range request.Graph.Prerequisites {
		dependencies[prerequisite.ConceptID] = append(dependencies[prerequisite.ConceptID], prerequisite.RequiredConceptID)
	}

	result := curriculum.ZeroAssumptionAuditResult{
		BaselineID: request.Baseline.ID, BaselineVersion: request.Baseline.Version,
		LearnerProfile: request.Baseline.LearnerProfile, AuditedRequirements: len(request.Baseline.Requirements),
		AlgorithmVersion: curriculum.ZeroAssumptionAuditVersionV1,
	}
	requirements := append([]curriculum.FoundationRequirement(nil), request.Baseline.Requirements...)
	sort.Slice(requirements, func(i, j int) bool { return requirements[i].ID.String() < requirements[j].ID.String() })
	for _, requirement := range requirements {
		if err := ctx.Err(); err != nil {
			return curriculum.ZeroAssumptionAuditResult{}, ExternalError(operation, err)
		}
		if err := requireKnownEvidence(requirement.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, fmt.Errorf("foundation requirement %q: %w", requirement.ID, err))
		}
		if _, exists := assumed[requirement.ConceptID]; exists {
			result.AssumedRequirements++
			continue
		}
		foundation, present := concepts[requirement.ConceptID]
		for _, competencyID := range requirement.TargetCompetencyIDs {
			competency, exists := competencies[competencyID]
			if !exists {
				return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, fmt.Errorf("foundation requirement %q references missing competency %q", requirement.ID, competencyID))
			}
			for _, targetID := range competency.ConceptRefs {
				if _, exists := concepts[targetID]; !exists {
					return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, fmt.Errorf("competency %q references concept outside zero-assumption graph %q", competency.ID, targetID))
				}
				if targetID == requirement.ConceptID {
					continue
				}
				code, reason := curriculum.ZeroAssumptionMissingPrerequisite, "required foundation is not a prerequisite of the competency concept"
				if !present {
					code, reason = curriculum.ZeroAssumptionMissingFoundation, "relevant foundation is absent from the curriculum"
				} else if !foundation.Foundational {
					code, reason = curriculum.ZeroAssumptionNotFoundational, "required foundation concept is not declared foundational"
				} else if graphRequires(dependencies, targetID, requirement.ConceptID) {
					continue
				}
				result.Violations = append(result.Violations, curriculum.ZeroAssumptionViolation{
					Code: code, RequirementID: requirement.ID, FoundationConceptID: requirement.ConceptID,
					TargetCompetencyID: competency.ID, TargetConceptID: targetID,
					Severity: curriculum.CurriculumAuditError, Reason: reason + ": " + requirement.Reason,
					EvidenceRefs: append([]curriculum.EvidenceRef(nil), requirement.EvidenceRefs...),
				})
			}
		}
	}
	sort.Slice(result.Violations, func(i, j int) bool {
		left, right := result.Violations[i], result.Violations[j]
		if left.RequirementID != right.RequirementID {
			return left.RequirementID.String() < right.RequirementID.String()
		}
		if left.TargetCompetencyID != right.TargetCompetencyID {
			return left.TargetCompetencyID.String() < right.TargetCompetencyID.String()
		}
		if left.TargetConceptID != right.TargetConceptID {
			return left.TargetConceptID.String() < right.TargetConceptID.String()
		}
		return left.Code < right.Code
	})
	result.Passed = len(result.Violations) == 0
	if err := result.Validate(); err != nil {
		return curriculum.ZeroAssumptionAuditResult{}, Invalid(operation, err)
	}
	return result, nil
}

func requireExactAuditGraphConcepts(graphIDs []curriculum.ConceptID, concepts map[curriculum.ConceptID]curriculum.Concept) error {
	if len(graphIDs) != len(concepts) {
		return fmt.Errorf("zero-assumption concepts do not match knowledge graph")
	}
	for _, id := range graphIDs {
		if _, exists := concepts[id]; !exists {
			return fmt.Errorf("knowledge graph references missing zero-assumption concept %q", id)
		}
	}
	return nil
}
