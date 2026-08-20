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

func TestServiceRoutesOnboardingWithoutPresentationRules(t *testing.T) {
	t.Parallel()
	root := "/workspaces/onboarding-lab"
	store := memory.New()
	now := func() time.Time { return time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC) }
	profiles := learningapp.NewProfileService(learningapp.NewStudentService(store.Repositories().Students), learningapp.WithProfileClock(now))
	goals := learningapp.NewGoalLifecycleService(profiles, store, learningapp.WithGoalClock(now))
	onboarding := learningapp.NewOnboardingService(profiles, goals, store.Repositories().Onboarding, learningapp.WithOnboardingClock(now))
	factory := &fakeProfileStoreFactory{profiles: profiles, goals: goals, onboarding: onboarding}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)

	started, err := service.Execute(context.Background(), Command{Action: ActionOnboarding, Workspace: root, OnboardingOperation: "start"})
	if err != nil || started.Onboarding == nil || started.Onboarding.Interview.Status != learning.OnboardingInProgress {
		t.Fatalf("start onboarding = (%+v, %v)", started, err)
	}
	answered, err := service.Execute(context.Background(), Command{Action: ActionOnboarding, Workspace: root, OnboardingOperation: "submit", OnboardingAnswer: "Ada"})
	if err != nil || answered.Onboarding == nil || answered.Onboarding.Question.ID != learningapp.OnboardingGoalTitleQuestion {
		t.Fatalf("submit onboarding = (%+v, %v)", answered, err)
	}
	if factory.openRoot != root || factory.closed != 2 {
		t.Fatalf("onboarding factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}
