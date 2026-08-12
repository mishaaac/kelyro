package artifactfs

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSandboxResolveRejectsPathInjection(t *testing.T) {
	root := t.TempDir()
	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatalf("NewSandbox() error = %v", err)
	}

	invalid := []string{
		"",
		".",
		"..",
		filepath.Join("..", "outside"),
		"nested/../../outside",
		`nested\..\..\outside`,
		filepath.Join(root, "absolute"),
		`C:\outside`,
		`\\server\share\outside`,
	}
	for _, path := range invalid {
		t.Run(path, func(t *testing.T) {
			if _, err := sandbox.Resolve(path); !errors.Is(err, ErrInvalidPath) && !errors.Is(err, ErrSandboxEscape) {
				t.Fatalf("Resolve(%q) error = %v, want invalid or escaping path", path, err)
			}
		})
	}

	want := filepath.Join(root, "exercises", "01", "main.go")
	got, err := sandbox.Resolve(filepath.Join("exercises", "01", "main.go"))
	if err != nil {
		t.Fatalf("Resolve(valid) error = %v", err)
	}
	if got != want {
		t.Fatalf("Resolve(valid) = %q, want %q", got, want)
	}
}

func TestSandboxResolveRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation is not available: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}

	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatalf("NewSandbox() error = %v", err)
	}
	if _, err := sandbox.Resolve(filepath.Join("escape", "student.txt")); !errors.Is(err, ErrSandboxEscape) {
		t.Fatalf("Resolve(symlink escape) error = %v, want ErrSandboxEscape", err)
	}
}

func TestSandboxAllowsSymlinkWithinRoot(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("create target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(root, "linked")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symlink creation is not available: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}

	sandbox, err := NewSandbox(root)
	if err != nil {
		t.Fatalf("NewSandbox() error = %v", err)
	}
	if _, err := sandbox.Resolve(filepath.Join("linked", "artifact.md")); err != nil {
		t.Fatalf("Resolve(internal symlink) error = %v", err)
	}
}
