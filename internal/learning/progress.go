package learning

import "fmt"

// ExposureState describes where a concept is in the learning lifecycle. It is
// independent from MasteryScore and must not be used as a numeric substitute.
type ExposureState string

const (
	ExposureNotSeen    ExposureState = "not_seen"
	ExposureIntroduced ExposureState = "introduced"
	ExposureLearning   ExposureState = "learning"
	ExposurePracticing ExposureState = "practicing"
	ExposureMastered   ExposureState = "mastered"
	ExposureReviewDue  ExposureState = "review_due"
)

func (state ExposureState) Valid() bool {
	switch state {
	case ExposureNotSeen, ExposureIntroduced, ExposureLearning, ExposurePracticing, ExposureMastered, ExposureReviewDue:
		return true
	default:
		return false
	}
}

// ConceptState is the student's current state for one stable concept.
type ConceptState struct {
	StudentID    ID
	ConceptID    ID
	Exposure     ExposureState
	Mastery      MasteryScore
	IntroducedAt *Timestamp
	UpdatedAt    Timestamp
}

func (state ConceptState) Validate() error {
	if err := state.StudentID.Validate(); err != nil {
		return fmt.Errorf("concept state student: %w", err)
	}
	if err := state.ConceptID.Validate(); err != nil {
		return fmt.Errorf("concept state concept: %w", err)
	}
	if !state.Exposure.Valid() {
		return fmt.Errorf("exposure state %q is invalid", state.Exposure)
	}
	if err := state.Mastery.Validate(); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("concept introduced at", state.IntroducedAt); err != nil {
		return err
	}
	if err := state.UpdatedAt.Validate(); err != nil {
		return fmt.Errorf("concept state updated at: %w", err)
	}
	if state.Exposure != ExposureNotSeen && state.IntroducedAt == nil {
		return fmt.Errorf("introduced timestamp is required for exposure state %q", state.Exposure)
	}
	if state.Exposure == ExposureNotSeen && state.IntroducedAt != nil {
		return fmt.Errorf("not-seen concept cannot have an introduced timestamp")
	}
	if state.IntroducedAt != nil && state.UpdatedAt.Before(*state.IntroducedAt) {
		return fmt.Errorf("concept state update precedes introduction")
	}
	return nil
}

type EvidenceType string

const (
	EvidenceDiagnostic  EvidenceType = "diagnostic"
	EvidencePractice    EvidenceType = "practice"
	EvidenceAssessment  EvidenceType = "assessment"
	EvidenceReview      EvidenceType = "review"
	EvidenceObservation EvidenceType = "observation"
	EvidenceImport      EvidenceType = "import"
)

func (evidenceType EvidenceType) Valid() bool {
	switch evidenceType {
	case EvidenceDiagnostic, EvidencePractice, EvidenceAssessment, EvidenceReview, EvidenceObservation, EvidenceImport:
		return true
	default:
		return false
	}
}

// Evidence is an immutable observation that a future mastery policy may weigh.
// Score records the observation only; it does not directly become mastery.
type Evidence struct {
	ID         ID
	StudentID  ID
	ConceptID  ID
	Type       EvidenceType
	Source     string
	Score      MasteryScore
	ObservedAt Timestamp
}

func NewEvidence(id, studentID, conceptID ID, evidenceType EvidenceType, source string, score MasteryScore, observedAt Timestamp) (Evidence, error) {
	evidence := Evidence{
		ID: id, StudentID: studentID, ConceptID: conceptID, Type: evidenceType,
		Source: source, Score: score, ObservedAt: observedAt,
	}
	return evidence, evidence.Validate()
}

func (evidence Evidence) Validate() error {
	if err := evidence.ID.Validate(); err != nil {
		return fmt.Errorf("evidence: %w", err)
	}
	if err := evidence.StudentID.Validate(); err != nil {
		return fmt.Errorf("evidence student: %w", err)
	}
	if err := evidence.ConceptID.Validate(); err != nil {
		return fmt.Errorf("evidence concept: %w", err)
	}
	if !evidence.Type.Valid() {
		return fmt.Errorf("evidence type %q is invalid", evidence.Type)
	}
	if err := requireText("evidence source", evidence.Source); err != nil {
		return err
	}
	if err := evidence.Score.Validate(); err != nil {
		return fmt.Errorf("evidence: %w", err)
	}
	if err := evidence.ObservedAt.Validate(); err != nil {
		return fmt.Errorf("evidence observed at: %w", err)
	}
	return nil
}

// Mistake is a remembered misconception or error associated with one concept.
type Mistake struct {
	ID          ID
	StudentID   ID
	ConceptID   ID
	Description string
	OccurredAt  Timestamp
	ResolvedAt  *Timestamp
}

func NewMistake(id, studentID, conceptID ID, description string, occurredAt Timestamp) (Mistake, error) {
	mistake := Mistake{
		ID: id, StudentID: studentID, ConceptID: conceptID,
		Description: description, OccurredAt: occurredAt,
	}
	return mistake, mistake.Validate()
}

func (mistake Mistake) Validate() error {
	if err := mistake.ID.Validate(); err != nil {
		return fmt.Errorf("mistake: %w", err)
	}
	if err := mistake.StudentID.Validate(); err != nil {
		return fmt.Errorf("mistake student: %w", err)
	}
	if err := mistake.ConceptID.Validate(); err != nil {
		return fmt.Errorf("mistake concept: %w", err)
	}
	if err := requireText("mistake description", mistake.Description); err != nil {
		return err
	}
	if err := mistake.OccurredAt.Validate(); err != nil {
		return fmt.Errorf("mistake occurred at: %w", err)
	}
	if err := validateOptionalTimestamp("mistake resolved at", mistake.ResolvedAt); err != nil {
		return err
	}
	if mistake.ResolvedAt != nil && mistake.ResolvedAt.Before(mistake.OccurredAt) {
		return fmt.Errorf("mistake resolution precedes occurrence")
	}
	return nil
}

// ValidateMistakeConcept enforces aggregate-level membership without coupling a
// Mistake to a database or a curriculum repository.
func ValidateMistakeConcept(mistake Mistake, knownConcepts []Concept) error {
	if err := mistake.Validate(); err != nil {
		return err
	}
	for _, concept := range knownConcepts {
		if err := concept.Validate(); err != nil {
			return fmt.Errorf("known concept: %w", err)
		}
		if concept.ID == mistake.ConceptID {
			return nil
		}
	}
	return fmt.Errorf("mistake concept %q is unknown", mistake.ConceptID)
}
