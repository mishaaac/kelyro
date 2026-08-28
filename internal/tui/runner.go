package tui

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/platform"
)

type program interface {
	Run() (tea.Model, error)
}

type programFactory func(model tea.Model, options ...tea.ProgramOption) program

// Runner owns Bubble Tea lifecycle and terminal restoration concerns.
type Runner struct {
	service    Service
	input      io.Reader
	output     io.Writer
	lookupEnv  func(string) (string, bool)
	newProgram programFactory
	platform   platform.Platform
}

// WithPlatform attaches the native URL opener used by source detail views.
func (runner Runner) WithPlatform(native platform.Platform) Runner {
	runner.platform = native
	return runner
}

// NewRunner creates a real terminal runner with injectable streams.
func NewRunner(service Service, input io.Reader, output io.Writer) Runner {
	return Runner{
		service:   service,
		input:     input,
		output:    output,
		lookupEnv: os.LookupEnv,
		newProgram: func(model tea.Model, options ...tea.ProgramOption) program {
			return tea.NewProgram(model, options...)
		},
	}
}

// Run launches the alternate-screen interface. Bubble Tea restores terminal
// modes as Run unwinds; this recovery boundary converts a recoverable panic to
// a normal CLI failure after that cleanup has run.
func (runner Runner) Run(ctx context.Context, command app.Command) (runErr error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			runErr = fmt.Errorf("terminal interface panic: %v", recovered)
		}
	}()

	if runner.service == nil {
		return fmt.Errorf("Foundation service is unavailable")
	}
	_, noColor := runner.lookupEnv("NO_COLOR")
	if value, ok := command.ConfigOverrides[config.KeyUIColor]; ok && value.String() == "never" {
		noColor = true
	}
	model := NewModel(ctx, runner.service, command, noColor)
	model.platform = runner.platform
	program := runner.newProgram(
		model,
		tea.WithContext(ctx),
		tea.WithInput(runner.input),
		tea.WithOutput(runner.output),
		tea.WithAltScreen(),
	)
	_, err := program.Run()
	if err != nil {
		return fmt.Errorf("run terminal interface: %w", err)
	}
	return nil
}
