package app

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesWorkspaceProfileShowAndEdit(t *testing.T) {
	t.Parallel()

	root := "/workspaces/learning-lab"
	memoryStore := memory.New()
	profileService := learningapp.NewProfileService(
		learningapp.NewStudentService(memoryStore.Repositories().Students),
		learningapp.WithProfileClock(func() time.Time { return time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC) }),
	)
	factory := &fakeProfileStoreFactory{profiles: profileService}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)

	shown, err := service.Execute(context.Background(), Command{Action: ActionProfile, Workspace: root, ProfileOperation: "show"})
	if err != nil || shown.Profile == nil || !reflect.DeepEqual(shown.Profile.Profile, learning.DefaultStudentProfile()) {
		t.Fatalf("profile show = (%+v, %v)", shown, err)
	}
	name := "Ada"
	edited, err := service.Execute(context.Background(), Command{
		Action: ActionProfile, Workspace: root, ProfileOperation: "edit",
		ProfileChanges: learningapp.ProfileChanges{DisplayName: &name},
	})
	if err != nil || edited.Profile == nil || edited.Profile.Profile.DisplayName != "Ada" {
		t.Fatalf("profile edit = (%+v, %v)", edited, err)
	}
	if factory.openRoot != root || factory.closed != 2 {
		t.Fatalf("profile factory root=%q closed=%d", factory.openRoot, factory.closed)
	}
}

type fakeProfileStoreFactory struct {
	profiles learningapp.ProfileService
	goals    learningapp.GoalLifecycleService
	openRoot string
	closed   int
}

func (factory *fakeProfileStoreFactory) Open(_ context.Context, root string) (learningapp.ProfileStore, error) {
	factory.openRoot = root
	return &fakeProfileStore{profiles: factory.profiles, goals: factory.goals, close: func() { factory.closed++ }}, nil
}

type fakeProfileStore struct {
	profiles learningapp.ProfileService
	goals    learningapp.GoalLifecycleService
	close    func()
}

func (store *fakeProfileStore) Profiles() learningapp.ProfileService    { return store.profiles }
func (store *fakeProfileStore) Goals() learningapp.GoalLifecycleService { return store.goals }
func (store *fakeProfileStore) Close() error {
	store.close()
	return nil
}
