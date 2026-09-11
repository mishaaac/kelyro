package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/curriculum/application/memory"
)

func TestStoreRepositoriesRoundTripDefensiveCopiesAndConflicts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.NewStore()
	curricula := memory.CurriculumRepository{Store: store}
	packs := memory.PackRepository{Store: store}
	catalog := memory.PackCatalogRepository{Store: store}
	environments := memory.EnvironmentPackRepository{Store: store}
	compilations := memory.CompilationRepository{Store: store}

	definition := fixtureDefinition(t, "curriculum.z", "1")
	if err := curricula.Add(ctx, definition); err != nil {
		t.Fatalf("add curriculum: %v", err)
	}
	if err := curricula.Add(ctx, definition); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate curriculum error = %v", err)
	}
	got, err := curricula.Get(ctx, definition.ID, definition.Version)
	if err != nil {
		t.Fatalf("get curriculum: %v", err)
	}
	got.Concepts[0].Title = "mutated"
	again, err := curricula.Get(ctx, definition.ID, definition.Version)
	if err != nil || again.Concepts[0].Title != "Concept" {
		t.Fatalf("stored curriculum was mutated: title=%q err=%v", again.Concepts[0].Title, err)
	}

	pack := fixturePack(t, definition, "pack.z")
	if err := packs.Add(ctx, pack); err != nil {
		t.Fatalf("add pack: %v", err)
	}
	activation := application.PackActivation{PackID: pack.Manifest.ID, Version: pack.Manifest.Version, ActivatedAt: timestamp(t, 20)}
	if err := packs.Activate(ctx, activation); err != nil {
		t.Fatalf("activate pack: %v", err)
	}
	active, err := packs.Active(ctx)
	if err != nil || active.Manifest.ID != pack.Manifest.ID {
		t.Fatalf("active pack = %q, %v", active.Manifest.ID, err)
	}

	if err := catalog.Replace(ctx, []curriculum.PackManifest{pack.Manifest}); err != nil {
		t.Fatalf("replace catalog: %v", err)
	}
	found, err := catalog.FindByID(ctx, pack.Manifest.ID)
	if err != nil || len(found) != 1 {
		t.Fatalf("catalog find count=%d err=%v", len(found), err)
	}

	environment := fixtureEnvironment(t)
	if err := environments.Add(ctx, environment); err != nil {
		t.Fatalf("add environment: %v", err)
	}
	loadedEnvironment, err := environments.Get(ctx, environment.ID, environment.Version)
	if err != nil || loadedEnvironment.ID != environment.ID {
		t.Fatalf("environment get = %q, %v", loadedEnvironment.ID, err)
	}

	record := fixtureCompilation(t, definition)
	if err := compilations.Append(ctx, record); err != nil {
		t.Fatalf("append compilation: %v", err)
	}
	records, err := compilations.ListByCurriculum(ctx, definition.ID)
	if err != nil || len(records) != 1 || records[0].ID != record.ID {
		t.Fatalf("compilation list count=%d err=%v", len(records), err)
	}
}

func TestStoreListsDeterministicallyAndHonorsContext(t *testing.T) {
	t.Parallel()
	store := memory.NewStore()
	repository := memory.CurriculumRepository{Store: store}
	ctx := context.Background()
	for _, value := range []curriculum.CurriculumDefinition{
		fixtureDefinition(t, "curriculum.z", "2"),
		fixtureDefinition(t, "curriculum.a", "1"),
		fixtureDefinition(t, "curriculum.z", "1"),
	} {
		if err := repository.Add(ctx, value); err != nil {
			t.Fatal(err)
		}
	}
	values, err := repository.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{
		values[0].ID.String() + "@" + values[0].Version.String(),
		values[1].ID.String() + "@" + values[1].Version.String(),
		values[2].ID.String() + "@" + values[2].Version.String(),
	}
	want := []string{"curriculum.a@1", "curriculum.z@1", "curriculum.z@2"}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("list order = %v, want %v", got, want)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repository.List(cancelled); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("cancelled list error = %v", err)
	}
}

