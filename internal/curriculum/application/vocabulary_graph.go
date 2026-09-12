package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type VocabularyGraphBuilderV1 struct{}

func NewVocabularyGraphBuilderV1() VocabularyGraphBuilderV1 {
	return VocabularyGraphBuilderV1{}
}

type vocabularyResolution struct {
	canonical string
	baseline  bool
}

type vocabularyUseKey struct {
	canonical string
	usedAt    curriculum.ConceptID
}

func (VocabularyGraphBuilderV1) Build(ctx context.Context, request VocabularyGraphRequest) (curriculum.VocabularyGraphCompilation, error) {
	const operation = "build vocabulary graph"
	if err := ctx.Err(); err != nil {
		return curriculum.VocabularyGraphCompilation{}, ExternalError(operation, err)
	}
	concepts := make(map[curriculum.ConceptID]struct{}, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, fmt.Errorf("duplicate vocabulary concept %q", concept.ID))
		}
		concepts[concept.ID] = struct{}{}
	}
	if len(concepts) == 0 {
		return curriculum.VocabularyGraphCompilation{}, Invalid(operation, fmt.Errorf("vocabulary graph has no concepts"))
	}

	definitions := append([]curriculum.VocabularyDefinition(nil), request.Definitions...)
	for index := range definitions {
		definitions[index].Aliases = append([]string(nil), definitions[index].Aliases...)
		sort.Strings(definitions[index].Aliases)
	}
	sort.Slice(definitions, func(i, j int) bool {
		return normalizeVocabulary(definitions[i].Term) < normalizeVocabulary(definitions[j].Term)
	})
	baseline := append([]curriculum.DomainVocabularyBaselineTerm(nil), request.DomainBaseline...)
	for index := range baseline {
		baseline[index].Aliases = append([]string(nil), baseline[index].Aliases...)
		sort.Strings(baseline[index].Aliases)
	}
	sort.Slice(baseline, func(i, j int) bool {
		return normalizeVocabulary(baseline[i].Term) < normalizeVocabulary(baseline[j].Term)
	})

	lookup := make(map[string]vocabularyResolution)
	terms := make([]curriculum.VocabularyTerm, 0, len(definitions))
	termIndex := make(map[string]int, len(definitions))
	for _, definition := range definitions {
		if err := definition.Validate(); err != nil {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, err)
		}
		if _, exists := concepts[definition.CanonicalConceptID]; !exists {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, fmt.Errorf("vocabulary term %q references missing canonical concept %q", definition.Term, definition.CanonicalConceptID))
		}
		if _, exists := concepts[definition.IntroducedBy]; !exists {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, fmt.Errorf("vocabulary term %q references missing introduction concept %q", definition.Term, definition.IntroducedBy))
		}
		canonical := normalizeVocabulary(definition.Term)
		if err := registerVocabularyTokens(lookup, definition.Term, definition.Aliases, vocabularyResolution{canonical: definition.Term}); err != nil {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, err)
		}
		termIndex[canonical] = len(terms)
		terms = append(terms, curriculum.VocabularyTerm{
			Term: definition.Term, CanonicalConceptID: definition.CanonicalConceptID,
			IntroducedBy: definition.IntroducedBy, Aliases: append([]string(nil), definition.Aliases...), Scope: definition.Scope,
		})
	}
	for _, term := range baseline {
		if err := term.Validate(); err != nil {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, err)
		}
		if err := registerVocabularyTokens(lookup, term.Term, term.Aliases, vocabularyResolution{canonical: term.Term, baseline: true}); err != nil {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, err)
		}
	}

	uses := append([]curriculum.VocabularyUse(nil), request.Uses...)
	sort.Slice(uses, func(i, j int) bool {
		if uses[i].UsedAt != uses[j].UsedAt {
			return uses[i].UsedAt.String() < uses[j].UsedAt.String()
		}
		return normalizeVocabulary(uses[i].Term) < normalizeVocabulary(uses[j].Term)
	})
	resolvedByKey := make(map[vocabularyUseKey]curriculum.ResolvedVocabularyUse)
	usedBy := make(map[string]map[curriculum.ConceptID]struct{})
	for _, use := range uses {
		if err := ctx.Err(); err != nil {
			return curriculum.VocabularyGraphCompilation{}, ExternalError(operation, err)
		}
		if err := use.Validate(); err != nil {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, err)
		}
		if _, exists := concepts[use.UsedAt]; !exists {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, fmt.Errorf("vocabulary use %q references missing concept %q", use.Term, use.UsedAt))
		}
		resolution, exists := lookup[normalizeVocabulary(use.Term)]
		if !exists {
			return curriculum.VocabularyGraphCompilation{}, Invalid(operation, fmt.Errorf("vocabulary use %q has no declaration or explicit domain baseline", use.Term))
		}
		key := vocabularyUseKey{canonical: normalizeVocabulary(resolution.canonical), usedAt: use.UsedAt}
		candidate := curriculum.ResolvedVocabularyUse{ObservedTerm: use.Term, CanonicalTerm: resolution.canonical, UsedAt: use.UsedAt, Baseline: resolution.baseline}
		if existing, exists := resolvedByKey[key]; !exists || normalizeVocabulary(candidate.ObservedTerm) < normalizeVocabulary(existing.ObservedTerm) {
			resolvedByKey[key] = candidate
		}
		if !resolution.baseline {
			if usedBy[key.canonical] == nil {
				usedBy[key.canonical] = make(map[curriculum.ConceptID]struct{})
			}
			usedBy[key.canonical][use.UsedAt] = struct{}{}
		}
	}
	for canonical, ids := range usedBy {
		terms[termIndex[canonical]].UsedBy = sortedIDSet(ids)
	}
	resolved := make([]curriculum.ResolvedVocabularyUse, 0, len(resolvedByKey))
	for _, use := range resolvedByKey {
		resolved = append(resolved, use)
	}
	sort.Slice(resolved, func(i, j int) bool {
		left, right := normalizeVocabulary(resolved[i].CanonicalTerm), normalizeVocabulary(resolved[j].CanonicalTerm)
		if left != right {
			return left < right
		}
		if resolved[i].UsedAt != resolved[j].UsedAt {
			return resolved[i].UsedAt.String() < resolved[j].UsedAt.String()
		}
		return normalizeVocabulary(resolved[i].ObservedTerm) < normalizeVocabulary(resolved[j].ObservedTerm)
	})
	result := curriculum.VocabularyGraphCompilation{
		Graph: curriculum.VocabularyGraph{Terms: terms}, BaselineTerms: baseline,
		ResolvedUses: resolved, AlgorithmVersion: curriculum.VocabularyGraphBuilderVersionV1,
	}
	if err := result.Validate(); err != nil {
		return curriculum.VocabularyGraphCompilation{}, Invalid(operation, err)
	}
	return result, nil
}

func registerVocabularyTokens(lookup map[string]vocabularyResolution, term string, aliases []string, resolution vocabularyResolution) error {
	for _, token := range append([]string{term}, aliases...) {
		normalized := normalizeVocabulary(token)
		if previous, exists := lookup[normalized]; exists {
			return fmt.Errorf("vocabulary token %q conflicts with canonical term %q", token, previous.canonical)
		}
		lookup[normalized] = resolution
	}
	return nil
}

func normalizeVocabulary(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
