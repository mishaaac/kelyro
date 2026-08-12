package editoros

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/editor"
)

func TestDetectHonorsConfiguredExecutablePathWithSpaces(t *testing.T) {
	t.Parallel()

	configured := filepath.Join("Program Files", "Editor", "editor.exe")
	real := filepath.Join(string(filepath.Separator), "real tools", "editor.exe")
	service := testService("linux", map[string]string{configured: real}, nil)

	selection, err := service.Detect(configured)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if selection.Executable != real || selection.Name != configured || selection.SystemDefault {
		t.Errorf("Detect() = %+v", selection)
	}
}

func TestDetectUsesFirstInstalledSupportedEditor(t *testing.T) {
	t.Parallel()

	service := testService("linux", map[string]string{
		"vim":    "/usr/bin/vim",
		"cursor": "/opt/cursor",
	}, nil)
	selection, err := service.Detect("")
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if selection.Name != "Vim" || selection.Executable != "/usr/bin/vim" {
		t.Errorf("Detect() = %+v, want Vim", selection)
	}
}

func TestDetectRecognizesEverySupportedEditor(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		command string
		name    string
	}{
		{command: "code", name: "Visual Studio Code"},
		{command: "nvim", name: "Neovim"},
		{command: "vim", name: "Vim"},
		{command: "zed", name: "Zed"},
		{command: "cursor", name: "Cursor"},
	} {
		t.Run(test.command, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join("bin", test.command)
			service := testService("linux", map[string]string{test.command: path}, nil)
			selection, err := service.Detect("")
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if selection.Name != test.name || selection.Executable != path {
				t.Errorf("Detect() = %+v, want %s at %s", selection, test.name, path)
			}
		})
	}
}

func TestConfiguredEditorMissingDoesNotSilentlyFallback(t *testing.T) {
	t.Parallel()

	service := testService("linux", map[string]string{"xdg-open": "/usr/bin/xdg-open"}, nil)
	_, err := service.Detect("missing-editor")
	if !errors.Is(err, editor.ErrUnavailable) || !strings.Contains(err.Error(), "missing-editor") {
		t.Fatalf("Detect() error = %v, want actionable unavailable error", err)
	}
}

func TestOpenBuildsSeparateArgumentsForEditorsAndPathsWithSpaces(t *testing.T) {
	t.Parallel()

	target := filepath.Join("workspace with spaces", "LEARNING.md")
	var got command
	service := testService("linux", map[string]string{"code": "/usr/bin/code"}, func(_ context.Context, command command) error {
		got = command
		return nil
	})

	selection, err := service.Open(context.Background(), target, "")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if selection.Name != "Visual Studio Code" {
		t.Errorf("Open() selection = %+v", selection)
	}
	if got.executable != "/usr/bin/code" || !reflect.DeepEqual(got.args, []string{target}) {
		t.Errorf("launch command = %#v", got)
	}
}

func TestOpenBuildsNativeSystemDefaultCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		goos     string
		name     string
		path     string
		wantArgs []string
	}{
		{goos: "linux", name: "xdg-open", path: "/usr/bin/xdg-open", wantArgs: []string{"artifact path"}},
		{goos: "darwin", name: "open", path: "/usr/bin/open", wantArgs: []string{"artifact path"}},
		{goos: "windows", name: "rundll32.exe", path: `C:\\Windows\\System32\\rundll32.exe`, wantArgs: []string{"url.dll,FileProtocolHandler", "artifact path"}},
	}
	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			t.Parallel()
			var got command
			service := testService(test.goos, map[string]string{test.name: test.path}, func(_ context.Context, command command) error {
				got = command
				return nil
			})
			selection, err := service.Open(context.Background(), "artifact path", "")
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			if !selection.SystemDefault || selection.Name != "system default" {
				t.Errorf("Open() selection = %+v", selection)
			}
			if got.executable != test.path || !reflect.DeepEqual(got.args, test.wantArgs) {
				t.Errorf("launch command = %#v, want executable %q args %#v", got, test.path, test.wantArgs)
			}
		})
	}
}

func TestOpenReportsMissingFallbackAndLaunchErrors(t *testing.T) {
	t.Parallel()

	service := testService("linux", nil, nil)
	if _, err := service.Open(context.Background(), "LEARNING.md", ""); !errors.Is(err, editor.ErrUnavailable) {
		t.Fatalf("Open() missing fallback error = %v", err)
	}

	wantErr := errors.New("launch failed")
	service = testService("linux", map[string]string{"nvim": "/usr/bin/nvim"}, func(context.Context, command) error { return wantErr })
	if _, err := service.Open(context.Background(), "LEARNING.md", ""); !errors.Is(err, wantErr) {
		t.Fatalf("Open() launch error = %v", err)
	}
}

func testService(goos string, paths map[string]string, run runFunc) *Service {
	if run == nil {
		run = func(context.Context, command) error { return nil }
	}
	return &Service{
		goos: goos,
		lookup: func(name string) (string, error) {
			if path, ok := paths[name]; ok {
				return path, nil
			}
			return "", errors.New("not found")
		},
		run: run,
	}
}