func fixtureDefinition(t *testing.T, curriculumID, versionValue string) curriculum.CurriculumDefinition {
	t.Helper()
	goalID := id(t, "goal.one")
	outcomeID := id(t, "outcome.one")
	competencyID := id(t, "competency.one")
	conceptID := conceptID(t, "concept.one")
	version, err := curriculum.NewCurriculumVersion(versionValue)
	if err != nil {
		t.Fatal(err)
	}
	return curriculum.CurriculumDefinition{
		ID: curriculumIDValue(t, curriculumID), Version: version, Title: "Curriculum", Description: "Fixture curriculum.",
		Goal:         curriculum.LearningGoalSpec{ID: goalID, Title: "Goal", Description: "Fixture goal.", Domain: "general", Outcomes: []curriculum.GoalOutcome{{ID: outcomeID, Statement: "Explain the concept.", Category: curriculum.OutcomeKnowledge, Capability: curriculum.OutcomeCapabilityExplain}}},
		Competencies: curriculum.CompetencyMatrix{Version: "matrix-v1", GoalID: goalID, Competencies: []curriculum.Competency{{ID: competencyID, Area: "general", OutcomeID: outcomeID, ExpectedLevel: curriculum.CompetencyUnderstand, ConceptRefs: []curriculum.ConceptID{conceptID}}}},
		Concepts:     []curriculum.Concept{{ID: conceptID, Title: "Concept", Definition: "A fixture concept.", Version: "1", Atomicity: curriculum.AtomicityAtomic, Difficulty: curriculum.DifficultyIntroductory, Status: curriculum.ConceptCurrent, Foundational: true}},
		Phases:       []curriculum.Phase{{ID: id(t, "phase.one"), Title: "Phase", Description: "Fixture phase.", Order: 0}},
		Modules:      []curriculum.Module{{ID: id(t, "module.one"), PhaseID: id(t, "phase.one"), Title: "Module", Description: "Fixture module.", Order: 0}},
		Lessons:      []curriculum.LessonSpec{{ID: id(t, "lesson.one"), ModuleID: id(t, "module.one"), Title: "Lesson", Description: "Fixture lesson.", Order: 0}},
		Topics:       []curriculum.TopicSpec{{ID: id(t, "topic.one"), LessonID: id(t, "lesson.one"), Title: "Topic", Description: "Fixture topic.", Order: 0, ConceptIDs: []curriculum.ConceptID{conceptID}}},
		SourcePolicy: curriculum.SourceReferencesOptionalForFixture, CreatedAt: timestamp(t, 18),
	}
}

func fixturePack(t *testing.T, definition curriculum.CurriculumDefinition, packID string) curriculum.LearningPack {
	t.Helper()
	version, err := curriculum.NewPackVersion("0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	return curriculum.LearningPack{Manifest: curriculum.PackManifest{
		ID: id(t, packID), Name: "Pack", Description: "Fixture pack.", Version: version,
		SchemaVersion: "learning-pack/v1", Domain: "general", Target: "Fixture target",
		Authors: []string{"Kelyro"}, Maintainers: []string{"Kelyro"}, License: "CC-BY-4.0",
		CreatedAt: timestamp(t, 19), MinimumKelyroVersion: version,
		CurriculumEntry: "curriculum/curriculum.yaml", SourceEvidenceEntry: "sources/evidence-report.json",
		Status: curriculum.ConceptPreview, CurriculumID: definition.ID,
	}, Curriculum: definition}
}

func fixtureEnvironment(t *testing.T) curriculum.EnvironmentPack {
	t.Helper()
	version, err := curriculum.NewPackVersion("0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	return curriculum.EnvironmentPack{ID: id(t, "environment.one"), Version: version, Tools: []curriculum.ToolRequirement{{ID: id(t, "tool.one"), Purpose: "Fixture tool", Level: curriculum.ToolRecommended}}}
}

func fixtureCompilation(t *testing.T, definition curriculum.CurriculumDefinition) application.CompilationRecord {
	t.Helper()
	input := curriculum.CompilationInput{Goal: definition.Goal, RequestedAt: timestamp(t, 17)}
	config := curriculum.CompilationConfig{CompilerVersion: "compiler-v1", SourcePolicy: curriculum.SourceReferencesOptionalForFixture}
	result := curriculum.CompilationResult{Curriculum: definition, Passes: []curriculum.CompilationPass{{Name: "fixture", Version: "fixture-v1", InputHash: "hash-in", OutputHash: "hash-out"}}}
	return application.CompilationRecord{ID: id(t, "compilation.one"), Input: input, Config: config, Result: result, CreatedAt: timestamp(t, 20)}
}

func id(t *testing.T, value string) curriculum.ID {
	t.Helper()
	result, err := curriculum.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func conceptID(t *testing.T, value string) curriculum.ConceptID {
	t.Helper()
	result, err := curriculum.NewConceptID(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func curriculumIDValue(t *testing.T, value string) curriculum.CurriculumID {
	t.Helper()
	result, err := curriculum.NewCurriculumID(value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func timestamp(t *testing.T, hour int) curriculum.Timestamp {
	t.Helper()
	result, err := curriculum.NewTimestamp(time.Date(2026, 9, 9, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return result
}
