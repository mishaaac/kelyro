package doctor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestEngineRunsFoundationAndToolChecks(t *testing.T) {
	t.Parallel()

	environment := &fakeEnvironment{
		platform: "linux",
		resolved: map[string]string{"go": "/tools/go", "git": "/tools/git", "nvim": "/tools/nvim"},
		versions: map[string]string{"/tools/go": "go version go1.24.3 linux/amd64", "/tools/git": "git version 2.49.0", "/tools/nvim": "NVIM v0.11.2\nBuild type: Release"},
	}
	engine := New(environment, fakeStorage{}, DefaultRegistry())
	report := engine.Run(context.Background(), Input{WorkspaceRoot: "/project", InternalDirectory: "/project/.kelyro"}, Context{})

	if report.Failed() {
		t.Fatalf("healthy report unexpectedly failed: %#v", report.Checks)
	}
	assertCheck(t, report, "platform.os", Pass, "linux")
	assertCheck(t, report, "tool.go", Pass, "go1.24.3")
	assertCheck(t, report, "tool.git", Pass, "2.49.0")
	assertCheck(t, report, "tool.neovim", Pass, "v0.11.2")
	assertCheck(t, report, "tool.docker", Miss, "not found")
	if len(environment.writablePaths) != 2 || environment.writablePaths[0] != "/project" || environment.writablePaths[1] != "/project/.kelyro" {
		t.Errorf("writable probes = %v", environment.writablePaths)
	}
}

func TestContextSelectsAndStrengthensRelevantTool(t *testing.T) {
	t.Parallel()

	engine := New(&fakeEnvironment{platform: "linux"}, fakeStorage{}, DefaultRegistry())
	report := engine.Run(context.Background(), Input{WorkspaceRoot: "/project", InternalDirectory: "/project/.kelyro"}, Context{ToolRequirements: []ToolRequirement{{
		ToolID: "docker", Requirement: Required, WhyNeeded: "Docker required in module Containers",
	}}})

	if !report.Failed() {
		t.Fatal("missing context-required Docker did not fail report")
	}
	var tools []Check
	for _, check := range report.Checks {
		if strings.HasPrefix(check.ID, "tool.") {
			tools = append(tools, check)
		}
	}
	if len(tools) != 1 || tools[0].ID != "tool.docker" || tools[0].Requirement != Required || tools[0].WhyNeeded != "Docker required in module Containers" {
		t.Fatalf("contextual tool checks = %#v", tools)
	}
}

func TestVersionProbeIsBoundedAndDoesNotFailDetectedTool(t *testing.T) {
	t.Parallel()

	environment := &fakeEnvironment{platform: "linux", resolved: map[string]string{"go": "/tools/go"}}
	environment.version = func(ctx context.Context, _ string, _ []string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	registry, err := NewRegistry(Tool{ID: "go", DisplayName: "Go", CommandCandidates: []string{"go"}, Requirement: Recommended, VersionArgs: []string{"version"}})
	if err != nil {
		t.Fatal(err)
	}
	engine := New(environment, fakeStorage{}, registry)
	engine.timeout = 10 * time.Millisecond
	report := engine.Run(context.Background(), Input{WorkspaceRoot: "/project", InternalDirectory: "/project/.kelyro"}, Context{})

	assertCheck(t, report, "tool.go", Pass, "timed out")
	if report.Failed() {
		t.Fatal("version timeout made a recommended detected tool fail")
	}
}

func TestRequiredFoundationFailuresAreIndependent(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("permission denied")
	environment := &fakeEnvironment{platform: "linux", writableErr: wantErr}
	storage := fakeStorage{health: StorageHealth{DatabaseError: errors.New("corrupt"), MigrationError: errors.New("old"), ArtifactIndexError: errors.New("missing table")}}
	report := New(environment, storage, Registry{}).Run(context.Background(), Input{
		WorkspaceRoot: "/project", InternalDirectory: "/project/.kelyro", ConfigurationError: errors.New("bad config"),
	}, Context{})

	if !report.Failed() {
		t.Fatal("required failures did not fail report")
	}
	for _, id := range []string{"platform.workspace_writable", "platform.internal_writable", "kelyro.config", "kelyro.database", "kelyro.migrations", "kelyro.artifact_index"} {
		assertCheck(t, report, id, Fail, "")
	}
}

func TestRegistryRejectsInvalidMetadataAndDefensivelyCopies(t *testing.T) {
	t.Parallel()

	if _, err := NewRegistry(Tool{ID: "", DisplayName: "Go", CommandCandidates: []string{"go"}, Requirement: Required}); err == nil {
		t.Fatal("NewRegistry() accepted empty id")
	}
	if _, err := NewRegistry(
		Tool{ID: "go", DisplayName: "Go", CommandCandidates: []string{"go"}, Requirement: Required},
		Tool{ID: "go", DisplayName: "Again", CommandCandidates: []string{"go2"}, Requirement: Optional},
	); err == nil {
		t.Fatal("NewRegistry() accepted duplicate id")
	}

	candidates := []string{"go"}
	registry, err := NewRegistry(Tool{ID: "go", DisplayName: "Go", CommandCandidates: candidates, Requirement: Required})
	if err != nil {
		t.Fatal(err)
	}
	candidates[0] = "changed"
	tools := registry.Tools()
	tools[0].CommandCandidates[0] = "also-changed"
	if got := registry.Tools()[0].CommandCandidates[0]; got != "go" {
		t.Errorf("registry candidate = %q, want defensive copy", got)
	}
}

func assertCheck(t *testing.T, report Report, id string, state State, detailContains string) {
	t.Helper()
	for _, check := range report.Checks {
		if check.ID != id {
			continue
		}
		if check.State != state || (detailContains != "" && !strings.Contains(check.Detail, detailContains)) {
			t.Fatalf("check %s = %#v, want state %s and detail containing %q", id, check, state, detailContains)
		}
		return
	}
	t.Fatalf("check %s not found in %#v", id, report.Checks)
}

type fakeEnvironment struct {
	platform      string
	resolved      map[string]string
	versions      map[string]string
	writableErr   error
	writablePaths []string
	version       func(context.Context, string, []string) (string, error)
}

func (environment *fakeEnvironment) Platform() string { return environment.platform }
func (environment *fakeEnvironment) Writable(path string) error {
	environment.writablePaths = append(environment.writablePaths, path)
	return environment.writableErr
}
func (environment *fakeEnvironment) Resolve(candidates []string) (string, bool) {
	for _, candidate := range candidates {
		if path := environment.resolved[candidate]; path != "" {
			return path, true
		}
	}
	return "", false
}
func (environment *fakeEnvironment) Version(ctx context.Context, executable string, args []string) (string, error) {
	if environment.version != nil {
		return environment.version(ctx, executable, args)
	}
	return environment.versions[executable], nil
}

type fakeStorage struct{ health StorageHealth }

func (storage fakeStorage) Check(context.Context, string) StorageHealth { return storage.health }
