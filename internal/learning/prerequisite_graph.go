package learning

import (
	"container/heap"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

var ErrUnknownCurriculumConcept = errors.New("unknown curriculum concept")

const PrerequisitePolicyVersion = "prerequisite-v1"

// StudentStateSnapshot is an immutable-by-construction lookup used while one
// prerequisite decision is evaluated. Repositories are not consulted by graph
// traversal.
type StudentStateSnapshot struct {
	studentID  ID
	hasStudent bool
	byConcept  map[ID]ConceptState
}

func NewStudentStateSnapshot(states []ConceptState) (StudentStateSnapshot, error) {
	snapshot := StudentStateSnapshot{byConcept: make(map[ID]ConceptState, len(states))}
	for _, state := range states {
		if err := state.Validate(); err != nil {
			return StudentStateSnapshot{}, fmt.Errorf("student state snapshot: %w", err)
		}
		if !snapshot.hasStudent {
			snapshot.studentID = state.StudentID
			snapshot.hasStudent = true
		} else if state.StudentID != snapshot.studentID {
			return StudentStateSnapshot{}, fmt.Errorf("student state snapshot mixes students %q and %q", snapshot.studentID, state.StudentID)
		}
		if _, exists := snapshot.byConcept[state.ConceptID]; exists {
			return StudentStateSnapshot{}, fmt.Errorf("student state snapshot contains duplicate concept %q", state.ConceptID)
		}
		if state.IntroducedAt != nil {
			introducedAt := *state.IntroducedAt
			state.IntroducedAt = &introducedAt
		}
		snapshot.byConcept[state.ConceptID] = state
	}
	return snapshot, nil
}

func (snapshot StudentStateSnapshot) State(conceptID ID) (ConceptState, bool) {
	state, found := snapshot.byConcept[conceptID]
	if !found {
		return ConceptState{}, false
	}
	if state.IntroducedAt != nil {
		introducedAt := *state.IntroducedAt
		state.IntroducedAt = &introducedAt
	}
	return state, true
}

type PrerequisiteCheckReason string

const (
	PrerequisiteSatisfiedIntroduced PrerequisiteCheckReason = "satisfied_introduced"
	PrerequisiteSatisfiedMastery    PrerequisiteCheckReason = "satisfied_mastery"
	PrerequisiteMissingState        PrerequisiteCheckReason = "missing_state"
	PrerequisiteNotIntroduced       PrerequisiteCheckReason = "not_introduced"
	PrerequisiteBelowMastery        PrerequisiteCheckReason = "below_mastery"
)

// PrerequisiteCheck contains both the policy inputs and the result needed to
// explain one direct prerequisite without consulting persistence or UI code.
type PrerequisiteCheck struct {
	ConceptID       ID
	Title           string
	Requirement     PrerequisiteRequirement
	Satisfied       bool
	StatePresent    bool
	Exposure        ExposureState
	Mastery         MasteryScore
	RequiredMastery *MasteryThreshold
	Reason          PrerequisiteCheckReason
}

func (check PrerequisiteCheck) ExplanationLine() string {
	mark := "✓"
	if !check.Satisfied {
		mark = "✗"
	}
	if !check.StatePresent {
		if check.Requirement == PrerequisiteMastered && check.RequiredMastery != nil {
			return fmt.Sprintf("%s %s — no student state (requires %s mastery)", mark, check.Title, formatGraphPercent(check.RequiredMastery.Value()))
		}
		return fmt.Sprintf("%s %s — no student state (requires introduction)", mark, check.Title)
	}
	if check.Requirement == PrerequisiteMastered && check.RequiredMastery != nil {
		return fmt.Sprintf("%s %s — %s (requires %s)", mark, check.Title, formatGraphPercent(check.Mastery.Value()), formatGraphPercent(check.RequiredMastery.Value()))
	}
	if check.Satisfied {
		return fmt.Sprintf("%s %s — introduced (%s)", mark, check.Title, check.Exposure)
	}
	return fmt.Sprintf("%s %s — not introduced (%s)", mark, check.Title, check.Exposure)
}

type IntroductionDecision struct {
	ConceptID     ID
	Title         string
	CanIntroduce  bool
	Checks        []PrerequisiteCheck
	PolicyVersion string
	MasteryPolicy ResolvedMasteryThreshold
}

func (decision IntroductionDecision) Explanation() string {
	var builder strings.Builder
	if decision.CanIntroduce {
		fmt.Fprintf(&builder, "%s can be introduced.", decision.Title)
	} else {
		fmt.Fprintf(&builder, "%s is locked.", decision.Title)
	}
	if len(decision.Checks) == 0 {
		builder.WriteString("\nNo prerequisites are required.")
		return builder.String()
	}
	builder.WriteString("\n\nRequired:")
	for _, check := range decision.Checks {
		builder.WriteString("\n")
		builder.WriteString(check.ExplanationLine())
	}
	return builder.String()
}

// KnowledgeGraph indexes a validated curriculum definition. It contains no
// repositories and never mutates curriculum or student state.
type KnowledgeGraph struct {
	reference     CurriculumRef
	concepts      map[ID]CurriculumNode
	prerequisites map[ID][]ConceptPrerequisite
	dependents    map[ID][]ID
	topological   []ID
}

func NewKnowledgeGraph(curriculum Curriculum) (*KnowledgeGraph, error) {
	if err := curriculum.Validate(); err != nil {
		return nil, fmt.Errorf("build knowledge graph: %w", err)
	}
	graph := &KnowledgeGraph{
		reference:     curriculum.Reference,
		concepts:      make(map[ID]CurriculumNode, len(curriculum.Nodes)),
		prerequisites: make(map[ID][]ConceptPrerequisite, len(curriculum.Nodes)),
		dependents:    make(map[ID][]ID, len(curriculum.Nodes)),
	}
	for _, node := range curriculum.Nodes {
		if node.Type != CurriculumNodeConcept {
			continue
		}
		graph.concepts[node.ID] = cloneCurriculumNode(node)
		graph.prerequisites[node.ID] = append([]ConceptPrerequisite(nil), node.Concept.Prerequisites...)
		for _, prerequisite := range node.Concept.Prerequisites {
			graph.dependents[prerequisite.ConceptID] = append(graph.dependents[prerequisite.ConceptID], node.ID)
		}
	}
	for conceptID := range graph.prerequisites {
		sort.Slice(graph.prerequisites[conceptID], func(i, j int) bool {
			return graph.prerequisites[conceptID][i].ConceptID.String() < graph.prerequisites[conceptID][j].ConceptID.String()
		})
	}
	for conceptID := range graph.dependents {
		sort.Slice(graph.dependents[conceptID], func(i, j int) bool {
			return graph.dependents[conceptID][i].String() < graph.dependents[conceptID][j].String()
		})
	}
	order, err := graph.calculateTopologicalOrder()
	if err != nil {
		return nil, err
	}
	graph.topological = order
	return graph, nil
}

// Reference identifies the immutable curriculum definition indexed by this
// graph so application services cannot evaluate an instance against another
// version's prerequisite rules.
func (graph *KnowledgeGraph) Reference() CurriculumRef {
	if graph == nil {
		return CurriculumRef{}
	}
	return graph.reference
}

func (graph *KnowledgeGraph) GetPrerequisites(conceptID ID) ([]ConceptPrerequisite, error) {
	if _, err := graph.concept(conceptID); err != nil {
		return nil, err
	}
	return append([]ConceptPrerequisite(nil), graph.prerequisites[conceptID]...), nil
}

func (graph *KnowledgeGraph) GetDependents(conceptID ID) ([]ID, error) {
	if _, err := graph.concept(conceptID); err != nil {
		return nil, err
	}
	return append([]ID(nil), graph.dependents[conceptID]...), nil
}

// Ancestors returns every transitive prerequisite in deterministic topological
// order, with foundations before the requested concept's nearer ancestors.
func (graph *KnowledgeGraph) Ancestors(conceptID ID) ([]ID, error) {
	if _, err := graph.concept(conceptID); err != nil {
		return nil, err
	}
	ancestors := make(map[ID]struct{})
	stack := make([]ID, 0, len(graph.prerequisites[conceptID]))
	for _, prerequisite := range graph.prerequisites[conceptID] {
		stack = append(stack, prerequisite.ConceptID)
	}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if _, visited := ancestors[current]; visited {
			continue
		}
		ancestors[current] = struct{}{}
		for _, prerequisite := range graph.prerequisites[current] {
			stack = append(stack, prerequisite.ConceptID)
		}
	}
	ordered := make([]ID, 0, len(ancestors))
	for _, candidate := range graph.topological {
		if _, present := ancestors[candidate]; present {
			ordered = append(ordered, candidate)
		}
	}
	return ordered, nil
}

