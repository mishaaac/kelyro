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

func TestServiceCoordinatesWorkspaceLearningGoalOperations(t *testing.T) {
	t.Parallel()

	root := "/workspaces/learning-lab"
	memoryStore := memory.New()
	profiles := learningapp.NewProfileService(
		learningapp.NewStudentService(memoryStore.Repositories().Students),
		learningapp.WithProfileClock(func() time.Time { return time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC) }),
	)
	id, _ := learning.NewID("goal.go")
	goals := learningapp.NewGoalLifecycleService(
		profiles, memoryStore,
		learningapp.WithGoalClock(func() time.Time { return time.Date(2026, 8, 19, 13, 0, 0, 0, time.UTC) }),
		learningapp.WithGoalIDGenerator(func() (learning.ID, error) { return id, nil }),
	)
	factory := &fakeProfileStoreFactory{profiles: profiles, goals: goals}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)
	threshold, _ := learning.NewMasteryThreshold(.8)

	set, err := service.Execute(context.Background(), Command{
		Action: ActionGoal, Workspace: root, GoalOperation: "set",
		GoalInput: learningapp.SetGoalInput{
			Title: "Backend Engineer with Go", Domain: "Software engineering",
			TargetOutcome: "Build production backend services", StartingLevel: learning.ExperienceBeginner,
			MasteryThreshold: threshold,
		},
	})
	if err != nil || set.Goal == nil || set.Goal.Status != learning.GoalActive {
		t.Fatalf("goal set = (%+v, %v)", set, err)
	}
	shown, err := service.Execute(context.Background(), Command{Action: ActionGoal, Workspace: root, GoalOperation: "show"})
	if err != nil || len(shown.Goals) != 1 || shown.Goals[0].ID != id {
		t.Fatalf("goal show = (%+v, %v)", shown, err)
	}
	paused, err := service.Execute(context.Background(), Command{Action: ActionGoal, Workspace: root, GoalOperation: "pause"})
	if err != nil || paused.Goal == nil || paused.Goal.Status != learning.GoalPaused {
		t.Fatalf("goal pause = (%+v, %v)", paused, err)
	}
	resumed, err := service.Execute(context.Background(), Command{Action: ActionGoal, Workspace: root, GoalOperation: "resume"})
	if err != nil || resumed.Goal == nil || resumed.Goal.Status != learning.GoalActive {
		t.Fatalf("goal resume = (%+v, %v)", resumed, err)
	}
	if factory.openRoot != root || factory.closed != 4 {
		t.Fatalf("goal factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}
