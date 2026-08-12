package tui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
)

func TestRunnerRespectsNoColorEnvironmentAndFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		present bool
		command app.Command
	}{
		{name: "NO_COLOR is present even when empty", present: true},
		{name: "no color CLI override", command: app.Command{ConfigOverrides: config.Settings{config.KeyUIColor: config.StringValue("never")}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var captured Model
			runner := NewRunner(&fakeService{}, strings.NewReader(""), &bytes.Buffer{})
			runner.lookupEnv = func(string) (string, bool) { return "", test.present }
			runner.newProgram = func(model tea.Model, _ ...tea.ProgramOption) program {
				captured = model.(Model)
				return &fakeProgram{}
			}
			if err := runner.Run(context.Background(), test.command); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if !captured.forceNoColor {
				t.Error("runner did not force color off")
			}
		})
	}
}

func TestRunnerReturnsProgramErrorsAndRecoversPanics(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("terminal unavailable")
	runner := NewRunner(&fakeService{}, strings.NewReader(""), &bytes.Buffer{})
	runner.newProgram = func(tea.Model, ...tea.ProgramOption) program { return &fakeProgram{err: wantErr} }
	if err := runner.Run(context.Background(), app.Command{}); !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want program error", err)
	}

	runner.newProgram = func(tea.Model, ...tea.ProgramOption) program { return &fakeProgram{panicValue: "boom"} }
	if err := runner.Run(context.Background(), app.Command{}); err == nil || !strings.Contains(err.Error(), "terminal interface panic: boom") {
		t.Fatalf("Run() panic error = %v", err)
	}
}

type fakeProgram struct {
	err        error
	panicValue any
}

func (program *fakeProgram) Run() (tea.Model, error) {
	if program.panicValue != nil {
		panic(program.panicValue)
	}
	return nil, program.err
}
