package application

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestVocabularyGraphBuilderV1ResolvesAcronymsAliasesAndFirstUse(t *testing.T) {
	t.Parallel()
	intro := graphConcept(t, "concept.api", true)
	client := graphConcept(t, "concept.client", false)
	definition := curriculum.VocabularyDefinition{
		Term: "application programming interface", CanonicalConceptID: intro.ID,
		IntroducedBy: intro.ID, Aliases: []string{"API", "programming interface"}, Scope: "web",
	}
	request := VocabularyGraphRequest{
		Concepts: []curriculum.Concept{client, intro}, Definitions: []curriculum.VocabularyDefinition{definition},
		Uses: []curriculum.VocabularyUse{{Term: "api", UsedAt: client.ID}, {Term: "programming interface", UsedAt: intro.ID}},
	}
	result, err := NewVocabularyGraphBuilderV1().Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.AlgorithmVersion != curriculum.VocabularyGraphBuilderVersionV1 || len(result.Graph.Terms) != 1 || len(result.ResolvedUses) != 2 {
		t.Fatalf("vocabulary result = %+v", result)
	}
	assertConceptIDs(t, result.Graph.Terms[0].UsedBy, "concept.api", "concept.client")
	if result.ResolvedUses[1].ObservedTerm != "api" || result.ResolvedUses[1].CanonicalTerm != definition.Term || result.ResolvedUses[1].Baseline {
		t.Fatalf("acronym resolution = %+v", result.ResolvedUses)
	}

	reordered := request
	reordered.Concepts = []curriculum.Concept{intro, client}
	reordered.Uses = []curriculum.VocabularyUse{request.Uses[1], request.Uses[0]}
	repeated, err := NewVocabularyGraphBuilderV1().Build(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered vocabulary differs: %+v / %+v / %v", result, repeated, err)
	}
}

func TestVocabularyGraphBuilderV1AllowsOnlyExplicitDomainBaseline(t *testing.T) {
	t.Parallel()
	concept := graphConcept(t, "concept.loop", true)
	baseline := curriculum.DomainVocabularyBaselineTerm{Term: "number", Aliases: []string{"numeric value"}, Scope: "beginner programming", Reason: "Declared entry-level arithmetic vocabulary."}
	result, err := NewVocabularyGraphBuilderV1().Build(context.Background(), VocabularyGraphRequest{
		Concepts: []curriculum.Concept{concept}, DomainBaseline: []curriculum.DomainVocabularyBaselineTerm{baseline},
		Uses: []curriculum.VocabularyUse{{Term: "numeric value", UsedAt: concept.ID}},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(result.BaselineTerms) != 1 || len(result.ResolvedUses) != 1 || !result.ResolvedUses[0].Baseline || len(result.Graph.Terms) != 0 {
		t.Fatalf("baseline result = %+v", result)
	}

	_, err = NewVocabularyGraphBuilderV1().Build(context.Background(), VocabularyGraphRequest{
		Concepts: []curriculum.Concept{concept}, Uses: []curriculum.VocabularyUse{{Term: "number", UsedAt: concept.ID}},
	})
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "no declaration or explicit domain baseline") {
		t.Fatalf("undeclared use error = %v", err)
	}
}

func TestVocabularyGraphBuilderV1RejectsAmbiguousAlias(t *testing.T) {
	t.Parallel()
	first := graphConcept(t, "concept.first", true)
	second := graphConcept(t, "concept.second", true)
	_, err := NewVocabularyGraphBuilderV1().Build(context.Background(), VocabularyGraphRequest{
		Concepts: []curriculum.Concept{first, second},
		Definitions: []curriculum.VocabularyDefinition{
			{Term: "application programming interface", CanonicalConceptID: first.ID, IntroducedBy: first.ID, Aliases: []string{"API"}, Scope: "web"},
			{Term: "active pharmaceutical ingredient", CanonicalConceptID: second.ID, IntroducedBy: second.ID, Aliases: []string{"api"}, Scope: "pharma"},
		},
	})
	if !errors.Is(err, ErrInvalidState) || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("ambiguous alias error = %v", err)
	}
}
