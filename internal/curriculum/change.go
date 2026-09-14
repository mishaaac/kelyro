package curriculum

import "fmt"

const CurriculumChangeClassifierVersionV1 = "curriculum-change-classifier-v1"

type CurriculumChangeKind string

const (
	ChangeMetadataOnly        CurriculumChangeKind = "metadata_only"
	ChangeSourceRefresh       CurriculumChangeKind = "source_refresh"
	ChangeConceptAdded        CurriculumChangeKind = "concept_added"
	ChangeConceptRemoved      CurriculumChangeKind = "concept_removed"
	ChangeConceptSplit        CurriculumChangeKind = "concept_split"
	ChangeConceptMerged       CurriculumChangeKind = "concept_merged"
	ChangePrerequisiteChanged CurriculumChangeKind = "prerequisite_changed"
	ChangeHierarchyChanged    CurriculumChangeKind = "hierarchy_changed"
	ChangeStatusChanged       CurriculumChangeKind = "status_changed"
	ChangeEnvironmentChanged  CurriculumChangeKind = "environment_changed"
)

func (kind CurriculumChangeKind) Validate() error {
	switch kind {
	case ChangeMetadataOnly, ChangeSourceRefresh, ChangeConceptAdded,
		ChangeConceptRemoved, ChangeConceptSplit, ChangeConceptMerged,
		ChangePrerequisiteChanged, ChangeHierarchyChanged, ChangeStatusChanged,
		ChangeEnvironmentChanged:
		return nil
	default:
		return fmt.Errorf("invalid curriculum change kind %q", kind)
	}
}

type MigrationClass string

const (
	MigrationSafe                  MigrationClass = "safe"
	MigrationRequiresRecompile     MigrationClass = "requires_recompile"
	MigrationRequiresStudentReview MigrationClass = "requires_student_review"
	MigrationBreaking              MigrationClass = "breaking"
)

func (class MigrationClass) Validate() error {
	switch class {
	case MigrationSafe, MigrationRequiresRecompile, MigrationRequiresStudentReview, MigrationBreaking:
		return nil
	default:
		return fmt.Errorf("invalid migration class %q", class)
	}
}

type CurriculumChange struct {
	ID               ID
	FromVersion      CurriculumVersion
	ToVersion        CurriculumVersion
	Kind             CurriculumChangeKind
	Migration        MigrationClass
	AffectedConcepts []ConceptID
	Rationale        string
}

// ConceptIdentityMapping is explicit author/reviewer evidence that removed and
// added Concept IDs represent a split or merge. The classifier never infers
// identity continuity from similar text.
type ConceptIdentityMapping struct {
	OldConceptIDs []ConceptID
	NewConceptIDs []ConceptID
	Rationale     string
}

func (mapping ConceptIdentityMapping) Validate() error {
	if err := validateConceptIDs("old concept mapping", mapping.OldConceptIDs); err != nil {
		return err
	}
	if err := validateConceptIDs("new concept mapping", mapping.NewConceptIDs); err != nil {
		return err
	}
	if !((len(mapping.OldConceptIDs) == 1 && len(mapping.NewConceptIDs) > 1) ||
		(len(mapping.OldConceptIDs) > 1 && len(mapping.NewConceptIDs) == 1)) {
		return fmt.Errorf("concept identity mapping must describe one split or one merge")
	}
	return requireText("concept identity mapping rationale", mapping.Rationale)
}

type CurriculumChangeClassification struct {
	FromVersion      CurriculumVersion
	ToVersion        CurriculumVersion
	Changes          []CurriculumChange
	AlgorithmVersion string
}

func (classification CurriculumChangeClassification) Validate() error {
	if err := classification.FromVersion.Validate(); err != nil {
		return err
	}
	if err := classification.ToVersion.Validate(); err != nil {
		return err
	}
	if classification.FromVersion == classification.ToVersion {
		return fmt.Errorf("curriculum change classification versions are identical")
	}
	if len(classification.Changes) == 0 {
		return fmt.Errorf("curriculum change classification is empty")
	}
	seenIDs := make(map[ID]struct{}, len(classification.Changes))
	seenKinds := make(map[CurriculumChangeKind]struct{}, len(classification.Changes))
	for _, change := range classification.Changes {
		if err := change.Validate(); err != nil {
			return err
		}
		if change.FromVersion != classification.FromVersion || change.ToVersion != classification.ToVersion {
			return fmt.Errorf("curriculum change versions do not match classification")
		}
		if _, exists := seenIDs[change.ID]; exists {
			return fmt.Errorf("duplicate curriculum change %q", change.ID)
		}
		if _, exists := seenKinds[change.Kind]; exists {
			return fmt.Errorf("duplicate curriculum change kind %q", change.Kind)
		}
		seenIDs[change.ID] = struct{}{}
		seenKinds[change.Kind] = struct{}{}
	}
	if classification.AlgorithmVersion != CurriculumChangeClassifierVersionV1 {
		return fmt.Errorf("unsupported curriculum change classifier version %q", classification.AlgorithmVersion)
	}
	return nil
}

func (change CurriculumChange) Validate() error {
	if err := change.ID.Validate(); err != nil {
		return fmt.Errorf("curriculum change: %w", err)
	}
	if err := change.FromVersion.Validate(); err != nil {
		return err
	}
	if err := change.ToVersion.Validate(); err != nil {
		return err
	}
	if change.FromVersion == change.ToVersion {
		return fmt.Errorf("curriculum change versions are identical")
	}
	if err := change.Kind.Validate(); err != nil {
		return err
	}
	if err := change.Migration.Validate(); err != nil {
		return err
	}
	if err := validateConceptIDs("curriculum change concepts", change.AffectedConcepts); err != nil {
		return err
	}
	return requireText("curriculum change rationale", change.Rationale)
}
