package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/platform"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

// DoctorRunner is the diagnostics boundary consumed by application services.
type DoctorRunner interface {
	Run(ctx context.Context, input doctor.Input, diagnosticContext doctor.Context) doctor.Report
	Explain(toolID string) (doctor.Guidance, error)
}

func (service *Service) executeDoctor(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if service.diagnostics == nil {
		return Result{}, fmt.Errorf("diagnostic service is unavailable")
	}
	if command.DoctorExplain != "" {
		guidance, err := service.diagnostics.Explain(command.DoctorExplain)
		if err != nil {
			return Result{}, err
		}
		return Result{Guidance: &guidance}, nil
	}
	input, err := service.doctorInput(ctx, command, workspace.Workspace{})
	if err != nil && errors.Is(err, context.Canceled) {
		return Result{}, err
	}
	if err != nil {
		input.WorkspaceError = err
		input.ConfigurationError = err
	}
	report := service.diagnostics.Run(ctx, input, command.DoctorContext)
	return Result{Diagnostics: &report, Failed: report.Failed()}, nil
}

func (service *Service) doctorInput(ctx context.Context, command Command, known workspace.Workspace) (doctor.Input, error) {
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
	settings, configErr := service.resolvedConfigForWorkspace(found.Root, command.ConfigOverrides)
	input.ConfigurationError = configErr
	input.ResearchSearch = service.researchSearchReadiness(ctx, settings, configErr)
	return input, nil
}

func (service *Service) doctorReport(ctx context.Context, command Command, found workspace.Workspace) doctor.Report {
	if service.diagnostics == nil {
		return doctor.Report{}
	}
	input, err := service.doctorInput(ctx, command, found)
	if err != nil {
		input.WorkspaceError = err
		input.ConfigurationError = err
	}
	return service.diagnostics.Run(ctx, input, command.DoctorContext)
}

func (service *Service) researchSearchReadiness(ctx context.Context, settings config.Settings, configErr error) doctor.ResearchSearchReadiness {
	readiness := doctor.ResearchSearchReadiness{
		ProviderState:   string(researchapp.LiveSearchProviderUnavailable),
		CredentialState: string(researchapp.LiveSearchCredentialNotApplicable),
	}
	if configErr != nil {
		return readiness
	}
	policy, err := policyFromSettings(settings)
	if err != nil {
		return readiness
	}
	readiness.NetworkPolicyAvailable = true
	readiness.NetworkPolicyEnabled = policy.AllowNetwork
	search, err := config.ResearchSearchFromResolved(settings)
	if err != nil {
		return readiness
	}
	probe, ok := service.researchSearch.(researchapp.LiveSearchReadinessProbe)
	if !ok {
		return readiness
	}
	result := probe.Probe(ctx, researchapp.LiveSearchProviderSettings{
		Provider: search.Provider, MaxResultsPerQuery: search.MaxResultsPerQuery,
		MaxQueriesPerRun: search.MaxQueriesPerRun,
	}, service.secrets)
	readiness.ProviderState = string(result.Provider)
	readiness.CredentialState = string(result.Credential)
	return readiness
}
