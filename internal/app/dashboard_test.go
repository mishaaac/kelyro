package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/artifacts"
	artifactmarkdown "github.com/mishaaac/kelyro/internal/artifacts/markdown"
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

func TestServiceExportsProgressDashboardArtifacts(t *testing.T) {
	t.Parallel()
	root := filepath.Join("workspaces", "dashboard-lab")
	dashboard := &fakeDashboardService{view: learningapp.ProgressDashboard{ReadModelVersion: learningapp.ProgressDashboardReadModelVersion}}
	profiles := &fakeProfileStoreFactory{dashboard: dashboard}
	artifactStore := &recordingArtifactStore{}
	artifactsFactory := &recordingArtifactStoreFactory{store: artifactStore}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).
		WithProfiles(profiles).WithArtifactStores(artifactsFactory)

	result, err := service.Execute(context.Background(), Command{Action: ActionProgress, ProgressOperation: "export", Workspace: root})
	if err != nil {
		t.Fatalf("Execute(progress export) error = %v", err)
	}
	if dashboard.calls != 1 || profiles.closed != 1 || !artifactStore.closed {
		t.Fatalf("dashboard calls=%d profile closes=%d artifact closed=%v", dashboard.calls, profiles.closed, artifactStore.closed)
	}
	if artifactsFactory.openRoot != root || len(artifactStore.requests) != 3 {
		t.Fatalf("artifact root=%q requests=%d", artifactsFactory.openRoot, len(artifactStore.requests))
	}
	wants := []struct{ path, version string }{
		{"LEARNING.md", artifactmarkdown.LearningProgressTemplateVersion},
		{filepath.Join("00-roadmap", "ROADMAP.md"), artifactmarkdown.RoadmapProgressTemplateVersion},
		{filepath.Join("00-roadmap", "PROGRESS.md"), artifactmarkdown.ProgressTemplateVersion},
	}
	for index, want := range wants {
		request := artifactStore.requests[index]
		if request.Path != want.path || request.ExpectedVersion != want.version || request.CreatedBy != artifactmarkdown.ProgressCreator || request.Ownership != artifacts.SystemGeneratedHumanReadable {
			t.Errorf("request[%d] = %+v, want path=%q version=%q", index, request, want.path, want.version)
		}
		if !strings.Contains(result.Message, filepath.ToSlash(want.path)) {
			t.Errorf("result message lacks %q: %q", filepath.ToSlash(want.path), result.Message)
		}
	}
}

func TestServiceProgressExportPreservesArtifactWriteConflict(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("generated artifact was modified externally")
	artifactStore := &recordingArtifactStore{writeErr: wantErr}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: "/workspace"}}, nil).
		WithProfiles(&fakeProfileStoreFactory{dashboard: &fakeDashboardService{view: learningapp.ProgressDashboard{ReadModelVersion: learningapp.ProgressDashboardReadModelVersion}}}).
		WithArtifactStores(&recordingArtifactStoreFactory{store: artifactStore})

	_, err := service.Execute(context.Background(), Command{Action: ActionProgress, ProgressOperation: "export", Workspace: "/workspace"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute(progress export) error = %v, want %v", err, wantErr)
	}
	if !artifactStore.closed {
		t.Fatal("artifact store was not closed after write conflict")
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
