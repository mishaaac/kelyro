package auditsqlite

import (
	"context"
	"os"
	"testing"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/platform"
)

func TestAuditTrailSurvivesRestart(t *testing.T) {
	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatalf("WorkspaceInternalDir() error = %v", err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	factory := NewFactory("1.2.3")
	store, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if err := store.Record(context.Background(), audit.Event{
		Name: "import.completed", Actor: audit.ActorUser, Subject: "course-1",
		Metadata: map[string]string{"source": "local", "provider_token": "must-not-persist"},
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	defer reopened.Close()
	entries, err := reopened.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, entry := range entries {
		if entry.Event == "import.completed" {
			if entry.Actor != audit.ActorUser || entry.AppVersion != "1.2.3" || entry.Metadata["provider_token"] != "[REDACTED]" {
				t.Fatalf("persisted event = %+v", entry)
			}
			return
		}
	}
	t.Fatalf("import.completed event missing from %+v", entries)
}
