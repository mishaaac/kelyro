package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/workspace"
)

// DoctorRunner is the diagnostics boundary consumed by application services.
type DoctorRunner interface {
	Run(ctx context.Context, input doctor.Input, diagnosticContext doctor.Context) doctor.Report
}

func (service *Service) executeDoctor(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	input, err := service.doctorInput(command, workspace.Workspace{})
	if err != nil && errors.Is(err, context.Canceled) {
		return Result{}, err
	}
	if err != nil {
		input.WorkspaceError = err
		input.ConfigurationError = err
	}
	if service.diagnostics == nil {
		return Result{}, fmt.Errorf("diagnostic service is unavailable")
	}
	report := service.diagnostics.Run(ctx, input, command.DoctorContext)
	return Result{Diagnostics: &report, Failed: report.Failed()}, nil
}

func (service *Service) doctorInput(command Command, known workspace.Workspace) (doctor.Input, error) {
	found := known
	if found.Root == "" {
		var err error
		found, err = service.discoverWorkspace(command)
		if err != nil {
			return doctor.Input{}, err
		}
	}

	internal, err := platform.WorkspaceInternalDir(found.Root)
	if err != nil {
		return doctor.Input{}, err
	}
	input := doctor.Input{WorkspaceRoot: found.Root, InternalDirectory: internal}
	if service.configs == nil {
		input.ConfigurationError = errors.New("configuration store is unavailable")
		return input, nil
	}
	_, input.ConfigurationError = service.resolvedConfigForWorkspace(found.Root, command.ConfigOverrides)
	return input, nil
}

func (service *Service) doctorReport(ctx context.Context, command Command, found workspace.Workspace) doctor.Report {
	if service.diagnostics == nil {
		return doctor.Report{}
	}
	input, err := service.doctorInput(command, found)
	if err != nil {
		input.WorkspaceError = err
		input.ConfigurationError = err
	}
	return service.diagnostics.Run(ctx, input, command.DoctorContext)
}
