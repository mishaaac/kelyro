//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const commandTimeout = 30 * time.Second

func TestFoundationWorkspaceLifecycle(t *testing.T) {
	root := moduleRoot(t)
	binary := buildBinary(t, root)

	t.Run("new workspace", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")

		for _, relative := range []string{
			".kelyro",
			filepath.Join(".kelyro", "state"),
			filepath.Join(".kelyro", "cache"),
			filepath.Join(".kelyro", "backups"),
			filepath.Join(".kelyro", "logs"),
		} {
			assertDirectory(t, filepath.Join(test.workspace, relative))
		}
		for _, relative := range []string{
			filepath.Join(".kelyro", "workspace.json"),
			filepath.Join(".kelyro", "learning.db"),
			"LEARNING.md",
			filepath.Join("00-roadmap", "ROADMAP.md"),
		} {
			assertRegularFile(t, filepath.Join(test.workspace, relative))
		}
	})

	t.Run("reopen and resume", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("piped stdin is not a Windows console handle; session and TUI state are covered by unit tests")
		}
		test := newScenario(t, binary)
		test.mustRun("init")
		completeLearnerSetup(t, test)
		workspaceLabel := "Workspace: " + filepath.Base(test.workspace)

		first := test.runInteractive(
			interaction{waitFor: workspaceLabel, send: "d"},
			interaction{waitFor: "\n  Doctor\n", send: "q"},
		)
		if !strings.Contains(normalizeOutput(first), "\n  Doctor\n") {
			t.Fatalf("first TUI session did not reach Doctor:\n%s", first)
		}

		second := test.runInteractive(interaction{waitFor: "\n  Doctor\n", send: "q"})
		if !strings.Contains(normalizeOutput(second), "\n  Doctor\n") {
			t.Fatalf("second TUI session did not resume Doctor:\n%s", second)
		}
	})

	t.Run("integrated onboarding initializes learner state", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("piped stdin is not a Windows console handle; setup persistence is covered by integration tests")
		}
		test := newScenario(t, binary)
		test.mustRun("init")
		output := completeLearnerSetup(t, test)
		for _, expected := range []string{"Review your setup before applying it.", "Apply this learner setup?", "Setup complete.", "Learning path ready."} {
			if !strings.Contains(normalizeOutput(output), expected) {
				t.Fatalf("setup output missing %q:\n%s", expected, output)
			}
		}
		status := test.mustRun("setup", "status")
		for _, expected := range []string{"Status: completed", "Curriculum: foundation-demo@1.0.0", "Source: fixture"} {
			if !strings.Contains(status, expected) {
				t.Fatalf("setup status missing %q:\n%s", expected, status)
			}
		}
		profile := test.mustRun("profile", "show")
		if !strings.Contains(profile, "Display name: Ada") {
			t.Fatalf("profile not initialized:\n%s", profile)
		}
		goal := test.mustRun("goal", "show")
		if !strings.Contains(goal, "[active] Understand ratios") {
			t.Fatalf("goal not initialized:\n%s", goal)
		}
	})

	t.Run("doctor is coherent and preserves managed state", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		tracked := []string{
			"LEARNING.md",
			filepath.Join("00-roadmap", "ROADMAP.md"),
			filepath.Join(".kelyro", "workspace.json"),
			filepath.Join(".kelyro", "learning.db"),
		}
		before := fileDigests(t, test.workspace, tracked)

		output := test.mustRun("doctor")
		for _, section := range []string{"Platform", "Kelyro"} {
			if !strings.Contains(output, section) {
				t.Fatalf("doctor output does not contain %q:\n%s", section, output)
			}
		}
		after := fileDigests(t, test.workspace, tracked)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("doctor changed managed or student state\nbefore: %v\nafter:  %v", before, after)
		}
	})

	t.Run("configuration survives restart and respects precedence", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		test.mustRun("--global", "config", "set", "ui.color", "always")
		test.mustRun("--project", "config", "set", "ui.color", "never")

		if got := strings.TrimSpace(test.mustRun("config", "get", "ui.color")); got != "never" {
			t.Fatalf("resolved ui.color after restart = %q, want never", got)
		}
		if got := strings.TrimSpace(test.mustRun("--global", "config", "get", "ui.color")); got != "always" {
			t.Fatalf("global ui.color after restart = %q, want always", got)
		}
		shown := test.mustRun("config", "show")
		if !strings.Contains(shown, "secret.e2e = configured (reference: fake:e2e)") {
			t.Fatalf("config output did not use the fake secret store:\n%s", shown)
		}
		if strings.Contains(shown, "e2e-secret-value-never-render") {
			t.Fatal("config output exposed the fake secret value")
		}
	})

	t.Run("student artifact is protected", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		learning := filepath.Join(test.workspace, "LEARNING.md")
		const studentChange = "\nStudent-owned E2E note.\n"
		file, err := os.OpenFile(learning, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString(studentChange); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		before := readFile(t, learning)

		output, code := test.run("init")
		if code == 0 {
			t.Fatalf("regeneration succeeded after a student edit:\n%s", output)
		}
		if !strings.Contains(strings.ToLower(output), "modified") {
			t.Fatalf("regeneration error is not actionable:\n%s", output)
		}
		if after := readFile(t, learning); !bytes.Equal(after, before) {
			t.Fatal("regeneration overwrote the student-owned LEARNING.md")
		}
	})

	t.Run("backup and restore", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		test.mustRun("config", "set", "workspace.name", "Before Backup")
		created := strings.Fields(test.mustRun("backup", "create"))
		if len(created) < 3 {
			t.Fatalf("unexpected backup create output: %q", strings.Join(created, " "))
		}
		backupID := created[len(created)-1]
		test.mustRun("config", "set", "workspace.name", "After Backup")
		test.mustRun("--yes", "backup", "restore", backupID)

		if got := strings.TrimSpace(test.mustRun("config", "get", "workspace.name")); got != "Before Backup" {
			t.Fatalf("restored workspace.name = %q, want Before Backup", got)
		}
	})

	t.Run("full export and import", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		test.mustRun("config", "set", "workspace.name", "Portable Workspace")
		learning := filepath.Join(test.workspace, "LEARNING.md")
		file, err := os.OpenFile(learning, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString("\nPortable student state.\n"); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}

		archive := filepath.Join(test.base, "foundation-export.kelyro.tar.gz")
		test.mustRun("--full", "--output", archive, "export")
		destination := filepath.Join(test.base, "imported workspace")
		if err := os.MkdirAll(destination, 0o755); err != nil {
			t.Fatal(err)
		}
		test.mustRun("--workspace", destination, "import", archive)

		for _, relative := range []string{
			"LEARNING.md",
			filepath.Join("00-roadmap", "ROADMAP.md"),
			filepath.Join(".kelyro", "config.toml"),
		} {
			source := readFile(t, filepath.Join(test.workspace, relative))
			imported := readFile(t, filepath.Join(destination, relative))
			if !bytes.Equal(source, imported) {
				t.Fatalf("imported %s differs from source", relative)
			}
		}
		if got := strings.TrimSpace(test.mustRun("--workspace", destination, "config", "get", "workspace.name")); got != "Portable Workspace" {
			t.Fatalf("imported workspace.name = %q", got)
		}
		test.mustRun("--workspace", destination, "doctor")
	})

	t.Run("offline Foundation commands", func(t *testing.T) {
		test := newScenario(t, binary)
		test.mustRun("init")
		if got := strings.TrimSpace(test.mustRun("config", "get", "privacy.allow_network")); got != "false" {
			t.Fatalf("privacy.allow_network = %q, want false", got)
		}
		updateOutput := test.mustRun("update", "check")
		if !strings.Contains(updateOutput, "network access is disabled by privacy policy") {
			t.Fatalf("offline update result is incoherent:\n%s", updateOutput)
		}
		if strings.Contains(updateOutput, "E2E network adapter was invoked") {
			t.Fatalf("offline command reached the fake network adapter:\n%s", updateOutput)
		}

		for _, command := range [][]string{
			{"roadmap"},
			{"doctor"},
			{"status"},
			{"backup", "list"},
			{"secrets", "status"},
		} {
			test.mustRun(command...)
		}
		assertNoFileNamed(t, test.cache, "updates.json")
	})
}

