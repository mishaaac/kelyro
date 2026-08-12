package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/artifacts"
	artifactmarkdown "github.com/mishaaac/kelyro/internal/artifacts/markdown"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/editor"
	"github.com/mishaaac/kelyro/internal/storage"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestBootstrapServiceSupportsFoundationActions(t *testing.T) {
	t.Parallel()

	for _, action := range []Action{
		ActionTUI,
		ActionStatus,
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

func TestServiceOpensFoundationArtifactsWithResolvedEditor(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "workspace with spaces")
	workspaces := &recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}
	configs := &recordingConfigStore{
		global:  config.Settings{config.KeyEditorCommand: config.StringValue("vim")},
		project: config.Settings{config.KeyEditorCommand: config.StringValue("code")},
	}
	editors := &recordingEditorService{selection: editor.Selection{Name: "Visual Studio Code", Executable: "/usr/bin/code"}}
	service := NewService(workspaces, func() (string, error) { return filepath.Join(root, "lessons"), nil }).
		WithConfig(configs).
		WithEditor(editors)

	tests := []struct {
		name       string
		openTarget string
		wantPath   string
	}{
		{name: "learning by default", wantPath: filepath.Join(root, "LEARNING.md")},
		{name: "roadmap", openTarget: "roadmap", wantPath: filepath.Join(root, "00-roadmap", "ROADMAP.md")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := service.Execute(context.Background(), Command{Action: ActionOpen, OpenTarget: test.openTarget})
			if err != nil {
				t.Fatalf("Execute(open) error = %v", err)
			}
			if editors.target != test.wantPath || editors.configured != "code" {
				t.Errorf("Open() target = %q configured = %q, want %q and code", editors.target, editors.configured, test.wantPath)
			}
			if result.Message != "Opened "+filepath.Base(test.wantPath)+" with Visual Studio Code" {
				t.Errorf("Execute(open) message = %q", result.Message)
			}
		})
	}
	if workspaces.discoverStart != filepath.Join(root, "lessons") {
		t.Errorf("Discover() start = %q", workspaces.discoverStart)
	}
	if configs.projectRoot != root {
		t.Errorf("LoadProject() root = %q, want %q", configs.projectRoot, root)
	}
}

func TestServiceReportsLocalRoadmapPath(t *testing.T) {
	t.Parallel()
	root := filepath.Join(string(filepath.Separator), "workspace with spaces")
	service := NewService(
		&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}},
		func() (string, error) { return filepath.Join(root, "lesson"), nil },
	)

	result, err := service.Execute(context.Background(), Command{Action: ActionRoadmap})
	if err != nil {
		t.Fatalf("Execute(roadmap) error = %v", err)
	}
	want := filepath.Join(root, "00-roadmap", "ROADMAP.md")
	if result.Message != want {
		t.Errorf("Execute(roadmap) = %q, want %q", result.Message, want)
	}
}

