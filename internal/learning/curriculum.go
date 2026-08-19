package learning

import "fmt"

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
