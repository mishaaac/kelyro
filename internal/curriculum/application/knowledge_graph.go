package application

import (
	"container/heap"
	"context"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type KnowledgeGraphCompilerV1 struct{}

func NewKnowledgeGraphCompilerV1() KnowledgeGraphCompilerV1 {
	return KnowledgeGraphCompilerV1{}
}

type conceptIDHeap []curriculum.ConceptID

func (values conceptIDHeap) Len() int           { return len(values) }
func (values conceptIDHeap) Less(i, j int) bool { return values[i].String() < values[j].String() }
func (values conceptIDHeap) Swap(i, j int)      { values[i], values[j] = values[j], values[i] }
func (values *conceptIDHeap) Push(value any)    { *values = append(*values, value.(curriculum.ConceptID)) }
func (values *conceptIDHeap) Pop() any {
	previous := *values
	last := len(previous) - 1
	value := previous[last]
	*values = previous[:last]
	return value
}

type conceptPair struct {
	required  curriculum.ConceptID
	dependent curriculum.ConceptID
}

func (KnowledgeGraphCompilerV1) Compile(ctx context.Context, request KnowledgeGraphCompilationRequest) (curriculum.KnowledgeGraphCompilation, error) {
	const operation = "compile knowledge graph"
	if err := ctx.Err(); err != nil {
		return curriculum.KnowledgeGraphCompilation{}, ExternalError(operation, err)
	}
	concepts := make(map[curriculum.ConceptID]curriculum.Concept, len(request.Concepts))
	for _, concept := range request.Concepts {
		if err := concept.Validate(); err != nil {
			return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, err)
		}
		if concept.Atomicity != curriculum.AtomicityAtomic {
			return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, fmt.Errorf("knowledge graph concept %q is not atomic", concept.ID))
		}
		if _, exists := concepts[concept.ID]; exists {
			return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, fmt.Errorf("duplicate knowledge graph concept %q", concept.ID))
		}
		concepts[concept.ID] = cloneConcept(concept)
	}
	if len(concepts) == 0 {
		return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, fmt.Errorf("knowledge graph has no concepts"))
	}

	prerequisites := cloneAndSortPrerequisites(request.Prerequisites)
	seenEdges := make(map[prerequisiteEdgeKey]struct{}, len(prerequisites))
	dependencies := make(map[curriculum.ConceptID]map[curriculum.ConceptID]struct{}, len(concepts))
	dependents := make(map[curriculum.ConceptID]map[curriculum.ConceptID]struct{}, len(concepts))
	undirected := make(map[curriculum.ConceptID]map[curriculum.ConceptID]struct{}, len(concepts))
	for id := range concepts {
		dependencies[id] = make(map[curriculum.ConceptID]struct{})
		dependents[id] = make(map[curriculum.ConceptID]struct{})
		undirected[id] = make(map[curriculum.ConceptID]struct{})
	}
	for _, prerequisite := range prerequisites {
		if err := prerequisite.Validate(); err != nil {
			return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, err)
		}
		if _, exists := concepts[prerequisite.ConceptID]; !exists {
			return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, fmt.Errorf("prerequisite references missing concept %q", prerequisite.ConceptID))
		}
		if _, exists := concepts[prerequisite.RequiredConceptID]; !exists {
			return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, fmt.Errorf("prerequisite references missing required concept %q", prerequisite.RequiredConceptID))
		}
		key := prerequisiteEdgeKey{concept: prerequisite.ConceptID, required: prerequisite.RequiredConceptID, kind: prerequisite.Kind}
		if _, exists := seenEdges[key]; exists {
			return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, fmt.Errorf("duplicate prerequisite from %q to %q", prerequisite.ConceptID, prerequisite.RequiredConceptID))
		}
		seenEdges[key] = struct{}{}
		dependencies[prerequisite.ConceptID][prerequisite.RequiredConceptID] = struct{}{}
		dependents[prerequisite.RequiredConceptID][prerequisite.ConceptID] = struct{}{}
		undirected[prerequisite.ConceptID][prerequisite.RequiredConceptID] = struct{}{}
		undirected[prerequisite.RequiredConceptID][prerequisite.ConceptID] = struct{}{}
	}

	order, err := graphTopologicalOrder(ctx, concepts, dependencies, dependents)
	if err != nil {
		if ctx.Err() != nil {
			return curriculum.KnowledgeGraphCompilation{}, ExternalError(operation, ctx.Err())
		}
		return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, err)
	}
	roots := make([]curriculum.ConceptID, 0)
	for _, id := range sortedConceptMapIDs(concepts) {
		if len(dependencies[id]) == 0 {
			roots = append(roots, id)
		}
	}
	anchored := make(map[curriculum.ConceptID]struct{})
	queue := make([]curriculum.ConceptID, 0)
	for _, id := range roots {
		if concepts[id].Foundational {
			anchored[id] = struct{}{}
			queue = append(queue, id)
		}
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, dependent := range sortedIDSet(dependents[current]) {
			if _, exists := anchored[dependent]; exists {
				continue
			}
			anchored[dependent] = struct{}{}
			queue = append(queue, dependent)
		}
	}
	unreachable := make([]curriculum.ConceptID, 0)
	for _, id := range sortedConceptMapIDs(concepts) {
		if _, exists := anchored[id]; !exists {
			unreachable = append(unreachable, id)
		}
	}

	paths := longestGraphPaths(order, dependents)
	components := graphComponents(concepts, roots, undirected, paths)
	criticalPath := curriculum.KnowledgeGraphCriticalPath{}
	for _, component := range components {
		if betterConceptPath(component.CriticalPath.ConceptIDs, criticalPath.ConceptIDs) {
			criticalPath = component.CriticalPath
		}
	}
	result := curriculum.KnowledgeGraphCompilation{
		AlgorithmVersion: curriculum.KnowledgeGraphCompilerVersionV1, ConsumptionContract: curriculum.CurriculumConsumptionContractV1,
		ConceptIDs: sortedConceptMapIDs(concepts), Prerequisites: prerequisites, TopologicalOrder: order,
		RootConceptIDs: roots, UnreachableConceptIDs: unreachable, Components: components,
		CriticalPath: criticalPath, ConsumptionPrerequisites: consumptionPrerequisites(prerequisites),
	}
	if err := result.Validate(); err != nil {
		return curriculum.KnowledgeGraphCompilation{}, Invalid(operation, err)
	}
	return result, nil
}

