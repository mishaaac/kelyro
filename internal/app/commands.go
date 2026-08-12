package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mishaaac/kelyro/internal/artifacts"
	artifactmarkdown "github.com/mishaaac/kelyro/internal/artifacts/markdown"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/editor"
	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/session"
	"github.com/mishaaac/kelyro/internal/storage"
	"github.com/mishaaac/kelyro/internal/workspace"
)

// Action identifies a Foundation operation requested by a presentation
// adapter.
type Action string

const (
	ActionTUI     Action = "tui"
	ActionInit    Action = "init"
	ActionDoctor  Action = "doctor"
	ActionConfig  Action = "config"
	ActionSecrets Action = "secrets"
	ActionStatus  Action = "status"
	ActionOpen    Action = "open"
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
	SecretOperation string
	SecretName      string
	SecretValue     string
	OpenTarget      string
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
	secrets          storage.SecretStore
	artifactStores   artifacts.WorkspaceStoreFactory
	sessionStores    session.WorkspaceStoreFactory
	editors          editor.Service
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

// WithSecrets attaches replaceable secret storage without exposing its native
// backend to application policy.
func (service *Service) WithSecrets(secrets storage.SecretStore) *Service {
	service.secrets = secrets
	return service
}

// WithArtifactStores attaches the per-workspace persistence used for generated
// human-readable documents.
func (service *Service) WithArtifactStores(stores artifacts.WorkspaceStoreFactory) *Service {
	service.artifactStores = stores
	return service
}

// WithSessionStores attaches versioned workspace session persistence.
func (service *Service) WithSessionStores(stores session.WorkspaceStoreFactory) *Service {
	service.sessionStores = stores
	return service
}

// WithEditor attaches the native editor integration behind its replaceable
// application contract.
func (service *Service) WithEditor(editors editor.Service) *Service {
	service.editors = editors
	return service
}

// Execute coordinates implemented Foundation actions and delegates future
// actions to explicit placeholders.
func (service *Service) Execute(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if command.Action == ActionConfig {
		return service.executeConfig(command)
	}
	if command.Action == ActionSecrets {
		return service.executeSecrets(command)
	}
	if command.Action == ActionOpen {
		return service.executeOpen(ctx, command)
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
	if err := service.generateFoundationDocuments(ctx, created); err != nil {
		return Result{}, err
	}

	return Result{Message: fmt.Sprintf("Kelyro workspace ready at %s", created.Root)}, nil
}

func (service *Service) executeOpen(ctx context.Context, command Command) (Result, error) {
	if service.editors == nil {
		return Result{}, fmt.Errorf("editor service is unavailable")
	}
	if service.configs == nil {
		return Result{}, fmt.Errorf("configuration store is unavailable")
	}

	targetWorkspace, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	settings, err := service.resolvedConfigForWorkspace(targetWorkspace.Root, command.ConfigOverrides)
	if err != nil {
		return Result{}, err
	}
	configured, _ := settings[config.KeyEditorCommand].StringField()

	var target string
	switch command.OpenTarget {
	case "":
		target, err = platform.WorkspaceLearningPath(targetWorkspace.Root)
	case "roadmap":
		target, err = platform.WorkspaceRoadmapPath(targetWorkspace.Root)
	default:
		return Result{}, fmt.Errorf("unsupported artifact %q", command.OpenTarget)
	}
	if err != nil {
		return Result{}, fmt.Errorf("resolve artifact path: %w", err)
	}

	selection, err := service.editors.Open(ctx, target, configured)
	if err != nil {
		return Result{}, err
	}
	return Result{Message: fmt.Sprintf("Opened %s with %s", filepath.Base(target), selection.Name)}, nil
}

func (service *Service) discoverWorkspace(command Command) (workspace.Workspace, error) {
	if service.workspaces == nil {
		return workspace.Workspace{}, fmt.Errorf("workspace service is unavailable")
	}
	start := command.Workspace
	if start == "" {
		if service.currentDirectory == nil {
			return workspace.Workspace{}, fmt.Errorf("current directory provider is unavailable")
		}
		var err error
		start, err = service.currentDirectory()
		if err != nil {
			return workspace.Workspace{}, fmt.Errorf("find current directory: %w", err)
		}
	}
	found, err := service.workspaces.Discover(start)
	if err != nil {
		return workspace.Workspace{}, err
	}
	return found, nil
}

func (service *Service) resolvedConfigForWorkspace(root string, overrides config.Settings) (config.Settings, error) {
	global, err := service.configs.LoadGlobal()
	if err != nil {
		return nil, err
	}
	project, err := service.configs.LoadProject(root)
	if err != nil {
		return nil, err
	}
	return config.Resolve(global, project, overrides)
}

func (service *Service) generateFoundationDocuments(ctx context.Context, target workspace.Workspace) error {
	if service.artifactStores == nil {
		return fmt.Errorf("workspace artifact store is unavailable")
	}
	documents, err := artifactmarkdown.Generate(artifactmarkdown.Model{Workspace: filepath.Base(target.Root)})
	if err != nil {
		return err
	}
	store, err := service.artifactStores.Open(ctx, target.Root)
	if err != nil {
		return fmt.Errorf("open workspace artifact store: %w", err)
	}

	var writeErr error
	for _, document := range documents {
		_, err := store.Write(ctx, artifacts.WriteRequest{
			Path:            document.Path,
			Ownership:       artifacts.SystemGeneratedHumanReadable,
			CreatedBy:       artifactmarkdown.Creator,
			Content:         document.Content,
			ExpectedVersion: document.TemplateVersion,
		})
		if err != nil {
			writeErr = fmt.Errorf("generate workspace document %s: %w", filepath.ToSlash(document.Path), err)
			break
		}
	}
	if closeErr := store.Close(); closeErr != nil {
		writeErr = errors.Join(writeErr, closeErr)
	}
	return writeErr
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
		message := formatSettings(settings)
		if service.secrets != nil {
			statuses, err := service.secrets.Status()
			if err != nil {
				return Result{}, err
			}
			if rendered := formatSecretStatuses(statuses); rendered != "" {
				message += "\n" + rendered
			}
		}
		return Result{Message: message}, nil
	case "path":
		return service.configPaths(command)
	case "set":
		return service.setConfig(command)
	default:
		return Result{}, fmt.Errorf("unsupported config operation %q", command.ConfigOperation)
	}
}

func (service *Service) executeSecrets(command Command) (Result, error) {
	if service.secrets == nil {
		return Result{}, fmt.Errorf("secret store is unavailable")
	}

	switch command.SecretOperation {
	case "status":
		statuses, err := service.secrets.Status()
		if err != nil {
			return Result{}, err
		}
		message := formatSecretStatuses(statuses)
		if err := service.secrets.Availability(); err != nil {
			message += "\nkeychain: unavailable (" + err.Error() + ")"
		} else {
			message += "\nkeychain: available"
		}
		return Result{Message: message}, nil
	case "set":
		if err := service.secrets.Set(command.SecretName, command.SecretValue); err != nil {
			return Result{}, errors.New(storage.Redact(err.Error(), command.SecretValue))
		}
		return Result{Message: fmt.Sprintf("Secret %q configured in the OS keychain (reference: %s)", command.SecretName, command.SecretName)}, nil
	case "delete":
		if err := service.secrets.Delete(command.SecretName); err != nil {
			return Result{}, err
		}
		return Result{Message: fmt.Sprintf("Secret %q deleted from the OS keychain; environment variables are unchanged", command.SecretName)}, nil
	default:
		return Result{}, fmt.Errorf("unsupported secrets operation %q", command.SecretOperation)
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

func formatSecretStatuses(statuses []storage.SecretStatus) string {
	lines := make([]string, 0, len(statuses))
	for _, status := range statuses {
		state := "not configured"
		if status.Configured {
			state = "configured"
		}
		label := "secret." + status.Name
		if status.Name == "<name>" {
			label = "secret"
		}
		lines = append(lines, fmt.Sprintf("%s = %s (reference: %s)", label, state, status.Reference))
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
	default:
		return Result{}, fmt.Errorf("unsupported Foundation action %q", command.Action)
	}
}
