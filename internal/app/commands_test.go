package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestBootstrapServiceSupportsFoundationActions(t *testing.T) {
	t.Parallel()

	for _, action := range []Action{
		ActionTUI,
		ActionDoctor,
		ActionConfig,
		ActionStatus,
		ActionOpen,
	} {
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()

			result, err := (BootstrapService{}).Execute(context.Background(), Command{Action: action})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !strings.Contains(result.Message, "not implemented yet") {
				t.Errorf("Execute() message = %q, want explicit placeholder", result.Message)
			}
		})
	}
}

func TestServiceInitializesRequestedWorkspace(t *testing.T) {
	t.Parallel()

	workspaces := &recordingWorkspaceService{
		workspace: workspace.Workspace{Root: "/normalized/project"},
	}
	service := NewService(workspaces, func() (string, error) {
		t.Fatal("current directory called with explicit workspace")
		return "", nil
	})

	result, err := service.Execute(context.Background(), Command{
		Action:      ActionInit,
		Workspace:   "project",
		AllowNested: true,
	})
	if err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}
	if workspaces.initRoot != "project" {
		t.Errorf("Init() root = %q, want project", workspaces.initRoot)
	}
	if !workspaces.initOptions.AllowNested {
		t.Error("Init() AllowNested = false, want true")
	}
	if result.Message != "Kelyro workspace ready at /normalized/project" {
		t.Errorf("Execute(init) message = %q", result.Message)
	}
}

func TestServiceInitializesCurrentDirectoryByDefault(t *testing.T) {
	t.Parallel()

	workspaces := &recordingWorkspaceService{workspace: workspace.Workspace{Root: "/current"}}
	service := NewService(workspaces, func() (string, error) { return "/current", nil })

	if _, err := service.Execute(context.Background(), Command{Action: ActionInit}); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}
	if workspaces.initRoot != "/current" {
		t.Errorf("Init() root = %q, want /current", workspaces.initRoot)
	}
}

func TestServiceReportsCurrentDirectoryFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("cwd unavailable")
	service := NewService(&recordingWorkspaceService{}, func() (string, error) { return "", wantErr })

	_, err := service.Execute(context.Background(), Command{Action: ActionInit})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute(init) error = %v, want wrapped cwd error", err)
	}
}

func TestBootstrapServiceRejectsUnknownAction(t *testing.T) {
	t.Parallel()

	_, err := (BootstrapService{}).Execute(context.Background(), Command{Action: "unknown"})
	if err == nil {
		t.Fatal("Execute() error = nil, want unsupported action error")
	}
}

func TestBootstrapServiceHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (BootstrapService{}).Execute(ctx, Command{Action: ActionStatus})
	if err != context.Canceled {
		t.Fatalf("Execute() error = %v, want %v", err, context.Canceled)
	}
}

type recordingWorkspaceService struct {
	initRoot    string
	initOptions workspace.InitOptions
	workspace   workspace.Workspace
	err         error
}

func (service *recordingWorkspaceService) Discover(string) (workspace.Workspace, error) {
	return workspace.Workspace{}, workspace.ErrNotFound
}

func (service *recordingWorkspaceService) Init(root string, options workspace.InitOptions) (workspace.Workspace, error) {
	service.initRoot = root
	service.initOptions = options
	return service.workspace, service.err
}

func (service *recordingWorkspaceService) Validate(string) error { return nil }