func (graph *KnowledgeGraph) TopologicalOrder() []ID {
	if graph == nil {
		return nil
	}
	return append([]ID(nil), graph.topological...)
}

func (graph *KnowledgeGraph) CanIntroduce(conceptID ID, snapshot StudentStateSnapshot, policy ResolvedMasteryThreshold) (bool, error) {
	decision, err := graph.EvaluateIntroduction(conceptID, snapshot, policy)
	if err != nil {
		return false, err
	}
	return decision.CanIntroduce, nil
}

func (graph *KnowledgeGraph) MissingPrerequisites(conceptID ID, snapshot StudentStateSnapshot, policy ResolvedMasteryThreshold) ([]PrerequisiteCheck, error) {
	decision, err := graph.EvaluateIntroduction(conceptID, snapshot, policy)
	if err != nil {
		return nil, err
	}
	missing := make([]PrerequisiteCheck, 0)
	for _, check := range decision.Checks {
		if !check.Satisfied {
			missing = append(missing, check)
		}
	}
	return missing, nil
}

func (graph *KnowledgeGraph) EvaluateIntroduction(conceptID ID, snapshot StudentStateSnapshot, policy ResolvedMasteryThreshold) (IntroductionDecision, error) {
	concept, err := graph.concept(conceptID)
	if err != nil {
		return IntroductionDecision{}, err
	}
	if err := policy.Validate(); err != nil {
		return IntroductionDecision{}, fmt.Errorf("evaluate concept introduction: %w", err)
	}
	decision := IntroductionDecision{
		ConceptID: conceptID, Title: concept.Title, CanIntroduce: true,
		PolicyVersion: PrerequisitePolicyVersion,
		MasteryPolicy: policy,
		Checks:        make([]PrerequisiteCheck, 0, len(graph.prerequisites[conceptID])),
	}
	for _, prerequisite := range graph.prerequisites[conceptID] {
		required := graph.concepts[prerequisite.ConceptID]
		state, present := snapshot.State(prerequisite.ConceptID)
		check := PrerequisiteCheck{
			ConceptID: prerequisite.ConceptID, Title: required.Title,
			Requirement: prerequisite.Requirement, StatePresent: present,
			Reason: PrerequisiteMissingState,
		}
		if present {
			check.Exposure = state.Exposure
			check.Mastery = state.Mastery
			switch prerequisite.Requirement {
			case PrerequisiteIntroduced:
				check.Satisfied = state.Exposure != ExposureNotSeen
				if check.Satisfied {
					check.Reason = PrerequisiteSatisfiedIntroduced
				} else {
					check.Reason = PrerequisiteNotIntroduced
				}
			case PrerequisiteMastered:
				check.Satisfied = policy.Requirement.SatisfiedBy(state.Mastery)
				if check.Satisfied {
					check.Reason = PrerequisiteSatisfiedMastery
				} else {
					check.Reason = PrerequisiteBelowMastery
				}
			}
		}
		if prerequisite.Requirement == PrerequisiteMastered {
			threshold := policy.Requirement.Threshold
			check.RequiredMastery = &threshold
		}
		if !check.Satisfied {
			decision.CanIntroduce = false
		}
		decision.Checks = append(decision.Checks, check)
	}
	return decision, nil
}

