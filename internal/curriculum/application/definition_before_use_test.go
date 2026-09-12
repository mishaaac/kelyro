package application

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestDefinitionBeforeUseAuditV1AcceptsPrerequisiteIntroduction(t *testing.T) {
	t.Parallel()
	intro := graphConcept(t, "concept.request", true)
	use := graphConcept(t, "concept.handler", false)
	graph := compileAuditGraph(t, []curriculum.Concept{use, intro}, []curriculum.Prerequisite{graphPrerequisite(use, intro, curriculum.PrerequisiteVocabulary)})
	vocabulary := buildAuditVocabulary(t, []curriculum.Concept{intro, use}, []curriculum.VocabularyDefinition{{
		Term: "request", CanonicalConceptID: intro.ID, IntroducedBy: intro.ID, Scope: "http",
	}}, []curriculum.VocabularyUse{{Term: "request", UsedAt: use.ID}}, nil)

	result, err := NewDefinitionBeforeUseAuditV1().Audit(context.Background(), DefinitionBeforeUseAuditRequest{Graph: graph, Vocabulary: vocabulary})
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if !result.Passed || result.AuditedUseCount != 1 || result.BaselineUseCount != 0 || len(result.Violations) != 0 || result.AlgorithmVersion != curriculum.DefinitionBeforeUseAuditVersionV1 {
		t.Fatalf("valid audit = %+v", result)
	}
}

func TestDefinitionBeforeUseAuditV1ReportsInvalidAliasUse(t *testing.T) {
	t.Parallel()
	intro := graphConcept(t, "concept.interface", true)
	use := graphConcept(t, "concept.client", true)
	graph := compileAuditGraph(t, []curriculum.Concept{intro, use}, nil)
	vocabulary := buildAuditVocabulary(t, []curriculum.Concept{intro, use}, []curriculum.VocabularyDefinition{{
		Term: "application programming interface", CanonicalConceptID: intro.ID,
		IntroducedBy: intro.ID, Aliases: []string{"API"}, Scope: "web",
	}}, []curriculum.VocabularyUse{{Term: "api", UsedAt: use.ID}}, nil)

	result, err := NewDefinitionBeforeUseAuditV1().Audit(context.Background(), DefinitionBeforeUseAuditRequest{Graph: graph, Vocabulary: vocabulary})
	if err != nil {
		t.Fatalf("Audit() error = %v", err)
	}
	if result.Passed || len(result.Violations) != 1 {
		t.Fatalf("invalid audit = %+v", result)
	}
	violation := result.Violations[0]
	if violation.Code != curriculum.DefinitionBeforeUseMissingPrerequisite || violation.Term != "application programming interface" || violation.ObservedTerm != "api" || violation.UsedAt != use.ID || violation.ExpectedIntroduction != intro.ID || violation.SuggestedPrerequisite == nil || *violation.SuggestedPrerequisite != intro.ID || violation.Severity != curriculum.CurriculumAuditError {
		t.Fatalf("alias violation = %+v", violation)
	}
}

func TestDefinitionBeforeUseAuditV1UsesSameLessonConceptOrder(t *testing.T) {
	t.Parallel()
	intro := graphConcept(t, "concept.variable", true)
	use := graphConcept(t, "concept.assignment", true)
	graph := compileAuditGraph(t, []curriculum.Concept{intro, use}, nil)
	vocabulary := buildAuditVocabulary(t, []curriculum.Concept{intro, use}, []curriculum.VocabularyDefinition{{
		Term: "variable", CanonicalConceptID: intro.ID, IntroducedBy: intro.ID, Scope: "programming",
	}}, []curriculum.VocabularyUse{{Term: "variable", UsedAt: use.ID}}, nil)
	lessonID := curriculumID(t, "lesson.basics")
	topicID := curriculumID(t, "topic.variables")
	topic := func(concepts ...curriculum.ConceptID) curriculum.TopicSpec {
		return curriculum.TopicSpec{ID: topicID, LessonID: lessonID, Title: "Variables", Description: "Variable concepts.", Order: 0, ConceptIDs: concepts}
	}

	valid, err := NewDefinitionBeforeUseAuditV1().Audit(context.Background(), DefinitionBeforeUseAuditRequest{
		Graph: graph, Vocabulary: vocabulary, Topics: []curriculum.TopicSpec{topic(intro.ID, use.ID)},
	})
	if err != nil || !valid.Passed {
		t.Fatalf("same-lesson valid audit = %+v / %v", valid, err)
	}
	invalid, err := NewDefinitionBeforeUseAuditV1().Audit(context.Background(), DefinitionBeforeUseAuditRequest{
		Graph: graph, Vocabulary: vocabulary, Topics: []curriculum.TopicSpec{topic(use.ID, intro.ID)},
	})
	if err != nil || invalid.Passed || len(invalid.Violations) != 1 || invalid.Violations[0].Code != curriculum.DefinitionBeforeUseSameLessonOrder {
		t.Fatalf("same-lesson invalid audit = %+v / %v", invalid, err)
	}
}

func TestDefinitionBeforeUseAuditV1AcceptsExplicitBaseline(t *testing.T) {
	t.Parallel()
	use := graphConcept(t, "concept.loop", true)
	graph := compileAuditGraph(t, []curriculum.Concept{use}, nil)
	vocabulary := buildAuditVocabulary(t, []curriculum.Concept{use}, nil, []curriculum.VocabularyUse{{Term: "number", UsedAt: use.ID}}, []curriculum.DomainVocabularyBaselineTerm{{
		Term: "number", Scope: "beginner programming", Reason: "Explicit arithmetic baseline.",
	}})
	result, err := NewDefinitionBeforeUseAuditV1().Audit(context.Background(), DefinitionBeforeUseAuditRequest{Graph: graph, Vocabulary: vocabulary})
	if err != nil || !result.Passed || result.AuditedUseCount != 0 || result.BaselineUseCount != 1 {
		t.Fatalf("baseline audit = %+v / %v", result, err)
	}
}

func compileAuditGraph(t *testing.T, concepts []curriculum.Concept, prerequisites []curriculum.Prerequisite) curriculum.KnowledgeGraphCompilation {
	t.Helper()
	graph, err := NewKnowledgeGraphCompilerV1().Compile(context.Background(), KnowledgeGraphCompilationRequest{Concepts: concepts, Prerequisites: prerequisites})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func buildAuditVocabulary(t *testing.T, concepts []curriculum.Concept, definitions []curriculum.VocabularyDefinition, uses []curriculum.VocabularyUse, baseline []curriculum.DomainVocabularyBaselineTerm) curriculum.VocabularyGraphCompilation {
	t.Helper()
	graph, err := NewVocabularyGraphBuilderV1().Build(context.Background(), VocabularyGraphRequest{Concepts: concepts, Definitions: definitions, Uses: uses, DomainBaseline: baseline})
	if err != nil {
		t.Fatal(err)
	}
	return graph
}
