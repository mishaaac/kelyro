package application

import (
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type BeginnerSimulatorV1 struct{}

func NewBeginnerSimulatorV1() BeginnerSimulatorV1 { return BeginnerSimulatorV1{} }

func (BeginnerSimulatorV1) Simulate(ctx context.Context, request BeginnerSimulationRequest) (curriculum.BeginnerSimulationResult, error) {
	const operation = "simulate beginner curriculum traversal"
	if err := ctx.Err(); err != nil {
		return curriculum.BeginnerSimulationResult{}, ExternalError(operation, err)
	}
	if err := request.Graph.Validate(); err != nil {
		return curriculum.BeginnerSimulationResult{}, Invalid(operation, err)
	}
	if err := request.Hierarchy.Validate(request.Concepts); err != nil {
		return curriculum.BeginnerSimulationResult{}, Invalid(operation, err)
	}
	if err := request.Vocabulary.Validate(); err != nil {
		return curriculum.BeginnerSimulationResult{}, Invalid(operation, err)
	}

	knownConcepts := make(map[curriculum.ConceptID]struct{}, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.BeginnerSimulationResult{}, Invalid(operation, err)
		}
		knownConcepts[concept.ID] = struct{}{}
	}
	if len(knownConcepts) != len(request.Concepts) {
		return curriculum.BeginnerSimulationResult{}, Invalid(operation, fmt.Errorf("beginner simulation concepts contain duplicates"))
	}
	topicByConcept := make(map[curriculum.ConceptID]curriculum.ID, len(request.Concepts))
	for _, topic := range request.Hierarchy.Topics {
		for _, conceptID := range topic.ConceptIDs {
			topicByConcept[conceptID] = topic.ID
		}
	}

	prerequisites := make(map[curriculum.ConceptID][]curriculum.ConceptID)
	for _, edge := range request.Graph.Prerequisites {
		prerequisites[edge.ConceptID] = append(prerequisites[edge.ConceptID], edge.RequiredConceptID)
	}
	for conceptID := range prerequisites {
		sort.Slice(prerequisites[conceptID], func(i, j int) bool {
			return prerequisites[conceptID][i].String() < prerequisites[conceptID][j].String()
		})
	}

	vocabularyIntroductions := make(map[curriculum.ConceptID][]string)
	vocabularyUses := make(map[curriculum.ConceptID][]curriculum.ResolvedVocabularyUse)
	for _, term := range request.Vocabulary.Graph.Terms {
		vocabularyIntroductions[term.IntroducedBy] = append(vocabularyIntroductions[term.IntroducedBy], term.Term)
	}
	for _, use := range request.Vocabulary.ResolvedUses {
		vocabularyUses[use.UsedAt] = append(vocabularyUses[use.UsedAt], use)
	}
	for conceptID := range vocabularyIntroductions {
		sort.Strings(vocabularyIntroductions[conceptID])
	}
	for conceptID := range vocabularyUses {
		sort.Slice(vocabularyUses[conceptID], func(i, j int) bool {
			return vocabularyUses[conceptID][i].CanonicalTerm < vocabularyUses[conceptID][j].CanonicalTerm
		})
	}

	toolsByID := make(map[curriculum.ID]curriculum.ToolRequirement, len(request.Tools))
	toolIntroductions := make(map[curriculum.ConceptID][]curriculum.ID)
	for _, tool := range request.Tools {
		if err := tool.Validate(); err != nil {
			return curriculum.BeginnerSimulationResult{}, Invalid(operation, err)
		}
		if _, exists := toolsByID[tool.ID]; exists {
			return curriculum.BeginnerSimulationResult{}, Invalid(operation, fmt.Errorf("duplicate beginner simulation tool %q", tool.ID))
		}
		if tool.IntroducedAt != nil {
			if _, exists := knownConcepts[*tool.IntroducedAt]; !exists {
				return curriculum.BeginnerSimulationResult{}, Invalid(operation, fmt.Errorf("tool %q introduction references unknown concept", tool.ID))
			}
			toolIntroductions[*tool.IntroducedAt] = append(toolIntroductions[*tool.IntroducedAt], tool.ID)
		}
		toolsByID[tool.ID] = tool
	}
	toolUses := make(map[curriculum.ConceptID][]curriculum.ID)
	for _, use := range request.ToolUses {
		if err := use.Validate(); err != nil {
			return curriculum.BeginnerSimulationResult{}, Invalid(operation, err)
		}
		if _, exists := toolsByID[use.ToolID]; !exists {
			return curriculum.BeginnerSimulationResult{}, Invalid(operation, fmt.Errorf("tool use references unknown tool %q", use.ToolID))
		}
		if _, exists := knownConcepts[use.UsedAt]; !exists {
			return curriculum.BeginnerSimulationResult{}, Invalid(operation, fmt.Errorf("tool use references unknown concept %q", use.UsedAt))
		}
		toolUses[use.UsedAt] = append(toolUses[use.UsedAt], use.ToolID)
	}

	assumptionsByConcept := make(map[curriculum.ConceptID][]curriculum.BeginnerAssumption)
	resolutionsByConcept := make(map[curriculum.ConceptID][]curriculum.ID)
	seenAssumptions := make(map[curriculum.ID]struct{}, len(request.Assumptions))
	for _, assumption := range request.Assumptions {
		if err := assumption.Validate(); err != nil {
			return curriculum.BeginnerSimulationResult{}, Invalid(operation, err)
		}
		if _, exists := seenAssumptions[assumption.ID]; exists {
			return curriculum.BeginnerSimulationResult{}, Invalid(operation, fmt.Errorf("duplicate beginner assumption %q", assumption.ID))
		}
		if _, exists := knownConcepts[assumption.RequiredAt]; !exists {
			return curriculum.BeginnerSimulationResult{}, Invalid(operation, fmt.Errorf("assumption %q requirement references unknown concept", assumption.ID))
		}
		if assumption.SatisfiedBy != nil {
			if _, exists := knownConcepts[*assumption.SatisfiedBy]; !exists {
				return curriculum.BeginnerSimulationResult{}, Invalid(operation, fmt.Errorf("assumption %q resolution references unknown concept", assumption.ID))
			}
			resolutionsByConcept[*assumption.SatisfiedBy] = append(resolutionsByConcept[*assumption.SatisfiedBy], assumption.ID)
		}
		seenAssumptions[assumption.ID] = struct{}{}
		assumptionsByConcept[assumption.RequiredAt] = append(assumptionsByConcept[assumption.RequiredAt], assumption)
	}
	for conceptID := range toolIntroductions {
		sortIDs(toolIntroductions[conceptID])
	}
	for conceptID := range toolUses {
		sortIDs(toolUses[conceptID])
	}
	for conceptID := range resolutionsByConcept {
		sortIDs(resolutionsByConcept[conceptID])
	}
	for conceptID := range assumptionsByConcept {
		sort.Slice(assumptionsByConcept[conceptID], func(i, j int) bool {
			return assumptionsByConcept[conceptID][i].ID.String() < assumptionsByConcept[conceptID][j].ID.String()
		})
	}

	result := curriculum.BeginnerSimulationResult{AlgorithmVersion: curriculum.BeginnerSimulationVersionV1}
	introducedConcepts := make(map[curriculum.ConceptID]struct{}, len(request.Concepts))
	introducedVocabulary := make(map[string]struct{})
	introducedTools := make(map[curriculum.ID]struct{})
	resolvedAssumptions := make(map[curriculum.ID]struct{})
	for _, conceptID := range request.Graph.TopologicalOrder {
		if err := ctx.Err(); err != nil {
			return curriculum.BeginnerSimulationResult{}, ExternalError(operation, err)
		}
		for _, requiredID := range prerequisites[conceptID] {
			if _, exists := introducedConcepts[requiredID]; !exists {
				result.Gaps = append(result.Gaps, beginnerGap(curriculum.BeginnerGapPrerequisite, conceptID, requiredID.String(), "required concept has not been introduced before use"))
			}
		}
		for _, use := range vocabularyUses[conceptID] {
			if use.Baseline {
				continue
			}
			if _, exists := introducedVocabulary[use.CanonicalTerm]; !exists {
				result.Gaps = append(result.Gaps, beginnerGap(curriculum.BeginnerGapVocabulary, conceptID, use.CanonicalTerm, "vocabulary is used before its defining concept is introduced"))
			}
		}
		for _, toolID := range toolUses[conceptID] {
			if _, exists := introducedTools[toolID]; !exists {
				result.Gaps = append(result.Gaps, beginnerGap(curriculum.BeginnerGapTool, conceptID, toolID.String(), "tool is needed before its introduction"))
			}
		}
		for _, assumption := range assumptionsByConcept[conceptID] {
			if _, exists := resolvedAssumptions[assumption.ID]; !exists {
				result.Gaps = append(result.Gaps, beginnerGap(curriculum.BeginnerGapAssumption, conceptID, assumption.ID.String(), assumption.Reason))
			}
		}

		step := curriculum.BeginnerSimulationStep{ConceptID: conceptID, TopicID: topicByConcept[conceptID]}
		introducedConcepts[conceptID] = struct{}{}
		result.IntroducedConceptIDs = append(result.IntroducedConceptIDs, conceptID)
		for _, term := range vocabularyIntroductions[conceptID] {
			introducedVocabulary[term] = struct{}{}
			step.IntroducedVocabulary = append(step.IntroducedVocabulary, term)
		}
		for _, toolID := range toolIntroductions[conceptID] {
			introducedTools[toolID] = struct{}{}
			step.IntroducedToolIDs = append(step.IntroducedToolIDs, toolID)
		}
		for _, assumptionID := range resolutionsByConcept[conceptID] {
			resolvedAssumptions[assumptionID] = struct{}{}
			step.ResolvedAssumptionIDs = append(step.ResolvedAssumptionIDs, assumptionID)
		}
		result.Steps = append(result.Steps, step)
	}
	result.IntroducedVocabulary = sortedBeginnerStrings(introducedVocabulary)
	result.IntroducedToolIDs = sortedBeginnerIDs(introducedTools)
	result.ResolvedAssumptionIDs = sortedBeginnerIDs(resolvedAssumptions)
	result.Passed = len(result.Gaps) == 0
	if err := result.Validate(); err != nil {
		return curriculum.BeginnerSimulationResult{}, Invalid(operation, err)
	}
	return result, nil
}

func beginnerGap(kind curriculum.BeginnerGapKind, conceptID curriculum.ConceptID, requirement, reason string) curriculum.BeginnerGap {
	return curriculum.BeginnerGap{Kind: kind, ConceptID: conceptID, Requirement: requirement, Reason: reason}
}

func sortIDs(values []curriculum.ID) {
	sort.Slice(values, func(i, j int) bool { return values[i].String() < values[j].String() })
}

func sortedBeginnerIDs(values map[curriculum.ID]struct{}) []curriculum.ID {
	result := make([]curriculum.ID, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sortIDs(result)
	return result
}

func sortedBeginnerStrings(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

var _ BeginnerSimulationService = BeginnerSimulatorV1{}
