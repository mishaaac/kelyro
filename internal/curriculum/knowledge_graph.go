package curriculum

import "fmt"

const (
	KnowledgeGraphCompilerVersionV1 = "knowledge-graph-compiler-v1"
	CurriculumConsumptionContractV1 = "curriculum-consumption/v1"
)

type ConsumptionPrerequisiteRequirement string

const (
	ConsumptionPrerequisiteIntroduced ConsumptionPrerequisiteRequirement = "introduced"
	ConsumptionPrerequisiteMastered   ConsumptionPrerequisiteRequirement = "mastered"
)

func (requirement ConsumptionPrerequisiteRequirement) Validate() error {
	switch requirement {
	case ConsumptionPrerequisiteIntroduced, ConsumptionPrerequisiteMastered:
		return nil
	default:
		return fmt.Errorf("invalid consumption prerequisite requirement %q", requirement)
	}
}

// ConsumptionPrerequisite is the graph projection accepted by I-02. The
// hierarchy and remaining pedagogical metadata are assembled by later I-04
// passes; this projection never reads or mutates student state.
type ConsumptionPrerequisite struct {
	ConceptID         ConceptID
	RequiredConceptID ConceptID
	Requirement       ConsumptionPrerequisiteRequirement
}

func (prerequisite ConsumptionPrerequisite) Validate() error {
	if err := prerequisite.ConceptID.Validate(); err != nil {
		return fmt.Errorf("consumption prerequisite concept: %w", err)
	}
	if err := prerequisite.RequiredConceptID.Validate(); err != nil {
		return fmt.Errorf("consumption required concept: %w", err)
	}
	if prerequisite.ConceptID == prerequisite.RequiredConceptID {
		return fmt.Errorf("consumption prerequisite cannot reference itself")
	}
	return prerequisite.Requirement.Validate()
}

type KnowledgeGraphCriticalPath struct {
	ConceptIDs []ConceptID
	EdgeCount  int
}

func (path KnowledgeGraphCriticalPath) Validate() error {
	if len(path.ConceptIDs) == 0 {
		return fmt.Errorf("knowledge graph critical path is empty")
	}
	if path.EdgeCount != len(path.ConceptIDs)-1 {
		return fmt.Errorf("knowledge graph critical path edge count does not match its concepts")
	}
	return validateConceptIDs("knowledge graph critical path", path.ConceptIDs)
}

type KnowledgeGraphComponent struct {
	ConceptIDs          []ConceptID
	RootConceptIDs      []ConceptID
	FoundationalRootIDs []ConceptID
	CriticalPath        KnowledgeGraphCriticalPath
}

func (component KnowledgeGraphComponent) Validate() error {
	if len(component.ConceptIDs) == 0 {
		return fmt.Errorf("knowledge graph component is empty")
	}
	if err := validateConceptIDs("knowledge graph component concepts", component.ConceptIDs); err != nil {
		return err
	}
	if len(component.RootConceptIDs) == 0 {
		return fmt.Errorf("knowledge graph component has no root")
	}
	if err := validateConceptIDs("knowledge graph component roots", component.RootConceptIDs); err != nil {
		return err
	}
	if err := validateConceptIDs("knowledge graph component foundational roots", component.FoundationalRootIDs); err != nil {
		return err
	}
	return component.CriticalPath.Validate()
}

// KnowledgeGraphCompilation is a deterministic, learner-neutral graph
// artifact. Prerequisite direction remains dependent -> required, while the
// topological order always places required concepts before dependents.
type KnowledgeGraphCompilation struct {
	AlgorithmVersion         string
	ConsumptionContract      string
	ConceptIDs               []ConceptID
	Prerequisites            []Prerequisite
	TopologicalOrder         []ConceptID
	RootConceptIDs           []ConceptID
	UnreachableConceptIDs    []ConceptID
	Components               []KnowledgeGraphComponent
	CriticalPath             KnowledgeGraphCriticalPath
	ConsumptionPrerequisites []ConsumptionPrerequisite
}

