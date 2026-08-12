package doctorsqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestProbeChecksHealthyFoundationDatabase(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".kelyro"), 0o755); err != nil {
		t.Fatalf("create internal directory: %v", err)
	}
	health := New().Check(context.Background(), root)
	if health.DatabaseError != nil || health.MigrationError != nil || health.ArtifactIndexError != nil {
		t.Fatalf("Check() = %#v", health)
	}
}

func TestProbeKeepsStorageFailureVisibleInEveryDependentCheck(t *testing.T) {
	t.Parallel()

	health := New().Check(context.Background(), t.TempDir())
	if health.DatabaseError == nil || health.MigrationError == nil || health.ArtifactIndexError == nil {
		t.Fatalf("Check() = %#v, want all unavailable", health)
	}
}
