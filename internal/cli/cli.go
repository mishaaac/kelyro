// Package cli parses command-line input and dispatches Foundation operations to
// application services.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/version"
)

// Process exit codes used by Kelyro.
const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
)

const help = `Kelyro is a local-first learning workspace.

Usage:
  kelyro [options]
  kelyro [options] <command>

Commands:
  help     Show this help message
  version  Show build version information
  init     Initialize a workspace
  doctor   Run Foundation diagnostics
  config   Show or update layered configuration
  secrets  Manage secure credential references
  status   Show workspace status (placeholder)
  open     Open LEARNING.md or the roadmap in an editor

Options:
  -h, --help          Show this help message
      --version       Show build version information
      --no-color      Disable colored output
      --verbose       Enable verbose output
      --quiet         Suppress successful command output
      --workspace PATH  Override workspace discovery
      --allow-nested  Confirm initialization inside another workspace
      --global        Use global configuration scope
      --project       Use project configuration scope

Config commands:
  kelyro config show
  kelyro config path
  kelyro config get <key>
  kelyro config set <key> <value>

Secret commands:
  kelyro secrets status
  kelyro secrets set <name>
  kelyro secrets delete <name>

Open commands:
  kelyro open
  kelyro open roadmap
`

var actions = map[string]app.Action{
	"init":    app.ActionInit,
	"doctor":  app.ActionDoctor,
	"config":  app.ActionConfig,
	"secrets": app.ActionSecrets,
	"status":  app.ActionStatus,
	"open":    app.ActionOpen,
}

// Runner owns CLI parsing and rendering while delegating operations to an
// application service.
type Runner struct {
	service     app.FoundationService
	stdout      io.Writer
	stderr      io.Writer
	secrets     SecretReader
	interactive InteractiveRunner
}

// InteractiveRunner owns the full-screen terminal lifecycle for the default
// command without coupling CLI parsing to Bubble Tea.
type InteractiveRunner interface {
	Run(ctx context.Context, command app.Command) error
}

// NewRunner creates a testable CLI runner with explicit dependencies.
func NewRunner(service app.FoundationService, stdout, stderr io.Writer) Runner {
	return Runner{service: service, stdout: stdout, stderr: stderr}
}

// WithSecretReader attaches the terminal adapter used to collect secret values
// without placing them in process arguments or normal command output.
func (r Runner) WithSecretReader(reader SecretReader) Runner {
	r.secrets = reader
	return r
}

// WithInteractive attaches the full-screen presentation adapter used when no
// explicit CLI command is provided.
func (r Runner) WithInteractive(interactive InteractiveRunner) Runner {
	r.interactive = interactive
	return r
}

// Run parses args, renders immediate CLI output, or dispatches one Foundation
// action. It returns a process exit code and does not construct native process
// commands itself.
func (r Runner) Run(ctx context.Context, args []string) int {
	invocation, err := parse(args)
	if err != nil {
		return r.usageError("%v", err)
	}

	if invocation.help {
		fmt.Fprint(r.stdout, help)
		return ExitOK
	}
	if invocation.version {
		fmt.Fprintf(r.stdout, "kelyro %s\n", version.Current())
		return ExitOK
	}

	action := app.ActionTUI
	commandName := "tui"
	if invocation.command != "" {
		var found bool
		action, found = actions[invocation.command]
		if !found {
			return r.usageError("unknown command %q", invocation.command)
		}
		commandName = invocation.command
	}

	if r.service == nil {
		fmt.Fprintln(r.stderr, "kelyro: application service is unavailable")
		return ExitFailure
	}

	command := app.Command{
		Action:      action,
		Workspace:   invocation.workspace,
		AllowNested: invocation.allowNested,
		ConfigScope: invocation.configScope,
		OpenTarget:  invocation.openTarget,
	}
	if invocation.noColor {
		command.ConfigOverrides = config.Settings{config.KeyUIColor: config.StringValue("never")}
	}
	if action == app.ActionTUI && r.interactive != nil {
		if err := r.interactive.Run(ctx, command); err != nil {
			fmt.Fprintf(r.stderr, "kelyro tui: %v\n", err)
			return ExitFailure
		}
		return ExitOK
	}
	if action == app.ActionConfig {
		command.ConfigOperation = invocation.configOperation
		command.ConfigKey = invocation.configKey
		command.ConfigValue = invocation.configValue
	}
	if action == app.ActionSecrets {
		command.SecretOperation = invocation.secretOperation
		command.SecretName = invocation.secretName
		if command.SecretOperation == "set" {
			if r.secrets == nil {
				fmt.Fprintln(r.stderr, "kelyro secrets: secure terminal input is unavailable")
				return ExitFailure
			}
			command.SecretValue, err = r.secrets.ReadSecret("Secret value: ")
			if err != nil {
				fmt.Fprintf(r.stderr, "kelyro secrets: read secret: %v\n", err)
				return ExitFailure
			}
		}
	}

	result, err := r.service.Execute(ctx, command)
	if err != nil {
		fmt.Fprintf(r.stderr, "kelyro %s: %v\n", commandName, err)
		return ExitFailure
	}
	if result.Diagnostics != nil && (!invocation.quiet || result.Failed) {
		fmt.Fprintln(r.stdout, formatDiagnostics(*result.Diagnostics))
	} else if !invocation.quiet && result.Message != "" {
		fmt.Fprintln(r.stdout, result.Message)
	}
	if result.Failed {
		return ExitFailure
	}

	return ExitOK
}

