package curriculum

import (
	"fmt"
	"strings"
)

type Prerequisite struct {
	ConceptID         ConceptID
	RequiredConceptID ConceptID
	Kind              PrerequisiteKind
	EvidenceRefs      []EvidenceRef
}

func (prerequisite Prerequisite) Validate() error {
	if err := prerequisite.ConceptID.Validate(); err != nil {
		return fmt.Errorf("prerequisite concept: %w", err)
	}
	if err := prerequisite.RequiredConceptID.Validate(); err != nil {
		return fmt.Errorf("required concept: %w", err)
	}
	if prerequisite.ConceptID == prerequisite.RequiredConceptID {
		return fmt.Errorf("concept %q cannot require itself", prerequisite.ConceptID)
	}
	if err := prerequisite.Kind.Validate(); err != nil {
		return err
	}
	return validateEvidenceRefs("prerequisite evidence", prerequisite.EvidenceRefs)
}

type VocabularyTerm struct {
	Term               string
	CanonicalConceptID ConceptID
	IntroducedBy       ConceptID
	UsedBy             []ConceptID
	Aliases            []string
	Scope              string
}

func (term VocabularyTerm) Validate() error {
	if err := requireText("vocabulary term", term.Term); err != nil {
		return err
	}
	if err := term.CanonicalConceptID.Validate(); err != nil {
		return fmt.Errorf("vocabulary canonical concept: %w", err)
	}
	if err := term.IntroducedBy.Validate(); err != nil {
		return fmt.Errorf("vocabulary introduction: %w", err)
	}
	if err := validateConceptIDs("vocabulary uses", term.UsedBy); err != nil {
		return err
	}
	if err := requireText("vocabulary scope", term.Scope); err != nil {
		return err
	}
	seenAliases := make(map[string]struct{}, len(term.Aliases))
	canonical := strings.ToLower(term.Term)
	for index, alias := range term.Aliases {
		if err := requireText(fmt.Sprintf("vocabulary alias %d", index), alias); err != nil {
			return err
		}
		normalized := strings.ToLower(alias)
		if normalized == canonical {
			return fmt.Errorf("vocabulary alias %q duplicates canonical term", alias)
		}
		if _, exists := seenAliases[normalized]; exists {
			return fmt.Errorf("vocabulary contains duplicate alias %q", alias)
		}
		seenAliases[normalized] = struct{}{}
	}
	return nil
}

type VocabularyGraph struct {
	Terms []VocabularyTerm
}

func (graph VocabularyGraph) Validate() error {
	seen := make(map[string]string, len(graph.Terms))
	for _, term := range graph.Terms {
		if err := term.Validate(); err != nil {
			return err
		}
		normalized := normalizeVocabularyToken(term.Term)
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("vocabulary graph contains duplicate term %q", term.Term)
		}
		seen[normalized] = term.Term
		for _, alias := range term.Aliases {
			normalizedAlias := normalizeVocabularyToken(alias)
			if previous, exists := seen[normalizedAlias]; exists {
				return fmt.Errorf("vocabulary alias %q conflicts with %q", alias, previous)
			}
			seen[normalizedAlias] = term.Term
		}
	}
	return nil
}

func normalizeVocabularyToken(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
