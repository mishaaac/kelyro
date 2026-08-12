package platform

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "relative", path: filepath.Join("relative", "project")},
		{name: "spaces", path: filepath.Join("Learning Projects", "Go Basics")},
		{name: "dot segments", path: filepath.Join("relative", "discarded", "..", "MiXeD")},
		{name: "absolute", path: filepath.Join(t.TempDir(), "Project")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			want, err := filepath.Abs(test.path)
			if err != nil {
				t.Fatalf("filepath.Abs(%q): %v", test.path, err)
			}
			want = filepath.Clean(want)

			got, err := NormalizePath(test.path)
			if err != nil {
				t.Fatalf("NormalizePath(%q): %v", test.path, err)
			}
			if got != want {
				t.Errorf("NormalizePath(%q) = %q, want %q", test.path, got, want)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("NormalizePath(%q) = %q, want an absolute path", test.path, got)
			}
		})
	}
}

func TestNormalizePathRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	if _, err := NormalizePath(""); err == nil {
		t.Fatal("NormalizePath(\"\") error = nil, want an error")
	}
}

func TestWorkspacePaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		root string
	}{
		{name: "relative root", root: filepath.Join("Learning Projects", "Go")},
		{name: "cleaned root", root: filepath.Join("Learning Projects", "discarded", "..", "MiXeD")},
		{name: "absolute root with spaces", root: filepath.Join(t.TempDir(), "Backend Go")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root, err := NormalizePath(test.root)
			if err != nil {
				t.Fatalf("NormalizePath(%q): %v", test.root, err)
			}

			assertPathHelper(t, "WorkspaceInternalDir", WorkspaceInternalDir, test.root, filepath.Join(root, ".kelyro"))
			assertPathHelper(t, "WorkspaceDBPath", WorkspaceDBPath, test.root, filepath.Join(root, ".kelyro", "learning.db"))
			assertPathHelper(t, "WorkspaceMetadataPath", WorkspaceMetadataPath, test.root, filepath.Join(root, ".kelyro", "workspace.json"))
			assertPathHelper(t, "WorkspaceConfigPath", WorkspaceConfigPath, test.root, filepath.Join(root, ".kelyro", "config.toml"))
			assertPathHelper(t, "WorkspaceStatePath", WorkspaceStatePath, test.root, filepath.Join(root, ".kelyro", "state"))
			assertPathHelper(t, "WorkspaceCacheDir", WorkspaceCacheDir, test.root, filepath.Join(root, ".kelyro", "cache"))
			assertPathHelper(t, "WorkspaceBackupDir", WorkspaceBackupDir, test.root, filepath.Join(root, ".kelyro", "backups"))
			assertPathHelper(t, "WorkspaceLogDir", WorkspaceLogDir, test.root, filepath.Join(root, ".kelyro", "logs"))
			assertPathHelper(t, "WorkspaceLearningPath", WorkspaceLearningPath, test.root, filepath.Join(root, "LEARNING.md"))
			assertPathHelper(t, "WorkspaceRoadmapPath", WorkspaceRoadmapPath, test.root, filepath.Join(root, "00-roadmap", "ROADMAP.md"))
		})
	}
}

func TestWorkspacePathsRejectEmptyRoot(t *testing.T) {
	t.Parallel()

	helpers := []struct {
		name string
		call func(string) (string, error)
	}{
		{name: "WorkspaceInternalDir", call: WorkspaceInternalDir},
		{name: "WorkspaceDBPath", call: WorkspaceDBPath},
		{name: "WorkspaceMetadataPath", call: WorkspaceMetadataPath},
		{name: "WorkspaceConfigPath", call: WorkspaceConfigPath},
		{name: "WorkspaceStatePath", call: WorkspaceStatePath},
		{name: "WorkspaceCacheDir", call: WorkspaceCacheDir},
		{name: "WorkspaceBackupDir", call: WorkspaceBackupDir},
		{name: "WorkspaceLogDir", call: WorkspaceLogDir},
		{name: "WorkspaceLearningPath", call: WorkspaceLearningPath},
		{name: "WorkspaceRoadmapPath", call: WorkspaceRoadmapPath},
	}

	for _, helper := range helpers {
		t.Run(helper.name, func(t *testing.T) {
			t.Parallel()

			if _, err := helper.call(""); err == nil {
				t.Fatalf("%s(\"\") error = nil, want an error", helper.name)
			}
		})
	}
}

