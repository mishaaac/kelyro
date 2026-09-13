package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type FirstPrinciplesExpansionV1 struct{}

func NewFirstPrinciplesExpansionV1() FirstPrinciplesExpansionV1 {
	return FirstPrinciplesExpansionV1{}
}

func (FirstPrinciplesExpansionV1) Expand(ctx context.Context, request FirstPrinciplesExpansionRequest) (curriculum.FirstPrinciplesExpansionResult, error) {
	const operation = "expand first principles"
	if err := ctx.Err(); err != nil {
		return curriculum.FirstPrinciplesExpansionResult{}, ExternalError(operation, err)
	}
	if err := request.AuditResult.Validate(); err != nil {
		return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, err)
	}
	if request.AuditResult.LearnerProfile != curriculum.LearnerProfileZero {
		return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("first-principles expansion requires a zero-profile audit"))
	}
	knownEvidence, err := indexUsableEvidence(request.EvidenceSets)
	if err != nil {
		return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, err)
	}
	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("duplicate first-principles concept %q", concept.ID))
		}
		if err := requireKnownEvidence(concept.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("first-principles concept %q: %w", concept.ID, err))
		}
		concepts[concept.ID] = concept
	}
	adjacency := make(map[curriculum.ConceptID]map[curriculum.ConceptID]struct{})
	existingEdges := make(map[[2]curriculum.ConceptID]struct{}, len(request.Prerequisites))
	for _, prerequisite := range request.Prerequisites {
		if err := prerequisite.Validate(); err != nil {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, err)
		}
		if _, exists := concepts[prerequisite.ConceptID]; !exists {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("first-principles prerequisite references missing concept %q", prerequisite.ConceptID))
		}
		if _, exists := concepts[prerequisite.RequiredConceptID]; !exists {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("first-principles prerequisite references missing foundation %q", prerequisite.RequiredConceptID))
		}
		if err := requireKnownEvidence(prerequisite.EvidenceRefs, knownEvidence); err != nil {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("first-principles prerequisite: %w", err))
		}
		key := [2]curriculum.ConceptID{prerequisite.ConceptID, prerequisite.RequiredConceptID}
		existingEdges[key] = struct{}{}
		addAdjacency(adjacency, prerequisite.ConceptID, prerequisite.RequiredConceptID)
	}
	candidates := make(map[curriculum.ID]curriculum.FirstPrinciplesCandidate, len(request.Candidates))
	for _, candidate := range request.Candidates {
		if err := candidate.Validate(); err != nil {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, err)
		}
		if _, exists := candidates[candidate.RequirementID]; exists {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("duplicate first-principles candidate for requirement %q", candidate.RequirementID))
		}
		candidates[candidate.RequirementID] = candidate
	}

	violations := make(map[curriculum.ID][]curriculum.ZeroAssumptionViolation)
	for _, violation := range request.AuditResult.Violations {
		violations[violation.RequirementID] = append(violations[violation.RequirementID], violation)
	}
	result := curriculum.FirstPrinciplesExpansionResult{AlgorithmVersion: curriculum.FirstPrinciplesExpansionVersionV1}
	addedRoots := make(map[curriculum.ConceptID]curriculum.Concept)
	addedEdges := make(map[[2]curriculum.ConceptID]curriculum.Prerequisite)
	requirementIDs := make([]curriculum.ID, 0, len(violations))
	for id := range violations {
		requirementIDs = append(requirementIDs, id)
	}
	sort.Slice(requirementIDs, func(i, j int) bool { return requirementIDs[i].String() < requirementIDs[j].String() })
	for _, requirementID := range requirementIDs {
		if err := ctx.Err(); err != nil {
			return curriculum.FirstPrinciplesExpansionResult{}, ExternalError(operation, err)
		}
		findings := violations[requirementID]
		targets := firstPrinciplesTargets(findings)
		foundationID := findings[0].FoundationConceptID
		for _, finding := range findings[1:] {
			if finding.FoundationConceptID != foundationID {
				return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("audit requirement %q names multiple foundation concepts", requirementID))
			}
		}
		candidate, exists := candidates[requirementID]
		if !exists {
			result.UnresolvedResearchNeeds = append(result.UnresolvedResearchNeeds, firstPrinciplesNeed(requirementID, foundationID, targets, curriculum.FirstPrinciplesCandidateUnavailable, "no evidence-backed foundation candidate is available"))
			continue
		}
		if candidate.Concept.ID != foundationID {
			return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("candidate for requirement %q has foundation %q, want %q", requirementID, candidate.Concept.ID, foundationID))
		}
		if len(candidate.Concept.EvidenceRefs) == 0 || len(candidate.PrerequisiteEvidenceRefs) == 0 ||
			requireKnownEvidence(candidate.Concept.EvidenceRefs, knownEvidence) != nil || requireKnownEvidence(candidate.PrerequisiteEvidenceRefs, knownEvidence) != nil {
			result.UnresolvedResearchNeeds = append(result.UnresolvedResearchNeeds, firstPrinciplesNeed(requirementID, foundationID, targets, curriculum.FirstPrinciplesEvidenceInsufficient, "verified evidence is insufficient for the foundation concept or its prerequisite relation"))
			continue
		}
		if current, exists := concepts[foundationID]; exists {
			if !current.Foundational || current.Atomicity != curriculum.AtomicityAtomic {
				result.UnresolvedResearchNeeds = append(result.UnresolvedResearchNeeds, firstPrinciplesNeed(requirementID, foundationID, targets, curriculum.FirstPrinciplesImmutableConflict, "the existing immutable concept cannot be reclassified as an atomic foundation"))
				continue
			}
		} else {
			concepts[foundationID] = candidate.Concept
			addedRoots[foundationID] = candidate.Concept
		}
		cycleTargets := make([]curriculum.ConceptID, 0)
		for _, targetID := range targets {
			if _, exists := concepts[targetID]; !exists {
				return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, fmt.Errorf("first-principles target concept %q is missing", targetID))
			}
			key := [2]curriculum.ConceptID{targetID, foundationID}
			if _, exists := existingEdges[key]; exists {
				continue
			}
			if _, exists := addedEdges[key]; exists {
				continue
			}
			if pathExists(adjacency, foundationID, targetID) {
				cycleTargets = append(cycleTargets, targetID)
				continue
			}
			edge := curriculum.Prerequisite{
				ConceptID: targetID, RequiredConceptID: foundationID, Kind: candidate.PrerequisiteKind,
				EvidenceRefs: append([]curriculum.EvidenceRef(nil), candidate.PrerequisiteEvidenceRefs...),
			}
			addedEdges[key] = edge
			addAdjacency(adjacency, targetID, foundationID)
		}
		if len(cycleTargets) > 0 {
			result.UnresolvedResearchNeeds = append(result.UnresolvedResearchNeeds, firstPrinciplesNeed(requirementID, foundationID, cycleTargets, curriculum.FirstPrinciplesCyclePrevented, "adding the foundation relation would create a prerequisite cycle"))
		}
	}
	for _, id := range sortedConceptMapIDs(addedRoots) {
		result.ExpandedRootConcepts = append(result.ExpandedRootConcepts, cloneConcept(addedRoots[id]))
	}
	for _, edge := range addedEdges {
		edge.EvidenceRefs = append([]curriculum.EvidenceRef(nil), edge.EvidenceRefs...)
		sortEvidenceRefs(edge.EvidenceRefs)
		result.ExpandedPrerequisites = append(result.ExpandedPrerequisites, edge)
	}
	sort.Slice(result.ExpandedPrerequisites, func(i, j int) bool {
		return prerequisiteLess(result.ExpandedPrerequisites[i], result.ExpandedPrerequisites[j])
	})
	sort.Slice(result.UnresolvedResearchNeeds, func(i, j int) bool {
		left, right := result.UnresolvedResearchNeeds[i], result.UnresolvedResearchNeeds[j]
		if left.RequirementID != right.RequirementID {
			return left.RequirementID.String() < right.RequirementID.String()
		}
		return left.Code < right.Code
	})
	if err := result.Validate(); err != nil {
		return curriculum.FirstPrinciplesExpansionResult{}, Invalid(operation, err)
	}
	return result, nil
}

func firstPrinciplesTargets(findings []curriculum.ZeroAssumptionViolation) []curriculum.ConceptID {
	seen := make(map[curriculum.ConceptID]struct{}, len(findings))
	for _, finding := range findings {
		seen[finding.TargetConceptID] = struct{}{}
	}
	result := make([]curriculum.ConceptID, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func firstPrinciplesNeed(requirementID curriculum.ID, foundationID curriculum.ConceptID, targets []curriculum.ConceptID, code curriculum.FirstPrinciplesResearchNeedCode, reason string) curriculum.FirstPrinciplesResearchNeed {
	return curriculum.FirstPrinciplesResearchNeed{
		RequirementID: requirementID, FoundationConceptID: foundationID,
		TargetConceptIDs: append([]curriculum.ConceptID(nil), targets...), Code: code, Reason: reason,
	}
}
