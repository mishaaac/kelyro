package portabilityfs

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/infra/workspacefs"
	"github.com/mishaaac/kelyro/internal/portability"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestHumanAndFullExportImportRoundTrip(t *testing.T) {
	root := portableTestWorkspace(t)
	service := New("1.2.3", sqlite.SnapshotValidator{})

	humanArchive := filepath.Join(t.TempDir(), "human.kelyro.tar.gz")
	human, err := service.Export(context.Background(), root, portability.ExportOptions{Mode: portability.ModeHuman, OutputPath: humanArchive})
	if err != nil {
		t.Fatalf("Export(human) error = %v", err)
	}
	if human.FileCount != 3 || human.Mode != portability.ModeHuman {
		t.Fatalf("Export(human) = %+v", human)
	}
	humanDestination := filepath.Join(t.TempDir(), "readable copy")
	importedHuman, err := service.Import(context.Background(), portability.ImportOptions{
		ArchivePath: humanArchive, Destination: humanDestination, Conflicts: portability.ConflictFail,
	})
	if err != nil || len(importedHuman.Creates) != 3 {
		t.Fatalf("Import(human) = (%+v, %v)", importedHuman, err)
	}
	assertPortableContent(t, filepath.Join(humanDestination, "notes", "ideas.md"), "student notes\n")
	assertAbsent(t, filepath.Join(humanDestination, "exercise.go"))
	assertAbsent(t, filepath.Join(humanDestination, ".kelyro"))

	fullArchive := filepath.Join(t.TempDir(), "full.kelyro.tar.gz")
	full, err := service.Export(context.Background(), root, portability.ExportOptions{Mode: portability.ModeFull, OutputPath: fullArchive})
	if err != nil {
		t.Fatalf("Export(full) error = %v", err)
	}
	if full.FileCount != 7 || full.Mode != portability.ModeFull {
		t.Fatalf("Export(full) = %+v", full)
	}
	fullDestination := filepath.Join(t.TempDir(), "portable workspace")
	importedFull, err := service.Import(context.Background(), portability.ImportOptions{
		ArchivePath: fullArchive, Destination: fullDestination, Conflicts: portability.ConflictFail,
	})
	if err != nil || len(importedFull.Creates) != full.FileCount {
		t.Fatalf("Import(full) = (%+v, %v)", importedFull, err)
	}
	if err := workspacefs.New("1.2.3").Validate(fullDestination); err != nil {
		t.Fatalf("imported full workspace is invalid: %v", err)
	}
	assertPortableContent(t, filepath.Join(fullDestination, ".kelyro", "config.toml"), "[ui]\ncolor = \"auto\"\n")
	assertPortableContent(t, filepath.Join(fullDestination, ".kelyro", "state", "session.json"), "session state")
	assertAbsent(t, filepath.Join(fullDestination, ".kelyro", "logs", "kelyro.jsonl"))
	assertAbsent(t, filepath.Join(fullDestination, ".kelyro", "cache", "provider-token"))
}

func TestImportRejectsMalformedArchiveAndTraversalBeforeWriting(t *testing.T) {
	service := New("1.2.3", nil)
	destination := filepath.Join(t.TempDir(), "destination")
	malformed := filepath.Join(t.TempDir(), "malformed.tar.gz")
	if err := os.WriteFile(malformed, []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := service.Import(context.Background(), portability.ImportOptions{
		ArchivePath: malformed, Destination: destination, Conflicts: portability.ConflictFail,
	})
	if !errors.Is(err, portability.ErrMalformed) {
		t.Fatalf("Import(malformed) error = %v, want ErrMalformed", err)
	}
	assertAbsent(t, destination)

	content := []byte("escape")
	digest := sha256.Sum256(content)
	manifest := portability.Manifest{
		FormatVersion: portability.FormatVersion, Mode: portability.ModeHuman,
		CreatedAt: time.Now().UTC(), AppVersion: "1.2.3", WorkspaceID: "workspace-test", WorkspaceSchemaVersion: 1,
		Files: []portability.File{{Path: "../escape.md", Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:]), Ownership: artifacts.StudentOwned}},
	}
	traversal := filepath.Join(t.TempDir(), "traversal.tar.gz")
	writeTestArchive(t, traversal, manifest, map[string][]byte{"../escape.md": content})
	_, err = service.Import(context.Background(), portability.ImportOptions{
		ArchivePath: traversal, Destination: destination, Conflicts: portability.ConflictFail,
	})
	if !errors.Is(err, portability.ErrMalformed) {
		t.Fatalf("Import(traversal) error = %v, want ErrMalformed", err)
	}
	assertAbsent(t, filepath.Join(filepath.Dir(destination), "escape.md"))
}

