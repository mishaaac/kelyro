package curriculum

import "fmt"

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
