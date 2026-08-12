package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestBootstrapServiceSupportsFoundationActions(t *testing.T) {
	t.Parallel()

	for _, action := range []Action{
		ActionTUI,
		ActionDoctor,
		ActionStatus,
		ActionOpen,
	} {
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()

			result, err := (BootstrapService{}).Execute(context.Background(), Command{Action: action})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !strings.Contains(result.Message, "not implemented yet") {
				t.Errorf("Execute() message = %q, want explicit placeholder", result.Message)
			}
		})
	}
}

func TestServiceInitializesRequestedWorkspace(t *testing.T) {
	t.Parallel()

	workspaces := &recordingWorkspaceService{
		workspace: workspace.Workspace{Root: "/normalized/project"},
	}
	service := NewService(workspaces, func() (string, error) {
		t.Fatal("current directory called with explicit workspace")
		return "", nil
	})

	result, err := service.Execute(context.Background(), Command{
		Action:      ActionInit,
		Workspace:   "project",
		AllowNested: true,
	})
	if err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}
	if workspaces.initRoot != "project" {
		t.Errorf("Init() root = %q, want project", workspaces.initRoot)
	}
	if !workspaces.initOptions.AllowNested {
		t.Error("Init() AllowNested = false, want true")
	}
	if result.Message != "Kelyro workspace ready at /normalized/project" {
		t.Errorf("Execute(init) message = %q", result.Message)
	}
}

func TestServiceInitializesCurrentDirectoryByDefault(t *testing.T) {
	t.Parallel()

	workspaces := &recordingWorkspaceService{workspace: workspace.Workspace{Root: "/current"}}
	service := NewService(workspaces, func() (string, error) { return "/current", nil })

	if _, err := service.Execute(context.Background(), Command{Action: ActionInit}); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}
	if workspaces.initRoot != "/current" {
		t.Errorf("Init() root = %q, want /current", workspaces.initRoot)
	}
}

func TestServiceReportsCurrentDirectoryFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("cwd unavailable")
	service := NewService(&recordingWorkspaceService{}, func() (string, error) { return "", wantErr })

	_, err := service.Execute(context.Background(), Command{Action: ActionInit})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute(init) error = %v, want wrapped cwd error", err)
	}
}

func TestServiceShowsResolvedLayeredConfiguration(t *testing.T) {
	t.Parallel()

	workspaces := &recordingWorkspaceService{
		discovered: workspace.Workspace{Root: "/project"},
	}
	configs := &recordingConfigStore{
		global: config.Settings{
			config.KeyUIColor:     config.StringValue("always"),
			config.KeyUpdateCheck: config.BoolValue(false),
		},
		project: config.Settings{
			config.KeyUIColor:       config.StringValue("auto"),
			config.KeyWorkspaceName: config.StringValue("Backend Go"),
		},
	}
	service := NewService(workspaces, func() (string, error) { return "/project/lesson", nil }).WithConfig(configs)

	result, err := service.Execute(context.Background(), Command{
		Action:          ActionConfig,
		ConfigOperation: "show",
		ConfigOverrides: config.Settings{config.KeyUIColor: config.StringValue("never")},
	})
	if err != nil {
		t.Fatalf("Execute(config show) error = %v", err)
	}
	for _, want := range []string{
		`ui.color = "never"`,
		"updates.check = false",
		`workspace.name = "Backend Go"`,
		"privacy.allow_network = false",
	} {
		if !strings.Contains(result.Message, want) {
			t.Errorf("config show output does not contain %q:\n%s", want, result.Message)
		}
	}
	if configs.projectRoot != "/project" {
		t.Errorf("LoadProject() root = %q, want discovered root", configs.projectRoot)
	}
}

func TestServiceGetsDefaultsWithoutWorkspace(t *testing.T) {
	t.Parallel()

	service := NewService(&recordingWorkspaceService{}, func() (string, error) { return "/outside", nil }).WithConfig(&recordingConfigStore{})
	result, err := service.Execute(context.Background(), Command{
		Action:          ActionConfig,
		ConfigOperation: "get",
		ConfigKey:       config.KeyMasteryThreshold,
	})
	if err != nil {
		t.Fatalf("Execute(config get) error = %v", err)
	}
	if result.Message != "0.85" {
		t.Errorf("config get output = %q, want default 0.85", result.Message)
	}
}

