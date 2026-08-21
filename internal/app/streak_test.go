package app

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesStreakDisplay(t *testing.T) {
	t.Parallel()
	root := "/workspaces/streak-lab"
	studentID, err := learning.NewID("student.streak")
	if err != nil {
		t.Fatal(err)
	}
	state := learning.Streak{StudentID: studentID, PolicyVersion: learning.StreakPolicyVersion,
		Timezone: "UTC", MinimumActiveMinutes: 10}
	streaks := &fakeStreakService{result: state}
	factory := &fakeProfileStoreFactory{streaks: streaks}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)
	result, err := service.Execute(context.Background(), Command{Action: ActionStreak, Workspace: root})
	if err != nil || result.Streak == nil || *result.Streak != state || streaks.calls != 1 {
		t.Fatalf("streak result = (%+v, %v), calls=%d", result.Streak, err, streaks.calls)
	}
	if factory.openRoot != root || factory.closed != 1 {
		t.Fatalf("factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}

type fakeStreakService struct {
	result learning.Streak
	err    error
	calls  int
}

func (service *fakeStreakService) Show(context.Context) (learning.Streak, error) {
	service.calls++
	return service.result, service.err
}

var _ learningapp.StreakService = (*fakeStreakService)(nil)
