// Package editoros implements editor discovery and artifact opening with
// operating-system process APIs.
package editoros

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mishaaac/kelyro/internal/editor"
)

type candidate struct {
	command string
	name    string
}

var candidates = []candidate{
	{command: "code", name: "Visual Studio Code"},
	{command: "nvim", name: "Neovim"},
	{command: "vim", name: "Vim"},
	{command: "zed", name: "Zed"},
	{command: "cursor", name: "Cursor"},
}

type command struct {
	executable string
	args       []string
}

type lookupFunc func(string) (string, error)
type runFunc func(context.Context, command) error

// Service discovers real executable paths and launches them without invoking
// a command shell.
type Service struct {
	goos   string
	lookup lookupFunc
	run    runFunc
}

// New creates the native editor integration using the current process streams.
func New() *Service {
	return &Service{
		goos:   runtime.GOOS,
		lookup: exec.LookPath,
		run:    processRunner(os.Stdin, os.Stdout, os.Stderr),
	}
}

// Detect honors an explicit configured executable and otherwise selects the
// first installed supported editor. The system default is the final fallback.
func (service *Service) Detect(configured string) (editor.Selection, error) {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		path, err := service.lookup(configured)
		if err != nil {
			return editor.Selection{}, fmt.Errorf("configured editor %q was not found; editor.command accepts one executable name or path without arguments: %w", configured, editor.ErrUnavailable)
		}
		return editor.Selection{Name: configured, Executable: path}, nil
	}

	for _, candidate := range candidates {
		path, err := service.lookup(candidate.command)
		if err == nil {
			return editor.Selection{Name: candidate.name, Executable: path}, nil
		}
	}

	defaultCommand, err := service.systemDefaultCommand("")
	if err != nil {
		return editor.Selection{}, err
	}
	return editor.Selection{Name: "system default", Executable: defaultCommand.executable, SystemDefault: true}, nil
}

// Open resolves an editor, constructs its arguments separately, and waits for
// the process so terminal editors retain control of the current terminal.
func (service *Service) Open(ctx context.Context, target, configured string) (editor.Selection, error) {
	if err := ctx.Err(); err != nil {
		return editor.Selection{}, err
	}
	if strings.TrimSpace(target) == "" {
		return editor.Selection{}, errors.New("path to open must not be empty")
	}

	selection, err := service.Detect(configured)
	if err != nil {
		return editor.Selection{}, err
	}
	launch := command{executable: selection.Executable, args: []string{target}}
	if selection.SystemDefault {
		launch, err = service.systemDefaultCommand(target)
		if err != nil {
			return editor.Selection{}, err
		}
	}
	if err := service.run(ctx, launch); err != nil {
		return editor.Selection{}, fmt.Errorf("open %s with %s: %w", target, selection.Name, err)
	}
	return selection, nil
}

func (service *Service) systemDefaultCommand(target string) (command, error) {
	var executable string
	var args []string
	switch service.goos {
	case "darwin":
		executable = "open"
	case "windows":
		executable = "rundll32.exe"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		executable = "xdg-open"
	}

	path, err := service.lookup(executable)
	if err != nil {
		return command{}, fmt.Errorf("system-default opener %q was not found: %w", executable, editor.ErrUnavailable)
	}
	if target != "" {
		args = append(args, target)
	}
	return command{executable: path, args: args}, nil
}

func processRunner(stdin io.Reader, stdout, stderr io.Writer) runFunc {
	return func(ctx context.Context, specification command) error {
		process := exec.CommandContext(ctx, specification.executable, specification.args...)
		process.Stdin = stdin
		process.Stdout = stdout
		process.Stderr = stderr
		return process.Run()
	}
}
