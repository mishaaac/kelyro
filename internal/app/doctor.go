package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/curriculum"
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
	diagnosticContext, err := doctorContextForCommand(command)
	if err != nil {
		return Result{}, err
	}
	report := service.diagnostics.Run(ctx, input, diagnosticContext)
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
	diagnosticContext, contextErr := doctorContextForCommand(command)
	if contextErr != nil {
		return doctor.Report{Checks: []doctor.Check{{
			ID: "kelyro.curriculum_environment", Section: doctor.SectionKelyro,
			DisplayName: "Curriculum environment valid", Requirement: doctor.Required,
			State: doctor.Fail, Detail: contextErr.Error(),
		}}}
	}
	return service.diagnostics.Run(ctx, input, diagnosticContext)
}

// DoctorContextFromEnvironmentPlan maps inert curriculum metadata to Doctor's
// trusted diagnostic registry. It never accepts commands from a Learning Pack.
func DoctorContextFromEnvironmentPlan(plan curriculum.EnvironmentDoctorPlan) (doctor.Context, error) {
	if err := plan.Validate(); err != nil {
		return doctor.Context{}, fmt.Errorf("invalid curriculum environment diagnostic plan: %w", err)
	}
	result := doctor.Context{ToolRequirements: make([]doctor.ToolRequirement, 0, len(plan.Tools))}
	for _, tool := range plan.Tools {
		requirement, ok := doctorRequirement(tool.Level)
		if !ok {
			return doctor.Context{}, fmt.Errorf("invalid curriculum tool requirement %q", tool.Level)
		}
		result.ToolRequirements = append(result.ToolRequirements, doctor.ToolRequirement{
			ToolID: tool.ToolID.String(), DisplayName: tool.DisplayName,
			Requirement: requirement, MinimumVersion: tool.MinimumVersion,
			Timing: doctor.ToolTiming(tool.Timing), WhyNeeded: tool.WhyNeeded,
			OfficialSource: tool.OfficialSourceName, InstallGuidance: tool.InstallInstructions,
			LearnMore: tool.OfficialURL,
		})
	}
	return result, nil
}

func doctorContextForCommand(command Command) (doctor.Context, error) {
	if command.DoctorEnvironmentPlan == nil {
		return command.DoctorContext, nil
	}
	return DoctorContextFromEnvironmentPlan(*command.DoctorEnvironmentPlan)
}

func doctorRequirement(level curriculum.ToolRequirementLevel) (doctor.Requirement, bool) {
	switch level {
	case curriculum.ToolRequired:
		return doctor.Required, true
	case curriculum.ToolRecommended:
		return doctor.Recommended, true
	case curriculum.ToolOptional:
		return doctor.Optional, true
	default:
		return "", false
	}
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
