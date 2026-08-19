package learningdb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/platform"
)

func TestFactoryPersistsProfileAcrossStoreLifetimes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	internal, err := platform.WorkspaceInternalDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(internal, 0o755); err != nil {
		t.Fatal(err)
	}
	factory := NewFactory("test")
	factory.now = func() time.Time { return time.Date(2026, time.August, 19, 15, 0, 0, 0, time.UTC) }
	store, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	name := "Ada"
	if _, err := store.Profiles().Edit(context.Background(), application.ProfileChanges{DisplayName: &name}); err != nil {
		t.Fatalf("Edit() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := factory.Open(context.Background(), root)
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	student, err := reopened.Profiles().Show(context.Background())
	if err != nil || student.Profile.DisplayName != "Ada" {
		t.Fatalf("persisted profile = (%+v, %v)", student.Profile, err)
	}
}