func TestServiceOpenHonorsWorkspaceOverrideAndPropagatesFailures(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("configured editor missing")
	workspaces := &recordingWorkspaceService{discovered: workspace.Workspace{Root: "/project"}}
	editors := &recordingEditorService{err: wantErr}
	service := NewService(workspaces, func() (string, error) {
		t.Fatal("current directory called with workspace override")
		return "", nil
	}).WithConfig(&recordingConfigStore{}).WithEditor(editors)

	_, err := service.Execute(context.Background(), Command{Action: ActionOpen, Workspace: "/project/subdirectory"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute(open) error = %v, want editor error", err)
	}
	if workspaces.discoverStart != "/project/subdirectory" {
		t.Errorf("Discover() start = %q", workspaces.discoverStart)
	}
}

func TestServiceInitializesRequestedWorkspace(t *testing.T) {
	t.Parallel()

	workspaces := &recordingWorkspaceService{
		workspace: workspace.Workspace{Root: "/normalized/project"},
	}
	artifactStore := &recordingArtifactStore{}
	service := NewService(workspaces, func() (string, error) {
		t.Fatal("current directory called with explicit workspace")
		return "", nil
	}).WithArtifactStores(&recordingArtifactStoreFactory{store: artifactStore})

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
	if len(artifactStore.requests) != 2 || !artifactStore.closed {
		t.Fatalf("artifact writes = %d, closed = %v; want two writes and close", len(artifactStore.requests), artifactStore.closed)
	}
	assertGeneratedRequest(t, artifactStore.requests[0], "LEARNING.md", artifactmarkdown.LearningTemplateVersion)
	assertGeneratedRequest(t, artifactStore.requests[1], filepath.Join("00-roadmap", "ROADMAP.md"), artifactmarkdown.RoadmapTemplateVersion)
	if !strings.Contains(string(artifactStore.requests[0].Content), "Workspace: project\n") {
		t.Errorf("LEARNING.md request does not contain workspace display name:\n%s", artifactStore.requests[0].Content)
	}
}

func TestServiceInitializesCurrentDirectoryByDefault(t *testing.T) {
	t.Parallel()

	workspaces := &recordingWorkspaceService{workspace: workspace.Workspace{Root: "/current"}}
	service := NewService(workspaces, func() (string, error) { return "/current", nil }).
		WithArtifactStores(&recordingArtifactStoreFactory{store: &recordingArtifactStore{}})

	if _, err := service.Execute(context.Background(), Command{Action: ActionInit}); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}
	if workspaces.initRoot != "/current" {
		t.Errorf("Init() root = %q, want /current", workspaces.initRoot)
	}
}

