// Package workspacefs implements Kelyro workspace lifecycle operations using
// the local filesystem.
package workspacefs

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/workspace"
)

type fileSystem interface {
	Stat(string) (fs.FileInfo, error)
	ReadFile(string) ([]byte, error)
	MkdirTemp(string, string) (string, error)
	MkdirAll(string, fs.FileMode) error
	WriteFile(string, []byte, fs.FileMode) error
	Rename(string, string) error
	RemoveAll(string) error
}

type osFileSystem struct{}

func (osFileSystem) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }
func (osFileSystem) ReadFile(name string) ([]byte, error)  { return os.ReadFile(name) }
func (osFileSystem) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}
func (osFileSystem) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}
func (osFileSystem) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}
func (osFileSystem) Rename(oldPath, newPath string) error { return os.Rename(oldPath, newPath) }
func (osFileSystem) RemoveAll(path string) error          { return os.RemoveAll(path) }

// Service manages workspace identity and structure on the local filesystem.
type Service struct {
	fs         fileSystem
	now        func() time.Time
	random     io.Reader
	appVersion string
}

// New creates a local filesystem workspace service.
func New(appVersion string) *Service {
	if appVersion == "" {
		appVersion = "unknown"
	}

	return &Service{
		fs:         osFileSystem{},
		now:        time.Now,
		random:     rand.Reader,
		appVersion: appVersion,
	}
}

// Discover searches startDir and each parent for a valid .kelyro directory.
func (service *Service) Discover(startDir string) (workspace.Workspace, error) {
	root, err := service.existingDirectory(startDir)
	if err != nil {
		return workspace.Workspace{}, err
	}

	for {
		internal, pathErr := platform.WorkspaceInternalDir(root)
		if pathErr != nil {
			return workspace.Workspace{}, pathErr
		}

		info, statErr := service.fs.Stat(internal)
		switch {
		case statErr == nil:
			if !info.IsDir() {
				return workspace.Workspace{}, fmt.Errorf("%w: %s is not a directory", workspace.ErrInvalid, internal)
			}
			return service.load(root)
		case !errors.Is(statErr, fs.ErrNotExist):
			return workspace.Workspace{}, fmt.Errorf("inspect workspace marker %s: %w", internal, statErr)
		}

		parent := filepath.Dir(root)
		if parent == root {
			break
		}
		root = parent
	}

	return workspace.Workspace{}, fmt.Errorf("%w from %s", workspace.ErrNotFound, startDir)
}

// Init creates a workspace atomically enough to roll back every reported
// initialization failure. Existing valid workspaces are returned unchanged.
func (service *Service) Init(root string, options workspace.InitOptions) (workspace.Workspace, error) {
	normalizedRoot, err := service.existingDirectory(root)
	if err != nil {
		return workspace.Workspace{}, err
	}

	internal, err := platform.WorkspaceInternalDir(normalizedRoot)
	if err != nil {
		return workspace.Workspace{}, err
	}
	if _, statErr := service.fs.Stat(internal); statErr == nil {
		return service.load(normalizedRoot)
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return workspace.Workspace{}, fmt.Errorf("inspect workspace target %s: %w", internal, statErr)
	}

	if !options.AllowNested {
		parent := filepath.Dir(normalizedRoot)
		if parent != normalizedRoot {
			outer, discoverErr := service.Discover(parent)
			switch {
			case discoverErr == nil:
				return workspace.Workspace{}, fmt.Errorf("%w: %s is inside %s; use --allow-nested to confirm", workspace.ErrNested, normalizedRoot, outer.Root)
			case !errors.Is(discoverErr, workspace.ErrNotFound):
				return workspace.Workspace{}, discoverErr
			}
		}
	}

	metadata, err := service.newMetadata()
	if err != nil {
		return workspace.Workspace{}, err
	}
	encodedMetadata, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return workspace.Workspace{}, fmt.Errorf("encode workspace metadata: %w", err)
	}
	encodedMetadata = append(encodedMetadata, '\n')

	staging, err := service.fs.MkdirTemp(normalizedRoot, ".kelyro-init-")
	if err != nil {
		return workspace.Workspace{}, fmt.Errorf("create workspace staging directory: %w", err)
	}
	defer service.fs.RemoveAll(staging) // Best-effort cleanup before or after rename.

	for _, directory := range []string{"state", "cache", "backups", "logs"} {
		if err := service.fs.MkdirAll(filepath.Join(staging, directory), 0o755); err != nil {
			return workspace.Workspace{}, fmt.Errorf("create workspace %s directory: %w", directory, err)
		}
	}
	metadataPath := filepath.Join(staging, "workspace.json")
	if err := service.fs.WriteFile(metadataPath, encodedMetadata, 0o600); err != nil {
		return workspace.Workspace{}, fmt.Errorf("write workspace metadata: %w", err)
	}

	if err := service.fs.Rename(staging, internal); err != nil {
		return workspace.Workspace{}, fmt.Errorf("commit workspace internals: %w", err)
	}

	created, err := service.load(normalizedRoot)
	if err != nil {
		return workspace.Workspace{}, service.rollback(normalizedRoot, err)
	}

	return created, nil
}

