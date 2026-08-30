package backupfs

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/infra/artifactfs"
	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestCreateListAndRestoreAllowlistedWorkspaceState(t *testing.T) {
	root := createTestWorkspace(t)
	service := New("1.2.3", sqlite.SnapshotValidator{})

	created, err := service.Create(context.Background(), root, backup.CreateOptions{Reason: "manual", Retention: 5})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" || created.DatabaseSchemaVersion != sqlite.LatestSchemaVersion() || created.FileCount != 4 {
		t.Fatalf("Create() = %+v", created)
	}
	listed, err := service.List(context.Background(), root)
	if err != nil || len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("List() = (%+v, %v)", listed, err)
	}

	internal := filepath.Join(root, ".kelyro")
	mustWrite(t, filepath.Join(internal, "config.toml"), "changed config")
	mustWrite(t, filepath.Join(internal, "state", "session.json"), "changed state")
	mustWrite(t, filepath.Join(internal, "state", "new.json"), "remove me")
	restored, err := service.Restore(context.Background(), root, created.ID)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.ID != created.ID {
		t.Fatalf("Restore() = %+v", restored)
	}
	assertFileContent(t, filepath.Join(internal, "config.toml"), "project config")
	assertFileContent(t, filepath.Join(internal, "state", "session.json"), "session state")
	if _, err := os.Stat(filepath.Join(internal, "state", "new.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("new state after restore Stat() error = %v, want not exist", err)
	}

	backupPath := filepath.Join(internal, "backups", created.ID)
	err = filepath.WalkDir(backupPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		encoded, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(encoded), "secret-value") || strings.Contains(path, "cache") || strings.Contains(path, "logs") {
			t.Errorf("backup included secret or disposable data in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect backup: %v", err)
	}
}

func TestCorruptBackupFailsBeforeChangingWorkspace(t *testing.T) {
	root := createTestWorkspace(t)
	service := New("1.2.3", sqlite.SnapshotValidator{})
	created, err := service.Create(context.Background(), root, backup.CreateOptions{Retention: 5})
	if err != nil {
		t.Fatal(err)
	}
	currentConfig := filepath.Join(root, ".kelyro", "config.toml")
	mustWrite(t, currentConfig, "current must survive")
	archivedConfig := filepath.Join(root, ".kelyro", "backups", created.ID, "data", "config.toml")
	mustWrite(t, archivedConfig, "corrupt")

	_, err = service.Restore(context.Background(), root, created.ID)
	if !errors.Is(err, backup.ErrCorrupt) {
		t.Fatalf("Restore() error = %v, want ErrCorrupt", err)
	}
	assertFileContent(t, currentConfig, "current must survive")
	if _, err := service.List(context.Background(), root); !errors.Is(err, backup.ErrCorrupt) {
		t.Fatalf("List() error = %v, want ErrCorrupt", err)
	}
}

func TestRestoreCommitFailureRollsBackEveryOriginal(t *testing.T) {
	root := createTestWorkspace(t)
	service := New("1.2.3", sqlite.SnapshotValidator{})
	created, err := service.Create(context.Background(), root, backup.CreateOptions{Retention: 5})
	if err != nil {
		t.Fatal(err)
	}
	internal := filepath.Join(root, ".kelyro")
	mustWrite(t, filepath.Join(internal, "config.toml"), "current config")
	mustWrite(t, filepath.Join(internal, "state", "session.json"), "current state")

	realRename := service.rename
	failed := false
	service.rename = func(oldPath, newPath string) error {
		if !failed && strings.Contains(oldPath, ".restore-stage-") && filepath.Base(newPath) == "state" {
			failed = true
			return errors.New("injected commit failure")
		}
		return realRename(oldPath, newPath)
	}
	_, err = service.Restore(context.Background(), root, created.ID)
	if err == nil || !strings.Contains(err.Error(), "injected commit failure") {
		t.Fatalf("Restore() error = %v", err)
	}
	assertFileContent(t, filepath.Join(internal, "config.toml"), "current config")
	assertFileContent(t, filepath.Join(internal, "state", "session.json"), "current state")
}

