package curriculum

import "fmt"

const VocabularyGraphBuilderVersionV1 = "vocabulary-graph-v1"

type VocabularyDefinition struct {
	Term               string
	CanonicalConceptID ConceptID
	IntroducedBy       ConceptID
	Aliases            []string
	Scope              string
}

func (definition VocabularyDefinition) Validate() error {
	term := VocabularyTerm{
		Term: definition.Term, CanonicalConceptID: definition.CanonicalConceptID,
		IntroducedBy: definition.IntroducedBy, Aliases: definition.Aliases, Scope: definition.Scope,
	}
	return term.Validate()
}

type VocabularyUse struct {
	Term   string
	UsedAt ConceptID
}

func (use VocabularyUse) Validate() error {
	if err := requireText("vocabulary use term", use.Term); err != nil {
		return err
	}
	if err := use.UsedAt.Validate(); err != nil {
		return fmt.Errorf("vocabulary use concept: %w", err)
	}
	return nil
}

// DomainVocabularyBaselineTerm is an explicit, scoped exception for assumed
// common language. There is deliberately no package-level baseline list.
type DomainVocabularyBaselineTerm struct {
	Term    string
	Aliases []string
	Scope   string
	Reason  string
}

func (term DomainVocabularyBaselineTerm) Validate() error {
	for _, field := range []struct{ name, value string }{
		{name: "domain vocabulary baseline term", value: term.Term},
		{name: "domain vocabulary baseline scope", value: term.Scope},
		{name: "domain vocabulary baseline reason", value: term.Reason},
	} {
		if err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	seen := make(map[string]struct{}, len(term.Aliases))
	canonical := normalizeVocabularyToken(term.Term)
	for index, alias := range term.Aliases {
		if err := requireText(fmt.Sprintf("domain vocabulary baseline alias %d", index), alias); err != nil {
			return err
		}
		normalized := normalizeVocabularyToken(alias)
		if normalized == canonical {
			return fmt.Errorf("domain vocabulary baseline alias %q duplicates canonical term", alias)
		}
		if _, exists := seen[normalized]; exists {
			return fmt.Errorf("domain vocabulary baseline contains duplicate alias %q", alias)
		}
		seen[normalized] = struct{}{}
	}
	return nil
}

type ResolvedVocabularyUse struct {
	ObservedTerm  string
	CanonicalTerm string
	UsedAt        ConceptID
	Baseline      bool
}

func (use ResolvedVocabularyUse) Validate() error {
	if err := requireText("resolved vocabulary observed term", use.ObservedTerm); err != nil {
		return err
	}
	if err := requireText("resolved vocabulary canonical term", use.CanonicalTerm); err != nil {
		return err
	}
	if err := use.UsedAt.Validate(); err != nil {
		return fmt.Errorf("resolved vocabulary use concept: %w", err)
	}
	return nil
}

type VocabularyGraphCompilation struct {
	Graph            VocabularyGraph
	BaselineTerms    []DomainVocabularyBaselineTerm
	ResolvedUses     []ResolvedVocabularyUse
	AlgorithmVersion string
}

func (compilation VocabularyGraphCompilation) Validate() error {
	if compilation.AlgorithmVersion != VocabularyGraphBuilderVersionV1 {
		return fmt.Errorf("unsupported vocabulary graph version %q", compilation.AlgorithmVersion)
	}
	if err := compilation.Graph.Validate(); err != nil {
		return err
	}
	known := make(map[string]bool)
	for _, term := range compilation.Graph.Terms {
		for _, token := range append([]string{term.Term}, term.Aliases...) {
			known[normalizeVocabularyToken(token)] = false
		}
	}
	for _, term := range compilation.BaselineTerms {
		if err := term.Validate(); err != nil {
			return err
		}
		for _, token := range append([]string{term.Term}, term.Aliases...) {
			normalized := normalizeVocabularyToken(token)
			if _, exists := known[normalized]; exists {
				return fmt.Errorf("domain vocabulary baseline token %q conflicts with another vocabulary token", token)
			}
			known[normalized] = true
		}
	}
	seenUses := make(map[string]struct{}, len(compilation.ResolvedUses))
	for _, use := range compilation.ResolvedUses {
		if err := use.Validate(); err != nil {
			return err
		}
		baseline, exists := known[normalizeVocabularyToken(use.CanonicalTerm)]
		if !exists || baseline != use.Baseline {
			return fmt.Errorf("resolved vocabulary use %q has no matching declaration", use.CanonicalTerm)
		}
		key := normalizeVocabularyToken(use.CanonicalTerm) + "\x00" + use.UsedAt.String()
		if _, exists := seenUses[key]; exists {
			return fmt.Errorf("duplicate resolved vocabulary use of %q at %q", use.CanonicalTerm, use.UsedAt)
		}
		seenUses[key] = struct{}{}
	}
	return nil
}
