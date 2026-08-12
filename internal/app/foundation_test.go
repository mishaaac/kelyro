package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestLoadFoundationReturnsTypedHealthySnapshot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "projects", "foundations")
	workspaces := &recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}
	configs := &recordingConfigStore{project: config.Settings{
		config.KeyWorkspaceName: config.StringValue("My learning lab"),
		config.KeyUIColor:       config.StringValue("never"),
	}}
	store := &recordingArtifactStore{}
	factory := &recordingArtifactStoreFactory{store: store}
	service := NewService(workspaces, func() (string, error) { return filepath.Join(root, "00-roadmap"), nil }).
		WithConfig(configs).
		WithArtifactStores(factory)

	snapshot, err := service.LoadFoundation(context.Background(), Command{})
	if err != nil {
		t.Fatalf("LoadFoundation() error = %v", err)
	}
	if snapshot.WorkspaceName != "My learning lab" || snapshot.WorkspaceRoot != root {
		t.Errorf("snapshot workspace = %q at %q", snapshot.WorkspaceName, snapshot.WorkspaceRoot)
	}
	if len(snapshot.Checks) != 3 {
		t.Fatalf("checks = %#v, want 3", snapshot.Checks)
	}
	wantNames := []string{"Workspace initialized", "Database healthy", "Configuration loaded"}
	for index, check := range snapshot.Checks {
		if check.Name != wantNames[index] || !check.OK || check.Detail != "" {
			t.Errorf("check[%d] = %#v, want healthy %q", index, check, wantNames[index])
		}
	}
	if !store.closed || factory.openRoot != root {
		t.Errorf("database probe root = %q, closed = %v", factory.openRoot, store.closed)
	}
	if snapshot.Settings[config.KeyUIColor].String() != "never" {
		t.Errorf("resolved ui.color = %q", snapshot.Settings[config.KeyUIColor].String())
	}
	if snapshot.LearningPath {
		t.Error("Foundation snapshot unexpectedly reports a learning path")
	}
}

func TestLoadFoundationKeepsPartialDiagnostics(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "project")
	service := NewService(
		&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}},
		func() (string, error) { return root, nil },
	)

	snapshot, err := service.LoadFoundation(context.Background(), Command{})
	if err != nil {
		t.Fatalf("LoadFoundation() error = %v", err)
	}
	if !snapshot.Checks[0].OK || snapshot.Checks[1].OK || snapshot.Checks[2].OK {
		t.Fatalf("partial checks = %#v", snapshot.Checks)
	}
	if snapshot.Checks[1].Detail == "" || snapshot.Checks[2].Detail == "" {
		t.Errorf("failed checks lack actionable details: %#v", snapshot.Checks)
	}
	if snapshot.WorkspaceName != filepath.Base(root) {
		t.Errorf("fallback workspace name = %q", snapshot.WorkspaceName)
	}
}

func TestLoadFoundationPropagatesDiscoveryAndCancellation(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("workspace metadata is corrupt")
	service := NewService(
		&recordingWorkspaceService{discoverErr: wantErr},
		func() (string, error) { return "/project", nil },
	)
	if _, err := service.LoadFoundation(context.Background(), Command{}); !errors.Is(err, wantErr) {
		t.Fatalf("LoadFoundation() error = %v, want discovery error", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.LoadFoundation(ctx, Command{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadFoundation(cancelled) error = %v", err)
	}
}
