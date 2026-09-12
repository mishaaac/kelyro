package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type DefinitionBeforeUseAuditV1 struct{}

func NewDefinitionBeforeUseAuditV1() DefinitionBeforeUseAuditV1 {
	return DefinitionBeforeUseAuditV1{}
}

type curriculumConceptPosition struct {
	lesson       curriculum.ID
	topicOrder   int
	conceptOrder int
}

func (DefinitionBeforeUseAuditV1) Audit(ctx context.Context, request DefinitionBeforeUseAuditRequest) (curriculum.DefinitionBeforeUseAuditResult, error) {
	const operation = "audit definition before use"
	if err := ctx.Err(); err != nil {
		return curriculum.DefinitionBeforeUseAuditResult{}, ExternalError(operation, err)
	}
	if err := request.Graph.Validate(); err != nil {
		return curriculum.DefinitionBeforeUseAuditResult{}, Invalid(operation, err)
	}
	if err := request.Vocabulary.Validate(); err != nil {
		return curriculum.DefinitionBeforeUseAuditResult{}, Invalid(operation, err)
	}
	known := make(map[curriculum.ConceptID]struct{}, len(request.Graph.ConceptIDs))
	for _, id := range request.Graph.ConceptIDs {
		known[id] = struct{}{}
	}
	positions, err := definitionBeforeUsePositions(request.Topics, known)
	if err != nil {
		return curriculum.DefinitionBeforeUseAuditResult{}, Invalid(operation, err)
	}
	terms := make(map[string]curriculum.VocabularyTerm, len(request.Vocabulary.Graph.Terms))
	for _, term := range request.Vocabulary.Graph.Terms {
		if _, exists := known[term.CanonicalConceptID]; !exists {
			return curriculum.DefinitionBeforeUseAuditResult{}, Invalid(operation, fmt.Errorf("vocabulary term %q canonical concept is absent from knowledge graph", term.Term))
		}
		if _, exists := known[term.IntroducedBy]; !exists {
			return curriculum.DefinitionBeforeUseAuditResult{}, Invalid(operation, fmt.Errorf("vocabulary term %q introduction is absent from knowledge graph", term.Term))
		}
		terms[normalizeVocabulary(term.Term)] = term
	}
	dependencies := make(map[curriculum.ConceptID][]curriculum.ConceptID)
	for _, prerequisite := range request.Graph.Prerequisites {
		dependencies[prerequisite.ConceptID] = append(dependencies[prerequisite.ConceptID], prerequisite.RequiredConceptID)
	}
	result := curriculum.DefinitionBeforeUseAuditResult{AlgorithmVersion: curriculum.DefinitionBeforeUseAuditVersionV1}
	for _, use := range request.Vocabulary.ResolvedUses {
		if err := ctx.Err(); err != nil {
			return curriculum.DefinitionBeforeUseAuditResult{}, ExternalError(operation, err)
		}
		if use.Baseline {
			if _, exists := known[use.UsedAt]; !exists {
				return curriculum.DefinitionBeforeUseAuditResult{}, Invalid(operation, fmt.Errorf("baseline vocabulary use %q is absent from knowledge graph", use.UsedAt))
			}
			result.BaselineUseCount++
			continue
		}
		if _, exists := known[use.UsedAt]; !exists {
			return curriculum.DefinitionBeforeUseAuditResult{}, Invalid(operation, fmt.Errorf("vocabulary use %q is absent from knowledge graph", use.UsedAt))
		}
		result.AuditedUseCount++
		term, exists := terms[normalizeVocabulary(use.CanonicalTerm)]
		if !exists {
			return curriculum.DefinitionBeforeUseAuditResult{}, Invalid(operation, fmt.Errorf("resolved vocabulary use %q has no declared term", use.CanonicalTerm))
		}
		if use.UsedAt == term.IntroducedBy {
			continue
		}
		introductionPrecedes := graphRequires(dependencies, use.UsedAt, term.IntroducedBy)
		usePrecedesIntroduction := graphRequires(dependencies, term.IntroducedBy, use.UsedAt)
		introPosition, introPositioned := positions[term.IntroducedBy]
		usePosition, usePositioned := positions[use.UsedAt]
		sameLesson := introPositioned && usePositioned && introPosition.lesson == usePosition.lesson
		if introductionPrecedes {
			if sameLesson && !positionBefore(introPosition, usePosition) {
				result.Violations = append(result.Violations, definitionBeforeUseViolation(use, term, curriculum.DefinitionBeforeUseSameLessonOrder, "the visible lesson order uses the term before its prerequisite introduction", false))
			}
			continue
		}
		if usePrecedesIntroduction {
			result.Violations = append(result.Violations, definitionBeforeUseViolation(use, term, curriculum.DefinitionBeforeUseIntroducedAfterUse, "the knowledge graph places the term introduction after its use", false))
			continue
		}
		if sameLesson {
			if positionBefore(introPosition, usePosition) {
				continue
			}
			result.Violations = append(result.Violations, definitionBeforeUseViolation(use, term, curriculum.DefinitionBeforeUseSameLessonOrder, "the lesson uses the term before its introduction", false))
			continue
		}
		result.Violations = append(result.Violations, definitionBeforeUseViolation(use, term, curriculum.DefinitionBeforeUseMissingPrerequisite, "the introduction is neither an earlier same-lesson concept nor a prerequisite", true))
	}
	sort.Slice(result.Violations, func(i, j int) bool {
		left, right := strings.ToLower(result.Violations[i].Term), strings.ToLower(result.Violations[j].Term)
		if left != right {
			return left < right
		}
		if result.Violations[i].UsedAt != result.Violations[j].UsedAt {
			return result.Violations[i].UsedAt.String() < result.Violations[j].UsedAt.String()
		}
		return result.Violations[i].Code < result.Violations[j].Code
	})
	result.Passed = len(result.Violations) == 0
	if err := result.Validate(); err != nil {
		return curriculum.DefinitionBeforeUseAuditResult{}, Invalid(operation, err)
	}
	return result, nil
}

