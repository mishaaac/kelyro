package curriculum

import "fmt"

const CurriculumChangeClassifierVersionV1 = "curriculum-change-classifier-v1"

const CurriculumMigrationPlannerVersionV1 = "curriculum-migration-planner-v1"

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

// CurriculumMigrationActionKind describes how one group of stable Concept
// identities is handled. Split and merge actions deliberately do not transfer
// mastery; their mappings retain provenance only.
type CurriculumMigrationActionKind string

const (
	MigrationPreserveState      CurriculumMigrationActionKind = "preserve_state"
	MigrationInitializeUnknown  CurriculumMigrationActionKind = "initialize_unknown"
	MigrationPreserveHistorical CurriculumMigrationActionKind = "preserve_historical"
	MigrationSplitNoTransfer    CurriculumMigrationActionKind = "split_no_transfer"
	MigrationMergeNoTransfer    CurriculumMigrationActionKind = "merge_no_transfer"
)

func (kind CurriculumMigrationActionKind) Validate() error {
	switch kind {
	case MigrationPreserveState, MigrationInitializeUnknown, MigrationPreserveHistorical,
		MigrationSplitNoTransfer, MigrationMergeNoTransfer:
		return nil
	default:
		return fmt.Errorf("invalid curriculum migration action kind %q", kind)
	}
}

type CurriculumMigrationAction struct {
	Kind                       CurriculumMigrationActionKind
	FromConceptIDs             []ConceptID
	ToConceptIDs               []ConceptID
	PreserveMastery            bool
	PreserveEvidence           bool
	InitializeUnknown          bool
	PreserveHistoricalEvidence bool
	RequiresStudentReview      bool
	Rationale                  string
}

func (action CurriculumMigrationAction) Validate() error {
	if err := action.Kind.Validate(); err != nil {
		return err
	}
	if err := validateOptionalConceptIDs("migration source concepts", action.FromConceptIDs); err != nil {
		return err
	}
	if err := validateOptionalConceptIDs("migration target concepts", action.ToConceptIDs); err != nil {
		return err
	}
	if err := requireText("curriculum migration rationale", action.Rationale); err != nil {
		return err
	}
	switch action.Kind {
	case MigrationPreserveState:
		if len(action.FromConceptIDs) != 1 || len(action.ToConceptIDs) != 1 || action.FromConceptIDs[0] != action.ToConceptIDs[0] ||
			!action.PreserveMastery || !action.PreserveEvidence || action.InitializeUnknown {
			return fmt.Errorf("preserve-state action must retain one stable concept without initialization")
		}
	case MigrationInitializeUnknown:
		if len(action.FromConceptIDs) != 0 || len(action.ToConceptIDs) != 1 || action.PreserveMastery || action.PreserveEvidence ||
			!action.InitializeUnknown || action.PreserveHistoricalEvidence || action.RequiresStudentReview {
			return fmt.Errorf("initialize-unknown action must create one target concept without transferred state")
		}
	case MigrationPreserveHistorical:
		if len(action.FromConceptIDs) != 1 || len(action.ToConceptIDs) != 0 || action.PreserveMastery || action.PreserveEvidence ||
			action.InitializeUnknown || !action.PreserveHistoricalEvidence || !action.RequiresStudentReview {
			return fmt.Errorf("preserve-historical action must retain one removed concept as reviewed history")
		}
	case MigrationSplitNoTransfer:
		if len(action.FromConceptIDs) != 1 || len(action.ToConceptIDs) < 2 || action.PreserveMastery || action.PreserveEvidence ||
			!action.InitializeUnknown || !action.PreserveHistoricalEvidence || !action.RequiresStudentReview {
			return fmt.Errorf("split action must map one source to multiple unknown targets without mastery transfer")
		}
	case MigrationMergeNoTransfer:
		if len(action.FromConceptIDs) < 2 || len(action.ToConceptIDs) != 1 || action.PreserveMastery || action.PreserveEvidence ||
			!action.InitializeUnknown || !action.PreserveHistoricalEvidence || !action.RequiresStudentReview {
			return fmt.Errorf("merge action must map multiple sources to one unknown target without mastery transfer")
		}
	}
	return nil
}

type CurriculumMigrationPlan struct {
	ID                           ID
	CurriculumID                 CurriculumID
	FromVersion                  CurriculumVersion
	ToVersion                    CurriculumVersion
	Actions                      []CurriculumMigrationAction
	RecalculateUnlockEligibility bool
	RequiresStudentReview        bool
	AlgorithmVersion             string
}

func (plan CurriculumMigrationPlan) Validate() error {
	if err := plan.ID.Validate(); err != nil {
		return fmt.Errorf("curriculum migration plan: %w", err)
	}
	if err := plan.CurriculumID.Validate(); err != nil {
		return err
	}
	if err := plan.FromVersion.Validate(); err != nil {
		return err
	}
	if err := plan.ToVersion.Validate(); err != nil {
		return err
	}
	if plan.FromVersion == plan.ToVersion {
		return fmt.Errorf("curriculum migration plan versions are identical")
	}
	if len(plan.Actions) == 0 {
		return fmt.Errorf("curriculum migration plan is empty")
	}
	review := false
	seenFrom, seenTo := make(map[ConceptID]struct{}), make(map[ConceptID]struct{})
	for _, action := range plan.Actions {
		if err := action.Validate(); err != nil {
			return err
		}
		for _, id := range action.FromConceptIDs {
			if _, duplicate := seenFrom[id]; duplicate {
				return fmt.Errorf("source concept %q appears in multiple migration actions", id)
			}
			seenFrom[id] = struct{}{}
		}
		for _, id := range action.ToConceptIDs {
			if _, duplicate := seenTo[id]; duplicate {
				return fmt.Errorf("target concept %q appears in multiple migration actions", id)
			}
			seenTo[id] = struct{}{}
		}
		review = review || action.RequiresStudentReview
	}
	if review != plan.RequiresStudentReview {
		return fmt.Errorf("curriculum migration review summary does not match actions")
	}
	if plan.AlgorithmVersion != CurriculumMigrationPlannerVersionV1 {
		return fmt.Errorf("unsupported curriculum migration planner version %q", plan.AlgorithmVersion)
	}
	return nil
}

func validateOptionalConceptIDs(name string, values []ConceptID) error {
	for index, id := range values {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if index > 0 && values[index-1].String() >= id.String() {
			return fmt.Errorf("%s must be unique and sorted", name)
		}
	}
	return nil
}
