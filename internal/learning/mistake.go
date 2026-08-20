package learning

import (
	"fmt"
	"strings"
)

const (
	MaxMistakeKeyLength     = 128
	MaxMistakeSummaryLength = 500
	MaxMistakeSourceLength  = 256
)

// MistakeKey is an evaluator-owned stable classification key. It is scoped by
// student and concept, so unrelated concepts may use the same key safely.
type MistakeKey string

func (key MistakeKey) Validate() error {
	value := string(key)
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("mistake key is empty")
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("mistake key has surrounding whitespace")
	}
	if len(value) > MaxMistakeKeyLength {
		return fmt.Errorf("mistake key exceeds %d bytes", MaxMistakeKeyLength)
	}
	return nil
}

type MistakeCategory string

const (
	MistakeConceptual    MistakeCategory = "conceptual"
	MistakeSyntax        MistakeCategory = "syntax"
	MistakeProcedure     MistakeCategory = "procedure"
	MistakeMisconception MistakeCategory = "misconception"
	MistakeCareless      MistakeCategory = "careless"
	MistakeTooling       MistakeCategory = "tooling"
	MistakeUnknown       MistakeCategory = "unknown"
)

func (category MistakeCategory) Valid() bool {
	switch category {
	case MistakeConceptual, MistakeSyntax, MistakeProcedure, MistakeMisconception, MistakeCareless, MistakeTooling, MistakeUnknown:
		return true
	default:
		return false
	}
}

type MistakeStatus string

const (
	MistakeRecent     MistakeStatus = "recent"
	MistakeReinforced MistakeStatus = "reinforced"
	MistakeResolved   MistakeStatus = "resolved"
)

func (status MistakeStatus) Valid() bool {
	return status == MistakeRecent || status == MistakeReinforced || status == MistakeResolved
}

type MistakeEventType string

const (
	MistakeObservedEvent   MistakeEventType = "observed"
	MistakeReinforcedEvent MistakeEventType = "reinforced"
	MistakeResolvedEvent   MistakeEventType = "resolved"
)

func (eventType MistakeEventType) Valid() bool {
	return eventType == MistakeObservedEvent || eventType == MistakeReinforcedEvent || eventType == MistakeResolvedEvent
}

// Mistake is the current projection of one durable error pattern. Lifecycle
// events remain immutable and provide the complete history behind this view.
type Mistake struct {
	ID          ID
	StudentID   ID
	ConceptID   ID
	Key         MistakeKey
	Category    MistakeCategory
	Summary     string
	FirstSeenAt Timestamp
	LastSeenAt  Timestamp
	Occurrences int
	Status      MistakeStatus
	SourceRef   string
	ResolvedAt  *Timestamp
}

