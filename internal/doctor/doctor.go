// Package doctor defines presentation-independent environment diagnostics and
// the extensible registry of tools understood by Kelyro.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Requirement describes how important a diagnostic or tool is.
type Requirement string

const (
	Required    Requirement = "required"
	Recommended Requirement = "recommended"
	Optional    Requirement = "optional"
)

// State is the outcome of one diagnostic check.
type State string

const (
	Pass State = "pass"
	Fail State = "fail"
	Miss State = "missing"
)

const (
	SectionPlatform    = "Platform"
	SectionKelyro      = "Kelyro"
	SectionDevelopment = "Development"
	SectionOptional    = "Optional"
)

// Tool describes an executable without coupling detection to os/exec.
type Tool struct {
	ID                 string
	DisplayName        string
	CommandCandidates  []string
	Requirement        Requirement
	SupportedPlatforms []string
	WhyNeeded          string
	LearnMore          string
	VersionArgs        []string
}

// Registry stores immutable tool metadata in stable presentation order.
type Registry struct {
	tools []Tool
}

// NewRegistry validates tool metadata and rejects ambiguous identifiers.
func NewRegistry(tools ...Tool) (Registry, error) {
	seen := make(map[string]struct{}, len(tools))
	copyTools := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		tool.ID = strings.TrimSpace(tool.ID)
		tool.DisplayName = strings.TrimSpace(tool.DisplayName)
		if tool.ID == "" || tool.DisplayName == "" {
			return Registry{}, errors.New("tool id and display name are required")
		}
		if _, duplicate := seen[tool.ID]; duplicate {
			return Registry{}, fmt.Errorf("duplicate tool id %q", tool.ID)
		}
		if !validRequirement(tool.Requirement) {
			return Registry{}, fmt.Errorf("tool %q has invalid requirement %q", tool.ID, tool.Requirement)
		}
		if len(tool.CommandCandidates) == 0 {
			return Registry{}, fmt.Errorf("tool %q has no command candidates", tool.ID)
		}
		seen[tool.ID] = struct{}{}
		tool.CommandCandidates = append([]string(nil), tool.CommandCandidates...)
		tool.SupportedPlatforms = append([]string(nil), tool.SupportedPlatforms...)
		tool.VersionArgs = append([]string(nil), tool.VersionArgs...)
		copyTools = append(copyTools, tool)
	}
	return Registry{tools: copyTools}, nil
}

// Tools returns a defensive copy in registry order.
func (registry Registry) Tools() []Tool {
	tools := append([]Tool(nil), registry.tools...)
	for index := range tools {
		tools[index].CommandCandidates = append([]string(nil), tools[index].CommandCandidates...)
		tools[index].SupportedPlatforms = append([]string(nil), tools[index].SupportedPlatforms...)
		tools[index].VersionArgs = append([]string(nil), tools[index].VersionArgs...)
	}
	return tools
}

// DefaultRegistry returns the Foundation development and optional tools.
func DefaultRegistry() Registry {
	registry, err := NewRegistry(
		Tool{ID: "go", DisplayName: "Go", CommandCandidates: []string{"go"}, Requirement: Recommended, SupportedPlatforms: allPlatforms(), WhyNeeded: "Build and test Kelyro from source.", LearnMore: "https://go.dev/doc/install", VersionArgs: []string{"version"}},
		Tool{ID: "git", DisplayName: "Git", CommandCandidates: []string{"git"}, Requirement: Recommended, SupportedPlatforms: allPlatforms(), WhyNeeded: "Track learning workspace changes and source history.", LearnMore: "https://git-scm.com/downloads", VersionArgs: []string{"--version"}},
		Tool{ID: "vscode", DisplayName: "VS Code", CommandCandidates: []string{"code", "code-insiders", "codium"}, Requirement: Optional, SupportedPlatforms: allPlatforms(), WhyNeeded: "Open and edit learning artifacts.", LearnMore: "https://code.visualstudio.com/", VersionArgs: []string{"--version"}},
		Tool{ID: "neovim", DisplayName: "Neovim", CommandCandidates: []string{"nvim"}, Requirement: Optional, SupportedPlatforms: allPlatforms(), WhyNeeded: "Open and edit learning artifacts.", LearnMore: "https://neovim.io/", VersionArgs: []string{"--version"}},
		Tool{ID: "docker", DisplayName: "Docker", CommandCandidates: []string{"docker"}, Requirement: Optional, SupportedPlatforms: allPlatforms(), WhyNeeded: "Run isolated development environments when a module requires them.", LearnMore: "https://docs.docker.com/get-docker/", VersionArgs: []string{"--version"}},
		Tool{ID: "lazygit", DisplayName: "lazygit", CommandCandidates: []string{"lazygit"}, Requirement: Optional, SupportedPlatforms: allPlatforms(), WhyNeeded: "Use an optional terminal interface for Git.", LearnMore: "https://github.com/jesseduffield/lazygit", VersionArgs: []string{"--version"}},
	)
	if err != nil {
		panic(err)
	}
	return registry
}

