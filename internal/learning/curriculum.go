package learning

import (
	"fmt"
	"sort"
	"strings"
)

const CurriculumContractVersion = "curriculum-consumption/v1"

// CurriculumRef identifies a deterministic, versioned curriculum supplied by
// a future curriculum provider. It does not contain provenance or research logic.
type CurriculumRef struct {
	ID      ID
	Version string
}

func (reference CurriculumRef) Validate() error {
	if err := reference.ID.Validate(); err != nil {
		return fmt.Errorf("curriculum: %w", err)
	}
	return requireText("curriculum version", reference.Version)
}

type CurriculumNodeType string

const (
	CurriculumNodePhase   CurriculumNodeType = "phase"
	CurriculumNodeModule  CurriculumNodeType = "module"
	CurriculumNodeLesson  CurriculumNodeType = "lesson"
	CurriculumNodeTopic   CurriculumNodeType = "topic"
	CurriculumNodeConcept CurriculumNodeType = "concept"
)

func (nodeType CurriculumNodeType) Validate() error {
	switch nodeType {
	case CurriculumNodePhase, CurriculumNodeModule, CurriculumNodeLesson, CurriculumNodeTopic, CurriculumNodeConcept:
		return nil
	default:
		return fmt.Errorf("invalid curriculum node type %q", nodeType)
	}
}

type CurriculumNodeStatus string

const (
	CurriculumNodeDraft      CurriculumNodeStatus = "draft"
	CurriculumNodeActive     CurriculumNodeStatus = "active"
	CurriculumNodeDeprecated CurriculumNodeStatus = "deprecated"
)

type CurriculumStatusMetadata struct {
	State CurriculumNodeStatus
	Note  string
}

func (metadata CurriculumStatusMetadata) Validate() error {
	switch metadata.State {
	case CurriculumNodeDraft, CurriculumNodeActive, CurriculumNodeDeprecated:
		return nil
	default:
		return fmt.Errorf("invalid curriculum node status %q", metadata.State)
	}
}

// CurriculumDisplayHints affect presentation only. They never express
// prerequisites or progression policy.
type CurriculumDisplayHints struct {
	ShortTitle string
	Hidden     bool
}

type ConceptDifficulty int

const (
	ConceptDifficultyIntroductory ConceptDifficulty = 1
	ConceptDifficultyFoundational ConceptDifficulty = 2
	ConceptDifficultyIntermediate ConceptDifficulty = 3
	ConceptDifficultyAdvanced     ConceptDifficulty = 4
	ConceptDifficultyExpert       ConceptDifficulty = 5
)

func (difficulty ConceptDifficulty) Validate() error {
	if difficulty < ConceptDifficultyIntroductory || difficulty > ConceptDifficultyExpert {
		return fmt.Errorf("concept difficulty %d is outside 1..5", difficulty)
	}
	return nil
}

// PrerequisiteRequirement declares data consumed by the prerequisite engine
// in Step 9. This package validates the declaration but does not evaluate it.
type PrerequisiteRequirement string

const (
	PrerequisiteIntroduced PrerequisiteRequirement = "introduced"
	PrerequisiteMastered   PrerequisiteRequirement = "mastered"
)

type ConceptPrerequisite struct {
	ConceptID   ID
	Requirement PrerequisiteRequirement
}

func (prerequisite ConceptPrerequisite) Validate() error {
	if err := prerequisite.ConceptID.Validate(); err != nil {
		return fmt.Errorf("prerequisite concept: %w", err)
	}
	switch prerequisite.Requirement {
	case PrerequisiteIntroduced, PrerequisiteMastered:
		return nil
	default:
		return fmt.Errorf("invalid prerequisite requirement %q", prerequisite.Requirement)
	}
}

// ConceptDefinition contains pedagogical metadata, not generated lesson or
// assessment content.
type ConceptDefinition struct {
	Objectives             []string
	Prerequisites          []ConceptPrerequisite
	Difficulty             ConceptDifficulty
	EstimatedEffortMinutes int
	TheoryRequired         bool
	AssessmentExpectations []string
}