func NewMistake(id, studentID, conceptID ID, key MistakeKey, category MistakeCategory, summary string, observedAt Timestamp, sourceRef string) (Mistake, error) {
	mistake := Mistake{
		ID: id, StudentID: studentID, ConceptID: conceptID, Key: key,
		Category: category, Summary: summary, FirstSeenAt: observedAt,
		LastSeenAt: observedAt, Occurrences: 1, Status: MistakeRecent,
		SourceRef: sourceRef,
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
	if err := mistake.Key.Validate(); err != nil {
		return err
	}
	if !mistake.Category.Valid() {
		return fmt.Errorf("mistake category %q is invalid", mistake.Category)
	}
	if err := validateBoundedMistakeText("mistake summary", mistake.Summary, MaxMistakeSummaryLength); err != nil {
		return err
	}
	if err := mistake.FirstSeenAt.Validate(); err != nil {
		return fmt.Errorf("mistake first seen at: %w", err)
	}
	if err := mistake.LastSeenAt.Validate(); err != nil {
		return fmt.Errorf("mistake last seen at: %w", err)
	}
	if mistake.LastSeenAt.Before(mistake.FirstSeenAt) {
		return fmt.Errorf("mistake last seen precedes first seen")
	}
	if mistake.Occurrences < 1 {
		return fmt.Errorf("mistake occurrences must be positive")
	}
	if !mistake.Status.Valid() {
		return fmt.Errorf("mistake status %q is invalid", mistake.Status)
	}
	if err := validateBoundedMistakeText("mistake source reference", mistake.SourceRef, MaxMistakeSourceLength); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("mistake resolved at", mistake.ResolvedAt); err != nil {
		return err
	}
	if mistake.Status == MistakeResolved {
		if mistake.ResolvedAt == nil {
			return fmt.Errorf("resolved mistake has no resolution timestamp")
		}
		if mistake.ResolvedAt.Before(mistake.LastSeenAt) {
			return fmt.Errorf("mistake resolution precedes last occurrence")
		}
	} else if mistake.ResolvedAt != nil {
		return fmt.Errorf("unresolved mistake has a resolution timestamp")
	}
	return nil
}

// Observe increments the durable pattern and reopens a resolved mistake.
func (mistake Mistake) Observe(observedAt Timestamp, sourceRef string) (Mistake, error) {
	if err := mistake.Validate(); err != nil {
		return Mistake{}, err
	}
	if err := observedAt.Validate(); err != nil {
		return Mistake{}, fmt.Errorf("mistake observed at: %w", err)
	}
	if observedAt.Before(mistake.LastSeenAt) || (mistake.ResolvedAt != nil && observedAt.Before(*mistake.ResolvedAt)) {
		return Mistake{}, fmt.Errorf("mistake recurrence precedes current history")
	}
	if err := validateBoundedMistakeText("mistake source reference", sourceRef, MaxMistakeSourceLength); err != nil {
		return Mistake{}, err
	}
	mistake.LastSeenAt = observedAt
	mistake.Occurrences++
	mistake.Status = MistakeRecent
	mistake.SourceRef = sourceRef
	mistake.ResolvedAt = nil
	return mistake, mistake.Validate()
}

func (mistake Mistake) Reinforce(at Timestamp) (Mistake, error) {
	if err := mistake.validateTransitionTime("reinforcement", at); err != nil {
		return Mistake{}, err
	}
	if mistake.Status == MistakeResolved {
		return Mistake{}, fmt.Errorf("resolved mistake cannot be reinforced without a recurrence")
	}
	mistake.Status = MistakeReinforced
	mistake.ResolvedAt = nil
	return mistake, mistake.Validate()
}

func (mistake Mistake) Resolve(at Timestamp) (Mistake, error) {
	if err := mistake.validateTransitionTime("resolution", at); err != nil {
		return Mistake{}, err
	}
	if mistake.Status == MistakeResolved {
		return Mistake{}, fmt.Errorf("mistake is already resolved")
	}
	mistake.Status = MistakeResolved
	mistake.ResolvedAt = &at
	return mistake, mistake.Validate()
}

func (mistake Mistake) validateTransitionTime(name string, at Timestamp) error {
	if err := mistake.Validate(); err != nil {
		return err
	}
	if err := at.Validate(); err != nil {
		return fmt.Errorf("mistake %s: %w", name, err)
	}
	if at.Before(mistake.LastSeenAt) || (mistake.ResolvedAt != nil && at.Before(*mistake.ResolvedAt)) {
		return fmt.Errorf("mistake %s precedes current history", name)
	}
	return nil
}

type MistakeEvent struct {
	ID         ID
	MistakeID  ID
	Type       MistakeEventType
	OccurredAt Timestamp
	SourceRef  string
}

func NewMistakeEvent(id, mistakeID ID, eventType MistakeEventType, occurredAt Timestamp, sourceRef string) (MistakeEvent, error) {
	event := MistakeEvent{ID: id, MistakeID: mistakeID, Type: eventType, OccurredAt: occurredAt, SourceRef: sourceRef}
	return event, event.Validate()
}

func (event MistakeEvent) Validate() error {
	if err := event.ID.Validate(); err != nil {
		return fmt.Errorf("mistake event: %w", err)
	}
	if err := event.MistakeID.Validate(); err != nil {
		return fmt.Errorf("mistake event owner: %w", err)
	}
	if !event.Type.Valid() {
		return fmt.Errorf("mistake event type %q is invalid", event.Type)
	}
	if err := event.OccurredAt.Validate(); err != nil {
		return fmt.Errorf("mistake event occurred at: %w", err)
	}
	return validateBoundedMistakeText("mistake event source reference", event.SourceRef, MaxMistakeSourceLength)
}

// ValidateMistakeConcept enforces aggregate-level membership without coupling
// Mistake to persistence or a particular curriculum representation.
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

func validateBoundedMistakeText(name, value string, limit int) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is empty", name)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("%s has surrounding whitespace", name)
	}
	if len(value) > limit {
		return fmt.Errorf("%s exceeds %d bytes", name, limit)
	}
	return nil
}