func formatDiagnostics(report doctor.Report) string {
	var lines []string
	for _, section := range report.Sections() {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, section)
		for _, check := range report.ChecksIn(section) {
			marker := "✓"
			if check.State == doctor.Fail {
				marker = "✗"
			} else if check.State == doctor.Miss {
				marker = "○"
			}
			label := check.DisplayName
			if check.Requirement != doctor.Required {
				label += " [" + string(check.Requirement) + "]"
			}
			if check.Detail != "" {
				label += " — " + check.Detail
			}
			lines = append(lines, marker+" "+label)
			if check.WhyNeeded != "" {
				lines = append(lines, "  Why: "+check.WhyNeeded)
			}
			if check.State != doctor.Pass && check.LearnMore != "" {
				lines = append(lines, "  Learn more: "+check.LearnMore)
			}
		}
	}
	return strings.Join(lines, "\n")
}

type invocation struct {
	command         string
	workspace       string
	help            bool
	version         bool
	noColor         bool
	verbose         bool
	quiet           bool
	allowNested     bool
	arguments       []string
	configScope     config.Scope
	configOperation string
	configKey       string
	configValue     string
	secretOperation string
	secretName      string
	openTarget      string
}

func parse(args []string) (invocation, error) {
	var result invocation

	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "-h" || argument == "--help":
			result.help = true
		case argument == "--version":
			result.version = true
		case argument == "--no-color":
			result.noColor = true
		case argument == "--verbose":
			result.verbose = true
		case argument == "--quiet":
			result.quiet = true
		case argument == "--allow-nested":
			result.allowNested = true
		case argument == "--global":
			if result.configScope == config.ScopeProject {
				return invocation{}, fmt.Errorf("options --global and --project cannot be combined")
			}
			result.configScope = config.ScopeGlobal
		case argument == "--project":
			if result.configScope == config.ScopeGlobal {
				return invocation{}, fmt.Errorf("options --global and --project cannot be combined")
			}
			result.configScope = config.ScopeProject
		case argument == "--workspace":
			index++
			if index >= len(args) || args[index] == "" {
				return invocation{}, fmt.Errorf("option --workspace requires a path")
			}
			result.workspace = args[index]
		case strings.HasPrefix(argument, "--workspace="):
			result.workspace = strings.TrimPrefix(argument, "--workspace=")
			if result.workspace == "" {
				return invocation{}, fmt.Errorf("option --workspace requires a path")
			}
		case strings.HasPrefix(argument, "-"):
			return invocation{}, fmt.Errorf("unknown option %q", argument)
		case result.command == "":
			result.command = argument
		default:
			result.arguments = append(result.arguments, argument)
		}
	}

	if result.help && result.version {
		return invocation{}, fmt.Errorf("options --help and --version cannot be combined")
	}
	if result.verbose && result.quiet {
		return invocation{}, fmt.Errorf("options --verbose and --quiet cannot be combined")
	}
	if result.allowNested && result.command != "init" {
		return invocation{}, fmt.Errorf("option --allow-nested requires the init command")
	}
	if result.configScope != "" && result.command != "config" {
		return invocation{}, fmt.Errorf("configuration scope options require the config command")
	}
	if result.help {
		result.command = "help"
	}
	if result.version {
		if result.command != "" && result.command != "version" {
			return invocation{}, fmt.Errorf("option --version cannot be combined with a command")
		}
		result.command = "version"
	}

	switch result.command {
	case "help":
		result.help = true
	case "version":
		result.version = true
	case "config":
		if err := parseConfigArguments(&result); err != nil {
			return invocation{}, err
		}
	case "secrets":
		if err := parseSecretArguments(&result); err != nil {
			return invocation{}, err
		}
	case "open":
		if err := parseOpenArguments(&result); err != nil {
			return invocation{}, err
		}
	default:
		if len(result.arguments) > 0 {
			return invocation{}, fmt.Errorf("unexpected argument %q", result.arguments[0])
		}
	}

	return result, nil
}

func parseOpenArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		return nil
	}
	if len(result.arguments) != 1 || result.arguments[0] != "roadmap" {
		return fmt.Errorf("open accepts only the optional roadmap artifact")
	}
	result.openTarget = "roadmap"
	return nil
}

func parseSecretArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		return fmt.Errorf("secrets requires status, set, or delete")
	}
	result.secretOperation = result.arguments[0]
	switch result.secretOperation {
	case "status":
		if len(result.arguments) != 1 {
			return fmt.Errorf("secrets status does not accept arguments")
		}
	case "set", "delete":
		if len(result.arguments) != 2 {
			return fmt.Errorf("secrets %s requires exactly one name", result.secretOperation)
		}
		result.secretName = result.arguments[1]
	default:
		return fmt.Errorf("unknown secrets command %q", result.secretOperation)
	}
	return nil
}

func parseConfigArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		result.configOperation = "show"
		return nil
	}
	result.configOperation = result.arguments[0]
	switch result.configOperation {
	case "show", "path":
		if len(result.arguments) != 1 {
			return fmt.Errorf("config %s does not accept arguments", result.configOperation)
		}
	case "get":
		if len(result.arguments) != 2 {
			return fmt.Errorf("config get requires exactly one key")
		}
		result.configKey = result.arguments[1]
	case "set":
		if len(result.arguments) != 3 {
			return fmt.Errorf("config set requires a key and value")
		}
		result.configKey = result.arguments[1]
		result.configValue = result.arguments[2]
	default:
		return fmt.Errorf("unknown config command %q", result.configOperation)
	}
	return nil
}

func (r Runner) usageError(format string, args ...any) int {
	fmt.Fprintf(r.stderr, "kelyro: "+format+"\n", args...)
	fmt.Fprintln(r.stderr, "Run 'kelyro help' for usage.")
	return ExitUsage
}