func TestImportConflictStrategiesAndDryRun(t *testing.T) {
	root := portableTestWorkspace(t)
	service := New("1.2.3", sqlite.SnapshotValidator{})
	archive := filepath.Join(t.TempDir(), "human.tar.gz")
	if _, err := service.Export(context.Background(), root, portability.ExportOptions{Mode: portability.ModeHuman, OutputPath: archive}); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	conflicting := filepath.Join(destination, "notes", "ideas.md")
	mustPortableWrite(t, conflicting, "local changes\n")

	report, err := service.Import(context.Background(), portability.ImportOptions{
		ArchivePath: archive, Destination: destination, DryRun: true, Conflicts: portability.ConflictFail,
	})
	if err != nil || len(report.Conflicts) != 1 || report.Conflicts[0] != "notes/ideas.md" {
		t.Fatalf("Import(dry-run) = (%+v, %v)", report, err)
	}
	assertPortableContent(t, conflicting, "local changes\n")

	_, err = service.Import(context.Background(), portability.ImportOptions{
		ArchivePath: archive, Destination: destination, Conflicts: portability.ConflictFail,
	})
	if !errors.Is(err, portability.ErrConflict) {
		t.Fatalf("Import(fail) error = %v, want ErrConflict", err)
	}
	assertPortableContent(t, conflicting, "local changes\n")

	kept, err := service.Import(context.Background(), portability.ImportOptions{
		ArchivePath: archive, Destination: destination, Conflicts: portability.ConflictKeep,
	})
	if err != nil || len(kept.Skips) != 1 {
		t.Fatalf("Import(keep) = (%+v, %v)", kept, err)
	}
	assertPortableContent(t, conflicting, "local changes\n")

	replaced, err := service.Import(context.Background(), portability.ImportOptions{
		ArchivePath: archive, Destination: destination, Conflicts: portability.ConflictOverwrite,
	})
	if err != nil || len(replaced.Replaces) != 1 {
		t.Fatalf("Import(overwrite) = (%+v, %v)", replaced, err)
	}
	assertPortableContent(t, conflicting, "student notes\n")
}

func TestExportExcludesSecretsAndUsesPortablePaths(t *testing.T) {
	root := portableTestWorkspace(t)
	archivePath := filepath.Join(t.TempDir(), "full.tar.gz")
	service := New("1.2.3", sqlite.SnapshotValidator{})
	if _, err := service.Export(context.Background(), root, portability.ExportOptions{Mode: portability.ModeFull, OutputPath: archivePath}); err != nil {
		t.Fatal(err)
	}
	entries := readTestArchive(t, archivePath)
	for name, content := range entries {
		if strings.Contains(name, "\\") || strings.Contains(name, "logs/") || strings.Contains(name, "cache/") || strings.Contains(name, "backups/") ||
			strings.Contains(name, ".env") || strings.Contains(string(content), "secret-value") {
			t.Errorf("archive leaked excluded or non-portable data in %q", name)
		}
	}
	for _, invalid := range []string{
		"../notes.md", `notes\\ideas.md`, "C:/notes.md", "notes/CON.md", "notes/trailing. ",
		"notes/question?.md", "notes/star*.md", "notes/quote\".md", "notes/less<than.md",
		"notes/greater>than.md", "notes/pipe|name.md", "notes/new\nline.md", "notes/delete\x7f.md",
	} {
		if validPortablePath(invalid) {
			t.Errorf("validPortablePath(%q) = true", invalid)
		}
	}
	for _, valid := range []string{"notes/learning plan.md", "notas/ámbito-世界.md"} {
		if !validPortablePath(valid) {
			t.Errorf("validPortablePath(%q) = false", valid)
		}
	}
}

func portableTestWorkspace(t *testing.T) string {
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
	mustPortableWrite(t, filepath.Join(internal, "config.toml"), "[ui]\ncolor = \"auto\"\n")
	mustPortableWrite(t, filepath.Join(internal, "state", "session.json"), "session state")
	mustPortableWrite(t, filepath.Join(internal, "logs", "kelyro.jsonl"), "secret-value")
	mustPortableWrite(t, filepath.Join(internal, "cache", "provider-token"), "secret-value")
	mustPortableWrite(t, filepath.Join(root, ".env"), "TOKEN=secret-value\n")
	mustPortableWrite(t, filepath.Join(root, "LEARNING.md"), "# Learning\n")
	mustPortableWrite(t, filepath.Join(root, "00-roadmap", "ROADMAP.md"), "# Roadmap\n")
	mustPortableWrite(t, filepath.Join(root, "notes", "ideas.md"), "student notes\n")
	mustPortableWrite(t, filepath.Join(root, "exercise.go"), "package exercise\n")
	database, err := sqlite.Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeTestArchive(t *testing.T, archivePath string, manifest portability.Manifest, files map[string][]byte) {
	t.Helper()
	output, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	compressed := gzip.NewWriter(output)
	archive := tar.NewWriter(compressed)
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := archive.WriteHeader(&tar.Header{Name: manifestName, Mode: 0o600, Size: int64(len(encoded))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(encoded); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := archive.WriteHeader(&tar.Header{Name: dataPrefix + name, Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func readTestArchive(t *testing.T, archivePath string) map[string][]byte {
	t.Helper()
	input, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	compressed, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	archive := tar.NewReader(compressed)
	entries := make(map[string][]byte)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return entries
		}
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(archive)
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = content
	}
}

func mustPortableWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertPortableContent(t *testing.T, path, want string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if string(content) != want {
		t.Fatalf("ReadFile(%s) = %q, want %q", path, content, want)
	}
}

func assertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat(%s) error = %v, want not exist", path, err)
	}
}
