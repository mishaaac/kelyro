package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceExecutesDoctorWithWorkspaceAndContext(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "workspace with spaces")
	runner := &recordingDoctor{report: doctor.Report{Checks: []doctor.Check{{ID: "tool.docker", Requirement: doctor.Required, State: doctor.Miss}}}}
	contextInput := doctor.Context{ToolRequirements: []doctor.ToolRequirement{{ToolID: "docker", Requirement: doctor.Required}}}
	service := NewService(
		&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}},
		func() (string, error) { return filepath.Join(root, "lessons"), nil },
	).WithConfig(&recordingConfigStore{}).WithDoctor(runner)

	result, err := service.Execute(context.Background(), Command{Action: ActionDoctor, DoctorContext: contextInput})
	if err != nil {
		t.Fatalf("Execute(doctor) error = %v", err)
	}
	if result.Diagnostics == nil || !result.Failed {
		t.Fatalf("doctor result = %#v", result)
	}
	if runner.input.WorkspaceRoot != root || runner.input.InternalDirectory != filepath.Join(root, ".kelyro") || runner.input.ConfigurationError != nil {
		t.Errorf("doctor input = %#v", runner.input)
	}
	if len(runner.context.ToolRequirements) != 1 || runner.context.ToolRequirements[0].ToolID != "docker" {
		t.Errorf("doctor context = %#v", runner.context)
	}
}

func TestServiceTurnsWorkspaceDiscoveryErrorIntoDiagnosticInput(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("workspace metadata is invalid")
	runner := &recordingDoctor{}
	service := NewService(&recordingWorkspaceService{discoverErr: wantErr}, func() (string, error) { return "/project", nil }).WithDoctor(runner)

	if _, err := service.Execute(context.Background(), Command{Action: ActionDoctor}); err != nil {
		t.Fatalf("Execute(doctor) error = %v, want rendered diagnostic", err)
	}
	if !errors.Is(runner.input.WorkspaceError, wantErr) || !errors.Is(runner.input.ConfigurationError, wantErr) {
		t.Fatalf("doctor input errors = %#v", runner.input)
	}
}

func TestServiceExplainsToolWithoutWorkspaceDiscovery(t *testing.T) {
	t.Parallel()

	want := doctor.Guidance{ToolID: "lazygit", DisplayName: "lazygit", Requirement: doctor.Optional, Description: "A Git interface."}
	runner := &recordingDoctor{guidance: want}
	service := NewService(
		&recordingWorkspaceService{discoverErr: errors.New("workspace should not be discovered")},
		func() (string, error) { return "", errors.New("current directory should not be read") },
	).WithDoctor(runner)

	result, err := service.Execute(context.Background(), Command{Action: ActionDoctor, DoctorExplain: "lazygit"})
	if err != nil {
		t.Fatalf("Execute(doctor explain) error = %v", err)
	}
	if result.Guidance == nil || *result.Guidance != want {
		t.Fatalf("doctor explanation result = %#v", result)
	}
	if runner.explainID != "lazygit" {
		t.Errorf("explained tool = %q", runner.explainID)
	}
	if runner.input.WorkspaceRoot != "" {
		t.Errorf("doctor explanation unexpectedly collected diagnostic input: %#v", runner.input)
	}
}

type recordingDoctor struct {
	input      doctor.Input
	context    doctor.Context
	report     doctor.Report
	guidance   doctor.Guidance
	explainID  string
	explainErr error
}

func (runner *recordingDoctor) Run(_ context.Context, input doctor.Input, diagnosticContext doctor.Context) doctor.Report {
	runner.input = input
	runner.context = diagnosticContext
	return runner.report
}

func (runner *recordingDoctor) Explain(toolID string) (doctor.Guidance, error) {
	runner.explainID = toolID
	return runner.guidance, runner.explainErr
}
