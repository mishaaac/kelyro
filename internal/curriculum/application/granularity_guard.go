package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type GranularityGuardV1 struct {
	policy AtomicityPolicy
}

func NewGranularityGuardV1(policy AtomicityPolicy) *GranularityGuardV1 {
	return &GranularityGuardV1{policy: policy}
}

func (guard *GranularityGuardV1) Review(ctx context.Context, request GranularityReviewRequest) (curriculum.GranularityResult, error) {
	const operation = "review curriculum granularity"
	if err := ctx.Err(); err != nil {
		return curriculum.GranularityResult{}, ExternalError(operation, err)
	}
	if guard == nil || guard.policy == nil {
		return curriculum.GranularityResult{}, RequireDependency(operation, "atomicity policy", nil)
	}
	if guard.policy.Version() != curriculum.AtomicConceptCriteriaVersionV1 {
		return curriculum.GranularityResult{}, Invalid(operation, fmt.Errorf("unsupported atomicity policy %q", guard.policy.Version()))
	}

	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.GranularityResult{}, Invalid(operation, err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.GranularityResult{}, Invalid(operation, fmt.Errorf("duplicate concept %q", concept.ID))
		}
		concepts[concept.ID] = concept
	}
	if len(concepts) == 0 {
		return curriculum.GranularityResult{}, Invalid(operation, fmt.Errorf("granularity review has no concepts"))
	}

	result := curriculum.GranularityResult{AlgorithmVersion: curriculum.GranularityGuardVersionV1}
	conceptIDs := sortedConceptMapIDs(concepts)
	for _, conceptID := range conceptIDs {
		switch concepts[conceptID].Atomicity {
		case curriculum.AtomicityNeedsSplit:
			result.ForcedSplit = append(result.ForcedSplit, conceptID)
			result.Warnings = append(result.Warnings, curriculum.GranularityWarning{Code: "concept_requires_split", ConceptIDs: []curriculum.ConceptID{conceptID}, Message: "concept has multiple independently assessable parts"})
		case curriculum.AtomicityTooFragmented:
			result.Warnings = append(result.Warnings, curriculum.GranularityWarning{Code: "concept_too_fragmented", ConceptIDs: []curriculum.ConceptID{conceptID}, Message: "concept should remain identifiable but may be grouped visually"})
		case curriculum.AtomicityUnknown:
			result.Warnings = append(result.Warnings, curriculum.GranularityWarning{Code: "concept_atomicity_unknown", ConceptIDs: []curriculum.ConceptID{conceptID}, Message: "concept atomicity requires resolution"})
		}
	}

	proposals := append([]curriculum.ConceptMergeProposal(nil), request.MergeProposals...)
	for index := range proposals {
		proposals[index].ConceptIDs = append([]curriculum.ConceptID(nil), proposals[index].ConceptIDs...)
		proposals[index].MergedCriteria.IndependentlyAssessableParts = append([]string(nil), proposals[index].MergedCriteria.IndependentlyAssessableParts...)
		sort.Slice(proposals[index].ConceptIDs, func(i, j int) bool {
			return proposals[index].ConceptIDs[i].String() < proposals[index].ConceptIDs[j].String()
		})
	}
	sort.Slice(proposals, func(i, j int) bool { return proposals[i].ID.String() < proposals[j].ID.String() })
	seenProposals := make(map[curriculum.ID]struct{}, len(proposals))
	for _, proposal := range proposals {
		if err := proposal.Validate(); err != nil {
			return curriculum.GranularityResult{}, Invalid(operation, err)
		}
		if _, exists := seenProposals[proposal.ID]; exists {
			return curriculum.GranularityResult{}, Invalid(operation, fmt.Errorf("duplicate merge proposal %q", proposal.ID))
		}
		seenProposals[proposal.ID] = struct{}{}
		for _, conceptID := range proposal.ConceptIDs {
			if _, exists := concepts[conceptID]; !exists {
				return curriculum.GranularityResult{}, Invalid(operation, fmt.Errorf("merge proposal %q references missing concept %q", proposal.ID, conceptID))
			}
		}
		assessment, err := guard.policy.Assess(proposal.MergedCriteria)
		if err != nil {
			return curriculum.GranularityResult{}, Invalid(operation, fmt.Errorf("merge proposal %q: %w", proposal.ID, err))
		}
		allowed := assessment.Result == curriculum.AtomicityAtomic
		result.MergeDecisions = append(result.MergeDecisions, curriculum.ConceptMergeDecision{ProposalID: proposal.ID, Allowed: allowed, Reasons: append([]string(nil), assessment.Reasons...)})
		if !allowed {
			result.Warnings = append(result.Warnings, curriculum.GranularityWarning{Code: "merge_rejected", ConceptIDs: append([]curriculum.ConceptID(nil), proposal.ConceptIDs...), Message: "merge would not preserve atomicity and independent evaluability"})
		}
	}

	groups := append([]curriculum.VisualConceptGroup(nil), request.VisualGroups...)
	for index := range groups {
		groups[index].ConceptIDs = append([]curriculum.ConceptID(nil), groups[index].ConceptIDs...)
		sort.Slice(groups[index].ConceptIDs, func(i, j int) bool {
			return groups[index].ConceptIDs[i].String() < groups[index].ConceptIDs[j].String()
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].LessonID.String() < groups[j].LessonID.String() })
	seenLessons := make(map[curriculum.ID]struct{}, len(groups))
	seenVisualConcepts := make(map[curriculum.ConceptID]struct{})
	for _, group := range groups {
		if err := group.Validate(); err != nil {
			return curriculum.GranularityResult{}, Invalid(operation, err)
		}
		if _, exists := seenLessons[group.LessonID]; exists {
			return curriculum.GranularityResult{}, Invalid(operation, fmt.Errorf("duplicate visual group for lesson %q", group.LessonID))
		}
		seenLessons[group.LessonID] = struct{}{}
		for _, conceptID := range group.ConceptIDs {
			if _, exists := concepts[conceptID]; !exists {
				return curriculum.GranularityResult{}, Invalid(operation, fmt.Errorf("visual group %q references missing concept %q", group.LessonID, conceptID))
			}
			if _, exists := seenVisualConcepts[conceptID]; exists {
				return curriculum.GranularityResult{}, Invalid(operation, fmt.Errorf("concept %q appears in multiple visual groups", conceptID))
			}
			seenVisualConcepts[conceptID] = struct{}{}
		}
		result.SafeVisualGrouping = append(result.SafeVisualGrouping, group)
	}
	if err := result.Validate(); err != nil {
		return curriculum.GranularityResult{}, Invalid(operation, err)
	}
	return result, nil
}

func sortedConceptMapIDs(values map[curriculum.ConceptID]curriculum.Concept) []curriculum.ConceptID {
	result := make([]curriculum.ConceptID, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

var _ GranularityGuardService = (*GranularityGuardV1)(nil)
