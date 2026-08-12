package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/workspace"
)

// Action identifies a Foundation operation requested by a presentation
// adapter.
type Action string

const (
	ActionTUI    Action = "tui"
	ActionInit   Action = "init"
	ActionDoctor Action = "doctor"
	ActionConfig Action = "config"
	ActionStatus Action = "status"
	ActionOpen   Action = "open"
)

// Command contains presentation-independent input for a Foundation action.
type Command struct {
	Action          Action
	Workspace       string
	AllowNested     bool
	ConfigOperation string
	ConfigScope     config.Scope
	ConfigKey       string
	ConfigValue     string
	ConfigOverrides config.Settings
}

// Result contains presentation-independent output from a Foundation action.
type Result struct {
	Message string
}

// FoundationService executes the operations currently exposed by the CLI.
type FoundationService interface {
	Execute(ctx context.Context, command Command) (Result, error)
}

// Service coordinates implemented Foundation operations while retaining
// explicit placeholders for operations assigned to later steps.
type Service struct {
	workspaces       workspace.Service
	configs          config.Store
	currentDirectory func() (string, error)
	bootstrap        BootstrapService
}

// NewService creates the application service with explicit infrastructure
// dependencies.
func NewService(workspaces workspace.Service, currentDirectory func() (string, error)) *Service {
	return &Service{workspaces: workspaces, currentDirectory: currentDirectory}
}

// WithConfig attaches the configuration persistence adapter used by config
// commands. It keeps construction explicit without coupling the application
// package to a filesystem implementation.
func (service *Service) WithConfig(configs config.Store) *Service {
	service.configs = configs
	return service
}

// Execute initializes workspaces and delegates future actions to placeholders.
func (service *Service) Execute(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if command.Action == ActionConfig {
		return service.executeConfig(command)
	}
	if command.Action != ActionInit {
		return service.bootstrap.Execute(ctx, command)
	}
	if service.workspaces == nil {
		return Result{}, fmt.Errorf("workspace service is unavailable")
	}

	root := command.Workspace
	if root == "" {
		if service.currentDirectory == nil {
			return Result{}, fmt.Errorf("current directory provider is unavailable")
		}
		var err error
		root, err = service.currentDirectory()
		if err != nil {
			return Result{}, fmt.Errorf("find current directory: %w", err)
		}
	}

	created, err := service.workspaces.Init(root, workspace.InitOptions{AllowNested: command.AllowNested})
	if err != nil {
		return Result{}, err
	}

	return Result{Message: fmt.Sprintf("Kelyro workspace ready at %s", created.Root)}, nil
}

func (service *Service) executeConfig(command Command) (Result, error) {
	if service.configs == nil {
		return Result{}, fmt.Errorf("configuration store is unavailable")
	}

	switch command.ConfigOperation {
	case "show", "get":
		settings, err := service.resolvedConfig(command)
		if err != nil {
			return Result{}, err
		}
		if command.ConfigOperation == "get" {
			value, ok := settings[command.ConfigKey]
			if !ok {
				return Result{}, fmt.Errorf("unknown configuration key %q", command.ConfigKey)
			}
			return Result{Message: value.String()}, nil
		}
		return Result{Message: formatSettings(settings)}, nil
	case "path":
		return service.configPaths(command)
	case "set":
		return service.setConfig(command)
	default:
		return Result{}, fmt.Errorf("unsupported config operation %q", command.ConfigOperation)
	}
}

func (service *Service) resolvedConfig(command Command) (config.Settings, error) {
	global, err := service.configs.LoadGlobal()
	if err != nil {
		return nil, err
	}
	layers := []config.Settings{global}
	if command.ConfigScope != config.ScopeGlobal {
		root, found, err := service.configWorkspace(command)
		if err != nil {
			return nil, err
		}
		if found {
			project, err := service.configs.LoadProject(root)
			if err != nil {
				return nil, err
			}
			layers = append(layers, project)
		}
	}
	layers = append(layers, command.ConfigOverrides)
	return config.Resolve(layers...)
}

