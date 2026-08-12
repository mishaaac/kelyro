package configfs

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/config"
)

func TestStoreSaveReloadAndProjectOverride(t *testing.T) {
	t.Parallel()

	store, globalPath, root := testStore(t)
	global := config.Settings{
		config.KeyUIColor:        config.StringValue("always"),
		config.KeyAllowNetwork:   config.BoolValue(false),
		config.KeyAllowAIContent: config.BoolValue(false),
		config.KeyAllowTelemetry: config.BoolValue(false),
		config.KeyEditorCommand:  config.StringValue("code"),
		config.KeyEditorPrompt:   config.BoolValue(false),
	}
	project := config.Settings{
		config.KeyUIColor:          config.StringValue("never"),
		config.KeyWorkspaceName:    config.StringValue("Backend Engineering with Go"),
		config.KeyMasteryThreshold: config.NumberValue(0.9),
	}

	if err := store.SaveGlobal(global); err != nil {
		t.Fatalf("SaveGlobal() error = %v", err)
	}
	if err := store.SaveProject(root, project); err != nil {
		t.Fatalf("SaveProject() error = %v", err)
	}
	loadedGlobal, err := store.LoadGlobal()
	if err != nil {
		t.Fatalf("LoadGlobal() error = %v", err)
	}
	loadedProject, err := store.LoadProject(root)
	if err != nil {
		t.Fatalf("LoadProject() error = %v", err)
	}
	resolved, err := config.Resolve(loadedGlobal, loadedProject)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := resolved[config.KeyUIColor].String(); got != "never" {
		t.Errorf("resolved ui.color = %q, want project override never", got)
	}
	if got := resolved[config.KeyEditorCommand].String(); got != "code" {
		t.Errorf("resolved editor.command = %q", got)
	}
	if got := resolved[config.KeyEditorPrompt].String(); got != "false" {
		t.Errorf("resolved editor.prompt = %q", got)
	}

	encoded, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile(global): %v", err)
	}
	for _, want := range []string{
		"schema_version = 1", "[ui]", `color = "always"`, "[privacy]",
		"allow_ai_content = false", "allow_network = false", "allow_usage_telemetry = false",
	} {
		if !bytes.Contains(encoded, []byte(want)) {
			t.Errorf("global TOML does not contain %q:\n%s", want, encoded)
		}
	}
}

func TestLoadRejectsInvalidTOMLWithReadableKeyErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "unknown key", content: "[privacy]\napi_key = \"secret\"\n", want: `unknown configuration key "privacy.api_key"`},
		{name: "wrong type", content: "[privacy]\nallow_network = \"false\"\n", want: `configuration key "privacy.allow_network" expects true or false`},
		{name: "invalid value", content: "[ui]\ncolor = \"rainbow\"\n", want: `configuration key "ui.color" expects auto, always, or never`},
		{name: "unsupported schema", content: "schema_version = 99\n", want: "unsupported schema_version 99"},
		{name: "duplicate key", content: "[updates]\ncheck = true\ncheck = false\n", want: `duplicate key "updates.check"`},
		{name: "unknown table", content: "[provider]\n", want: `unknown configuration table "provider"`},
		{name: "non TOML escape", content: "[editor]\ncommand = \"a\\x20b\"\n", want: "unsupported escape"},
		{name: "project key in global", content: "[workspace]\nname = \"Backend\"\n", want: "only valid in project configuration"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			store, globalPath, _ := testStore(t)
			if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
				t.Fatalf("MkdirAll(): %v", err)
			}
			if err := os.WriteFile(globalPath, []byte(test.content), 0o600); err != nil {
				t.Fatalf("WriteFile(): %v", err)
			}
			_, err := store.LoadGlobal()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadGlobal() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestSetPreservesCommentsAndUnrelatedFormatting(t *testing.T) {
	t.Parallel()

	store, globalPath, _ := testStore(t)
	if err := os.MkdirAll(filepath.Dir(globalPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	original := "# User note\nschema_version = 1\n\n[ui]\ncolor = \"auto\" # keep inline\n\n[editor]\n# Preferred editor\ncommand = \"vim\"\n"
	if err := os.WriteFile(globalPath, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}

	if err := store.SetGlobal(config.KeyUIColor, config.StringValue("never")); err != nil {
		t.Fatalf("SetGlobal(existing) error = %v", err)
	}
	if err := store.SetGlobal(config.KeyUpdateCheck, config.BoolValue(false)); err != nil {
		t.Fatalf("SetGlobal(new) error = %v", err)
	}
	updated, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile(): %v", err)
	}
	for _, want := range []string{"# User note", `color = "never" # keep inline`, "# Preferred editor", `command = "vim"`, "[updates]", "check = false"} {
		if !bytes.Contains(updated, []byte(want)) {
			t.Errorf("updated TOML does not contain %q:\n%s", want, updated)
		}
	}
}

func TestAtomicWriteFailurePreservesPreviousFile(t *testing.T) {
	t.Parallel()

	store, globalPath, _ := testStore(t)
	initial := config.Settings{config.KeyUIColor: config.StringValue("auto")}
	if err := store.SaveGlobal(initial); err != nil {
		t.Fatalf("initial SaveGlobal() error = %v", err)
	}
	want, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile(initial): %v", err)
	}

	commitErr := errors.New("injected rename failure")
	store.rename = func(string, string) error { return commitErr }
	err = store.SaveGlobal(config.Settings{config.KeyUIColor: config.StringValue("always")})
	if !errors.Is(err, commitErr) {
		t.Fatalf("SaveGlobal() error = %v, want injected rename failure", err)
	}
	got, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile(after failure): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("failed atomic write changed target:\ngot %s\nwant %s", got, want)
	}
	temporary, err := filepath.Glob(filepath.Join(filepath.Dir(globalPath), ".config-*.tmp"))
	if err != nil {
		t.Fatalf("Glob(): %v", err)
	}
	if len(temporary) != 0 {
		t.Errorf("failed atomic write left staging files: %v", temporary)
	}
}

func TestMissingFilesLoadAsEmptyLayers(t *testing.T) {
	t.Parallel()

	store, _, root := testStore(t)
	global, err := store.LoadGlobal()
	if err != nil || len(global) != 0 {
		t.Fatalf("LoadGlobal() = %#v, %v; want empty layer", global, err)
	}
	project, err := store.LoadProject(root)
	if err != nil || len(project) != 0 {
		t.Fatalf("LoadProject() = %#v, %v; want empty layer", project, err)
	}
}

func testStore(t *testing.T) (*Store, string, string) {
	t.Helper()
	base := t.TempDir()
	globalPath := filepath.Join(base, "global", "config.toml")
	root := filepath.Join(base, "workspace")
	if err := os.MkdirAll(filepath.Join(root, ".kelyro"), 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace): %v", err)
	}
	store := New()
	store.globalPath = func() (string, error) { return globalPath, nil }
	store.projectPath = func(projectRoot string) (string, error) {
		return filepath.Join(projectRoot, ".kelyro", "config.toml"), nil
	}
	return store, globalPath, root
}
