package app

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesAchievementRefresh(t *testing.T) {
	t.Parallel()
	root := "/workspaces/achievement-lab"
	evaluatedAt, err := learning.NewTimestamp(time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	refresh := learningapp.AchievementRefresh{EvaluatedAt: evaluatedAt, PolicyVersion: learning.AchievementPolicyVersion}
	achievements := &fakeAchievementService{result: refresh}
	factory := &fakeProfileStoreFactory{achievements: achievements}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)
	result, err := service.Execute(context.Background(), Command{Action: ActionAchievements, Workspace: root})
	if err != nil || result.Achievements == nil || result.Achievements.PolicyVersion != learning.AchievementPolicyVersion || achievements.calls != 1 {
		t.Fatalf("achievement result = (%+v, %v), calls=%d", result.Achievements, err, achievements.calls)
	}
	if factory.openRoot != root || factory.closed != 1 {
		t.Fatalf("factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}

type fakeAchievementService struct {
	result learningapp.AchievementRefresh
	err    error
	calls  int
}

func (service *fakeAchievementService) Refresh(context.Context) (learningapp.AchievementRefresh, error) {
	service.calls++
	return service.result, service.err
}

var _ learningapp.AchievementService = (*fakeAchievementService)(nil)
