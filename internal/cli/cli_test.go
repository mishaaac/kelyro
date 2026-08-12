package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
)

func TestRunnerDispatchesFoundationCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantAction app.Action
	}{
		{name: "default TUI", wantAction: app.ActionTUI},
		{name: "init", args: []string{"init"}, wantAction: app.ActionInit},
		{name: "doctor", args: []string{"doctor"}, wantAction: app.ActionDoctor},
		{name: "config", args: []string{"config"}, wantAction: app.ActionConfig},
		{name: "secrets", args: []string{"secrets", "status"}, wantAction: app.ActionSecrets},
		{name: "status", args: []string{"status"}, wantAction: app.ActionStatus},
		{name: "open", args: []string{"open"}, wantAction: app.ActionOpen},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeService{result: app.Result{Message: "done"}}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := NewRunner(service, &stdout, &stderr)

			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitOK {
				t.Fatalf("Run() exit code = %d, want %d", exitCode, ExitOK)
			}
			if len(service.commands) != 1 {
				t.Fatalf("service calls = %d, want 1", len(service.commands))
			}
			if got := service.commands[0].Action; got != test.wantAction {
				t.Errorf("dispatched action = %q, want %q", got, test.wantAction)
			}
			if got, want := stdout.String(), "done\n"; got != want {
				t.Errorf("stdout = %q, want %q", got, want)
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunnerPassesWorkspaceAndAcceptsReservedFlags(t *testing.T) {
	t.Parallel()

	workspacePath := filepath.Join("projects", "learning")
	service := &fakeService{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr)

	args := []string{"--no-color", "status", "--verbose", "--workspace", workspacePath}
	if exitCode := runner.Run(context.Background(), args); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d, want %d; stderr = %q", exitCode, ExitOK, stderr.String())
	}
	if len(service.commands) != 1 {
		t.Fatalf("service calls = %d, want 1", len(service.commands))
	}
	if got := service.commands[0].Workspace; got != workspacePath {
		t.Errorf("workspace = %q, want %q", got, workspacePath)
	}
}

func TestRunnerLaunchesInteractiveAdapterForDefaultCommand(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	interactive := &fakeInteractiveRunner{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr).WithInteractive(interactive)

	if exitCode := runner.Run(context.Background(), []string{"--workspace", "learning lab", "--no-color"}); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
	}
	if len(service.commands) != 0 {
		t.Errorf("application Execute calls = %d, want TUI to own its service calls", len(service.commands))
	}
	if len(interactive.commands) != 1 {
		t.Fatalf("interactive calls = %d", len(interactive.commands))
	}
	command := interactive.commands[0]
	if command.Action != app.ActionTUI || command.Workspace != "learning lab" {
		t.Errorf("interactive command = %#v", command)
	}
	if command.ConfigOverrides[config.KeyUIColor].String() != "never" {
		t.Errorf("interactive color override = %q", command.ConfigOverrides[config.KeyUIColor].String())
	}
}

func TestRunnerReportsInteractiveFailure(t *testing.T) {
	t.Parallel()

	interactive := &fakeInteractiveRunner{err: errors.New("terminal unavailable")}
	var stderr bytes.Buffer
	runner := NewRunner(&fakeService{}, &bytes.Buffer{}, &stderr).WithInteractive(interactive)
	if exitCode := runner.Run(context.Background(), nil); exitCode != ExitFailure {
		t.Fatalf("Run() exit code = %d, want failure", exitCode)
	}
	if got := stderr.String(); got != "kelyro tui: terminal unavailable\n" {
		t.Errorf("stderr = %q", got)
	}
}

func TestRunnerPassesExplicitNestedInitialization(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr)

	if exitCode := runner.Run(context.Background(), []string{"init", "--allow-nested"}); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d, want %d; stderr = %q", exitCode, ExitOK, stderr.String())
	}
	if len(service.commands) != 1 || !service.commands[0].AllowNested {
		t.Errorf("commands = %#v, want one command with AllowNested", service.commands)
	}
}

func TestRunnerParsesOpenTargets(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		args       []string
		wantTarget string
	}{
		{name: "learning by default", args: []string{"open"}},
		{name: "roadmap", args: []string{"open", "roadmap"}, wantTarget: "roadmap"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{}
			var stderr bytes.Buffer
			runner := NewRunner(service, &bytes.Buffer{}, &stderr)
			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitOK {
				t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
			}
			if len(service.commands) != 1 || service.commands[0].OpenTarget != test.wantTarget {
				t.Errorf("commands = %#v, want open target %q", service.commands, test.wantTarget)
			}
		})
	}
}

