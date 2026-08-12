// Package cli parses command-line input and dispatches Foundation operations to
// application services.
package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mishaaac/kelyro/internal/app"
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
  init     Initialize a workspace (placeholder)
  doctor   Run Foundation diagnostics (placeholder)
  config   Manage configuration (placeholder)
  status   Show workspace status (placeholder)
  open     Open the workspace (placeholder)

Options:
  -h, --help          Show this help message
      --version       Show build version information
      --no-color      Disable colored output
      --verbose       Enable verbose output
      --quiet         Suppress successful command output
      --workspace PATH  Override workspace discovery
`

var actions = map[string]app.Action{
	"init":   app.ActionInit,
	"doctor": app.ActionDoctor,
	"config": app.ActionConfig,
	"status": app.ActionStatus,
	"open":   app.ActionOpen,
}

// Runner owns CLI parsing and rendering while delegating operations to an
// application service.
type Runner struct {
	service app.FoundationService
	stdout  io.Writer
	stderr  io.Writer
}

// NewRunner creates a testable CLI runner with explicit dependencies.
func NewRunner(service app.FoundationService, stdout, stderr io.Writer) Runner {
	return Runner{service: service, stdout: stdout, stderr: stderr}
}

// Run executes the production CLI with the temporary bootstrap service.
func Run(args []string, stdout, stderr io.Writer) int {
	runner := NewRunner(app.BootstrapService{}, stdout, stderr)
	return runner.Run(context.Background(), args)
}

// Run parses args, renders immediate CLI output, or dispatches one Foundation
// action. It returns a process exit code and never starts a child process.
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

	result, err := r.service.Execute(ctx, app.Command{
		Action:    action,
		Workspace: invocation.workspace,
	})
	if err != nil {
		fmt.Fprintf(r.stderr, "kelyro %s: %v\n", commandName, err)
		return ExitFailure
	}
	if !invocation.quiet && result.Message != "" {
		fmt.Fprintln(r.stdout, result.Message)
	}

	return ExitOK
}

type invocation struct {
	command   string
	workspace string
	help      bool
	version   bool
	noColor   bool
	verbose   bool
	quiet     bool
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
			return invocation{}, fmt.Errorf("unexpected argument %q", argument)
		}
	}

	if result.help && result.version {
		return invocation{}, fmt.Errorf("options --help and --version cannot be combined")
	}
	if result.verbose && result.quiet {
		return invocation{}, fmt.Errorf("options --verbose and --quiet cannot be combined")
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
	}

	return result, nil
}

func (r Runner) usageError(format string, args ...any) int {
	fmt.Fprintf(r.stderr, "kelyro: "+format+"\n", args...)
	fmt.Fprintln(r.stderr, "Run 'kelyro help' for usage.")
	return ExitUsage
}
