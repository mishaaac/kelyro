package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	applicationDirectoryName      = "kelyro"
	globalConfigFileName          = "config.toml"
	workspaceDirectoryName        = ".kelyro"
	workspaceConfigFileName       = "config.toml"
	workspaceDatabaseFileName     = "learning.db"
	workspaceMetadataFileName     = "workspace.json"
	workspaceStateDirectoryName   = "state"
	workspaceCacheDirectoryName   = "cache"
	workspaceBackupDirectoryName  = "backups"
	workspaceLogDirectoryName     = "logs"
	workspaceLearningFileName     = "LEARNING.md"
	workspaceRoadmapDirectoryName = "00-roadmap"
	workspaceRoadmapFileName      = "ROADMAP.md"
)

var errEmptyPath = errors.New("path must not be empty")

// NormalizePath resolves path against the current working directory and
// returns its cleaned absolute representation. It does not require the path to
// exist.
func NormalizePath(path string) (string, error) {
	if path == "" {
		return "", errEmptyPath
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make path absolute: %w", err)
	}

	return filepath.Clean(absolute), nil
}

// UserHomeDir returns the current user's home directory as a cleaned absolute
// path.
func UserHomeDir() (string, error) {
	directory, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home directory: %w", err)
	}

	return normalizeStandardDirectory("user home", directory)
}

// UserConfigDir returns the operating system's user-specific configuration
// directory as a cleaned absolute path.
func UserConfigDir() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}

	return normalizeStandardDirectory("user config", directory)
}

// UserCacheDir returns the operating system's user-specific cache directory as
// a cleaned absolute path.
func UserCacheDir() (string, error) {
	directory, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache directory: %w", err)
	}

	return normalizeStandardDirectory("user cache", directory)
}

// TempDir returns the operating system's default temporary directory as a
// cleaned absolute path.
func TempDir() (string, error) {
	return normalizeStandardDirectory("temporary", os.TempDir())
}

// GlobalConfigDir returns Kelyro's directory below the current user's native
// configuration directory.
func GlobalConfigDir() (string, error) {
	base, err := UserConfigDir()
	if err != nil {
		return "", err
	}

	return childPath(base, applicationDirectoryName)
}

// GlobalConfigPath returns Kelyro's user-scoped configuration file.
func GlobalConfigPath() (string, error) {
	directory, err := GlobalConfigDir()
	if err != nil {
		return "", err
	}

	return childPath(directory, globalConfigFileName)
}

// GlobalCacheDir returns Kelyro's directory below the current user's native
// cache directory.
func GlobalCacheDir() (string, error) {
	base, err := UserCacheDir()
	if err != nil {
		return "", err
	}

	return childPath(base, applicationDirectoryName)
}

// WorkspaceInternalDir returns the machine-owned directory for a workspace.
func WorkspaceInternalDir(root string) (string, error) {
	return childPath(root, workspaceDirectoryName)
}

// WorkspaceDBPath returns the path reserved for the workspace database.
func WorkspaceDBPath(root string) (string, error) {
	return workspaceChildPath(root, workspaceDatabaseFileName)
}

// WorkspaceMetadataPath returns the machine-owned workspace identity file.
func WorkspaceMetadataPath(root string) (string, error) {
	return workspaceChildPath(root, workspaceMetadataFileName)
}

// WorkspaceConfigPath returns the project-scoped configuration file.
func WorkspaceConfigPath(root string) (string, error) {
	return workspaceChildPath(root, workspaceConfigFileName)
}

// WorkspaceStatePath returns the path reserved for workspace state files.
func WorkspaceStatePath(root string) (string, error) {
	return workspaceChildPath(root, workspaceStateDirectoryName)
}

// WorkspaceCacheDir returns the directory for workspace-local disposable data.
func WorkspaceCacheDir(root string) (string, error) {
	return workspaceChildPath(root, workspaceCacheDirectoryName)
}

// WorkspaceBackupDir returns the directory reserved for workspace backups.
func WorkspaceBackupDir(root string) (string, error) {
	return workspaceChildPath(root, workspaceBackupDirectoryName)
}

// WorkspaceLogDir returns the directory for workspace-local logs.
func WorkspaceLogDir(root string) (string, error) {
	return workspaceChildPath(root, workspaceLogDirectoryName)
}

// WorkspaceLearningPath returns the human-owned learning document path.
func WorkspaceLearningPath(root string) (string, error) {
	return childPath(root, workspaceLearningFileName)
}

// WorkspaceRoadmapPath returns the visible Foundation roadmap document path.
func WorkspaceRoadmapPath(root string) (string, error) {
	directory, err := childPath(root, workspaceRoadmapDirectoryName)
	if err != nil {
		return "", err
	}
	return childPath(directory, workspaceRoadmapFileName)
}

func workspaceChildPath(root, name string) (string, error) {
	internal, err := WorkspaceInternalDir(root)
	if err != nil {
		return "", err
	}

	return childPath(internal, name)
}

func childPath(parent, name string) (string, error) {
	if parent == "" {
		return "", errEmptyPath
	}

	return NormalizePath(filepath.Join(parent, name))
}

func normalizeStandardDirectory(name, directory string) (string, error) {
	if directory == "" {
		return "", fmt.Errorf("%s directory: %w", name, errEmptyPath)
	}

	normalized, err := NormalizePath(directory)
	if err != nil {
		return "", fmt.Errorf("normalize %s directory: %w", name, err)
	}

	return normalized, nil
}
