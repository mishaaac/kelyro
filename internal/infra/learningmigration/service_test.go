package learningmigration

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/infra/learningdb"
	"github.com/mishaaac/kelyro/internal/infra/workspacefs"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestProjectCurriculumProducesDeterministicStudentCoreContract(t *testing.T) {
	t.Parallel()
	definition := projectionFixture(t)
	projected, err := ProjectCurriculum(definition)
	if err != nil {
		t.Fatal(err)
	}
	if projected.ContractVersion != learning.CurriculumContractVersion || projected.Reference.ID.String() != definition.ID.String() || projected.Reference.Version != definition.Version.String() {
		t.Fatalf("projection identity = %+v", projected)
	}
	root, exists := projected.Node(mustLearningID(t, "concept.root"))
	if !exists || root.Concept.Difficulty != learning.ConceptDifficultyFoundational || root.Concept.EstimatedEffortMinutes != 60 {
		t.Fatalf("root projection = %+v", root)
	}
	use, exists := projected.Node(mustLearningID(t, "concept.use"))
	if !exists || use.Status.State != learning.CurriculumNodeDeprecated || len(use.Concept.Prerequisites) != 1 ||
		use.Concept.Prerequisites[0].ConceptID != root.ID || use.Concept.Prerequisites[0].Requirement != learning.PrerequisiteMastered {
		t.Fatalf("dependent projection = %+v", use)
	}
}

func TestServiceActivatesPackCurriculumForActiveGoalIdempotently(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if _, err := workspacefs.New("test").Init(root, workspace.InitOptions{}); err != nil {
		t.Fatal(err)
	}
	stores := learningdb.NewFactory("test")
	store, err := stores.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Profiles().Show(ctx); err != nil {
		t.Fatal(err)
	}
	threshold, _ := learning.NewMasteryThreshold(.8)
	if _, err := store.Goals().Set(ctx, learningapp.SetGoalInput{
		Title: "Projection", Domain: "general", TargetOutcome: "Apply the concepts.",
		StartingLevel: learning.ExperienceBeginner, MasteryThreshold: threshold,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	service := New(stores, nil)
	request := curriculumapp.StudentCurriculumActivationRequest{WorkspaceRoot: root, Curriculum: projectionFixture(t)}
	if err := service.Preflight(ctx, request); err != nil {
		t.Fatalf("Preflight() error = %v", err)
	}
	first, err := service.ActivateCurriculum(ctx, request)
	if err != nil || !first.Created || first.CurriculumInstanceID == "" {
		t.Fatalf("first activation = %+v, %v", first, err)
	}
	second, err := service.ActivateCurriculum(ctx, request)
	if err != nil || second.Created || second.CurriculumInstanceID != first.CurriculumInstanceID {
		t.Fatalf("repeated activation = %+v, %v", second, err)
	}
	current, err := service.CurrentConcept(ctx, root)
	if err != nil || current.String() != "concept.root" {
		t.Fatalf("current concept = %q, %v", current, err)
	}
	store, err = stores.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	dashboard, err := store.Dashboard().Show(ctx)
	if err != nil || dashboard.Curriculum == nil || dashboard.Curriculum.Instance.Source != learning.CurriculumSourcePack {
		t.Fatalf("roadmap curriculum = %+v, %v", dashboard.Curriculum, err)
	}
}

func projectionFixture(t *testing.T) curriculum.CurriculumDefinition {
	t.Helper()
	id := func(value string) curriculum.ID {
		result, err := curriculum.NewID(value)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	conceptID := func(value string) curriculum.ConceptID {
		result, err := curriculum.NewConceptID(value)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	curriculumID, _ := curriculum.NewCurriculumID("curriculum.projection")
	version, _ := curriculum.NewCurriculumVersion("2026.09.14.1")
	createdAt, _ := curriculum.NewTimestamp(time.Date(2026, 9, 14, 18, 0, 0, 0, time.UTC))
	goalID, outcomeID := id("goal.projection"), id("outcome.projection")
	root, use := conceptID("concept.root"), conceptID("concept.use")
	return curriculum.CurriculumDefinition{
		ID: curriculumID, Version: version, Title: "Projection", Description: "Projection fixture.",
		Goal:         curriculum.LearningGoalSpec{ID: goalID, Title: "Goal", Description: "Projection goal.", Domain: "general", Outcomes: []curriculum.GoalOutcome{{ID: outcomeID, Statement: "Apply the concepts.", Category: curriculum.OutcomeApplication, Capability: curriculum.OutcomeCapabilityBuild}}},
		Competencies: curriculum.CompetencyMatrix{Version: curriculum.CompetencyMatrixVersionV1, GoalID: goalID, Competencies: []curriculum.Competency{{ID: id("competency.projection"), AreaID: id("area.projection"), Area: "general", OutcomeID: outcomeID, ExpectedLevel: curriculum.CompetencyApply, ConceptRefs: []curriculum.ConceptID{root, use}}}},
		Concepts: []curriculum.Concept{
			{ID: root, Title: "Root", Definition: "Understand the root concept.", Version: "1", Atomicity: curriculum.AtomicityAtomic, Difficulty: curriculum.DifficultyFoundational, Status: curriculum.ConceptCurrent, Foundational: true},
			{ID: use, Title: "Use", Definition: "Apply the root concept.", Version: "1", Atomicity: curriculum.AtomicityAtomic, Difficulty: curriculum.DifficultyIntermediate, Status: curriculum.ConceptDeprecated},
		},
		Prerequisites: []curriculum.Prerequisite{{ConceptID: use, RequiredConceptID: root, Kind: curriculum.PrerequisiteHard}},
		Phases:        []curriculum.Phase{{ID: id("phase.projection"), Title: "Phase", Description: "Phase.", Order: 0}},
		Modules:       []curriculum.Module{{ID: id("module.projection"), PhaseID: id("phase.projection"), Title: "Module", Description: "Module.", Order: 0}},
		Lessons:       []curriculum.LessonSpec{{ID: id("lesson.projection"), ModuleID: id("module.projection"), Title: "Lesson", Description: "Lesson.", Order: 0}},
		Topics:        []curriculum.TopicSpec{{ID: id("topic.projection"), LessonID: id("lesson.projection"), Title: "Topic", Description: "Topic.", Order: 0, ConceptIDs: []curriculum.ConceptID{root, use}}},
		SourcePolicy:  curriculum.SourceReferencesOptionalForFixture, CreatedAt: createdAt,
	}
}

func mustLearningID(t *testing.T, value string) learning.ID {
	t.Helper()
	id, err := learning.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
