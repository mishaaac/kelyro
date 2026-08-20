package learning

import (
	"fmt"
	"math"
)

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
	EvidenceDiagnosticObjective  EvidenceType = "diagnostic_objective"
	EvidenceDiagnosticSelfReport EvidenceType = "diagnostic_self_report"
	EvidenceKnowledgeCheck       EvidenceType = "knowledge_check"
	EvidencePracticeSuccess      EvidenceType = "practice_success"
	EvidencePracticeFailure      EvidenceType = "practice_failure"
	EvidenceAssessment           EvidenceType = "assessment"
	EvidenceProject              EvidenceType = "project_evidence"
	EvidenceReviewRecall         EvidenceType = "review_recall"
	EvidenceManualImport         EvidenceType = "manual_import"

	// Compatibility aliases retain the domain vocabulary published before the
	// evidence model acquired mastery-specific metadata.
	EvidenceDiagnostic  = EvidenceDiagnosticObjective
	EvidencePractice    = EvidencePracticeSuccess
	EvidenceReview      = EvidenceReviewRecall
	EvidenceObservation = EvidenceProject
	EvidenceImport      = EvidenceManualImport
)

func (evidenceType EvidenceType) Valid() bool {
	switch evidenceType {
	case EvidenceDiagnosticObjective, EvidenceDiagnosticSelfReport, EvidenceKnowledgeCheck,
		EvidencePracticeSuccess, EvidencePracticeFailure, EvidenceAssessment,
		EvidenceProject, EvidenceReviewRecall, EvidenceManualImport:
		return true
	default:
		return false
	}
}

const LegacyEvidenceAlgorithmVersion = "legacy-evidence/v1"

// EvidenceMetadata describes how strongly an immutable observation should be
// trusted. All numeric values use [0,1]; Difficulty is normalized rather than
// tied to a particular subject's scale.
type EvidenceMetadata struct {
	Confidence       float64
	Independence     float64
	Difficulty       float64
	AlgorithmVersion string
}

func DefaultEvidenceMetadata() EvidenceMetadata {
	return EvidenceMetadata{Confidence: 1, Independence: 1, Difficulty: .5, AlgorithmVersion: LegacyEvidenceAlgorithmVersion}
}

func (metadata EvidenceMetadata) Validate() error {
	for _, field := range []struct {
		name  string
		value float64
	}{
		{name: "confidence", value: metadata.Confidence},
		{name: "independence", value: metadata.Independence},
		{name: "difficulty", value: metadata.Difficulty},
	} {
		if math.IsNaN(field.value) || math.IsInf(field.value, 0) || field.value < 0 || field.value > 1 {
			return fmt.Errorf("evidence %s must be between 0 and 1", field.name)
		}
	}
	if metadata.Confidence == 0 {
		return fmt.Errorf("evidence confidence must be greater than 0")
	}
	if err := requireText("evidence algorithm version", metadata.AlgorithmVersion); err != nil {
		return err
	}
	return nil
}

// Evidence is an immutable, source-traceable observation. Score is the
// observed outcome and never directly becomes the learner's mastery.
type Evidence struct {
	ID               ID
	StudentID        ID
	ConceptID        ID
	Type             EvidenceType
	Source           string
	Score            MasteryScore
	Confidence       float64
	Independence     float64
	Difficulty       float64
	ObservedAt       Timestamp
	AlgorithmVersion string
}

func NewEvidence(id, studentID, conceptID ID, evidenceType EvidenceType, source string, score MasteryScore, observedAt Timestamp) (Evidence, error) {
	return NewEvidenceWithMetadata(id, studentID, conceptID, evidenceType, source, score, DefaultEvidenceMetadata(), observedAt)
}

func NewEvidenceWithMetadata(id, studentID, conceptID ID, evidenceType EvidenceType, source string, score MasteryScore, metadata EvidenceMetadata, observedAt Timestamp) (Evidence, error) {
	evidence := Evidence{
		ID: id, StudentID: studentID, ConceptID: conceptID, Type: evidenceType,
		Source: source, Score: score, Confidence: metadata.Confidence,
		Independence: metadata.Independence, Difficulty: metadata.Difficulty,
		ObservedAt: observedAt, AlgorithmVersion: metadata.AlgorithmVersion,
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
	if err := (EvidenceMetadata{
		Confidence: evidence.Confidence, Independence: evidence.Independence,
		Difficulty: evidence.Difficulty, AlgorithmVersion: evidence.AlgorithmVersion,
	}).Validate(); err != nil {
		return err
	}
	if err := evidence.ObservedAt.Validate(); err != nil {
		return fmt.Errorf("evidence observed at: %w", err)
	}
	return nil
}
