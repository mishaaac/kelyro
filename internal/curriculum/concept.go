package curriculum

import "fmt"

// Concept is the smallest independently assessable and learner-trackable unit
// emitted by the compiler. Lesson prose and exercises remain outside I-04.
type Concept struct {
	ID           ConceptID
	Title        string
	Definition   string
	Version      string
	Atomicity    Atomicity
	Difficulty   Difficulty
	Status       ConceptStatus
	Foundational bool
	EvidenceRefs []EvidenceRef
}

func (concept Concept) Validate() error {
	if err := concept.ID.Validate(); err != nil {
		return fmt.Errorf("concept: %w", err)
	}
	for _, field := range []struct{ name, value string }{
		{name: "concept title", value: concept.Title},
		{name: "concept definition", value: concept.Definition},
		{name: "concept version", value: concept.Version},
	} {
		if err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	if err := concept.Atomicity.Validate(); err != nil {
		return err
	}
	if err := concept.Difficulty.Validate(); err != nil {
		return err
	}
	if err := concept.Status.Validate(); err != nil {
		return err
	}
	return validateEvidenceRefs("concept evidence", concept.EvidenceRefs)
}
