package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCurriculumMigrationPlannerV1PreservesStableStateAndInitializesAdditions(t *testing.T) {
	t.Parallel()
	old := changeDefinitionFixture(t)
	newDefinition := nextDefinition(t, old, "2026.09.14.2")
	newDefinition.Concepts = append([]curriculum.Concept(nil), old.Concepts...)
	newDefinition.Concepts[0].Status = curriculum.ConceptDeprecated
	addedID, _ := curriculum.NewConceptID("concept.http.responses")
	added := old.Concepts[0]
	added.ID, added.Title, added.Definition = addedID, "HTTP responses", "An HTTP response carries a server result."
	newDefinition.Concepts = append(newDefinition.Concepts, added)
	newDefinition.Competencies.Competencies = append([]curriculum.Competency(nil), old.Competencies.Competencies...)
	newDefinition.Competencies.Competencies[0].ConceptRefs = append(newDefinition.Competencies.Competencies[0].ConceptRefs, addedID)
	newDefinition.Topics = append([]curriculum.TopicSpec(nil), old.Topics...)
	newDefinition.Topics[0].ConceptIDs = append(newDefinition.Topics[0].ConceptIDs, addedID)
	newDefinition.Prerequisites = append([]curriculum.Prerequisite(nil), old.Prerequisites...)
	newDefinition.Prerequisites = append(newDefinition.Prerequisites, curriculum.Prerequisite{
		ConceptID: addedID, RequiredConceptID: old.Concepts[0].ID, Kind: curriculum.PrerequisiteHard,
		EvidenceRefs: old.Concepts[0].EvidenceRefs,
	})

	classification, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), CurriculumChangeClassificationRequest{Old: old, New: newDefinition})
	if err != nil {
		t.Fatal(err)
	}
	request := CurriculumMigrationPlanningRequest{Old: old, New: newDefinition, Classification: classification}
	plan, err := NewCurriculumMigrationPlannerV1().Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	stable := migrationActionForTarget(t, plan, old.Concepts[0].ID)
	if stable.Kind != curriculum.MigrationPreserveState || !stable.PreserveMastery || !stable.PreserveEvidence || !stable.PreserveHistoricalEvidence || !stable.RequiresStudentReview {
		t.Fatalf("stable migration = %+v", stable)
	}
	addedAction := migrationActionForTarget(t, plan, addedID)
	if addedAction.Kind != curriculum.MigrationInitializeUnknown || !addedAction.InitializeUnknown || addedAction.PreserveMastery || addedAction.PreserveEvidence {
		t.Fatalf("added migration = %+v", addedAction)
	}
	if !plan.RecalculateUnlockEligibility || !plan.RequiresStudentReview || plan.AlgorithmVersion != curriculum.CurriculumMigrationPlannerVersionV1 {
		t.Fatalf("plan summary = %+v", plan)
	}
	repeated, err := NewCurriculumMigrationPlannerV1().Plan(context.Background(), request)
	if err != nil || !reflect.DeepEqual(plan, repeated) {
		t.Fatalf("migration plan is not deterministic: %+v / %+v / %v", plan, repeated, err)
	}
}

