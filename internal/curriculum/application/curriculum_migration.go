package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

type CurriculumMigrationPlannerV1 struct{}

func NewCurriculumMigrationPlannerV1() CurriculumMigrationPlannerV1 {
	return CurriculumMigrationPlannerV1{}
}

func (CurriculumMigrationPlannerV1) Plan(ctx context.Context, request CurriculumMigrationPlanningRequest) (curriculum.CurriculumMigrationPlan, error) {
	const operation = "plan student-safe curriculum migration"
	if err := ctx.Err(); err != nil {
		return curriculum.CurriculumMigrationPlan{}, ExternalError(operation, err)
	}
	if err := request.Old.Validate(); err != nil {
		return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("old curriculum: %w", err))
	}
	if err := request.New.Validate(); err != nil {
		return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("new curriculum: %w", err))
	}
	if request.Old.ID != request.New.ID || request.Old.Version == request.New.Version {
		return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("migration requires two versions of the same curriculum"))
	}
	if err := request.Classification.Validate(); err != nil {
		return curriculum.CurriculumMigrationPlan{}, Invalid(operation, err)
	}
	if request.Classification.FromVersion != request.Old.Version || request.Classification.ToVersion != request.New.Version {
		return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("change classification does not match curriculum versions"))
	}

	oldConcepts, newConcepts := conceptsByID(request.Old.Concepts), conceptsByID(request.New.Concepts)
	mappedOld, mappedNew := make(map[curriculum.ConceptID]struct{}), make(map[curriculum.ConceptID]struct{})
	actions := make([]curriculum.CurriculumMigrationAction, 0, len(oldConcepts)+len(newConcepts))
	mappings := canonicalIdentityMappings(request.IdentityMappings)
	for _, mapping := range mappings {
		if err := mapping.Validate(); err != nil {
			return curriculum.CurriculumMigrationPlan{}, Invalid(operation, err)
		}
		for _, id := range mapping.OldConceptIDs {
			if _, exists := oldConcepts[id]; !exists {
				return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("identity mapping references missing old concept %q", id))
			}
			if _, stable := newConcepts[id]; stable {
				return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("identity mapping source concept %q was not removed", id))
			}
			if _, duplicate := mappedOld[id]; duplicate {
				return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("old concept %q appears in multiple identity mappings", id))
			}
			mappedOld[id] = struct{}{}
		}
		for _, id := range mapping.NewConceptIDs {
			if _, exists := newConcepts[id]; !exists {
				return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("identity mapping references missing new concept %q", id))
			}
			if _, stable := oldConcepts[id]; stable {
				return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("identity mapping target concept %q was not added", id))
			}
			if _, duplicate := mappedNew[id]; duplicate {
				return curriculum.CurriculumMigrationPlan{}, Invalid(operation, fmt.Errorf("new concept %q appears in multiple identity mappings", id))
			}
			mappedNew[id] = struct{}{}
		}
		kind := curriculum.MigrationMergeNoTransfer
		if len(mapping.OldConceptIDs) == 1 {
			kind = curriculum.MigrationSplitNoTransfer
		}
		actions = append(actions, curriculum.CurriculumMigrationAction{
			Kind: kind, FromConceptIDs: mapping.OldConceptIDs, ToConceptIDs: mapping.NewConceptIDs,
			InitializeUnknown: true, PreserveHistoricalEvidence: true, RequiresStudentReview: true,
			Rationale: mapping.Rationale + " Historical evidence remains attached to the old instance; target mastery starts unknown.",
		})
	}
	if err := validateMappingClassification(request.Classification, mappings); err != nil {
		return curriculum.CurriculumMigrationPlan{}, Invalid(operation, err)
	}

	for _, id := range migrationSortedConceptIDs(oldConcepts) {
		newConcept, stable := newConcepts[id]
		if stable {
			historical := newConcept.Status == curriculum.ConceptDeprecated || newConcept.Status == curriculum.ConceptHistorical || newConcept.Status == curriculum.ConceptLegacy
			review := conceptRequiresReview(request.Classification, id)
			rationale := "Stable Concept ID preserves learner mastery and evidence across hierarchy or metadata changes."
			if historical {
				rationale = "Stable deprecated, legacy, or historical Concept ID preserves learner mastery and historical evidence."
			}
			actions = append(actions, curriculum.CurriculumMigrationAction{
				Kind: curriculum.MigrationPreserveState, FromConceptIDs: []curriculum.ConceptID{id}, ToConceptIDs: []curriculum.ConceptID{id},
				PreserveMastery: true, PreserveEvidence: true, PreserveHistoricalEvidence: historical,
				RequiresStudentReview: review, Rationale: rationale,
			})
			continue
		}
		if _, mapped := mappedOld[id]; mapped {
			continue
		}
		actions = append(actions, curriculum.CurriculumMigrationAction{
			Kind: curriculum.MigrationPreserveHistorical, FromConceptIDs: []curriculum.ConceptID{id},
			PreserveHistoricalEvidence: true, RequiresStudentReview: true,
			Rationale: "Removed Concept state remains readable only on the historical curriculum instance; no mastery is transferred.",
		})
	}
	for _, id := range migrationSortedConceptIDs(newConcepts) {
		if _, stable := oldConcepts[id]; stable {
			continue
		}
		if _, mapped := mappedNew[id]; mapped {
			continue
		}
		actions = append(actions, curriculum.CurriculumMigrationAction{
			Kind: curriculum.MigrationInitializeUnknown, ToConceptIDs: []curriculum.ConceptID{id}, InitializeUnknown: true,
			Rationale: "Added Concept starts with unknown mastery and no invented evidence.",
		})
	}
	sort.Slice(actions, func(i, j int) bool { return migrationActionKey(actions[i]) < migrationActionKey(actions[j]) })
	plan := curriculum.CurriculumMigrationPlan{
		CurriculumID: request.Old.ID, FromVersion: request.Old.Version, ToVersion: request.New.Version,
		Actions: actions, RecalculateUnlockEligibility: hasChange(request.Classification, curriculum.ChangePrerequisiteChanged),
		AlgorithmVersion: curriculum.CurriculumMigrationPlannerVersionV1,
	}
	for _, action := range actions {
		plan.RequiresStudentReview = plan.RequiresStudentReview || action.RequiresStudentReview
	}
	plan.ID = migrationPlanID(plan)
	if err := plan.Validate(); err != nil {
		return curriculum.CurriculumMigrationPlan{}, Invalid(operation, err)
	}
	return plan, nil
}

