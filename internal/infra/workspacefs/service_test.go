package workspacefs

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestInitCreatesWorkspaceAndIsIdempotent(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	root := filepath.Join(parent, "Backend Go")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatalf("Mkdir(%q): %v", root, err)
	}

	createdAt := time.Date(2026, time.August, 12, 14, 30, 0, 0, time.FixedZone("test", -5*60*60))
	service := deterministicService(createdAt)
	created, err := service.Init(root, workspace.InitOptions{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	if created.Root != root {
		t.Errorf("Init() root = %q, want %q", created.Root, root)
	}
	if created.Metadata.WorkspaceID != "00010203-0405-4607-8809-0a0b0c0d0e0f" {
		t.Errorf("workspace_id = %q, want deterministic UUID", created.Metadata.WorkspaceID)
	}
	if created.Metadata.SchemaVersion != workspace.SchemaVersion {
		t.Errorf("schema_version = %d, want %d", created.Metadata.SchemaVersion, workspace.SchemaVersion)
	}
	if want := createdAt.UTC(); !created.Metadata.CreatedAt.Equal(want) || created.Metadata.CreatedAt.Location() != time.UTC {
		t.Errorf("created_at = %v, want UTC %v", created.Metadata.CreatedAt, want)
	}
	if created.Metadata.AppVersion != "test-version" {
		t.Errorf("app_version = %q, want test-version", created.Metadata.AppVersion)
	}

	for _, path := range workspacePaths(t, root) {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Errorf("required path %q: %v", path, statErr)
			continue
		}
		if !info.IsDir() {
			t.Errorf("required path %q is not a directory", path)
		}
	}
	metadataPath, _ := platform.WorkspaceMetadataPath(root)
	if _, err := os.Stat(metadataPath); err != nil {
		t.Errorf("metadata path %q: %v", metadataPath, err)
	}
	internalPath, _ := platform.WorkspaceInternalDir(root)
	entries, err := os.ReadDir(internalPath)
	if err != nil {
		t.Fatalf("ReadDir(.kelyro): %v", err)
	}
	if len(entries) != 5 {
		t.Errorf(".kelyro entries = %d, want only metadata plus four required directories", len(entries))
	}
	repeated, err := service.Init(root, workspace.InitOptions{})
	if err != nil {
		t.Fatalf("repeated Init() error = %v", err)
	}
	if repeated.Metadata.WorkspaceID != created.Metadata.WorkspaceID {
		t.Errorf("repeated workspace_id = %q, want %q", repeated.Metadata.WorkspaceID, created.Metadata.WorkspaceID)
	}
	learningPath, _ := platform.WorkspaceLearningPath(root)
	if _, err := os.Stat(learningPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("workspace adapter created visible Markdown outside artifact pipeline: %v", err)
	}
}

func TestInitPreservesExistingLearningDocument(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	learningPath, _ := platform.WorkspaceLearningPath(root)
	want := []byte("# Existing human plan\n")
	if err := os.WriteFile(learningPath, want, 0o644); err != nil {
		t.Fatalf("WriteFile(LEARNING.md): %v", err)
	}

	service := deterministicService(time.Now())
	if _, err := service.Init(root, workspace.InitOptions{}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	got, err := os.ReadFile(learningPath)
	if err != nil {
		t.Fatalf("ReadFile(LEARNING.md): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Init() overwrote LEARNING.md: got %q, want %q", got, want)
	}
}

func TestDiscoverFindsWorkspaceFromSubdirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	service := deterministicService(time.Now())
	created, err := service.Init(root, workspace.InitOptions{})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	subdirectory := filepath.Join(root, "exercises", "week one")
	if err := os.MkdirAll(subdirectory, 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}

	discovered, err := service.Discover(subdirectory)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if discovered.Root != created.Root || discovered.Metadata.WorkspaceID != created.Metadata.WorkspaceID {
		t.Errorf("Discover() = %#v, want workspace %#v", discovered, created)
	}
}

func TestInitRejectsNestedWorkspaceUnlessExplicitlyAllowed(t *testing.T) {
	t.Parallel()

	outer := t.TempDir()
	service := deterministicService(time.Now())
	if _, err := service.Init(outer, workspace.InitOptions{}); err != nil {
		t.Fatalf("Init(outer) error = %v", err)
	}
	inner := filepath.Join(outer, "courses", "backend")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatalf("MkdirAll(inner): %v", err)
	}

	if _, err := service.Init(inner, workspace.InitOptions{}); !errors.Is(err, workspace.ErrNested) {
		t.Fatalf("Init(nested) error = %v, want ErrNested", err)
	}
	internal, _ := platform.WorkspaceInternalDir(inner)
	if _, err := os.Stat(internal); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("rejected nested init left %q: %v", internal, err)
	}

	created, err := service.Init(inner, workspace.InitOptions{AllowNested: true})
	if err != nil {
		t.Fatalf("Init(nested, allow) error = %v", err)
	}
	if created.Root != inner {
		t.Errorf("allowed nested root = %q, want %q", created.Root, inner)
	}
}