func definitionBeforeUsePositions(topics []curriculum.TopicSpec, known map[curriculum.ConceptID]struct{}) (map[curriculum.ConceptID]curriculumConceptPosition, error) {
	positions := make(map[curriculum.ConceptID]curriculumConceptPosition)
	seenTopics := make(map[curriculum.ID]struct{}, len(topics))
	topicOrders := make(map[struct {
		lesson curriculum.ID
		order  int
	}]curriculum.ID)
	for _, topic := range topics {
		if err := topic.Validate(); err != nil {
			return nil, err
		}
		if _, exists := seenTopics[topic.ID]; exists {
			return nil, fmt.Errorf("duplicate definition-before-use topic %q", topic.ID)
		}
		seenTopics[topic.ID] = struct{}{}
		orderKey := struct {
			lesson curriculum.ID
			order  int
		}{lesson: topic.LessonID, order: topic.Order}
		if previous, exists := topicOrders[orderKey]; exists {
			return nil, fmt.Errorf("definition-before-use topics %q and %q share lesson order %d", previous, topic.ID, topic.Order)
		}
		topicOrders[orderKey] = topic.ID
		for index, conceptID := range topic.ConceptIDs {
			if _, exists := known[conceptID]; !exists {
				return nil, fmt.Errorf("definition-before-use topic %q references concept absent from knowledge graph %q", topic.ID, conceptID)
			}
			if _, exists := positions[conceptID]; exists {
				return nil, fmt.Errorf("definition-before-use concept %q has multiple hierarchy positions", conceptID)
			}
			positions[conceptID] = curriculumConceptPosition{lesson: topic.LessonID, topicOrder: topic.Order, conceptOrder: index}
		}
	}
	return positions, nil
}

func graphRequires(dependencies map[curriculum.ConceptID][]curriculum.ConceptID, concept, required curriculum.ConceptID) bool {
	seen := make(map[curriculum.ConceptID]struct{})
	stack := append([]curriculum.ConceptID(nil), dependencies[concept]...)
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current == required {
			return true
		}
		if _, exists := seen[current]; exists {
			continue
		}
		seen[current] = struct{}{}
		stack = append(stack, dependencies[current]...)
	}
	return false
}

func positionBefore(left, right curriculumConceptPosition) bool {
	if left.topicOrder != right.topicOrder {
		return left.topicOrder < right.topicOrder
	}
	return left.conceptOrder < right.conceptOrder
}

func definitionBeforeUseViolation(use curriculum.ResolvedVocabularyUse, term curriculum.VocabularyTerm, code curriculum.DefinitionBeforeUseViolationCode, reason string, suggestPrerequisite bool) curriculum.DefinitionBeforeUseViolation {
	violation := curriculum.DefinitionBeforeUseViolation{
		Code: code, Term: term.Term, ObservedTerm: use.ObservedTerm, UsedAt: use.UsedAt,
		ExpectedIntroduction: term.IntroducedBy, Severity: curriculum.CurriculumAuditError, Reason: reason,
	}
	if suggestPrerequisite {
		suggested := term.IntroducedBy
		violation.SuggestedPrerequisite = &suggested
	}
	return violation
}