func (definition ConceptDefinition) Validate() error {
	if err := validateTexts("concept objectives", definition.Objectives); err != nil {
		return err
	}
	if len(definition.Objectives) == 0 {
		return fmt.Errorf("concept objectives are empty")
	}
	if err := definition.Difficulty.Validate(); err != nil {
		return err
	}
	if definition.EstimatedEffortMinutes <= 0 {
		return fmt.Errorf("concept estimated effort must be positive")
	}
	if err := validateTexts("assessment expectations", definition.AssessmentExpectations); err != nil {
		return err
	}
	if len(definition.AssessmentExpectations) == 0 {
		return fmt.Errorf("assessment expectations are empty")
	}
	seen := make(map[ID]struct{}, len(definition.Prerequisites))
	for _, prerequisite := range definition.Prerequisites {
		if err := prerequisite.Validate(); err != nil {
			return err
		}
		if _, exists := seen[prerequisite.ConceptID]; exists {
			return fmt.Errorf("duplicate prerequisite %q", prerequisite.ConceptID)
		}
		seen[prerequisite.ConceptID] = struct{}{}
	}
	return nil
}

// CurriculumNode is a flat, presentation-neutral node. ParentID expresses the
// visible hierarchy; concept prerequisites express the real knowledge graph.
type CurriculumNode struct {
	ID          ID
	Type        CurriculumNodeType
	ParentID    *ID
	Title       string
	Description string
	Order       int
	Display     CurriculumDisplayHints
	Status      CurriculumStatusMetadata
	Version     string
	Concept     *ConceptDefinition
}

func (node CurriculumNode) Validate() error {
	if err := node.ID.Validate(); err != nil {
		return fmt.Errorf("curriculum node: %w", err)
	}
	if err := node.Type.Validate(); err != nil {
		return err
	}
	if node.ParentID != nil {
		if err := node.ParentID.Validate(); err != nil {
			return fmt.Errorf("curriculum node parent: %w", err)
		}
		if *node.ParentID == node.ID {
			return fmt.Errorf("curriculum node %q cannot parent itself", node.ID)
		}
	}
	if err := requireText("curriculum node title", node.Title); err != nil {
		return err
	}
	if err := requireText("curriculum node description", node.Description); err != nil {
		return err
	}
	if node.Order < 0 {
		return fmt.Errorf("curriculum node %q has invalid order %d", node.ID, node.Order)
	}
	if err := node.Status.Validate(); err != nil {
		return err
	}
	if err := requireText("curriculum node version", node.Version); err != nil {
		return err
	}
	if node.Type == CurriculumNodeConcept {
		if node.Concept == nil {
			return fmt.Errorf("concept node %q has no concept definition", node.ID)
		}
		if err := node.Concept.Validate(); err != nil {
			return fmt.Errorf("concept node %q: %w", node.ID, err)
		}
		return nil
	}
	if node.Concept != nil {
		return fmt.Errorf("non-concept node %q contains concept metadata", node.ID)
	}
	return nil
}

// Curriculum is the immutable definition consumed by Student Core. Nodes are
// canonicalized so equivalent source documents produce identical definitions.
type Curriculum struct {
	ContractVersion string
	Reference       CurriculumRef
	Title           string
	Description     string
	Nodes           []CurriculumNode
}

func NewCurriculum(contractVersion string, reference CurriculumRef, title, description string, nodes []CurriculumNode) (Curriculum, error) {
	curriculum := Curriculum{
		ContractVersion: contractVersion,
		Reference:       reference,
		Title:           title,
		Description:     description,
		Nodes:           cloneCurriculumNodes(nodes),
	}
	canonicalizeCurriculumNodes(curriculum.Nodes)
	if err := curriculum.Validate(); err != nil {
		return Curriculum{}, err
	}
	return curriculum, nil
}

func (curriculum Curriculum) Validate() error {
	if curriculum.ContractVersion != CurriculumContractVersion {
		return fmt.Errorf("unsupported curriculum contract version %q", curriculum.ContractVersion)
	}
	if err := curriculum.Reference.Validate(); err != nil {
		return err
	}
	if err := requireText("curriculum title", curriculum.Title); err != nil {
		return err
	}
	if err := requireText("curriculum description", curriculum.Description); err != nil {
		return err
	}
	if len(curriculum.Nodes) == 0 {
		return fmt.Errorf("curriculum has no nodes")
	}

	byID := make(map[ID]CurriculumNode, len(curriculum.Nodes))
	for _, node := range curriculum.Nodes {
		if err := node.Validate(); err != nil {
			return err
		}
		if _, exists := byID[node.ID]; exists {
			return fmt.Errorf("duplicate curriculum node id %q", node.ID)
		}
		byID[node.ID] = node
	}
	for _, node := range curriculum.Nodes {
		if node.Type == CurriculumNodePhase {
			if node.ParentID != nil {
				return fmt.Errorf("phase %q must not have a parent", node.ID)
			}
			continue
		}
		if node.ParentID == nil {
			return fmt.Errorf("curriculum node %q is missing its parent", node.ID)
		}
		if _, exists := byID[*node.ParentID]; !exists {
			return fmt.Errorf("curriculum node %q references missing parent %q", node.ID, *node.ParentID)
		}
	}
	if err := validateParentCycles(curriculum.Nodes, byID); err != nil {
		return err
	}
	for _, node := range curriculum.Nodes {
		if node.ParentID == nil {
			continue
		}
		parent := byID[*node.ParentID]
		if !validParentType(node.Type, parent.Type) {
			return fmt.Errorf("%s node %q cannot have %s parent %q", node.Type, node.ID, parent.Type, parent.ID)
		}
	}
	if err := validateSiblingOrders(curriculum.Nodes); err != nil {
		return err
	}
	if err := validateConceptPrerequisites(curriculum.Nodes, byID); err != nil {
		return err
	}
	return nil
}

