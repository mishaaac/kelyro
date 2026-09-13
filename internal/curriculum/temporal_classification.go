package curriculum

import "fmt"

const TemporalClassificationVersionV1 = "temporal-classification-v1"

type TemporalTargetKind string

const (
	TemporalTargetConcept TemporalTargetKind = "concept"
	TemporalTargetLesson  TemporalTargetKind = "lesson"
)

func (kind TemporalTargetKind) Validate() error {
	switch kind {
	case TemporalTargetConcept, TemporalTargetLesson:
		return nil
	default:
		return fmt.Errorf("invalid temporal target kind %q", kind)
	}
}

// LessonTemporalInput supplies the evidence and Concept membership needed to
// classify a lesson before the hierarchy builder exists.
type LessonTemporalInput struct {
	Lesson         LessonSpec
	ConceptIDs     []ConceptID
	DeclaredStatus ConceptStatus
	ContextualUse  bool
	EvidenceRefs   []EvidenceRef
}

func (input LessonTemporalInput) Validate() error {
	if err := input.Lesson.Validate(); err != nil {
		return err
	}
	if err := input.DeclaredStatus.Validate(); err != nil {
		return err
	}
	if err := validateConceptIDs("lesson temporal concepts", input.ConceptIDs); err != nil {
		return err
	}
	if err := validateEvidenceRefs("lesson temporal evidence", input.EvidenceRefs); err != nil {
		return err
	}
	if len(input.ConceptIDs) == 0 && len(input.EvidenceRefs) == 0 {
		return fmt.Errorf("lesson %q has no temporal evidence or concepts", input.Lesson.ID)
	}
	return nil
}

// TemporalClassification is UI-ready metadata. It does not rewrite an
// immutable Concept or LessonSpec.
type TemporalClassification struct {
	TargetKind            TemporalTargetKind
	TargetID              ID
	DeclaredStatus        ConceptStatus
	Status                ConceptStatus
	PrimaryRecommendation bool
	Separated             bool
	ContextOnly           bool
	ContextualUse         bool
	EvidenceRefs          []EvidenceRef
	Reasons               []string
}

func (classification TemporalClassification) Validate() error {
	if err := classification.TargetKind.Validate(); err != nil {
		return err
	}
	if err := classification.TargetID.Validate(); err != nil {
		return fmt.Errorf("temporal target: %w", err)
	}
	if err := classification.DeclaredStatus.Validate(); err != nil {
		return err
	}
	if err := classification.Status.Validate(); err != nil {
		return err
	}
	if err := validateEvidenceRefs("temporal classification evidence", classification.EvidenceRefs); err != nil {
		return err
	}
	if len(classification.EvidenceRefs) == 0 {
		return fmt.Errorf("temporal classification %q has no evidence", classification.TargetID)
	}
	if err := validateUniqueTexts("temporal classification reasons", classification.Reasons); err != nil {
		return err
	}
	if len(classification.Reasons) == 0 {
		return fmt.Errorf("temporal classification %q has no reasons", classification.TargetID)
	}
	switch classification.Status {
	case ConceptCurrent:
		if !classification.PrimaryRecommendation || classification.Separated || classification.ContextOnly {
			return fmt.Errorf("current classification %q has inconsistent presentation metadata", classification.TargetID)
		}
	case ConceptPreview, ConceptExperimental:
		if classification.PrimaryRecommendation || !classification.Separated || classification.ContextOnly {
			return fmt.Errorf("experimental classification %q has inconsistent presentation metadata", classification.TargetID)
		}
	case ConceptLegacy, ConceptHistorical:
		if classification.PrimaryRecommendation || classification.Separated || !classification.ContextOnly {
			return fmt.Errorf("context-only classification %q has inconsistent presentation metadata", classification.TargetID)
		}
	case ConceptDeprecated:
		if classification.PrimaryRecommendation || classification.Separated || classification.ContextOnly {
			return fmt.Errorf("deprecated classification %q has inconsistent presentation metadata", classification.TargetID)
		}
	}
	return nil
}

type TemporalClassificationResult struct {
	Classifications  []TemporalClassification
	AlgorithmVersion string
}

func (result TemporalClassificationResult) Validate() error {
	if len(result.Classifications) == 0 {
		return fmt.Errorf("temporal classification result is empty")
	}
	seen := make(map[string]struct{}, len(result.Classifications))
	for _, classification := range result.Classifications {
		if err := classification.Validate(); err != nil {
			return err
		}
		key := string(classification.TargetKind) + "\x00" + classification.TargetID.String()
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate temporal classification for %s %q", classification.TargetKind, classification.TargetID)
		}
		seen[key] = struct{}{}
	}
	if result.AlgorithmVersion != TemporalClassificationVersionV1 {
		return fmt.Errorf("unsupported temporal classification version %q", result.AlgorithmVersion)
	}
	return nil
}
