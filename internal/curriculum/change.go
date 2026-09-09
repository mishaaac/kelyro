package curriculum

import "fmt"

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