func TestCurriculumMigrationPlannerV1RequiresSplitMappingAndTransfersNoMastery(t *testing.T) {
	t.Parallel()
	old := changeDefinitionFixture(t)
	newDefinition := splitDefinition(t, old, "2026.09.14.2")
	mapping := curriculum.ConceptIdentityMapping{
		OldConceptIDs: []curriculum.ConceptID{old.Concepts[0].ID},
		NewConceptIDs: []curriculum.ConceptID{newDefinition.Concepts[1].ID, newDefinition.Concepts[0].ID},
		Rationale:     "The original unit had two independently assessable ideas.",
	}
	classification, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), CurriculumChangeClassificationRequest{Old: old, New: newDefinition, IdentityMappings: []curriculum.ConceptIdentityMapping{mapping}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewCurriculumMigrationPlannerV1().Plan(context.Background(), CurriculumMigrationPlanningRequest{Old: old, New: newDefinition, Classification: classification}); err == nil {
		t.Fatal("expected missing mapping to fail closed")
	}
	plan, err := NewCurriculumMigrationPlannerV1().Plan(context.Background(), CurriculumMigrationPlanningRequest{Old: old, New: newDefinition, Classification: classification, IdentityMappings: []curriculum.ConceptIdentityMapping{mapping}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("split actions = %+v", plan.Actions)
	}
	action := plan.Actions[0]
	if action.Kind != curriculum.MigrationSplitNoTransfer || action.PreserveMastery || action.PreserveEvidence || !action.InitializeUnknown || !action.PreserveHistoricalEvidence || !action.RequiresStudentReview {
		t.Fatalf("split migration = %+v", action)
	}
	if got := action.ToConceptIDs; !reflect.DeepEqual(got, []curriculum.ConceptID{newDefinition.Concepts[1].ID, newDefinition.Concepts[0].ID}) {
		t.Fatalf("canonical target IDs = %v", got)
	}
}

func TestCurriculumMigrationPlannerV1KeepsRemovedConceptHistoricalAndHierarchyStateStable(t *testing.T) {
	t.Parallel()
	old := changeDefinitionFixture(t)
	newDefinition := splitDefinition(t, old, "2026.09.14.2")
	removedID := old.Concepts[0].ID
	classification, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), CurriculumChangeClassificationRequest{Old: old, New: newDefinition})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewCurriculumMigrationPlannerV1().Plan(context.Background(), CurriculumMigrationPlanningRequest{Old: old, New: newDefinition, Classification: classification})
	if err != nil {
		t.Fatal(err)
	}
	removed := migrationActionForSource(t, plan, removedID)
	if removed.Kind != curriculum.MigrationPreserveHistorical || !removed.PreserveHistoricalEvidence || !removed.RequiresStudentReview || removed.PreserveMastery {
		t.Fatalf("removed migration = %+v", removed)
	}
	reorganized := nextDefinition(t, old, "2026.09.14.3")
	reorganized.Modules = append([]curriculum.Module(nil), old.Modules...)
	reorganized.Modules[0].Title = "Reorganized module"
	hierarchyClassification, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), CurriculumChangeClassificationRequest{Old: old, New: reorganized})
	if err != nil {
		t.Fatal(err)
	}
	hierarchyPlan, err := NewCurriculumMigrationPlannerV1().Plan(context.Background(), CurriculumMigrationPlanningRequest{Old: old, New: reorganized, Classification: hierarchyClassification})
	if err != nil {
		t.Fatal(err)
	}
	stable := migrationActionForTarget(t, hierarchyPlan, old.Concepts[0].ID)
	if stable.Kind != curriculum.MigrationPreserveState || !stable.PreserveMastery {
		t.Fatalf("hierarchy-stable migration = %+v", stable)
	}
}

func migrationActionForTarget(t *testing.T, plan curriculum.CurriculumMigrationPlan, id curriculum.ConceptID) curriculum.CurriculumMigrationAction {
	t.Helper()
	for _, action := range plan.Actions {
		for _, candidate := range action.ToConceptIDs {
			if candidate == id {
				return action
			}
		}
	}
	t.Fatalf("target action %q missing", id)
	return curriculum.CurriculumMigrationAction{}
}

func migrationActionForSource(t *testing.T, plan curriculum.CurriculumMigrationPlan, id curriculum.ConceptID) curriculum.CurriculumMigrationAction {
	t.Helper()
	for _, action := range plan.Actions {
		for _, candidate := range action.FromConceptIDs {
			if candidate == id {
				return action
			}
		}
	}
	t.Fatalf("source action %q missing", id)
	return curriculum.CurriculumMigrationAction{}
}