func TestGlobalDirectoriesUseNativeBases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		base   func() (string, error)
		global func() (string, error)
	}{
		{name: "config", base: UserConfigDir, global: GlobalConfigDir},
		{name: "cache", base: UserCacheDir, global: GlobalCacheDir},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			base, err := test.base()
			if err != nil {
				t.Fatalf("resolve %s base: %v", test.name, err)
			}

			got, err := test.global()
			if err != nil {
				t.Fatalf("resolve global %s directory: %v", test.name, err)
			}
			want := filepath.Join(base, "kelyro")
			if got != want {
				t.Errorf("global %s directory = %q, want %q", test.name, got, want)
			}
		})
	}
}

func TestGlobalConfigPathUsesNativeConfigDirectory(t *testing.T) {
	t.Parallel()

	directory, err := GlobalConfigDir()
	if err != nil {
		t.Fatalf("GlobalConfigDir() error = %v", err)
	}
	path, err := GlobalConfigPath()
	if err != nil {
		t.Fatalf("GlobalConfigPath() error = %v", err)
	}
	if want := filepath.Join(directory, "config.toml"); path != want {
		t.Errorf("GlobalConfigPath() = %q, want %q", path, want)
	}
}

func TestGlobalUpdateCachePathUsesNativeCacheDirectory(t *testing.T) {
	t.Parallel()
	directory, err := GlobalCacheDir()
	if err != nil {
		t.Fatalf("GlobalCacheDir() error = %v", err)
	}
	path, err := GlobalUpdateCachePath()
	if err != nil {
		t.Fatalf("GlobalUpdateCachePath() error = %v", err)
	}
	if want := filepath.Join(directory, "updates.json"); path != want {
		t.Errorf("GlobalUpdateCachePath() = %q, want %q", path, want)
	}
}

func TestStandardDirectoriesAreCleanAbsolutePaths(t *testing.T) {
	t.Parallel()

	directories := []struct {
		name string
		call func() (string, error)
	}{
		{name: "home", call: UserHomeDir},
		{name: "config", call: UserConfigDir},
		{name: "cache", call: UserCacheDir},
		{name: "temp", call: TempDir},
	}

	for _, directory := range directories {
		t.Run(directory.name, func(t *testing.T) {
			t.Parallel()

			got, err := directory.call()
			if err != nil {
				t.Fatalf("resolve %s directory: %v", directory.name, err)
			}
			if !filepath.IsAbs(got) {
				t.Errorf("%s directory = %q, want an absolute path", directory.name, got)
			}
			if got != filepath.Clean(got) {
				t.Errorf("%s directory = %q, want cleaned path %q", directory.name, got, filepath.Clean(got))
			}
		})
	}
}

func TestNormalizePathWithWindowsDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive-letter semantics are provided by filepath on Windows")
	}

	input := filepath.Join(`C:\Users`, "Learner", "Learning Projects", "..", "MiXeD")
	got, err := NormalizePath(input)
	if err != nil {
		t.Fatalf("NormalizePath(%q): %v", input, err)
	}
	if volume := filepath.VolumeName(got); volume == "" {
		t.Errorf("NormalizePath(%q) = %q, want a drive volume", input, got)
	}
	if got != filepath.Clean(input) {
		t.Errorf("NormalizePath(%q) = %q, want %q", input, got, filepath.Clean(input))
	}
}

func assertPathHelper(
	t *testing.T,
	name string,
	helper func(string) (string, error),
	root string,
	want string,
) {
	t.Helper()

	got, err := helper(root)
	if err != nil {
		t.Fatalf("%s(%q): %v", name, root, err)
	}
	if got != want {
		t.Errorf("%s(%q) = %q, want %q", name, root, got, want)
	}
}