func (compilation KnowledgeGraphCompilation) Validate() error {
	if compilation.AlgorithmVersion != KnowledgeGraphCompilerVersionV1 {
		return fmt.Errorf("unsupported knowledge graph compiler version %q", compilation.AlgorithmVersion)
	}
	if compilation.ConsumptionContract != CurriculumConsumptionContractV1 {
		return fmt.Errorf("unsupported curriculum consumption contract %q", compilation.ConsumptionContract)
	}
	if len(compilation.ConceptIDs) == 0 {
		return fmt.Errorf("knowledge graph has no concepts")
	}
	known, err := conceptIDSet("knowledge graph concepts", compilation.ConceptIDs)
	if err != nil {
		return err
	}
	if err := requireExactConceptSet("knowledge graph topological order", compilation.TopologicalOrder, known); err != nil {
		return err
	}
	if err := requireKnownConcepts("knowledge graph roots", compilation.RootConceptIDs, known); err != nil {
		return err
	}
	if len(compilation.RootConceptIDs) == 0 {
		return fmt.Errorf("knowledge graph has no roots")
	}
	if err := requireKnownConcepts("knowledge graph unreachable concepts", compilation.UnreachableConceptIDs, known); err != nil {
		return err
	}

	edges := make(map[[2]ConceptID]struct{})
	position := make(map[ConceptID]int, len(compilation.TopologicalOrder))
	for index, id := range compilation.TopologicalOrder {
		position[id] = index
	}
	for _, prerequisite := range compilation.Prerequisites {
		if err := prerequisite.Validate(); err != nil {
			return err
		}
		if _, exists := known[prerequisite.ConceptID]; !exists {
			return fmt.Errorf("knowledge graph prerequisite references missing concept %q", prerequisite.ConceptID)
		}
		if _, exists := known[prerequisite.RequiredConceptID]; !exists {
			return fmt.Errorf("knowledge graph prerequisite references missing required concept %q", prerequisite.RequiredConceptID)
		}
		if position[prerequisite.RequiredConceptID] >= position[prerequisite.ConceptID] {
			return fmt.Errorf("knowledge graph topological order violates prerequisite from %q to %q", prerequisite.ConceptID, prerequisite.RequiredConceptID)
		}
		edges[[2]ConceptID{prerequisite.RequiredConceptID, prerequisite.ConceptID}] = struct{}{}
	}

	componentConcepts := make(map[ConceptID]struct{}, len(known))
	for _, component := range compilation.Components {
		if err := component.Validate(); err != nil {
			return err
		}
		for _, id := range component.ConceptIDs {
			if _, exists := known[id]; !exists {
				return fmt.Errorf("knowledge graph component references missing concept %q", id)
			}
			if _, exists := componentConcepts[id]; exists {
				return fmt.Errorf("knowledge graph concept %q appears in multiple components", id)
			}
			componentConcepts[id] = struct{}{}
		}
		if err := validateCriticalPathEdges(component.CriticalPath, edges); err != nil {
			return err
		}
	}
	if len(componentConcepts) != len(known) {
		return fmt.Errorf("knowledge graph components do not cover all concepts")
	}
	if err := compilation.CriticalPath.Validate(); err != nil {
		return err
	}
	if err := validateCriticalPathEdges(compilation.CriticalPath, edges); err != nil {
		return err
	}
	for _, prerequisite := range compilation.ConsumptionPrerequisites {
		if err := prerequisite.Validate(); err != nil {
			return err
		}
		if _, exists := known[prerequisite.ConceptID]; !exists {
			return fmt.Errorf("consumption prerequisite references missing concept %q", prerequisite.ConceptID)
		}
		if _, exists := known[prerequisite.RequiredConceptID]; !exists {
			return fmt.Errorf("consumption prerequisite references missing required concept %q", prerequisite.RequiredConceptID)
		}
	}
	return nil
}

func conceptIDSet(name string, values []ConceptID) (map[ConceptID]struct{}, error) {
	if err := validateConceptIDs(name, values); err != nil {
		return nil, err
	}
	result := make(map[ConceptID]struct{}, len(values))
	for _, id := range values {
		if _, exists := result[id]; exists {
			return nil, fmt.Errorf("%s contains duplicate concept %q", name, id)
		}
		result[id] = struct{}{}
	}
	return result, nil
}

func requireKnownConcepts(name string, values []ConceptID, known map[ConceptID]struct{}) error {
	seen, err := conceptIDSet(name, values)
	if err != nil {
		return err
	}
	for id := range seen {
		if _, exists := known[id]; !exists {
			return fmt.Errorf("%s references missing concept %q", name, id)
		}
	}
	return nil
}

func requireExactConceptSet(name string, values []ConceptID, known map[ConceptID]struct{}) error {
	if err := requireKnownConcepts(name, values, known); err != nil {
		return err
	}
	if len(values) != len(known) {
		return fmt.Errorf("%s does not contain every concept", name)
	}
	return nil
}

func validateCriticalPathEdges(path KnowledgeGraphCriticalPath, edges map[[2]ConceptID]struct{}) error {
	for index := 1; index < len(path.ConceptIDs); index++ {
		if _, exists := edges[[2]ConceptID{path.ConceptIDs[index-1], path.ConceptIDs[index]}]; !exists {
			return fmt.Errorf("knowledge graph critical path contains non-edge from %q to %q", path.ConceptIDs[index-1], path.ConceptIDs[index])
		}
	}
	return nil
}
