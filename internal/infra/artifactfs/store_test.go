package artifactfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
)

var storeTestTime = time.Date(2026, time.August, 12, 10, 30, 0, 0, time.UTC)

type memoryIndex struct {
	entries map[string]artifacts.Artifact
}

func newMemoryIndex() *memoryIndex {
	return &memoryIndex{entries: map[string]artifacts.Artifact{}}
}

func (index *memoryIndex) Get(_ context.Context, path string) (artifacts.Artifact, bool, error) {
	artifact, found := index.entries[path]
	return artifact, found, nil
}

func (index *memoryIndex) Put(_ context.Context, artifact artifacts.Artifact) error {
	index.entries[artifact.Path] = artifact
	return nil
}

func (index *memoryIndex) Delete(_ context.Context, path string) error {
	delete(index.entries, path)
	return nil
}

func TestStoreCreatesAndSafelyRegeneratesHumanReadableArtifact(t *testing.T) {
	root := t.TempDir()
	index := newMemoryIndex()
	store := newTestStore(t, root, index)
	request := WriteRequest{
		Path:            "LEARNING.md",
		Ownership:       artifacts.SystemGeneratedHumanReadable,
		CreatedBy:       "foundation",
		Content:         []byte("first\n"),
		ExpectedVersion: "learning/v1",
	}

	created, err := store.Write(context.Background(), request)
	if err != nil {
		t.Fatalf("Write(create) error = %v", err)
	}
	if created.ContentHash != artifacts.Hash(request.Content) || created.CreatedAt != storeTestTime {
		t.Fatalf("created metadata = %+v", created)
	}

	store.now = func() time.Time { return storeTestTime.Add(time.Minute) }
	request.Content = []byte("second\n")
	regenerated, err := store.Write(context.Background(), request)
	if err != nil {
		t.Fatalf("Write(regenerate) error = %v", err)
	}
	if regenerated.CreatedAt != created.CreatedAt || !regenerated.LastGeneratedAt.Equal(storeTestTime.Add(time.Minute)) {
		t.Fatalf("regenerated timestamps = %+v, created = %+v", regenerated, created)
	}
	assertFileContent(t, filepath.Join(root, request.Path), request.Content)
	assertNoStagingFiles(t, root)
}

func TestStoreDetectsExternalModificationWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	index := newMemoryIndex()
	store := newTestStore(t, root, index)
	request := WriteRequest{
		Path:      "LEARNING.md",
		Ownership: artifacts.SystemGeneratedHumanReadable,
		CreatedBy: "foundation",
		Content:   []byte("generated\n"),
	}
	if _, err := store.Write(context.Background(), request); err != nil {
		t.Fatalf("Write(create) error = %v", err)
	}
	before := index.entries[request.Path]
	studentContent := []byte("my notes\n")
	if err := os.WriteFile(filepath.Join(root, request.Path), studentContent, 0o644); err != nil {
		t.Fatalf("modify artifact: %v", err)
	}

	request.Content = []byte("replacement\n")
	if _, err := store.Write(context.Background(), request); !errors.Is(err, ErrModified) {
		t.Fatalf("Write(modified) error = %v, want ErrModified", err)
	}
	assertFileContent(t, filepath.Join(root, request.Path), studentContent)
	if got := index.entries[request.Path]; got != before {
		t.Fatalf("conflict changed index: got %+v, want %+v", got, before)
	}
}

func TestStoreDoesNotOverwriteUntrackedOrStudentOwnedFiles(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root, newMemoryIndex())
	existing := filepath.Join(root, "ROADMAP.md")
	if err := os.WriteFile(existing, []byte("student roadmap\n"), 0o644); err != nil {
		t.Fatalf("create untracked file: %v", err)
	}

	_, err := store.Write(context.Background(), WriteRequest{
		Path: "ROADMAP.md", Ownership: artifacts.SystemGeneratedHumanReadable,
		CreatedBy: "foundation", Content: []byte("generated\n"),
	})
	if !errors.Is(err, ErrUntracked) {
		t.Fatalf("Write(untracked) error = %v, want ErrUntracked", err)
	}
	assertFileContent(t, existing, []byte("student roadmap\n"))

	_, err = store.Write(context.Background(), WriteRequest{
		Path: "main.go", Ownership: artifacts.StudentOwned,
		CreatedBy: "foundation", Content: []byte("package main\n"),
	})
	if !errors.Is(err, ErrStudentOwned) {
		t.Fatalf("Write(student-owned) error = %v, want ErrStudentOwned", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "main.go")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("student-owned path was created: %v", statErr)
	}
}

func TestStoreWritesMachineOwnedContentOnlyUnderInternalDirectory(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, root, newMemoryIndex())
	request := WriteRequest{
		Path: filepath.Join(".kelyro", "cache", "result"), Ownership: artifacts.MachineOwned,
		CreatedBy: "foundation", Content: []byte("opaque"),
	}
	if _, err := store.Write(context.Background(), request); err != nil {
		t.Fatalf("Write(machine-owned) error = %v", err)
	}
	assertFileContent(t, filepath.Join(root, request.Path), request.Content)

	request.Path = "notes.md"
	if _, err := store.Write(context.Background(), request); !errors.Is(err, ErrOwnershipMismatch) {
		t.Fatalf("Write(misclassified) error = %v, want ErrOwnershipMismatch", err)
	}
}

func TestStoreFailedAtomicReplacePreservesOriginalAndCleansStaging(t *testing.T) {
	root := t.TempDir()
	index := newMemoryIndex()
	store := newTestStore(t, root, index)
	request := WriteRequest{
		Path: "LEARNING.md", Ownership: artifacts.SystemGeneratedHumanReadable,
		CreatedBy: "foundation", Content: []byte("original\n"),
	}
	if _, err := store.Write(context.Background(), request); err != nil {
		t.Fatalf("Write(create) error = %v", err)
	}
	store.replace = func(_, _ string) error { return errors.New("injected replace failure") }
	request.Content = []byte("new\n")
	if _, err := store.Write(context.Background(), request); err == nil {
		t.Fatal("Write() succeeded with failing atomic replacement")
	}
	assertFileContent(t, filepath.Join(root, request.Path), []byte("original\n"))
	assertNoStagingFiles(t, root)
}

func newTestStore(t *testing.T, root string, index artifacts.Index) *Store {
	t.Helper()
	store, err := New(root, index)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	store.now = func() time.Time { return storeTestTime }
	return store
}

func assertFileContent(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("content of %s = %q, want %q", path, got, want)
	}
}

func assertNoStagingFiles(t *testing.T, root string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, ".kelyro-artifact-*.tmp"))
	if err != nil {
		t.Fatalf("glob staging files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("staging files remain: %v", matches)
	}
}
