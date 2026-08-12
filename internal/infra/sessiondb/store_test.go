package sessiondb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/session"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

func TestStorePersistsTransactionalSessionLifecycle(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".kelyro"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	database, err := sqlite.Open(ctx, root)
	if err != nil {
		t.Fatalf("sqlite.Open() error = %v", err)
	}
	if err := database.Repositories().State.Set(ctx, "foundation", "session", []byte("invalid")); err != nil {
		t.Fatalf("seed corrupt state: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close seed database: %v", err)
	}

	factory := NewFactory()
	factory.now = func() time.Time { return time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC) }
	store, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	resumed, err := store.Resume(ctx)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if !resumed.Recovered {
		t.Fatalf("Resume() = %+v, want recovery", resumed)
	}
	state := resumed.State
	state.LastView = session.ViewDoctor
	if err := store.Checkpoint(ctx, state); err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}
	if err := store.Complete(ctx, state); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	store, err = factory.Open(ctx, root)
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	resumed, err = store.Resume(ctx)
	if err != nil {
		t.Fatalf("resumed reopen error = %v", err)
	}
	if resumed.PreviousIncomplete || resumed.State.LastView != session.ViewDoctor {
		t.Fatalf("resumed reopen = %+v", resumed)
	}
}