func graphTopologicalOrder(ctx context.Context, concepts map[curriculum.ConceptID]curriculum.Concept, dependencies, dependents map[curriculum.ConceptID]map[curriculum.ConceptID]struct{}) ([]curriculum.ConceptID, error) {
	remaining := make(map[curriculum.ConceptID]int, len(concepts))
	ready := &conceptIDHeap{}
	heap.Init(ready)
	for id := range concepts {
		remaining[id] = len(dependencies[id])
		if remaining[id] == 0 {
			heap.Push(ready, id)
		}
	}
	order := make([]curriculum.ConceptID, 0, len(concepts))
	for ready.Len() > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		current := heap.Pop(ready).(curriculum.ConceptID)
		order = append(order, current)
		for _, dependent := range sortedIDSet(dependents[current]) {
			remaining[dependent]--
			if remaining[dependent] == 0 {
				heap.Push(ready, dependent)
			}
		}
	}
	if len(order) != len(concepts) {
		cyclic := make([]curriculum.ConceptID, 0)
		for id, count := range remaining {
			if count > 0 {
				cyclic = append(cyclic, id)
			}
		}
		sort.Slice(cyclic, func(i, j int) bool { return cyclic[i].String() < cyclic[j].String() })
		return nil, fmt.Errorf("knowledge graph contains a cycle involving %q", cyclic[0])
	}
	return order, nil
}

func longestGraphPaths(order []curriculum.ConceptID, dependents map[curriculum.ConceptID]map[curriculum.ConceptID]struct{}) map[curriculum.ConceptID][]curriculum.ConceptID {
	paths := make(map[curriculum.ConceptID][]curriculum.ConceptID, len(order))
	for _, id := range order {
		if len(paths[id]) == 0 {
			paths[id] = []curriculum.ConceptID{id}
		}
		for _, dependent := range sortedIDSet(dependents[id]) {
			candidate := append(append([]curriculum.ConceptID(nil), paths[id]...), dependent)
			if betterConceptPath(candidate, paths[dependent]) {
				paths[dependent] = candidate
			}
		}
	}
	return paths
}

