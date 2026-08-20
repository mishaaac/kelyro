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

func TestServiceRoutesMasteryThresholdOperations(t *testing.T) {
	t.Parallel()
	root := "/workspaces/mastery-lab"
	store := memory.New()
	current := time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC)
	now := func() time.Time {
		value := current
		current = current.Add(time.Minute)
		return value
	}
	profiles := learningapp.NewProfileService(learningapp.NewStudentService(store.Repositories().Students), learningapp.WithProfileClock(now))
	mastery := learningapp.NewMasteryPolicyService(profiles, store.Repositories().Mastery, learningapp.WithMasteryPolicyClock(now))
	factory := &fakeProfileStoreFactory{profiles: profiles, mastery: mastery}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)

	shown, err := service.Execute(context.Background(), Command{Action: ActionMastery, Workspace: root, MasteryOperation: "show"})
	if err != nil || shown.Mastery == nil || shown.Mastery.Requirement.Mode != learning.MasteryModeStandard {
		t.Fatalf("show mastery = (%+v, %v)", shown, err)
	}
	strict, _ := learning.NewMasteryThreshold(.85)
	set, err := service.Execute(context.Background(), Command{Action: ActionMastery, Workspace: root, MasteryOperation: "set", MasteryThreshold: strict})
	if err != nil || set.Mastery == nil || set.Mastery.Source != learning.MasterySourceWorkspaceOverride {
		t.Fatalf("set mastery = (%+v, %v)", set, err)
	}
	reset, err := service.Execute(context.Background(), Command{Action: ActionMastery, Workspace: root, MasteryOperation: "reset"})
	if err != nil || reset.Mastery == nil || reset.Mastery.Source != learning.MasterySourceStudentDefault {
		t.Fatalf("reset mastery = (%+v, %v)", reset, err)
	}
	if factory.openRoot != root || factory.closed != 3 {
		t.Fatalf("mastery factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}