func TestServiceSetsProjectByDefaultAndGlobalExplicitly(t *testing.T) {
	t.Parallel()

	workspaces := &recordingWorkspaceService{discovered: workspace.Workspace{Root: "/project"}}
	configs := &recordingConfigStore{globalPath: "/global/config.toml", projectPath: "/project/.kelyro/config.toml"}
	service := NewService(workspaces, func() (string, error) { return "/project", nil }).WithConfig(configs)

	result, err := service.Execute(context.Background(), Command{
		Action:          ActionConfig,
		ConfigOperation: "set",
		ConfigKey:       config.KeyWorkspaceName,
		ConfigValue:     "Backend Go",
	})
	if err != nil {
		t.Fatalf("Execute(project set) error = %v", err)
	}
	if configs.setProjectKey != config.KeyWorkspaceName || configs.setProjectRoot != "/project" {
		t.Errorf("SetProject() = root %q key %q", configs.setProjectRoot, configs.setProjectKey)
	}
	if !strings.Contains(result.Message, configs.projectPath) {
		t.Errorf("set result = %q, want project path", result.Message)
	}

	_, err = service.Execute(context.Background(), Command{
		Action:          ActionConfig,
		ConfigOperation: "set",
		ConfigScope:     config.ScopeGlobal,
		ConfigKey:       config.KeyUpdateCheck,
		ConfigValue:     "false",
	})
	if err != nil {
		t.Fatalf("Execute(global set) error = %v", err)
	}
	if configs.setGlobalKey != config.KeyUpdateCheck || configs.setGlobalValue.String() != "false" {
		t.Errorf("SetGlobal() = key %q value %q", configs.setGlobalKey, configs.setGlobalValue.String())
	}
}

func TestServiceConfigPathDistinguishesScopes(t *testing.T) {
	t.Parallel()

	workspaces := &recordingWorkspaceService{discovered: workspace.Workspace{Root: "/project"}}
	configs := &recordingConfigStore{globalPath: "/global/config.toml", projectPath: "/project/.kelyro/config.toml"}
	service := NewService(workspaces, func() (string, error) { return "/project", nil }).WithConfig(configs)

	result, err := service.Execute(context.Background(), Command{Action: ActionConfig, ConfigOperation: "path"})
	if err != nil {
		t.Fatalf("Execute(config path) error = %v", err)
	}
	want := "global: /global/config.toml\nproject: /project/.kelyro/config.toml"
	if result.Message != want {
		t.Errorf("config path output = %q, want %q", result.Message, want)
	}
}

func TestBootstrapServiceRejectsUnknownAction(t *testing.T) {
	t.Parallel()

	_, err := (BootstrapService{}).Execute(context.Background(), Command{Action: "unknown"})
	if err == nil {
		t.Fatal("Execute() error = nil, want unsupported action error")
	}
}

func TestBootstrapServiceHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (BootstrapService{}).Execute(ctx, Command{Action: ActionStatus})
	if err != context.Canceled {
		t.Fatalf("Execute() error = %v, want %v", err, context.Canceled)
	}
}

type recordingWorkspaceService struct {
	initRoot    string
	initOptions workspace.InitOptions
	workspace   workspace.Workspace
	err         error
	discovered  workspace.Workspace
	discoverErr error
}

func (service *recordingWorkspaceService) Discover(string) (workspace.Workspace, error) {
	if service.discoverErr != nil {
		return workspace.Workspace{}, service.discoverErr
	}
	if service.discovered.Root == "" {
		return workspace.Workspace{}, workspace.ErrNotFound
	}
	return service.discovered, nil
}

func (service *recordingWorkspaceService) Init(root string, options workspace.InitOptions) (workspace.Workspace, error) {
	service.initRoot = root
	service.initOptions = options
	return service.workspace, service.err
}

func (service *recordingWorkspaceService) Validate(string) error { return nil }

type recordingConfigStore struct {
	global          config.Settings
	project         config.Settings
	globalPath      string
	projectPath     string
	projectRoot     string
	setGlobalKey    string
	setGlobalValue  config.Value
	setProjectRoot  string
	setProjectKey   string
	setProjectValue config.Value
}

func (store *recordingConfigStore) GlobalPath() (string, error) {
	return store.globalPath, nil
}
func (store *recordingConfigStore) ProjectPath(string) (string, error) {
	return store.projectPath, nil
}
func (store *recordingConfigStore) LoadGlobal() (config.Settings, error) {
	return store.global, nil
}
func (store *recordingConfigStore) LoadProject(root string) (config.Settings, error) {
	store.projectRoot = root
	return store.project, nil
}
func (store *recordingConfigStore) SaveGlobal(config.Settings) error { return nil }
func (store *recordingConfigStore) SaveProject(string, config.Settings) error {
	return nil
}
func (store *recordingConfigStore) SetGlobal(key string, value config.Value) error {
	store.setGlobalKey, store.setGlobalValue = key, value
	return nil
}
func (store *recordingConfigStore) SetProject(root, key string, value config.Value) error {
	store.setProjectRoot, store.setProjectKey, store.setProjectValue = root, key, value
	return nil
}
