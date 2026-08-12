package artifactfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/platform"
)

func TestFactoryPersistsOwnershipAcrossWorkspaceStoreSessions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatalf("WorkspaceInternalDir() error = %v", err)
	}
	if err := os.Mkdir(internal, 0o755); err != nil {
		t.Fatalf("Mkdir(.kelyro): %v", err)
	}

	request := artifacts.WriteRequest{
		Path:            filepath.Join("00-roadmap", "ROADMAP.md"),
		Ownership:       artifacts.SystemGeneratedHumanReadable,
		CreatedBy:       "foundation-markdown",
		Content:         []byte("# Roadmap\n"),
		ExpectedVersion: "foundation-roadmap/v1",
	}
	factory := NewFactory()
	first, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open(first) error = %v", err)
	}
	if _, err := first.Write(context.Background(), request); err != nil {
		t.Fatalf("Write(first) error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close(first) error = %v", err)
	}

	destination := filepath.Join(root, request.Path)
	humanContent := []byte("# My roadmap\n")
	if err := os.WriteFile(destination, humanContent, 0o644); err != nil {
		t.Fatalf("WriteFile(human edit): %v", err)
	}
	second, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open(second) error = %v", err)
	}
	defer second.Close()

	if _, err := second.Write(context.Background(), request); !errors.Is(err, ErrModified) {
		t.Fatalf("Write(modified) error = %v, want ErrModified", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile(modified): %v", err)
	}
	if string(got) != string(humanContent) {
		t.Fatalf("modified document = %q, want %q", got, humanContent)
	}
}
