package curriculum

import "fmt"

const CurriculumHierarchyBuilderVersionV1 = "curriculum-hierarchy-builder-v1"

type Phase struct {
	ID          ID
	Title       string
	Description string
	Order       int
}

func (phase Phase) Validate() error {
	return validateHierarchyNode("phase", phase.ID, phase.Title, phase.Description, phase.Order)
}

type Module struct {
	ID          ID
	PhaseID     ID
	Title       string
	Description string
	Order       int
}

func (module Module) Validate() error {
	if err := validateHierarchyNode("module", module.ID, module.Title, module.Description, module.Order); err != nil {
		return err
	}
	if err := module.PhaseID.Validate(); err != nil {
		return fmt.Errorf("module phase: %w", err)
	}
	return nil
}

type LessonSpec struct {
	ID          ID
	ModuleID    ID
	Title       string
	Description string
	Order       int
}

func (lesson LessonSpec) Validate() error {
	if err := validateHierarchyNode("lesson", lesson.ID, lesson.Title, lesson.Description, lesson.Order); err != nil {
		return err
	}
	if err := lesson.ModuleID.Validate(); err != nil {
		return fmt.Errorf("lesson module: %w", err)
	}
	return nil
}

type TopicSpec struct {
	ID          ID
	LessonID    ID
	Title       string
	Description string
	Order       int
	ConceptIDs  []ConceptID
}

// CurriculumHierarchy is the deterministic UX projection of a prerequisite
// graph. It deliberately carries no prerequisite edges: the graph remains the
// pedagogical source of truth.
type CurriculumHierarchy struct {
	Phases           []Phase
	Modules          []Module
	Lessons          []LessonSpec
	Topics           []TopicSpec
	AlgorithmVersion string
}

func (hierarchy CurriculumHierarchy) Validate(concepts []Concept) error {
	known := make(map[ConceptID]struct{}, len(concepts))
	for _, concept := range concepts {
		if err := concept.Validate(); err != nil {
			return err
		}
		if _, exists := known[concept.ID]; exists {
			return fmt.Errorf("hierarchy concepts contain duplicate concept %q", concept.ID)
		}
		known[concept.ID] = struct{}{}
	}
	if hierarchy.AlgorithmVersion != CurriculumHierarchyBuilderVersionV1 {
		return fmt.Errorf("unsupported curriculum hierarchy builder version %q", hierarchy.AlgorithmVersion)
	}
	return validateHierarchyParts(hierarchy.Phases, hierarchy.Modules, hierarchy.Lessons, hierarchy.Topics, known)
}

// PracticeContextAssignment is a pack-authored UX hint. It only affects Topic
// grouping and never changes Concept identity or prerequisite semantics.
type PracticeContextAssignment struct {
	ConceptID ConceptID
	Context   string
}

func (assignment PracticeContextAssignment) Validate() error {
	if err := assignment.ConceptID.Validate(); err != nil {
		return fmt.Errorf("practice context concept: %w", err)
	}
	return requireText("practice context", assignment.Context)
}

func (topic TopicSpec) Validate() error {
	if err := validateHierarchyNode("topic", topic.ID, topic.Title, topic.Description, topic.Order); err != nil {
		return err
	}
	if err := topic.LessonID.Validate(); err != nil {
		return fmt.Errorf("topic lesson: %w", err)
	}
	if len(topic.ConceptIDs) == 0 {
		return fmt.Errorf("topic %q has no concepts", topic.ID)
	}
	return validateConceptIDs("topic concepts", topic.ConceptIDs)
}

func validateHierarchyNode(kind string, id ID, title, description string, order int) error {
	if err := id.Validate(); err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}
	if err := requireText(kind+" title", title); err != nil {
		return err
	}
	if err := requireText(kind+" description", description); err != nil {
		return err
	}
	if order < 0 {
		return fmt.Errorf("%s %q has negative order", kind, id)
	}
	return nil
}