func TestServicePreservesArtifactWriteErrorsAndClosesStore(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("generated artifact was modified externally")
	store := &recordingArtifactStore{writeErr: wantErr}
	service := NewService(
		&recordingWorkspaceService{workspace: workspace.Workspace{Root: "/project"}},
		func() (string, error) { return "/project", nil },
	).WithArtifactStores(&recordingArtifactStoreFactory{store: store})

	_, err := service.Execute(context.Background(), Command{Action: ActionInit})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute(init) error = %v, want wrapped write error", err)
	}
	if !store.closed {
		t.Fatal("artifact store was not closed after write error")
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

func TestServiceShowsSecretStateWithoutValuesInConfig(t *testing.T) {
	t.Parallel()

	secret := "must-never-appear-in-config-output"
	secrets := &recordingSecretStore{
		statuses: []storage.SecretStatus{{Name: "openai", Reference: "KELYRO_SECRET_OPENAI", Configured: true}},
		values:   map[string]string{"openai": secret},
	}
	service := NewService(&recordingWorkspaceService{}, func() (string, error) { return "/outside", nil }).
		WithConfig(&recordingConfigStore{}).
		WithSecrets(secrets)

	result, err := service.Execute(context.Background(), Command{Action: ActionConfig, ConfigOperation: "show"})
	if err != nil {
		t.Fatalf("Execute(config show) error = %v", err)
	}
	if !strings.Contains(result.Message, "secret.openai = configured (reference: KELYRO_SECRET_OPENAI)") {
		t.Fatalf("config show output = %q", result.Message)
	}
	if strings.Contains(result.Message, secret) {
		t.Fatal("config show exposed secret value")
	}
}

func TestServiceExecutesSecretCommandsWithoutRenderingValues(t *testing.T) {
	t.Parallel()

	secret := "manual-sensitive-value"
	secrets := &recordingSecretStore{
		statuses: []storage.SecretStatus{{Name: "provider", Reference: "keychain:kelyro/provider", Configured: true}},
		values:   make(map[string]string),
	}
	service := NewService(nil, nil).WithSecrets(secrets)

	setResult, err := service.Execute(context.Background(), Command{
		Action:          ActionSecrets,
		SecretOperation: "set",
		SecretName:      "provider",
		SecretValue:     secret,
	})
	if err != nil {
		t.Fatalf("Execute(secrets set) error = %v", err)
	}
	if secrets.values["provider"] != secret {
		t.Fatal("secret store did not receive the supplied value")
	}
	if strings.Contains(setResult.Message, secret) {
		t.Fatal("set result exposed secret value")
	}

	statusResult, err := service.Execute(context.Background(), Command{Action: ActionSecrets, SecretOperation: "status"})
	if err != nil {
		t.Fatalf("Execute(secrets status) error = %v", err)
	}
	if !strings.Contains(statusResult.Message, "configured") || strings.Contains(statusResult.Message, secret) {
		t.Fatalf("status result = %q", statusResult.Message)
	}

	if _, err := service.Execute(context.Background(), Command{Action: ActionSecrets, SecretOperation: "delete", SecretName: "provider"}); err != nil {
		t.Fatalf("Execute(secrets delete) error = %v", err)
	}
	if _, found := secrets.values["provider"]; found {
		t.Fatal("delete left secret in fake store")
	}
}

func TestServiceRedactsSecretStoreErrors(t *testing.T) {
	t.Parallel()

	secret := "backend-echoed-secret"
	secrets := &recordingSecretStore{setErr: errors.New("backend rejected " + secret)}
	service := NewService(nil, nil).WithSecrets(secrets)
	_, err := service.Execute(context.Background(), Command{
		Action: ActionSecrets, SecretOperation: "set", SecretName: "provider", SecretValue: secret,
	})
	if err == nil || strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("Execute(secrets set) error = %v", err)
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
	initRoot      string
	initOptions   workspace.InitOptions
	workspace     workspace.Workspace
	err           error
	discovered    workspace.Workspace
	discoverErr   error
	discoverStart string
}

type recordingArtifactStoreFactory struct {
	store    *recordingArtifactStore
	openRoot string
	openErr  error
}

func (factory *recordingArtifactStoreFactory) Open(_ context.Context, root string) (artifacts.WorkspaceStore, error) {
	factory.openRoot = root
	if factory.openErr != nil {
		return nil, factory.openErr
	}
	return factory.store, nil
}

type recordingArtifactStore struct {
	requests []artifacts.WriteRequest
	writeErr error
	closeErr error
	closed   bool
}

func (store *recordingArtifactStore) Write(_ context.Context, request artifacts.WriteRequest) (artifacts.Artifact, error) {
	store.requests = append(store.requests, request)
	return artifacts.Artifact{}, store.writeErr
}

func (store *recordingArtifactStore) Close() error {
	store.closed = true
	return store.closeErr
}

func assertGeneratedRequest(t *testing.T, request artifacts.WriteRequest, path, version string) {
	t.Helper()
	if request.Path != path || request.Ownership != artifacts.SystemGeneratedHumanReadable ||
		request.CreatedBy != artifactmarkdown.Creator || request.ExpectedVersion != version {
		t.Errorf("generated request = %+v, want path %q and version %q", request, path, version)
	}
}

func (service *recordingWorkspaceService) Discover(start string) (workspace.Workspace, error) {
	service.discoverStart = start
	if service.discoverErr != nil {
		return workspace.Workspace{}, service.discoverErr
	}
	if service.discovered.Root == "" {
		return workspace.Workspace{}, workspace.ErrNotFound
	}
	return service.discovered, nil
}

type recordingEditorService struct {
	selection  editor.Selection
	target     string
	configured string
	err        error
}

func (service *recordingEditorService) Detect(string) (editor.Selection, error) {
	return service.selection, service.err
}

func (service *recordingEditorService) Open(_ context.Context, target, configured string) (editor.Selection, error) {
	service.target = target
	service.configured = configured
	return service.selection, service.err
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

type recordingSecretStore struct {
	values       map[string]string
	statuses     []storage.SecretStatus
	availability error
	setErr       error
}

func (store *recordingSecretStore) Get(name string) (string, error) {
	value, found := store.values[name]
	if !found {
		return "", storage.ErrSecretNotFound
	}
	return value, nil
}

func (store *recordingSecretStore) Set(name, value string) error {
	if store.setErr != nil {
		return store.setErr
	}
	if store.values == nil {
		store.values = make(map[string]string)
	}
	store.values[name] = value
	return nil
}

func (store *recordingSecretStore) Delete(name string) error {
	delete(store.values, name)
	return nil
}

func (store *recordingSecretStore) Status() ([]storage.SecretStatus, error) {
	return store.statuses, nil
}

func (store *recordingSecretStore) Availability() error { return store.availability }

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