func TestInitRejectsInvalidRoots(t *testing.T) {
	t.Parallel()

	service := deterministicService(time.Now())
	parent := t.TempDir()
	file := filepath.Join(parent, "not-a-directory")
	if err := os.WriteFile(file, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	for _, root := range []string{filepath.Join(parent, "missing"), file} {
		if _, err := service.Init(root, workspace.InitOptions{}); err == nil {
			t.Errorf("Init(%q) error = nil, want invalid root error", root)
		}
	}
}

func TestInitReportsPermissionFailureWithoutCreatingWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	service := deterministicService(time.Now())
	service.fs = failingFileSystem{fileSystem: osFileSystem{}, mkdirTempErr: fs.ErrPermission}

	_, err := service.Init(root, workspace.InitOptions{})
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Init() error = %v, want permission error", err)
	}
	assertWorkspaceAbsent(t, root)
}

func TestInitRollsBackAfterCommittedInternalsFailValidation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	service := deterministicService(time.Now())
	service.fs = failingFileSystem{fileSystem: osFileSystem{}, readFileErr: errors.New("injected metadata read failure")}

	if _, err := service.Init(root, workspace.InitOptions{}); err == nil {
		t.Fatal("Init() error = nil, want injected failure")
	}
	assertWorkspaceAbsent(t, root)
}

func TestInitDoesNotRepairOrOverwriteInvalidWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	internal, _ := platform.WorkspaceInternalDir(root)
	if err := os.Mkdir(internal, 0o755); err != nil {
		t.Fatalf("Mkdir(.kelyro): %v", err)
	}

	service := deterministicService(time.Now())
	if _, err := service.Init(root, workspace.InitOptions{}); !errors.Is(err, workspace.ErrInvalid) {
		t.Fatalf("Init(invalid existing) error = %v, want ErrInvalid", err)
	}
	entries, err := os.ReadDir(internal)
	if err != nil {
		t.Fatalf("ReadDir(.kelyro): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Init(invalid existing) modified .kelyro: %v", entries)
	}
}

func TestValidateRejectsMalformedMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	service := deterministicService(time.Now())
	if _, err := service.Init(root, workspace.InitOptions{}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	metadataPath, _ := platform.WorkspaceMetadataPath(root)
	if err := os.WriteFile(metadataPath, []byte(`{"schema_version":1}`), 0o600); err != nil {
		t.Fatalf("WriteFile(metadata): %v", err)
	}

	if err := service.Validate(root); !errors.Is(err, workspace.ErrInvalid) {
		t.Fatalf("Validate() error = %v, want ErrInvalid", err)
	}
}

func deterministicService(now time.Time) *Service {
	service := New("test-version")
	service.now = func() time.Time { return now }
	pattern := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}
	service.random = bytes.NewReader(bytes.Repeat(pattern, 16))
	return service
}

func workspacePaths(t *testing.T, root string) []string {
	t.Helper()

	helpers := []func(string) (string, error){
		platform.WorkspaceInternalDir,
		platform.WorkspaceStatePath,
		platform.WorkspaceCacheDir,
		platform.WorkspaceBackupDir,
		platform.WorkspaceLogDir,
	}
	paths := make([]string, 0, len(helpers))
	for _, helper := range helpers {
		path, err := helper(root)
		if err != nil {
			t.Fatalf("resolve required path: %v", err)
		}
		paths = append(paths, path)
	}
	return paths
}

func assertWorkspaceAbsent(t *testing.T, root string) {
	t.Helper()

	internal, _ := platform.WorkspaceInternalDir(root)
	if _, err := os.Stat(internal); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("workspace internals remain at %q: %v", internal, err)
	}
	staging, err := filepath.Glob(filepath.Join(root, ".kelyro-init-*"))
	if err != nil {
		t.Fatalf("Glob(staging): %v", err)
	}
	if len(staging) != 0 {
		t.Errorf("workspace staging directories remain: %v", staging)
	}
}

type failingFileSystem struct {
	fileSystem
	mkdirTempErr error
	readFileErr  error
}

func (filesystem failingFileSystem) MkdirTemp(dir, pattern string) (string, error) {
	if filesystem.mkdirTempErr != nil {
		return "", filesystem.mkdirTempErr
	}
	return filesystem.fileSystem.MkdirTemp(dir, pattern)
}

func (filesystem failingFileSystem) ReadFile(name string) ([]byte, error) {
	if filesystem.readFileErr != nil {
		return nil, filesystem.readFileErr
	}
	return filesystem.fileSystem.ReadFile(name)
}
