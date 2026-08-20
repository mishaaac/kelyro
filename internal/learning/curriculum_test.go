package learning

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestCurriculumValidatesContractAndCanonicalizesNodes(t *testing.T) {
	t.Parallel()

	nodes := validCurriculumNodes(t)
	reversed := append([]CurriculumNode(nil), nodes...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	first := mustCurriculum(t, nodes)
	second := mustCurriculum(t, reversed)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("equivalent definitions are not canonical:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if got := first.Nodes[len(first.Nodes)-1].ID.String(); got != "concept.b" {
		t.Fatalf("canonical final node = %q, want concept.b", got)
	}

	conceptID := curriculumID(t, "concept.b")
	concept, found := first.Node(conceptID)
	if !found || concept.Concept == nil || len(concept.Concept.Prerequisites) != 1 {
		t.Fatalf("Node(concept.b) = %+v, %v", concept, found)
	}
	concept.Concept.Objectives[0] = "mutated"
	again, _ := first.Node(conceptID)
	if again.Concept.Objectives[0] == "mutated" {
		t.Fatal("Node returned mutable curriculum storage")
	}
}

func TestCurriculumRejectsInvalidGraphAndHierarchy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func([]CurriculumNode)
		want string
	}{
		{
			name: "duplicate ID",
			edit: func(nodes []CurriculumNode) { nodes[5].ID = nodes[4].ID },
			want: "duplicate curriculum node id",
		},
		{
			name: "missing parent",
			edit: func(nodes []CurriculumNode) { nodes[1].ParentID = curriculumIDPointer(t, "phase.missing") },
			want: "references missing parent",
		},
		{
			name: "invalid node type",
			edit: func(nodes []CurriculumNode) { nodes[1].Type = CurriculumNodeType("chapter") },
			want: "invalid curriculum node type",
		},
		{
			name: "hierarchy cycle",
			edit: func(nodes []CurriculumNode) {
				nodes[1].ParentID = &nodes[2].ID
				nodes[2].ParentID = &nodes[1].ID
			},
			want: "hierarchy contains a cycle",
		},
		{
			name: "invalid order",
			edit: func(nodes []CurriculumNode) { nodes[4].Order = -1 },
			want: "invalid order",
		},
		{
			name: "duplicate sibling order",
			edit: func(nodes []CurriculumNode) { nodes[5].Order = nodes[4].Order },
			want: "share order",
		},
		{
			name: "unknown prerequisite",
			edit: func(nodes []CurriculumNode) {
				nodes[5].Concept.Prerequisites[0].ConceptID = curriculumID(t, "concept.missing")
			},
			want: "unknown prerequisite",
		},
		{
			name: "prerequisite cycle",
			edit: func(nodes []CurriculumNode) {
				nodes[4].Concept.Prerequisites = []ConceptPrerequisite{{ConceptID: nodes[5].ID, Requirement: PrerequisiteMastered}}
			},
			want: "prerequisites contain a cycle",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			nodes := validCurriculumNodes(t)
			test.edit(nodes)
			_, err := NewCurriculum(
				CurriculumContractVersion,
				CurriculumRef{ID: curriculumID(t, "fixture.curriculum"), Version: "1.0.0"},
				"Fixture curriculum",
				"A deterministic test curriculum.",
				nodes,
			)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewCurriculum() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestCurriculumAcceptsLargeDeterministicFixture(t *testing.T) {
	t.Parallel()

	const conceptCount = 1500
	nodes := validCurriculumNodes(t)[:4]
	topicID := curriculumID(t, "topic.a")
	for index := 0; index < conceptCount; index++ {
		conceptID := curriculumID(t, fmt.Sprintf("concept.large.%04d", index))
		definition := ConceptDefinition{
			Objectives:             []string{fmt.Sprintf("Understand deterministic concept %d", index)},
			Difficulty:             ConceptDifficultyFoundational,
			EstimatedEffortMinutes: 10,
			TheoryRequired:         index%2 == 0,
			AssessmentExpectations: []string{fmt.Sprintf("Explain deterministic concept %d", index)},
		}
		if index > 0 {
			definition.Prerequisites = []ConceptPrerequisite{{
				ConceptID:   curriculumID(t, fmt.Sprintf("concept.large.%04d", index-1)),
				Requirement: PrerequisiteMastered,
			}}
		}
		nodes = append(nodes, CurriculumNode{
			ID: conceptID, Type: CurriculumNodeConcept, ParentID: &topicID,
			Title: fmt.Sprintf("Concept %d", index), Description: "A generated deterministic large-graph fixture node.",
			Order: index, Status: activeCurriculumStatus(), Version: "1.0.0", Concept: &definition,
		})
	}
	curriculum := mustCurriculum(t, nodes)
	if got, want := len(curriculum.Nodes), conceptCount+4; got != want {
		t.Fatalf("large fixture node count = %d, want %d", got, want)
	}
}

func validCurriculumNodes(t *testing.T) []CurriculumNode {
	t.Helper()
	phaseID := curriculumID(t, "phase.a")
	moduleID := curriculumID(t, "module.a")
	lessonID := curriculumID(t, "lesson.a")
	topicID := curriculumID(t, "topic.a")
	conceptAID := curriculumID(t, "concept.a")
	conceptBID := curriculumID(t, "concept.b")
	concept := func(objective string) *ConceptDefinition {
		return &ConceptDefinition{
			Objectives: []string{objective}, Difficulty: ConceptDifficultyFoundational,
			EstimatedEffortMinutes: 20, TheoryRequired: true,
			AssessmentExpectations: []string{"Explain the idea with a neutral example"},
		}
	}
	conceptB := concept("Apply concept B")
	conceptB.Prerequisites = []ConceptPrerequisite{{ConceptID: conceptAID, Requirement: PrerequisiteMastered}}
	return []CurriculumNode{
		{ID: phaseID, Type: CurriculumNodePhase, Title: "Phase A", Description: "Phase description.", Order: 0, Status: activeCurriculumStatus(), Version: "1.0.0"},
		{ID: moduleID, Type: CurriculumNodeModule, ParentID: &phaseID, Title: "Module A", Description: "Module description.", Order: 0, Status: activeCurriculumStatus(), Version: "1.0.0"},
		{ID: lessonID, Type: CurriculumNodeLesson, ParentID: &moduleID, Title: "Lesson A", Description: "Lesson description.", Order: 0, Status: activeCurriculumStatus(), Version: "1.0.0"},
		{ID: topicID, Type: CurriculumNodeTopic, ParentID: &lessonID, Title: "Topic A", Description: "Topic description.", Order: 0, Status: activeCurriculumStatus(), Version: "1.0.0"},
		{ID: conceptAID, Type: CurriculumNodeConcept, ParentID: &topicID, Title: "Concept A", Description: "Concept A description.", Order: 0, Status: activeCurriculumStatus(), Version: "1.0.0", Concept: concept("Understand concept A")},
		{ID: conceptBID, Type: CurriculumNodeConcept, ParentID: &topicID, Title: "Concept B", Description: "Concept B description.", Order: 1, Status: activeCurriculumStatus(), Version: "1.0.0", Concept: conceptB},
	}
}

func mustCurriculum(t *testing.T, nodes []CurriculumNode) Curriculum {
	t.Helper()
	curriculum, err := NewCurriculum(
		CurriculumContractVersion,
		CurriculumRef{ID: curriculumID(t, "fixture.curriculum"), Version: "1.0.0"},
		"Fixture curriculum",
		"A deterministic test curriculum.",
		nodes,
	)
	if err != nil {
		t.Fatalf("NewCurriculum() error = %v", err)
	}
	return curriculum
}

func curriculumID(t *testing.T, value string) ID {
	t.Helper()
	id, err := NewID(value)
	if err != nil {
		t.Fatalf("NewID(%q) error = %v", value, err)
	}
	return id
}

func curriculumIDPointer(t *testing.T, value string) *ID {
	t.Helper()
	id := curriculumID(t, value)
	return &id
}

func activeCurriculumStatus() CurriculumStatusMetadata {
	return CurriculumStatusMetadata{State: CurriculumNodeActive}
}
