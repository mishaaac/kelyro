package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/doctor"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceExecutesDoctorWithWorkspaceAndContext(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "workspace with spaces")
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

func TestServiceMapsCurriculumEnvironmentPlanIntoDoctorContext(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "curriculum doctor")
	runner := &recordingDoctor{}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).
		WithConfig(&recordingConfigStore{}).
		WithDoctor(runner)
	plan := appEnvironmentDoctorPlan(t)

	if _, err := service.Execute(context.Background(), Command{Action: ActionDoctor, DoctorEnvironmentPlan: &plan}); err != nil {
		t.Fatalf("Execute(doctor) error = %v", err)
	}
	if len(runner.context.ToolRequirements) != 1 {
		t.Fatalf("doctor context = %+v", runner.context)
	}
	requirement := runner.context.ToolRequirements[0]
	if requirement.ToolID != "docker" || requirement.Requirement != doctor.Required || requirement.Timing != doctor.ToolNotNeededYet ||
		requirement.MinimumVersion != "28.0.0" || requirement.OfficialSource != "Docker project" || requirement.LearnMore != "https://docs.docker.com/get-docker/" {
		t.Fatalf("mapped requirement = %+v", requirement)
	}
}

func TestServiceRejectsInvalidCurriculumEnvironmentPlan(t *testing.T) {
	t.Parallel()
	runner := &recordingDoctor{}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: t.TempDir()}}, nil).
		WithConfig(&recordingConfigStore{}).
		WithDoctor(runner)
	invalid := appEnvironmentDoctorPlan(t)
	invalid.Tools[0].OfficialURL += "?token=secret"

	if _, err := service.Execute(context.Background(), Command{Action: ActionDoctor, DoctorEnvironmentPlan: &invalid}); err == nil {
		t.Fatal("Execute(doctor) accepted unsafe curriculum install URL")
	}
	if len(runner.context.ToolRequirements) != 0 {
		t.Fatalf("Doctor ran with invalid plan: %+v", runner.context)
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

func TestServiceProvidesResolvedResearchSearchReadinessToDoctor(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "research-doctor")
	runner := &recordingDoctor{}
	search := &recordingLiveSearchFactory{readiness: researchapp.LiveSearchReadiness{
		Provider: researchapp.LiveSearchProviderConfigured, Credential: researchapp.LiveSearchCredentialAvailable,
	}}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}, nil).
		WithConfig(&recordingConfigStore{project: config.Settings{
			config.KeyAllowNetwork: config.BoolValue(true), config.KeyResearchSearchProvider: config.StringValue("brave"),
		}}).
		WithSecrets(&recordingSecretStore{}).
		WithResearchSearch(search).
		WithDoctor(runner)

	if _, err := service.Execute(context.Background(), Command{Action: ActionDoctor, Workspace: root}); err != nil {
		t.Fatal(err)
	}
	readiness := runner.input.ResearchSearch
	if !readiness.NetworkPolicyAvailable || !readiness.NetworkPolicyEnabled || readiness.ProviderState != "configured" || readiness.CredentialState != "available" || search.probes != 1 {
		t.Fatalf("doctor research readiness = %+v, probes = %d", readiness, search.probes)
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

func appEnvironmentDoctorPlan(t *testing.T) curriculum.EnvironmentDoctorPlan {
	t.Helper()
	packID, err := curriculum.NewID("environment.backend")
	if err != nil {
		t.Fatal(err)
	}
	toolID, err := curriculum.NewID("docker")
	if err != nil {
		t.Fatal(err)
	}
	conceptID, err := curriculum.NewConceptID("concept.foundation")
	if err != nil {
		t.Fatal(err)
	}
	version, err := curriculum.NewPackVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	return curriculum.EnvironmentDoctorPlan{
		EnvironmentPack: curriculum.EnvironmentPackReference{ID: packID, Version: version},
		Platform:        curriculum.EnvironmentPlatformLinux, CurrentConceptID: conceptID,
		AlgorithmVersion: curriculum.EnvironmentDoctorPlannerVersionV1,
		Tools: []curriculum.EnvironmentToolDiagnostic{{
			ToolID: toolID, DisplayName: "Docker", Level: curriculum.ToolRequired,
			MinimumVersion: "28.0.0", Timing: curriculum.EnvironmentToolNotNeededYet,
			WhyNeeded: "Containers are introduced later.", CurrentPhase: "Foundations", CurrentModule: "Tooling",
			NeededPhase: "Services", NeededModule: "Containers", OfficialSourceName: "Docker project",
			OfficialURL: "https://docs.docker.com/get-docker/", InstallInstructions: "Follow the official instructions.",
		}},
	}
}
