package config

import (
	"strings"
	"testing"
)

func TestDefaultsAreSafeAndIndependent(t *testing.T) {
	t.Parallel()

	first := Defaults()
	second := Defaults()
	first[KeyAllowNetwork] = BoolValue(true)

	if got := second[KeyUIColor].String(); got != "auto" {
		t.Errorf("ui.color default = %q, want auto", got)
	}
	if got := second[KeyAllowNetwork].String(); got != "false" {
		t.Errorf("privacy.allow_network default = %q, want false", got)
	}
	if got := second[KeyEditorPrompt].String(); got != "true" {
		t.Errorf("editor.prompt default = %q, want true", got)
	}
	if got := second[KeyUpdateCheck].String(); got != "true" {
		t.Errorf("updates.check default = %q, want true", got)
	}
	if got := second[KeyMasteryThreshold].String(); got != "0.85" {
		t.Errorf("learning.mastery_threshold default = %q, want 0.85", got)
	}
	if got := second[KeyBackupRetention].String(); got != "5" {
		t.Errorf("backup.retention default = %q, want 5", got)
	}
}

func TestResolveUsesMostSpecificLayer(t *testing.T) {
	t.Parallel()

	global := Settings{KeyUIColor: StringValue("always"), KeyUpdateCheck: BoolValue(false)}
	project := Settings{KeyUIColor: StringValue("auto"), KeyWorkspaceName: StringValue("Backend Go")}
	overrides := Settings{KeyUIColor: StringValue("never")}

	resolved, err := Resolve(global, project, overrides)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got := resolved[KeyUIColor].String(); got != "never" {
		t.Errorf("ui.color = %q, want CLI override never", got)
	}
	if got := resolved[KeyUpdateCheck].String(); got != "false" {
		t.Errorf("updates.check = %q, want global false", got)
	}
	if got := resolved[KeyWorkspaceName].String(); got != "Backend Go" {
		t.Errorf("workspace.name = %q, want project value", got)
	}
	if got := resolved[KeyAllowNetwork].String(); got != "false" {
		t.Errorf("privacy.allow_network = %q, want default false", got)
	}
}

func TestParseValueAndValidateLayerRejectInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{name: "unknown key", key: "ai.api_key", value: "secret", want: "unknown configuration key"},
		{name: "invalid color", key: KeyUIColor, value: "sometimes", want: "auto, always, or never"},
		{name: "invalid bool", key: KeyAllowNetwork, value: "yes", want: "expects true or false"},
		{name: "invalid threshold", key: KeyMasteryThreshold, value: "1.1", want: "at most 1"},
		{name: "non-finite threshold", key: KeyMasteryThreshold, value: "NaN", want: "at most 1"},
		{name: "fractional backup retention", key: KeyBackupRetention, value: "2.5", want: "integer from 1 to 100"},
		{name: "zero backup retention", key: KeyBackupRetention, value: "0", want: "integer from 1 to 100"},
		{name: "empty project name", key: KeyWorkspaceName, value: " ", want: "must not be empty"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseValue(test.key, test.value)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseValue() error = %v, want containing %q", err, test.want)
			}
		})
	}

	err := ValidateLayer(Settings{KeyWorkspaceName: StringValue("Backend")}, ScopeGlobal)
	if err == nil || !strings.Contains(err.Error(), "only valid in project") {
		t.Fatalf("ValidateLayer(global project key) error = %v", err)
	}
}

func TestDefinitionsPrepareCommonSettingsWizard(t *testing.T) {
	t.Parallel()

	definitions := Definitions()
	for _, key := range CommonKeys() {
		if !definitions[key].Common {
			t.Errorf("definition %q is not marked common", key)
		}
	}
	if definitions[KeyBackupRetention].Common {
		t.Error("backup.retention should remain CLI/file-only in the minimal wizard")
	}
	delete(definitions, KeyUIColor)
	if _, ok := Definitions()[KeyUIColor]; !ok {
		t.Error("Definitions() returned shared mutable state")
	}
}