func TestRunnerParsesConfigCommandsScopesAndOverrides(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantOperation string
		wantScope     config.Scope
		wantKey       string
		wantValue     string
		wantColor     string
	}{
		{name: "config defaults to show", args: []string{"config"}, wantOperation: "show"},
		{name: "show global", args: []string{"--global", "config", "show"}, wantOperation: "show", wantScope: config.ScopeGlobal},
		{name: "project path", args: []string{"config", "path", "--project"}, wantOperation: "path", wantScope: config.ScopeProject},
		{name: "get", args: []string{"config", "get", "ui.color"}, wantOperation: "get", wantKey: "ui.color"},
		{name: "set", args: []string{"config", "set", "editor.command", "code --wait"}, wantOperation: "set", wantKey: "editor.command", wantValue: "code --wait"},
		{name: "CLI color override", args: []string{"--no-color", "config", "show"}, wantOperation: "show", wantColor: "never"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := NewRunner(service, &stdout, &stderr)

			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitOK {
				t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
			}
			if len(service.commands) != 1 {
				t.Fatalf("service calls = %d, want 1", len(service.commands))
			}
			command := service.commands[0]
			if command.ConfigOperation != test.wantOperation || command.ConfigScope != test.wantScope || command.ConfigKey != test.wantKey || command.ConfigValue != test.wantValue {
				t.Errorf("config command = %#v", command)
			}
			if test.wantColor != "" && command.ConfigOverrides[config.KeyUIColor].String() != test.wantColor {
				t.Errorf("ui.color override = %q, want %q", command.ConfigOverrides[config.KeyUIColor].String(), test.wantColor)
			}
		})
	}
}

func TestRunnerParsesAndDispatchesSecretCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantOperation string
		wantName      string
	}{
		{name: "status", args: []string{"secrets", "status"}, wantOperation: "status"},
		{name: "delete", args: []string{"secrets", "delete", "openai"}, wantOperation: "delete", wantName: "openai"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := NewRunner(service, &stdout, &stderr)
			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitOK {
				t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
			}
			command := service.commands[0]
			if command.SecretOperation != test.wantOperation || command.SecretName != test.wantName {
				t.Fatalf("secret command = %#v", command)
			}
		})
	}
}

func TestRunnerReadsSecretOutsideArgumentsAndOutput(t *testing.T) {
	t.Parallel()

	secret := "sensitive-manual-input"
	service := &fakeService{result: app.Result{Message: "configured"}}
	reader := &fakeSecretReader{value: secret}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr).WithSecretReader(reader)

	if exitCode := runner.Run(context.Background(), []string{"secrets", "set", "openai"}); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d; stderr = %q", exitCode, stderr.String())
	}
	if reader.prompt != "Secret value: " {
		t.Fatalf("secret prompt = %q", reader.prompt)
	}
	if len(service.commands) != 1 || service.commands[0].SecretValue != secret || service.commands[0].SecretName != "openai" {
		t.Fatalf("commands = %#v", service.commands)
	}
	if strings.Contains(stdout.String(), secret) || strings.Contains(stderr.String(), secret) {
		t.Fatal("CLI output exposed secret")
	}
}

func TestRunnerHelp(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"help"}, {"--help"}, {"init", "--help"}} {
		service := &fakeService{}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		runner := NewRunner(service, &stdout, &stderr)

		if exitCode := runner.Run(context.Background(), args); exitCode != ExitOK {
			t.Fatalf("Run(%q) exit code = %d, want %d", args, exitCode, ExitOK)
		}
		for _, expected := range []string{"Usage:", "Commands:", "init", "doctor", "--no-color", "--workspace PATH"} {
			if !strings.Contains(stdout.String(), expected) {
				t.Errorf("Run(%q) output does not contain %q", args, expected)
			}
		}
		if len(service.commands) != 0 {
			t.Errorf("Run(%q) dispatched %d service calls, want 0", args, len(service.commands))
		}
		if stderr.Len() != 0 {
			t.Errorf("Run(%q) stderr = %q, want empty", args, stderr.String())
		}
	}
}

func TestRunnerVersion(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"version"}, {"--version"}} {
		service := &fakeService{}
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		runner := NewRunner(service, &stdout, &stderr)

		if exitCode := runner.Run(context.Background(), args); exitCode != ExitOK {
			t.Fatalf("Run(%q) exit code = %d, want %d", args, exitCode, ExitOK)
		}
		if got, want := stdout.String(), "kelyro dev (commit unknown, built unknown)\n"; got != want {
			t.Errorf("Run(%q) output = %q, want %q", args, got, want)
		}
		if len(service.commands) != 0 {
			t.Errorf("Run(%q) dispatched %d service calls, want 0", args, len(service.commands))
		}
		if stderr.Len() != 0 {
			t.Errorf("Run(%q) stderr = %q, want empty", args, stderr.String())
		}
	}
}

func TestRunnerRejectsInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{name: "unknown command", args: []string{"learn"}, message: `unknown command "learn"`},
		{name: "unknown option", args: []string{"--json"}, message: `unknown option "--json"`},
		{name: "missing workspace", args: []string{"--workspace"}, message: "option --workspace requires a path"},
		{name: "empty workspace", args: []string{"--workspace="}, message: "option --workspace requires a path"},
		{name: "extra argument", args: []string{"status", "extra"}, message: `unexpected argument "extra"`},
		{name: "conflicting output modes", args: []string{"--verbose", "--quiet"}, message: "options --verbose and --quiet cannot be combined"},
		{name: "version with command", args: []string{"init", "--version"}, message: "option --version cannot be combined with a command"},
		{name: "nested without init", args: []string{"status", "--allow-nested"}, message: "option --allow-nested requires the init command"},
		{name: "scope conflict", args: []string{"config", "--global", "--project"}, message: "options --global and --project cannot be combined"},
		{name: "scope without config", args: []string{"status", "--global"}, message: "configuration scope options require the config command"},
		{name: "unknown config command", args: []string{"config", "edit"}, message: `unknown config command "edit"`},
		{name: "get missing key", args: []string{"config", "get"}, message: "config get requires exactly one key"},
		{name: "set missing value", args: []string{"config", "set", "ui.color"}, message: "config set requires a key and value"},
		{name: "show extra argument", args: []string{"config", "show", "extra"}, message: "config show does not accept arguments"},
		{name: "secrets missing operation", args: []string{"secrets"}, message: "secrets requires status, set, or delete"},
		{name: "unknown secrets operation", args: []string{"secrets", "get", "openai"}, message: `unknown secrets command "get"`},
		{name: "secrets status extra", args: []string{"secrets", "status", "openai"}, message: "secrets status does not accept arguments"},
		{name: "secrets set missing name", args: []string{"secrets", "set"}, message: "secrets set requires exactly one name"},
		{name: "unknown open artifact", args: []string{"open", "lesson"}, message: "open accepts only the optional roadmap artifact"},
		{name: "too many open artifacts", args: []string{"open", "roadmap", "extra"}, message: "open accepts only the optional roadmap artifact"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &fakeService{}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			runner := NewRunner(service, &stdout, &stderr)

			if exitCode := runner.Run(context.Background(), test.args); exitCode != ExitUsage {
				t.Fatalf("Run() exit code = %d, want %d", exitCode, ExitUsage)
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), "kelyro: "+test.message) {
				t.Errorf("stderr = %q, want error containing %q", stderr.String(), test.message)
			}
			if !strings.Contains(stderr.String(), "Run 'kelyro help' for usage.") {
				t.Errorf("stderr = %q, want usage hint", stderr.String())
			}
			if len(service.commands) != 0 {
				t.Errorf("service calls = %d, want 0", len(service.commands))
			}
		})
	}
}

func TestRunnerReturnsFailureForServiceError(t *testing.T) {
	t.Parallel()

	service := &fakeService{err: errors.New("diagnostic failed")}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr)

	if exitCode := runner.Run(context.Background(), []string{"doctor"}); exitCode != ExitFailure {
		t.Fatalf("Run() exit code = %d, want %d", exitCode, ExitFailure)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if got, want := stderr.String(), "kelyro doctor: diagnostic failed\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestRunnerQuietSuppressesSuccessfulOutput(t *testing.T) {
	t.Parallel()

	service := &fakeService{result: app.Result{Message: "hidden"}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	runner := NewRunner(service, &stdout, &stderr)

	if exitCode := runner.Run(context.Background(), []string{"--quiet", "status"}); exitCode != ExitOK {
		t.Fatalf("Run() exit code = %d, want %d", exitCode, ExitOK)
	}
	if stdout.Len() != 0 {
		t.Errorf("stdout = %q, want empty", stdout.String())
	}
	if len(service.commands) != 1 || service.commands[0].Action != app.ActionStatus {
		t.Errorf("commands = %#v, want one status command", service.commands)
	}
}

type fakeService struct {
	commands []app.Command
	result   app.Result
	err      error
}

type fakeSecretReader struct {
	value  string
	err    error
	prompt string
}

type fakeInteractiveRunner struct {
	commands []app.Command
	err      error
}

func (runner *fakeInteractiveRunner) Run(_ context.Context, command app.Command) error {
	runner.commands = append(runner.commands, command)
	return runner.err
}

func (reader *fakeSecretReader) ReadSecret(prompt string) (string, error) {
	reader.prompt = prompt
	return reader.value, reader.err
}

func (service *fakeService) Execute(_ context.Context, command app.Command) (app.Result, error) {
	service.commands = append(service.commands, command)
	return service.result, service.err
}