func allPlatforms() []string { return []string{"linux", "darwin", "windows"} }

// ToolRequirement lets a future curriculum phase select and strengthen a tool
// requirement, for example Docker required by one module.
type ToolRequirement struct {
	ToolID      string
	Requirement Requirement
	WhyNeeded   string
}

// Context narrows tool diagnostics to the requirements relevant to a phase.
// An empty context retains the complete Foundation registry.
type Context struct {
	ToolRequirements []ToolRequirement
}

// Input contains Foundation facts gathered by the application layer.
type Input struct {
	WorkspaceRoot      string
	InternalDirectory  string
	WorkspaceError     error
	ConfigurationError error
}

// StorageHealth reports independent workspace database checks.
type StorageHealth struct {
	DatabaseError      error
	MigrationError     error
	ArtifactIndexError error
}

// StorageProbe inspects persistence without exposing SQLite to the core.
type StorageProbe interface {
	Check(ctx context.Context, workspaceRoot string) StorageHealth
}

// CommandResolver safely detects executables and queries their versions. Tests
// use fakes so Doctor never depends on the machine running the test suite.
type CommandResolver interface {
	Resolve(commandCandidates []string) (path string, found bool)
	Version(ctx context.Context, executable string, args []string) (string, error)
}

// Environment performs the OS-dependent operations required by diagnostics.
type Environment interface {
	CommandResolver
	Platform() string
	Writable(path string) error
}

// Check is a safe, reusable diagnostic result for CLI and TUI presentation.
type Check struct {
	ID          string
	Section     string
	DisplayName string
	Requirement Requirement
	State       State
	Detail      string
	WhyNeeded   string
	LearnMore   string
}

// Report is the presentation-neutral output of Doctor.
type Report struct {
	Checks []Check
}

// Failed reports whether any required check failed. Recommended and optional
// tools never make the overall diagnostic fail.
func (report Report) Failed() bool {
	for _, check := range report.Checks {
		if check.Requirement == Required && check.State != Pass {
			return true
		}
	}
	return false
}

// Sections returns non-empty section names in report order.
func (report Report) Sections() []string {
	var sections []string
	for _, check := range report.Checks {
		if len(sections) == 0 || sections[len(sections)-1] != check.Section {
			sections = append(sections, check.Section)
		}
	}
	return sections
}

// ChecksIn returns the checks belonging to section.
func (report Report) ChecksIn(section string) []Check {
	var checks []Check
	for _, check := range report.Checks {
		if check.Section == section {
			checks = append(checks, check)
		}
	}
	return checks
}

// Engine evaluates Foundation health and registered tools.
type Engine struct {
	environment Environment
	storage     StorageProbe
	registry    Registry
	timeout     time.Duration
}

// New creates a diagnostics engine with a bounded version probe timeout.
func New(environment Environment, storage StorageProbe, registry Registry) *Engine {
	return &Engine{environment: environment, storage: storage, registry: registry, timeout: 2 * time.Second}
}

// Run evaluates checks independently so one failure does not hide the rest.
func (engine *Engine) Run(ctx context.Context, input Input, diagnosticContext Context) Report {
	if engine == nil || engine.environment == nil {
		return Report{Checks: []Check{failedCheck("platform.os", SectionPlatform, "OS detected", errors.New("diagnostic environment is unavailable"))}}
	}
	platformName := engine.environment.Platform()
	report := Report{Checks: []Check{{ID: "platform.os", Section: SectionPlatform, DisplayName: "OS detected", Requirement: Required, State: Pass, Detail: platformName}}}
	if strings.TrimSpace(platformName) == "" {
		report.Checks[0] = failedCheck("platform.os", SectionPlatform, "OS detected", errors.New("operating system was not identified"))
	}
	report.Checks = append(report.Checks,
		writableCheck(engine.environment, "platform.workspace_writable", "Workspace writable", input.WorkspaceRoot, input.WorkspaceError),
		writableCheck(engine.environment, "platform.internal_writable", "Internal directory writable", input.InternalDirectory, input.WorkspaceError),
		resultCheck("kelyro.config", SectionKelyro, "Config valid", input.ConfigurationError),
	)

	health := StorageHealth{}
	if input.WorkspaceError != nil {
		health.DatabaseError = input.WorkspaceError
		health.MigrationError = input.WorkspaceError
		health.ArtifactIndexError = input.WorkspaceError
	} else if engine.storage == nil {
		err := errors.New("storage diagnostic is unavailable")
		health = StorageHealth{DatabaseError: err, MigrationError: err, ArtifactIndexError: err}
	} else {
		health = engine.storage.Check(ctx, input.WorkspaceRoot)
	}
	report.Checks = append(report.Checks,
		resultCheck("kelyro.database", SectionKelyro, "Database healthy", health.DatabaseError),
		resultCheck("kelyro.migrations", SectionKelyro, "Migrations current", health.MigrationError),
		resultCheck("kelyro.artifact_index", SectionKelyro, "Artifact index healthy", health.ArtifactIndexError),
	)

	for _, tool := range contextualTools(engine.registry.Tools(), platformName, diagnosticContext) {
		report.Checks = append(report.Checks, engine.checkTool(ctx, tool))
	}
	return report
}