type scenario struct {
	testing   *testing.T
	binary    string
	base      string
	workspace string
	cache     string
	environ   []string
}

func newScenario(t *testing.T, binary string) scenario {
	t.Helper()
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace with spaces")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	configuration := filepath.Join(base, "config")
	cache := filepath.Join(base, "cache")
	emptyPath := filepath.Join(base, "empty-path")
	for _, directory := range []string{home, configuration, cache, emptyPath} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	environ := isolatedEnvironment(os.Environ(), map[string]string{
		"HOME":            home,
		"USERPROFILE":     home,
		"XDG_CONFIG_HOME": configuration,
		"XDG_CACHE_HOME":  cache,
		"APPDATA":         configuration,
		"LOCALAPPDATA":    cache,
		"PATH":            emptyPath,
		"NO_COLOR":        "1",
		"TERM":            "dumb",
	})
	return scenario{testing: t, binary: binary, base: base, workspace: workspace, cache: cache, environ: environ}
}

func (test scenario) run(args ...string) (string, int) {
	test.testing.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, test.binary, args...)
	command.Dir = test.workspace
	command.Env = test.environ
	output, err := command.CombinedOutput()
	if ctx.Err() != nil {
		test.testing.Fatalf("kelyro %s timed out:\n%s", strings.Join(args, " "), output)
	}
	if err == nil {
		return string(output), 0
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		test.testing.Fatalf("run kelyro %s: %v", strings.Join(args, " "), err)
	}
	return string(output), exitError.ExitCode()
}

