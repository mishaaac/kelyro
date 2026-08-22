package app

import (
	"context"
	"errors"
	"testing"

	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesProgressDashboard(t *testing.T) {
	t.Parallel()
	for _, action := range []Action{ActionDashboard, ActionStatus, ActionProgress, ActionRoadmap, ActionToday} {
		action := action
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()
			root := "/workspaces/dashboard-lab"
			want := learningapp.ProgressDashboard{ReadModelVersion: learningapp.ProgressDashboardReadModelVersion}
			dashboard := &fakeDashboardService{view: want}
			factory := &fakeProfileStoreFactory{dashboard: dashboard}
			service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).WithProfiles(factory)

			result, err := service.Execute(context.Background(), Command{Action: action, Workspace: root})
			if err != nil || result.Dashboard == nil || result.Dashboard.ReadModelVersion != want.ReadModelVersion || dashboard.calls != 1 {
				t.Fatalf("dashboard result = (%+v, %v), calls=%d", result.Dashboard, err, dashboard.calls)
			}
			if factory.openRoot != root || factory.closed != 1 {
				t.Fatalf("factory root=%q closed=%d", factory.openRoot, factory.closed)
			}
		})
	}
}

func TestServiceDashboardReportsUninitializedWorkspace(t *testing.T) {
	t.Parallel()
	workspaces := &recordingWorkspaceService{discoverErr: workspace.ErrNotFound}
	service := NewService(workspaces, nil).WithProfiles(&fakeProfileStoreFactory{})

	_, err := service.Execute(context.Background(), Command{Action: ActionStatus, Workspace: "/outside"})
	if !errors.Is(err, workspace.ErrNotFound) {
		t.Fatalf("Execute(status) error=%v, want %v", err, workspace.ErrNotFound)
	}
	if workspaces.discoverStart != "/outside" {
		t.Errorf("Discover() start=%q, want /outside", workspaces.discoverStart)
	}
}

type fakeDashboardService struct {
	view  learningapp.ProgressDashboard
	err   error
	calls int
}

func (service *fakeDashboardService) Show(context.Context) (learningapp.ProgressDashboard, error) {
	service.calls++
	return service.view, service.err
}

var _ learningapp.ProgressDashboardService = (*fakeDashboardService)(nil)