// Validate checks workspace identity, metadata, and required machine-owned
// directories without modifying them.
func (service *Service) Validate(root string) error {
	_, err := service.load(root)
	return err
}

func (service *Service) load(root string) (workspace.Workspace, error) {
	normalizedRoot, err := service.existingDirectory(root)
	if err != nil {
		return workspace.Workspace{}, err
	}

	requiredDirectories := []func(string) (string, error){
		platform.WorkspaceInternalDir,
		platform.WorkspaceStatePath,
		platform.WorkspaceCacheDir,
		platform.WorkspaceBackupDir,
		platform.WorkspaceLogDir,
	}
	for _, resolve := range requiredDirectories {
		path, pathErr := resolve(normalizedRoot)
		if pathErr != nil {
			return workspace.Workspace{}, pathErr
		}
		info, statErr := service.fs.Stat(path)
		if statErr != nil {
			return workspace.Workspace{}, fmt.Errorf("%w: required directory %s: %v", workspace.ErrInvalid, path, statErr)
		}
		if !info.IsDir() {
			return workspace.Workspace{}, fmt.Errorf("%w: %s is not a directory", workspace.ErrInvalid, path)
		}
	}

	metadataPath, err := platform.WorkspaceMetadataPath(normalizedRoot)
	if err != nil {
		return workspace.Workspace{}, err
	}
	encoded, err := service.fs.ReadFile(metadataPath)
	if err != nil {
		return workspace.Workspace{}, fmt.Errorf("%w: read metadata %s: %v", workspace.ErrInvalid, metadataPath, err)
	}

	var metadata workspace.Metadata
	if err := json.Unmarshal(encoded, &metadata); err != nil {
		return workspace.Workspace{}, fmt.Errorf("%w: decode metadata %s: %v", workspace.ErrInvalid, metadataPath, err)
	}
	if metadata.WorkspaceID == "" {
		return workspace.Workspace{}, fmt.Errorf("%w: workspace_id is empty", workspace.ErrInvalid)
	}
	if metadata.SchemaVersion != workspace.SchemaVersion {
		return workspace.Workspace{}, fmt.Errorf("%w: unsupported schema_version %d", workspace.ErrInvalid, metadata.SchemaVersion)
	}
	if metadata.CreatedAt.IsZero() {
		return workspace.Workspace{}, fmt.Errorf("%w: created_at is empty", workspace.ErrInvalid)
	}
	if metadata.AppVersion == "" {
		return workspace.Workspace{}, fmt.Errorf("%w: app_version is empty", workspace.ErrInvalid)
	}

	return workspace.Workspace{Root: normalizedRoot, Metadata: metadata}, nil
}

func (service *Service) existingDirectory(path string) (string, error) {
	normalized, err := platform.NormalizePath(path)
	if err != nil {
		return "", fmt.Errorf("normalize workspace path: %w", err)
	}
	info, err := service.fs.Stat(normalized)
	if err != nil {
		return "", fmt.Errorf("inspect workspace root %s: %w", normalized, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace root %s is not a directory", normalized)
	}

	return normalized, nil
}

func (service *Service) newMetadata() (workspace.Metadata, error) {
	identifier, err := newWorkspaceID(service.random)
	if err != nil {
		return workspace.Metadata{}, fmt.Errorf("generate workspace ID: %w", err)
	}

	return workspace.Metadata{
		WorkspaceID:   identifier,
		SchemaVersion: workspace.SchemaVersion,
		CreatedAt:     service.now().UTC(),
		AppVersion:    service.appVersion,
	}, nil
}

func (service *Service) rollback(root string, cause error) error {
	internal, pathErr := platform.WorkspaceInternalDir(root)
	if pathErr != nil {
		return fmt.Errorf("initialize workspace: %w; resolve rollback path: %v", cause, pathErr)
	}
	if removeErr := service.fs.RemoveAll(internal); removeErr != nil {
		return fmt.Errorf("initialize workspace: %w; rollback internals: %v", cause, removeErr)
	}
	return cause
}

func newWorkspaceID(random io.Reader) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(random, value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

var _ workspace.Service = (*Service)(nil)
