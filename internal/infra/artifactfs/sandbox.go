// Package artifactfs protects workspace files through ownership-aware writes
// and path confinement.
package artifactfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrInvalidPath identifies an empty, absolute, or traversing sandbox path.
	ErrInvalidPath = errors.New("invalid sandbox path")
	// ErrSandboxEscape identifies a path whose symlinks resolve outside the root.
	ErrSandboxEscape = errors.New("path escapes sandbox")
)

// Sandbox resolves relative paths beneath one physical root.
type Sandbox struct {
	root         string
	physicalRoot string
}

// NewSandbox validates an existing directory as the confinement root.
func NewSandbox(root string) (*Sandbox, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve sandbox root %s: %w", root, err)
	}
	absolute = filepath.Clean(absolute)
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, fmt.Errorf("inspect sandbox root %s: %w", absolute, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("sandbox root %s is not a directory", absolute)
	}
	physical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve sandbox root symlinks: %w", err)
	}
	physical, err = filepath.Abs(physical)
	if err != nil {
		return nil, fmt.Errorf("normalize physical sandbox root: %w", err)
	}
	return &Sandbox{root: absolute, physicalRoot: filepath.Clean(physical)}, nil
}

// Resolve returns the lexical destination for a relative path after proving
// that every existing ancestor remains under the sandbox's physical root.
func (sandbox *Sandbox) Resolve(relative string) (string, error) {
	if invalidRelativePath(relative) {
		return "", fmt.Errorf("%w: %q", ErrInvalidPath, relative)
	}

	cleaned := filepath.Clean(relative)
	target := filepath.Join(sandbox.root, cleaned)
	if !within(sandbox.root, target) {
		return "", fmt.Errorf("%w: %q", ErrSandboxEscape, relative)
	}

	existing := target
	for {
		_, err := os.Lstat(existing)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("inspect sandbox path %s: %w", existing, err)
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return "", fmt.Errorf("%w: no existing sandbox ancestor for %q", ErrSandboxEscape, relative)
		}
		existing = parent
	}

	physical, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", fmt.Errorf("resolve sandbox path %s: %w", existing, err)
	}
	physical, err = filepath.Abs(physical)
	if err != nil {
		return "", fmt.Errorf("normalize sandbox path %s: %w", physical, err)
	}
	if !within(sandbox.physicalRoot, filepath.Clean(physical)) {
		return "", fmt.Errorf("%w: %q resolves through %s", ErrSandboxEscape, relative, existing)
	}
	return target, nil
}

func invalidRelativePath(path string) bool {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return true
	}
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `\`) {
		return true
	}
	if len(path) >= 3 && isASCIIAlpha(path[0]) && path[1] == ':' && (path[2] == '/' || path[2] == '\\') {
		return true
	}
	for _, component := range strings.FieldsFunc(path, func(character rune) bool {
		return character == '/' || character == '\\'
	}) {
		if component == ".." {
			return true
		}
	}
	return filepath.Clean(path) == "."
}

func within(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func isASCIIAlpha(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}
