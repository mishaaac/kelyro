package app

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceRoutesIntegratedSetupWithoutPresentationRules(t *testing.T) {
	t.Parallel()
	root := "/workspaces/setup-lab"
	store := memory.New()
	current := time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC)
	now := func() time.Time { value := current; current = current.Add(time.Minute); return value }
	profiles := learningapp.NewProfileService(learningapp.NewStudentService(store.Repositories().Students), learningapp.WithProfileClock(now))
	goals := learningapp.NewGoalLifecycleService(profiles, store, learningapp.WithGoalClock(now))
	onboarding := learningapp.NewOnboardingService(profiles, goals, store.Repositories().Onboarding, learningapp.WithOnboardingClock(now))
	curriculum := appSetupCurriculum(t)
	instances := learningapp.NewCurriculumInstanceService(profiles, store, learningapp.WithCurriculumInstanceClock(now))
	diagnostic := appSetupDiagnostic(t, curriculum.Reference)
	diagnostics := learningapp.NewDiagnosticService(profiles, store, learningapp.WithDiagnosticClock(now))
	setup := learningapp.NewLearnerSetupService(profiles, onboarding, instances, diagnostics, store, curriculum, diagnostic, learningapp.WithLearnerSetupClock(now))
	factory := &fakeProfileStoreFactory{profiles: profiles, goals: goals, onboarding: onboarding, setup: setup}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)

	started, err := service.Execute(context.Background(), Command{Action: ActionSetup, Workspace: root, SetupOperation: "start"})
	if err != nil || started.Setup == nil || started.Setup.Onboarding == nil || started.Setup.Onboarding.Interview.Status != learning.OnboardingInProgress {
		t.Fatalf("start setup = (%+v, %v)", started, err)
	}
	answered, err := service.Execute(context.Background(), Command{Action: ActionSetup, Workspace: root, SetupOperation: "onboarding-submit", SetupAnswers: []string{"Ada"}})
	if err != nil || answered.Setup == nil || answered.Setup.Onboarding.Question.ID != learningapp.OnboardingGoalTitleQuestion {
		t.Fatalf("submit setup = (%+v, %v)", answered, err)
	}
	if factory.openRoot != root || factory.closed != 2 {
		t.Fatalf("setup factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}

func appSetupCurriculum(t *testing.T) learning.Curriculum {
	t.Helper()
	id := func(value string) learning.ID {
		parsed, err := learning.NewID(value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	phase, module, lesson, topic, concept := id("phase.setup"), id("module.setup"), id("lesson.setup"), id("topic.setup"), id("concept.setup")
	status := learning.CurriculumStatusMetadata{State: learning.CurriculumNodeActive}
	curriculum, err := learning.NewCurriculum(learning.CurriculumContractVersion, learning.CurriculumRef{ID: id("fixture.setup"), Version: "1.0.0"}, "Setup", "Setup fixture.", []learning.CurriculumNode{
		{ID: phase, Type: learning.CurriculumNodePhase, Title: "Phase", Description: "Phase.", Status: status, Version: "1.0.0"},
		{ID: module, Type: learning.CurriculumNodeModule, ParentID: &phase, Title: "Module", Description: "Module.", Status: status, Version: "1.0.0"},
		{ID: lesson, Type: learning.CurriculumNodeLesson, ParentID: &module, Title: "Lesson", Description: "Lesson.", Status: status, Version: "1.0.0"},
		{ID: topic, Type: learning.CurriculumNodeTopic, ParentID: &lesson, Title: "Topic", Description: "Topic.", Status: status, Version: "1.0.0"},
		{ID: concept, Type: learning.CurriculumNodeConcept, ParentID: &topic, Title: "Concept", Description: "Concept.", Status: status, Version: "1.0.0", Concept: &learning.ConceptDefinition{Objectives: []string{"Understand"}, Difficulty: learning.ConceptDifficultyIntroductory, EstimatedEffortMinutes: 10, AssessmentExpectations: []string{"Explain"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return curriculum
}

func appSetupDiagnostic(t *testing.T, curriculum learning.CurriculumRef) learning.Diagnostic {
	t.Helper()
	id := func(value string) learning.ID {
		parsed, err := learning.NewID(value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	diagnostic, err := learning.NewDiagnostic(learning.DiagnosticContractVersion, learning.DiagnosticScoringPolicyVersion,
		learning.DiagnosticRef{ID: id("diagnostic.setup"), Version: "1.0.0"}, curriculum, "Setup diagnostic",
		[]learning.DiagnosticSection{{ID: id("section.setup"), Title: "Setup", Items: []learning.DiagnosticItem{{ID: id("item.setup"), ConceptID: id("concept.setup"), Kind: learning.DiagnosticSingleChoice, Prompt: "Choose yes", Options: []learning.DiagnosticOption{{Value: "yes", Label: "Yes"}, {Value: "no", Label: "No"}}, AcceptedAnswers: []string{"yes"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	return diagnostic
}