func graphComponents(concepts map[curriculum.ConceptID]curriculum.Concept, roots []curriculum.ConceptID, undirected map[curriculum.ConceptID]map[curriculum.ConceptID]struct{}, paths map[curriculum.ConceptID][]curriculum.ConceptID) []curriculum.KnowledgeGraphComponent {
	seen := make(map[curriculum.ConceptID]struct{}, len(concepts))
	rootSet := make(map[curriculum.ConceptID]struct{}, len(roots))
	for _, id := range roots {
		rootSet[id] = struct{}{}
	}
	components := make([]curriculum.KnowledgeGraphComponent, 0)
	for _, start := range sortedConceptMapIDs(concepts) {
		if _, exists := seen[start]; exists {
			continue
		}
		component := curriculum.KnowledgeGraphComponent{}
		queue := []curriculum.ConceptID{start}
		seen[start] = struct{}{}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			component.ConceptIDs = append(component.ConceptIDs, current)
			if _, exists := rootSet[current]; exists {
				component.RootConceptIDs = append(component.RootConceptIDs, current)
				if concepts[current].Foundational {
					component.FoundationalRootIDs = append(component.FoundationalRootIDs, current)
				}
			}
			if betterConceptPath(paths[current], component.CriticalPath.ConceptIDs) {
				component.CriticalPath.ConceptIDs = append([]curriculum.ConceptID(nil), paths[current]...)
				component.CriticalPath.EdgeCount = len(paths[current]) - 1
			}
			for _, adjacent := range sortedIDSet(undirected[current]) {
				if _, exists := seen[adjacent]; exists {
					continue
				}
				seen[adjacent] = struct{}{}
				queue = append(queue, adjacent)
			}
		}
		sort.Slice(component.ConceptIDs, func(i, j int) bool { return component.ConceptIDs[i].String() < component.ConceptIDs[j].String() })
		sort.Slice(component.RootConceptIDs, func(i, j int) bool {
			return component.RootConceptIDs[i].String() < component.RootConceptIDs[j].String()
		})
		sort.Slice(component.FoundationalRootIDs, func(i, j int) bool {
			return component.FoundationalRootIDs[i].String() < component.FoundationalRootIDs[j].String()
		})
		components = append(components, component)
	}
	return components
}

func betterConceptPath(candidate, current []curriculum.ConceptID) bool {
	if len(candidate) != len(current) {
		return len(candidate) > len(current)
	}
	for index := range candidate {
		if candidate[index] == current[index] {
			continue
		}
		return candidate[index].String() < current[index].String()
	}
	return false
}

func sortedIDSet(values map[curriculum.ConceptID]struct{}) []curriculum.ConceptID {
	result := make([]curriculum.ConceptID, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func consumptionPrerequisites(prerequisites []curriculum.Prerequisite) []curriculum.ConsumptionPrerequisite {
	byPair := make(map[conceptPair]curriculum.ConsumptionPrerequisiteRequirement)
	for _, prerequisite := range prerequisites {
		if prerequisite.Kind == curriculum.PrerequisiteRecommended {
			continue
		}
		key := conceptPair{required: prerequisite.RequiredConceptID, dependent: prerequisite.ConceptID}
		requirement := curriculum.ConsumptionPrerequisiteIntroduced
		if prerequisite.Kind == curriculum.PrerequisiteHard {
			requirement = curriculum.ConsumptionPrerequisiteMastered
		}
		if existing, exists := byPair[key]; !exists || requirement == curriculum.ConsumptionPrerequisiteMastered || existing != curriculum.ConsumptionPrerequisiteMastered {
			byPair[key] = requirement
		}
	}
	keys := make([]conceptPair, 0, len(byPair))
	for key := range byPair {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].dependent != keys[j].dependent {
			return keys[i].dependent.String() < keys[j].dependent.String()
		}
		return keys[i].required.String() < keys[j].required.String()
	})
	result := make([]curriculum.ConsumptionPrerequisite, 0, len(keys))
	for _, key := range keys {
		result = append(result, curriculum.ConsumptionPrerequisite{ConceptID: key.dependent, RequiredConceptID: key.required, Requirement: byPair[key]})
	}
	return result
}