func (test scenario) mustRun(args ...string) string {
	test.testing.Helper()
	output, code := test.run(args...)
	if code != 0 {
		test.testing.Fatalf("kelyro %s exited %d:\n%s", strings.Join(args, " "), code, output)
	}
	return output
}

type interaction struct {
	waitFor string
	send    string
}

func completeLearnerSetup(t *testing.T, test scenario) string {
	t.Helper()
	return test.runInteractive(
		interaction{waitFor: "What should Kelyro call you?", send: "Ada\r"},
		interaction{waitFor: "What do you want to learn?", send: "Understand ratios\r"},
		interaction{waitFor: "What subject or domain", send: "Mathematics\r"},
		interaction{waitFor: "What outcome would make this goal successful?", send: "Solve ratio problems\r"},
		interaction{waitFor: "What is your general learning experience?", send: "\r"},
		interaction{waitFor: "How much experience do you have with this subject?", send: "\r"},
		interaction{waitFor: "How much time can you study", send: "\r"},
		interaction{waitFor: "How many days per week", send: "\r"},
		interaction{waitFor: "How do you prefer to learn?", send: "\r"},
		interaction{waitFor: "How much mastery should be required", send: "\r"},
		interaction{waitFor: "Would you like a diagnostic after setup?", send: "\x1b[B\r"},
		interaction{waitFor: "Review your setup before applying it.", send: "\r"},
		interaction{waitFor: "Apply this learner setup?", send: "\r"},
		interaction{waitFor: "Setup complete.", send: "\r"},
		interaction{waitFor: "Learning path ready.", send: "q"},
	)
}

func (test scenario) runInteractive(interactions ...interaction) string {
	test.testing.Helper()
	command := exec.Command(test.binary, "--no-color")
	command.Dir = test.workspace
	command.Env = test.environ
	input, err := command.StdinPipe()
	if err != nil {
		test.testing.Fatal(err)
	}
	output := &safeBuffer{}
	command.Stdout = output
	command.Stderr = output
	if err := command.Start(); err != nil {
		test.testing.Fatal(err)
	}
	defer func() { _ = command.Process.Kill() }()

	for _, next := range interactions {
		waitForOutput(test.testing, output, next.waitFor)
		if _, err := input.Write([]byte(next.send)); err != nil {
			test.testing.Fatalf("send %q to TUI: %v\n%s", next.send, err, output.String())
		}
	}
	if err := input.Close(); err != nil {
		test.testing.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			test.testing.Fatalf("TUI exited with error: %v\n%s", err, output.String())
		}
	case <-time.After(commandTimeout):
		test.testing.Fatalf("TUI did not exit:\n%s", output.String())
	}
	return output.String()
}

type safeBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (buffer *safeBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Write(value)
}

func (buffer *safeBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.String()
}

func waitForOutput(t *testing.T, output *safeBuffer, expected string) {
	t.Helper()
	deadline := time.Now().Add(commandTimeout)
	for time.Now().Before(deadline) {
		if strings.Contains(normalizeOutput(output.String()), expected) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("TUI output did not contain %q:\n%s", expected, output.String())
}

func normalizeOutput(output string) string {
	return strings.ReplaceAll(output, "\r", "")
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate E2E source file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
}

func buildBinary(t *testing.T, root string) string {
	t.Helper()
	name := "kelyro-e2e"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	ldflags := "-X github.com/mishaaac/kelyro/internal/version.Version=v0.1.0-alpha.1 " +
		"-X github.com/mishaaac/kelyro/internal/version.Commit=e2e " +
		"-X github.com/mishaaac/kelyro/internal/version.Date=2026-08-12T00:00:00Z"
	command := exec.Command("go", "build", "-tags=e2e", "-ldflags", ldflags, "-o", binary, "./cmd/kelyro")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build E2E binary: %v\n%s", err, output)
	}
	return binary
}

func isolatedEnvironment(source []string, overrides map[string]string) []string {
	result := make([]string, 0, len(source)+len(overrides))
	for _, assignment := range source {
		name, _, _ := strings.Cut(assignment, "=")
		overridden := false
		for key := range overrides {
			if strings.EqualFold(name, key) {
				overridden = true
				break
			}
		}
		if !overridden && !strings.HasPrefix(strings.ToUpper(name), "KELYRO_SECRET_") {
			result = append(result, assignment)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func assertDirectory(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		t.Fatalf("%s is not a directory: %v", path, err)
	}
}

func assertRegularFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("%s is not a regular file: %v", path, err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func fileDigests(t *testing.T, root string, paths []string) map[string][sha256.Size]byte {
	t.Helper()
	result := make(map[string][sha256.Size]byte, len(paths))
	for _, path := range paths {
		result[path] = sha256.Sum256(readFile(t, filepath.Join(root, path)))
	}
	return result
}

func assertNoFileNamed(t *testing.T, root, name string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == name {
			return fmt.Errorf("unexpected file %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
