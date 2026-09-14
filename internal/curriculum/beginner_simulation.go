package curriculum

import "fmt"

const BeginnerSimulationVersionV1 = "beginner-simulation-v1"

type BeginnerGapKind string

const (
	BeginnerGapPrerequisite BeginnerGapKind = "prerequisite_not_introduced"
	BeginnerGapVocabulary   BeginnerGapKind = "vocabulary_not_introduced"
	BeginnerGapTool         BeginnerGapKind = "tool_not_introduced"
	BeginnerGapAssumption   BeginnerGapKind = "assumption_not_resolved"
)

func (kind BeginnerGapKind) Validate() error {
	switch kind {
	case BeginnerGapPrerequisite, BeginnerGapVocabulary, BeginnerGapTool, BeginnerGapAssumption:
		return nil
	default:
		return fmt.Errorf("invalid beginner gap kind %q", kind)
	}
}

// BeginnerToolUse makes a tool need explicit at the concept where a novice
// first needs it. Tool installation and execution remain outside I-04.
type BeginnerToolUse struct {
	ToolID ID
	UsedAt ConceptID
}

func (use BeginnerToolUse) Validate() error {
	if err := use.ToolID.Validate(); err != nil {
		return fmt.Errorf("beginner tool use: %w", err)
	}
	if err := use.UsedAt.Validate(); err != nil {
		return fmt.Errorf("beginner tool use concept: %w", err)
	}
	return nil
}

// BeginnerAssumption declares knowledge that a concept needs beyond explicit
// graph, vocabulary, and tool requirements. SatisfiedBy is nil when the
// curriculum has no concept that resolves the assumption.
type BeginnerAssumption struct {
	ID          ID
	RequiredAt  ConceptID
	SatisfiedBy *ConceptID
	Reason      string
}

func (assumption BeginnerAssumption) Validate() error {
	if err := assumption.ID.Validate(); err != nil {
		return fmt.Errorf("beginner assumption: %w", err)
	}
	if err := assumption.RequiredAt.Validate(); err != nil {
		return fmt.Errorf("beginner assumption requirement: %w", err)
	}
	if assumption.SatisfiedBy != nil {
		if err := assumption.SatisfiedBy.Validate(); err != nil {
			return fmt.Errorf("beginner assumption resolution: %w", err)
		}
	}
	return requireText("beginner assumption reason", assumption.Reason)
}

type BeginnerGap struct {
	Kind        BeginnerGapKind
	ConceptID   ConceptID
	Requirement string
	Reason      string
}

func (gap BeginnerGap) Validate() error {
	if err := gap.Kind.Validate(); err != nil {
		return err
	}
	if err := gap.ConceptID.Validate(); err != nil {
		return fmt.Errorf("beginner gap concept: %w", err)
	}
	if err := requireText("beginner gap requirement", gap.Requirement); err != nil {
		return err
	}
	return requireText("beginner gap reason", gap.Reason)
}

type BeginnerSimulationStep struct {
	ConceptID             ConceptID
	TopicID               ID
	IntroducedVocabulary  []string
	IntroducedToolIDs     []ID
	ResolvedAssumptionIDs []ID
}

func (step BeginnerSimulationStep) Validate() error {
	if err := step.ConceptID.Validate(); err != nil {
		return fmt.Errorf("beginner simulation step concept: %w", err)
	}
	if err := step.TopicID.Validate(); err != nil {
		return fmt.Errorf("beginner simulation step topic: %w", err)
	}
	if err := validateUniqueTexts("introduced vocabulary", step.IntroducedVocabulary); err != nil {
		return err
	}
	if err := validateIDs("introduced tools", step.IntroducedToolIDs); err != nil {
		return err
	}
	return validateIDs("resolved assumptions", step.ResolvedAssumptionIDs)
}

type BeginnerSimulationResult struct {
	Passed                bool
	Steps                 []BeginnerSimulationStep
	Gaps                  []BeginnerGap
	IntroducedConceptIDs  []ConceptID
	IntroducedVocabulary  []string
	IntroducedToolIDs     []ID
	ResolvedAssumptionIDs []ID
	AlgorithmVersion      string
}

func (result BeginnerSimulationResult) Validate() error {
	if result.AlgorithmVersion != BeginnerSimulationVersionV1 {
		return fmt.Errorf("unsupported beginner simulation version %q", result.AlgorithmVersion)
	}
	if result.Passed != (len(result.Gaps) == 0) {
		return fmt.Errorf("beginner simulation pass state does not match gaps")
	}
	if len(result.Steps) == 0 || len(result.Steps) != len(result.IntroducedConceptIDs) {
		return fmt.Errorf("beginner simulation must visit every introduced concept exactly once")
	}
	if err := validateConceptIDs("beginner introduced concepts", result.IntroducedConceptIDs); err != nil {
		return err
	}
	seenConcepts := make(map[ConceptID]struct{}, len(result.Steps))
	for index, step := range result.Steps {
		if err := step.Validate(); err != nil {
			return err
		}
		if step.ConceptID != result.IntroducedConceptIDs[index] {
			return fmt.Errorf("beginner simulation step order does not match introduced concepts")
		}
		if _, exists := seenConcepts[step.ConceptID]; exists {
			return fmt.Errorf("beginner simulation repeats concept %q", step.ConceptID)
		}
		seenConcepts[step.ConceptID] = struct{}{}
	}
	seenGaps := make(map[string]struct{}, len(result.Gaps))
	for _, gap := range result.Gaps {
		if err := gap.Validate(); err != nil {
			return err
		}
		key := string(gap.Kind) + "\x00" + gap.ConceptID.String() + "\x00" + gap.Requirement
		if _, exists := seenGaps[key]; exists {
			return fmt.Errorf("duplicate beginner gap %q", gap.Requirement)
		}
		seenGaps[key] = struct{}{}
	}
	if err := validateUniqueTexts("beginner introduced vocabulary", result.IntroducedVocabulary); err != nil {
		return err
	}
	if err := validateIDs("beginner introduced tools", result.IntroducedToolIDs); err != nil {
		return err
	}
	return validateIDs("beginner resolved assumptions", result.ResolvedAssumptionIDs)
}