func (curriculum Curriculum) Node(id ID) (CurriculumNode, bool) {
	for _, node := range curriculum.Nodes {
		if node.ID == id {
			return cloneCurriculumNode(node), true
		}
	}
	return CurriculumNode{}, false
}

func validParentType(child, parent CurriculumNodeType) bool {
	switch child {
	case CurriculumNodeModule:
		return parent == CurriculumNodePhase
	case CurriculumNodeLesson:
		return parent == CurriculumNodeModule
	case CurriculumNodeTopic:
		return parent == CurriculumNodeLesson
	case CurriculumNodeConcept:
		return parent == CurriculumNodeTopic
	default:
		return false
	}
}

func validateParentCycles(nodes []CurriculumNode, byID map[ID]CurriculumNode) error {
	const (
		visiting = 1
		visited  = 2
	)
	state := make(map[ID]int, len(nodes))
	var visit func(ID) error
	visit = func(id ID) error {
		switch state[id] {
		case visiting:
			return fmt.Errorf("curriculum hierarchy contains a cycle at %q", id)
		case visited:
			return nil
		}
		state[id] = visiting
		node := byID[id]
		if node.ParentID != nil {
			if err := visit(*node.ParentID); err != nil {
				return err
			}
		}
		state[id] = visited
		return nil
	}
	for _, node := range nodes {
		if err := visit(node.ID); err != nil {
			return err
		}
	}
	return nil
}

func validateSiblingOrders(nodes []CurriculumNode) error {
	type siblingKey struct {
		hasParent bool
		parent    ID
		order     int
	}
	seen := make(map[siblingKey]ID, len(nodes))
	for _, node := range nodes {
		key := siblingKey{order: node.Order}
		if node.ParentID != nil {
			key.hasParent = true
			key.parent = *node.ParentID
		}
		if previous, exists := seen[key]; exists {
			return fmt.Errorf("curriculum siblings %q and %q share order %d", previous, node.ID, node.Order)
		}
		seen[key] = node.ID
	}
	return nil
}