func (service *Service) configPaths(command Command) (Result, error) {
	globalPath, err := service.configs.GlobalPath()
	if err != nil {
		return Result{}, err
	}
	if command.ConfigScope == config.ScopeGlobal {
		return Result{Message: globalPath}, nil
	}

	root, found, err := service.configWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	if !found {
		return Result{Message: globalPath}, nil
	}
	projectPath, err := service.configs.ProjectPath(root)
	if err != nil {
		return Result{}, err
	}
	if command.ConfigScope == config.ScopeProject {
		return Result{Message: projectPath}, nil
	}
	return Result{Message: fmt.Sprintf("global: %s\nproject: %s", globalPath, projectPath)}, nil
}

func (service *Service) setConfig(command Command) (Result, error) {
	value, err := config.ParseValue(command.ConfigKey, command.ConfigValue)
	if err != nil {
		return Result{}, err
	}

	scope := command.ConfigScope
	root := ""
	if scope != config.ScopeGlobal {
		var found bool
		root, found, err = service.configWorkspace(command)
		if err != nil {
			return Result{}, err
		}
		if scope == config.ScopeProject && !found {
			return Result{}, fmt.Errorf("project configuration requires a Kelyro workspace")
		}
		if scope == "" {
			if found {
				scope = config.ScopeProject
			} else {
				scope = config.ScopeGlobal
			}
		}
	}

	var path string
	if scope == config.ScopeGlobal {
		if err := service.configs.SetGlobal(command.ConfigKey, value); err != nil {
			return Result{}, err
		}
		path, err = service.configs.GlobalPath()
	} else {
		if err := service.configs.SetProject(root, command.ConfigKey, value); err != nil {
			return Result{}, err
		}
		path, err = service.configs.ProjectPath(root)
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Message: fmt.Sprintf("Set %s in %s", command.ConfigKey, path)}, nil
}

func (service *Service) configWorkspace(command Command) (string, bool, error) {
	if service.workspaces == nil {
		if command.ConfigScope == config.ScopeProject || command.Workspace != "" {
			return "", false, fmt.Errorf("workspace service is unavailable")
		}
		return "", false, nil
	}

	start := command.Workspace
	if start == "" {
		if service.currentDirectory == nil {
			return "", false, fmt.Errorf("current directory provider is unavailable")
		}
		var err error
		start, err = service.currentDirectory()
		if err != nil {
			return "", false, fmt.Errorf("find current directory: %w", err)
		}
	}

	found, err := service.workspaces.Discover(start)
	if err == nil {
		return found.Root, true, nil
	}
	if errors.Is(err, workspace.ErrNotFound) && command.Workspace == "" && command.ConfigScope != config.ScopeProject {
		return "", false, nil
	}
	return "", false, err
}

func formatSettings(settings config.Settings) string {
	var lines []string
	for _, key := range config.Keys() {
		value, ok := settings[key]
		if !ok {
			continue
		}
		rendered := value.String()
		if text, stringValue := value.StringField(); stringValue {
			rendered = strconv.Quote(text)
		}
		lines = append(lines, fmt.Sprintf("%s = %s", key, rendered))
	}
	return strings.Join(lines, "\n")
}

// BootstrapService provides explicit placeholders for Foundation operations
// assigned to later steps.
type BootstrapService struct{}

// Execute returns an explicit placeholder for each reserved Foundation action.
func (BootstrapService) Execute(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	switch command.Action {
	case ActionTUI:
		return Result{Message: "Kelyro TUI bootstrap: interactive mode is not implemented yet."}, nil
	case ActionDoctor:
		return Result{Message: "kelyro doctor: diagnostics are not implemented yet."}, nil
	case ActionStatus:
		return Result{Message: "kelyro status: workspace status is not implemented yet."}, nil
	case ActionOpen:
		return Result{Message: "kelyro open: workspace opening is not implemented yet."}, nil
	default:
		return Result{}, fmt.Errorf("unsupported Foundation action %q", command.Action)
	}
}