func TestRestoreKeepsGeneratedArtifactsCoherentWithRestoredIndex(t *testing.T) {
	root := createTestWorkspace(t)
	service := New("1.2.3", sqlite.SnapshotValidator{})
	paths := []string{
		"LEARNING.md",
		filepath.Join("00-roadmap", "ROADMAP.md"),
		filepath.Join("00-roadmap", "PROGRESS.md"),
	}
	writeGeneratedArtifacts(t, root, paths, "backup state\n")

	created, err := service.Create(context.Background(), root, backup.CreateOptions{Retention: 5})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	writeGeneratedArtifacts(t, root, paths, "newer state\n")

	if _, err := service.Restore(context.Background(), root, created.ID); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	writeGeneratedArtifacts(t, root, paths, "regenerated restored state\n")
	for _, path := range paths {
		assertFileContent(t, filepath.Join(root, path), "regenerated restored state\n")
	}
}

func TestRestoreKeepsHumanEditProtectionForUnbackedGeneratedArtifact(t *testing.T) {
	root := createTestWorkspace(t)
	service := New("1.2.3", sqlite.SnapshotValidator{})
	path := "LEARNING.md"
	writeGeneratedArtifacts(t, root, []string{path}, "backup state\n")

	created, err := service.Create(context.Background(), root, backup.CreateOptions{Retention: 5})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	writeGeneratedArtifacts(t, root, []string{path}, "newer state\n")
	mustWrite(t, filepath.Join(root, path), "human edit\n")

	if _, err := service.Restore(context.Background(), root, created.ID); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	store, err := artifactfs.NewFactory("1.2.3").Open(context.Background(), root)
	if err != nil {
		t.Fatalf("open artifact store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close artifact store: %v", err)
		}
	}()
	_, err = store.Write(context.Background(), artifacts.WriteRequest{
		Path: path, Ownership: artifacts.SystemGeneratedHumanReadable,
		CreatedBy: "backup-regression", Content: []byte("regenerated restored state\n"), ExpectedVersion: "test/v1",
	})
	if !errors.Is(err, artifactfs.ErrModified) {
		t.Fatalf("Write() error = %v, want ErrModified", err)
	}
	assertFileContent(t, filepath.Join(root, path), "human edit\n")
}

func TestCreateAppliesRetention(t *testing.T) {
	root := createTestWorkspace(t)
	service := New("1.2.3", sqlite.SnapshotValidator{})
	base := time.Date(2026, time.August, 12, 10, 0, 0, 0, time.UTC)
	call := 0
	service.now = func() time.Time {
		call++
		return base.Add(time.Duration(call) * time.Second)
	}
	for index := 0; index < 3; index++ {
		if _, err := service.Create(context.Background(), root, backup.CreateOptions{Retention: 2}); err != nil {
			t.Fatalf("Create(%d) error = %v", index, err)
		}
	}
	listed, err := service.List(context.Background(), root)
	if err != nil || len(listed) != 2 {
		t.Fatalf("List() = (%+v, %v), want two retained backups", listed, err)
	}
}

func createTestWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	internal := filepath.Join(root, ".kelyro")
	for _, directory := range []string{"state", "cache", "backups", "logs"} {
		if err := os.MkdirAll(filepath.Join(internal, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	metadata, err := json.Marshal(workspace.Metadata{
		WorkspaceID: "workspace-test", SchemaVersion: workspace.SchemaVersion,
		CreatedAt: time.Date(2026, time.August, 12, 9, 0, 0, 0, time.UTC), AppVersion: "1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(internal, "workspace.json"), metadata, 0o600); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(internal, "config.toml"), "project config")
	mustWrite(t, filepath.Join(internal, "state", "session.json"), "session state")
	mustWrite(t, filepath.Join(internal, "cache", "provider-secret"), "secret-value")
	mustWrite(t, filepath.Join(internal, "logs", "kelyro.log"), "secret-value")
	mustWrite(t, filepath.Join(internal, "secret.txt"), "secret-value")
	database, err := sqlite.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := platform.WorkspaceBackupDir(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("ReadFile(%s) = %q, want %q", path, got, want)
	}
}

func writeGeneratedArtifacts(t *testing.T, root string, paths []string, content string) {
	t.Helper()
	store, err := artifactfs.NewFactory("1.2.3").Open(context.Background(), root)
	if err != nil {
		t.Fatalf("open artifact store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close artifact store: %v", err)
		}
	}()
	for _, path := range paths {
		if _, err := store.Write(context.Background(), artifacts.WriteRequest{
			Path: path, Ownership: artifacts.SystemGeneratedHumanReadable,
			CreatedBy: "backup-regression", Content: []byte(content), ExpectedVersion: "test/v1",
		}); err != nil {
			t.Fatalf("write generated artifact %s: %v", path, err)
		}
	}
}