func validateConceptPrerequisites(nodes []CurriculumNode, byID map[ID]CurriculumNode) error {
	edges := make(map[ID][]ID)
	for _, node := range nodes {
		if node.Type != CurriculumNodeConcept {
			continue
		}
		for _, prerequisite := range node.Concept.Prerequisites {
			required, exists := byID[prerequisite.ConceptID]
			if !exists || required.Type != CurriculumNodeConcept {
				return fmt.Errorf("concept %q has unknown prerequisite %q", node.ID, prerequisite.ConceptID)
			}
			if prerequisite.ConceptID == node.ID {
				return fmt.Errorf("concept %q cannot require itself", node.ID)
			}
			edges[node.ID] = append(edges[node.ID], prerequisite.ConceptID)
		}
	}
	state := make(map[ID]int, len(edges))
	var visit func(ID) error
	visit = func(id ID) error {
		if state[id] == 1 {
			return fmt.Errorf("curriculum prerequisites contain a cycle at %q", id)
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, required := range edges[id] {
			if err := visit(required); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for _, node := range nodes {
		if node.Type == CurriculumNodeConcept {
			if err := visit(node.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func canonicalizeCurriculumNodes(nodes []CurriculumNode) {
	for index := range nodes {
		if nodes[index].Concept != nil {
			sort.Slice(nodes[index].Concept.Prerequisites, func(i, j int) bool {
				return nodes[index].Concept.Prerequisites[i].ConceptID.String() < nodes[index].Concept.Prerequisites[j].ConceptID.String()
			})
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		left, right := nodes[i], nodes[j]
		leftRank, rightRank := curriculumNodeTypeRank(left.Type), curriculumNodeTypeRank(right.Type)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		leftParent, rightParent := "", ""
		if left.ParentID != nil {
			leftParent = left.ParentID.String()
		}
		if right.ParentID != nil {
			rightParent = right.ParentID.String()
		}
		if leftParent != rightParent {
			return leftParent < rightParent
		}
		if left.Order != right.Order {
			return left.Order < right.Order
		}
		return left.ID.String() < right.ID.String()
	})
}

func curriculumNodeTypeRank(nodeType CurriculumNodeType) int {
	switch nodeType {
	case CurriculumNodePhase:
		return 0
	case CurriculumNodeModule:
		return 1
	case CurriculumNodeLesson:
		return 2
	case CurriculumNodeTopic:
		return 3
	case CurriculumNodeConcept:
		return 4
	default:
		return 5
	}
}

func cloneCurriculumNodes(nodes []CurriculumNode) []CurriculumNode {
	cloned := make([]CurriculumNode, len(nodes))
	for index, node := range nodes {
		cloned[index] = cloneCurriculumNode(node)
	}
	return cloned
}

func cloneCurriculumNode(node CurriculumNode) CurriculumNode {
	if node.ParentID != nil {
		parent := *node.ParentID
		node.ParentID = &parent
	}
	if node.Concept != nil {
		definition := *node.Concept
		definition.Objectives = append([]string(nil), definition.Objectives...)
		definition.Prerequisites = append([]ConceptPrerequisite(nil), definition.Prerequisites...)
		definition.AssessmentExpectations = append([]string(nil), definition.AssessmentExpectations...)
		node.Concept = &definition
	}
	return node
}

func validateTexts(name string, values []string) error {
	for index, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s contains empty value at index %d", name, index)
		}
	}
	return nil
}

// Phase, Module, Lesson, Topic, and Concept remain the compact vocabulary used
// by persistence ports created before the full consumption contract.
type Phase struct {
	ID        ID
	Title     string
	ModuleIDs []ID
}

func (phase Phase) Validate() error {
	if err := phase.ID.Validate(); err != nil {
		return fmt.Errorf("phase: %w", err)
	}
	if err := requireText("phase title", phase.Title); err != nil {
		return err
	}
	return validateIDs("phase modules", phase.ModuleIDs)
}

type Module struct {
	ID        ID
	PhaseID   ID
	Title     string
	LessonIDs []ID
}

func (module Module) Validate() error {
	if err := module.ID.Validate(); err != nil {
		return fmt.Errorf("module: %w", err)
	}
	if err := module.PhaseID.Validate(); err != nil {
		return fmt.Errorf("module phase: %w", err)
	}
	if err := requireText("module title", module.Title); err != nil {
		return err
	}
	return validateIDs("module lessons", module.LessonIDs)
}

type Lesson struct {
	ID       ID
	ModuleID ID
	Title    string
	TopicIDs []ID
}

func (lesson Lesson) Validate() error {
	if err := lesson.ID.Validate(); err != nil {
		return fmt.Errorf("lesson: %w", err)
	}
	if err := lesson.ModuleID.Validate(); err != nil {
		return fmt.Errorf("lesson module: %w", err)
	}
	if err := requireText("lesson title", lesson.Title); err != nil {
		return err
	}
	return validateIDs("lesson topics", lesson.TopicIDs)
}

type Topic struct {
	ID         ID
	LessonID   ID
	Title      string
	ConceptIDs []ID
}

func (topic Topic) Validate() error {
	if err := topic.ID.Validate(); err != nil {
		return fmt.Errorf("topic: %w", err)
	}
	if err := topic.LessonID.Validate(); err != nil {
		return fmt.Errorf("topic lesson: %w", err)
	}
	if err := requireText("topic title", topic.Title); err != nil {
		return err
	}
	return validateIDs("topic concepts", topic.ConceptIDs)
}

// Concept is the smallest independently tracked knowledge unit. Its stable ID
// is independent from its mutable display title.
type Concept struct {
	ID      ID
	TopicID ID
	Title   string
}

func (concept Concept) Validate() error {
	if err := concept.ID.Validate(); err != nil {
		return fmt.Errorf("concept: %w", err)
	}
	if err := concept.TopicID.Validate(); err != nil {
		return fmt.Errorf("concept topic: %w", err)
	}
	return requireText("concept title", concept.Title)
}

type Prerequisite struct {
	ConceptID         ID
	RequiredConceptID ID
}

func NewPrerequisite(conceptID, requiredConceptID ID) (Prerequisite, error) {
	prerequisite := Prerequisite{ConceptID: conceptID, RequiredConceptID: requiredConceptID}
	return prerequisite, prerequisite.Validate()
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
	return nil
}
