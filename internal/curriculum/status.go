package curriculum

import "fmt"

// ConceptStatus preserves temporal guidance from I-03 without presenting old
// or experimental guidance as a current recommendation.
type ConceptStatus string

const (
	ConceptCurrent      ConceptStatus = "current"
	ConceptExperimental ConceptStatus = "experimental"
	ConceptPreview      ConceptStatus = "preview"
	ConceptLegacy       ConceptStatus = "legacy"
	ConceptHistorical   ConceptStatus = "historical"
	ConceptDeprecated   ConceptStatus = "deprecated"
)

func (status ConceptStatus) Validate() error {
	switch status {
	case ConceptCurrent, ConceptExperimental, ConceptPreview, ConceptLegacy, ConceptHistorical, ConceptDeprecated:
		return nil
	default:
		return fmt.Errorf("invalid concept status %q", status)
	}
}

// PackStatus shares the same temporal vocabulary while keeping pack and
// concept identities independent.
type PackStatus = ConceptStatus

type Atomicity string

const (
	AtomicityAtomic        Atomicity = "atomic"
	AtomicityNeedsSplit    Atomicity = "needs_split"
	AtomicityTooFragmented Atomicity = "too_fragmented"
	AtomicityUnknown       Atomicity = "unknown"
)

func (atomicity Atomicity) Validate() error {
	switch atomicity {
	case AtomicityAtomic, AtomicityNeedsSplit, AtomicityTooFragmented, AtomicityUnknown:
		return nil
	default:
		return fmt.Errorf("invalid atomicity %q", atomicity)
	}
}

type Difficulty int

const (
	DifficultyIntroductory Difficulty = 1
	DifficultyFoundational Difficulty = 2
	DifficultyIntermediate Difficulty = 3
	DifficultyAdvanced     Difficulty = 4
	DifficultyExpert       Difficulty = 5
)

func (difficulty Difficulty) Validate() error {
	if difficulty < DifficultyIntroductory || difficulty > DifficultyExpert {
		return fmt.Errorf("difficulty %d is outside 1..5", difficulty)
	}
	return nil
}

type CompetencyLevel string

const (
	CompetencyAwareness  CompetencyLevel = "awareness"
	CompetencyUnderstand CompetencyLevel = "understand"
	CompetencyApply      CompetencyLevel = "apply"
	CompetencyAnalyze    CompetencyLevel = "analyze"
	CompetencyDesign     CompetencyLevel = "design"
	CompetencyOperate    CompetencyLevel = "operate"
	CompetencyExplain    CompetencyLevel = "teach_explain"
)

func (level CompetencyLevel) Validate() error {
	switch level {
	case CompetencyAwareness, CompetencyUnderstand, CompetencyApply, CompetencyAnalyze,
		CompetencyDesign, CompetencyOperate, CompetencyExplain:
		return nil
	default:
		return fmt.Errorf("invalid competency level %q", level)
	}
}

type PrerequisiteKind string

const (
	PrerequisiteHard           PrerequisiteKind = "hard"
	PrerequisiteRecommended    PrerequisiteKind = "recommended"
	PrerequisiteExposureOnly   PrerequisiteKind = "exposure_only"
	PrerequisiteToolDependency PrerequisiteKind = "tool_dependency"
	PrerequisiteVocabulary     PrerequisiteKind = "vocabulary"
)

func (kind PrerequisiteKind) Validate() error {
	switch kind {
	case PrerequisiteHard, PrerequisiteRecommended, PrerequisiteExposureOnly,
		PrerequisiteToolDependency, PrerequisiteVocabulary:
		return nil
	default:
		return fmt.Errorf("invalid prerequisite kind %q", kind)
	}
}

type SourceReferencePolicy string

const (
	SourceReferencesRequired           SourceReferencePolicy = "required"
	SourceReferencesOptionalForFixture SourceReferencePolicy = "optional_for_fixture"
)

func (policy SourceReferencePolicy) Validate() error {
	switch policy {
	case SourceReferencesRequired, SourceReferencesOptionalForFixture:
		return nil
	default:
		return fmt.Errorf("invalid source reference policy %q", policy)
	}
}