func canonicalIdentityMappings(values []curriculum.ConceptIdentityMapping) []curriculum.ConceptIdentityMapping {
	result := make([]curriculum.ConceptIdentityMapping, len(values))
	for index, value := range values {
		result[index] = curriculum.ConceptIdentityMapping{
			OldConceptIDs: append([]curriculum.ConceptID(nil), value.OldConceptIDs...),
			NewConceptIDs: append([]curriculum.ConceptID(nil), value.NewConceptIDs...), Rationale: strings.TrimSpace(value.Rationale),
		}
		sort.Slice(result[index].OldConceptIDs, func(i, j int) bool {
			return result[index].OldConceptIDs[i].String() < result[index].OldConceptIDs[j].String()
		})
		sort.Slice(result[index].NewConceptIDs, func(i, j int) bool {
			return result[index].NewConceptIDs[i].String() < result[index].NewConceptIDs[j].String()
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return conceptIDsKey(result[i].OldConceptIDs)+">"+conceptIDsKey(result[i].NewConceptIDs) < conceptIDsKey(result[j].OldConceptIDs)+">"+conceptIDsKey(result[j].NewConceptIDs)
	})
	return result
}

func validateMappingClassification(classification curriculum.CurriculumChangeClassification, mappings []curriculum.ConceptIdentityMapping) error {
	hasSplit, hasMerge := hasChange(classification, curriculum.ChangeConceptSplit), hasChange(classification, curriculum.ChangeConceptMerged)
	wantSplit, wantMerge := false, false
	for _, mapping := range mappings {
		wantSplit = wantSplit || len(mapping.OldConceptIDs) == 1
		wantMerge = wantMerge || len(mapping.OldConceptIDs) > 1
	}
	if hasSplit != wantSplit || hasMerge != wantMerge {
		return fmt.Errorf("split/merge classification requires the same explicit identity mappings used by the classifier")
	}
	return nil
}

func conceptRequiresReview(classification curriculum.CurriculumChangeClassification, id curriculum.ConceptID) bool {
	for _, change := range classification.Changes {
		if change.Migration != curriculum.MigrationRequiresStudentReview && change.Migration != curriculum.MigrationBreaking {
			continue
		}
		for _, affected := range change.AffectedConcepts {
			if affected == id {
				return true
			}
		}
	}
	return false
}

func hasChange(classification curriculum.CurriculumChangeClassification, kind curriculum.CurriculumChangeKind) bool {
	for _, change := range classification.Changes {
		if change.Kind == kind {
			return true
		}
	}
	return false
}

func migrationSortedConceptIDs(values map[curriculum.ConceptID]curriculum.Concept) []curriculum.ConceptID {
	result := make([]curriculum.ConceptID, 0, len(values))
	for id := range values {
		result = append(result, id)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func migrationActionKey(action curriculum.CurriculumMigrationAction) string {
	return string(action.Kind) + "|" + conceptIDsKey(action.FromConceptIDs) + "|" + conceptIDsKey(action.ToConceptIDs)
}

func conceptIDsKey(values []curriculum.ConceptID) string {
	parts := make([]string, len(values))
	for index, id := range values {
		parts[index] = id.String()
	}
	return strings.Join(parts, ",")
}

func migrationPlanID(plan curriculum.CurriculumMigrationPlan) curriculum.ID {
	parts := []string{plan.CurriculumID.String(), plan.FromVersion.String(), plan.ToVersion.String(), plan.AlgorithmVersion, fmt.Sprintf("recalc=%t", plan.RecalculateUnlockEligibility)}
	for _, action := range plan.Actions {
		parts = append(parts, migrationActionKey(action), fmt.Sprintf("%t|%t|%t|%t|%t", action.PreserveMastery, action.PreserveEvidence, action.InitializeUnknown, action.PreserveHistoricalEvidence, action.RequiresStudentReview), action.Rationale)
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	id, _ := curriculum.NewID("curriculum-migration." + hex.EncodeToString(digest[:16]))
	return id
}

var _ CurriculumMigrationPlanningService = CurriculumMigrationPlannerV1{}