func (engine *Engine) checkTool(ctx context.Context, tool Tool) Check {
	section := SectionOptional
	if tool.Requirement != Optional {
		section = SectionDevelopment
	}
	check := Check{ID: "tool." + tool.ID, Section: section, DisplayName: tool.DisplayName, Requirement: tool.Requirement, State: Miss, WhyNeeded: tool.WhyNeeded, LearnMore: tool.LearnMore}
	executable, found := engine.environment.Resolve(tool.CommandCandidates)
	if !found {
		check.Detail = "not found"
		return check
	}
	check.State = Pass
	check.Detail = executable
	if len(tool.VersionArgs) == 0 {
		return check
	}
	versionContext, cancel := context.WithTimeout(ctx, engine.timeout)
	defer cancel()
	output, err := engine.environment.Version(versionContext, executable, tool.VersionArgs)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(versionContext.Err(), context.DeadlineExceeded) {
			check.Detail += " (version check timed out)"
		} else {
			check.Detail += " (version unavailable)"
		}
		return check
	}
	if version := parseVersion(output); version != "" {
		check.Detail += " (" + version + ")"
	}
	return check
}

func contextualTools(tools []Tool, platformName string, diagnosticContext Context) []Tool {
	requirements := make(map[string]ToolRequirement, len(diagnosticContext.ToolRequirements))
	for _, requirement := range diagnosticContext.ToolRequirements {
		requirements[requirement.ToolID] = requirement
	}
	contextual := len(requirements) > 0
	selected := make([]Tool, 0, len(tools))
	for _, tool := range tools {
		if !supports(tool, platformName) {
			continue
		}
		requirement, relevant := requirements[tool.ID]
		if contextual && !relevant {
			continue
		}
		if relevant {
			if validRequirement(requirement.Requirement) {
				tool.Requirement = requirement.Requirement
			}
			if strings.TrimSpace(requirement.WhyNeeded) != "" {
				tool.WhyNeeded = requirement.WhyNeeded
			}
		}
		selected = append(selected, tool)
	}
	return selected
}

func supports(tool Tool, platformName string) bool {
	if len(tool.SupportedPlatforms) == 0 {
		return true
	}
	for _, supported := range tool.SupportedPlatforms {
		if supported == platformName {
			return true
		}
	}
	return false
}

func writableCheck(environment Environment, id, name, path string, prior error) Check {
	if prior != nil {
		return failedCheck(id, SectionPlatform, name, prior)
	}
	if strings.TrimSpace(path) == "" {
		return failedCheck(id, SectionPlatform, name, errors.New("path is unavailable"))
	}
	return resultCheckIn(id, SectionPlatform, name, environment.Writable(path))
}

func resultCheck(id, section, name string, err error) Check {
	return resultCheckIn(id, section, name, err)
}

func resultCheckIn(id, section, name string, err error) Check {
	if err != nil {
		return failedCheck(id, section, name, err)
	}
	return Check{ID: id, Section: section, DisplayName: name, Requirement: Required, State: Pass}
}

func failedCheck(id, section, name string, err error) Check {
	return Check{ID: id, Section: section, DisplayName: name, Requirement: Required, State: Fail, Detail: err.Error()}
}

func validRequirement(requirement Requirement) bool {
	return requirement == Required || requirement == Recommended || requirement == Optional
}

var versionPattern = regexp.MustCompile(`(?i)(?:go|v)?\d+(?:\.\d+)+(?:[-+._][0-9a-z]+)*`)

func parseVersion(output string) string {
	line := strings.TrimSpace(output)
	if newline := strings.IndexByte(line, '\n'); newline >= 0 {
		line = line[:newline]
	}
	return versionPattern.FindString(line)
}