func (graph *KnowledgeGraph) concept(conceptID ID) (CurriculumNode, error) {
	if graph == nil {
		return CurriculumNode{}, fmt.Errorf("knowledge graph is nil")
	}
	if err := conceptID.Validate(); err != nil {
		return CurriculumNode{}, fmt.Errorf("knowledge graph concept: %w", err)
	}
	concept, exists := graph.concepts[conceptID]
	if !exists {
		return CurriculumNode{}, fmt.Errorf("%w %q", ErrUnknownCurriculumConcept, conceptID)
	}
	return concept, nil
}

func (graph *KnowledgeGraph) calculateTopologicalOrder() ([]ID, error) {
	indegree := make(map[ID]int, len(graph.concepts))
	available := make(idPriorityQueue, 0, len(graph.concepts))
	for conceptID := range graph.concepts {
		indegree[conceptID] = len(graph.prerequisites[conceptID])
		if indegree[conceptID] == 0 {
			heap.Push(&available, conceptID)
		}
	}
	order := make([]ID, 0, len(graph.concepts))
	for available.Len() > 0 {
		current := heap.Pop(&available).(ID)
		order = append(order, current)
		for _, dependent := range graph.dependents[current] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				heap.Push(&available, dependent)
			}
		}
	}
	if len(order) != len(graph.concepts) {
		return nil, fmt.Errorf("build knowledge graph: curriculum prerequisites contain a cycle")
	}
	return order, nil
}

type idPriorityQueue []ID

func (queue idPriorityQueue) Len() int           { return len(queue) }
func (queue idPriorityQueue) Less(i, j int) bool { return queue[i].String() < queue[j].String() }
func (queue idPriorityQueue) Swap(i, j int)      { queue[i], queue[j] = queue[j], queue[i] }

func (queue *idPriorityQueue) Push(value any) {
	*queue = append(*queue, value.(ID))
}

func (queue *idPriorityQueue) Pop() any {
	old := *queue
	last := len(old) - 1
	value := old[last]
	*queue = old[:last]
	return value
}

func formatGraphPercent(value float64) string {
	percentage := math.Round(value*10000) / 100
	return strconv.FormatFloat(percentage, 'f', -1, 64) + "%"
}
