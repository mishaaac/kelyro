package app_test

import (
	"context"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/storage"
	"github.com/mishaaac/kelyro/internal/workspace"
)

var (
	_ platform.Platform          = fakePlatform{}
	_ workspace.Service          = fakeWorkspaceService{}
	_ config.Store               = fakeConfigStore{}
	_ storage.StateStore         = fakeStateStore{}
	_ storage.WorkspaceMetaStore = fakeWorkspaceMetaStore{}
	_ storage.SecretStore        = fakeSecretStore{}
	_ artifacts.Index            = fakeArtifactIndex{}
	_ audit.Recorder             = fakeAuditRecorder{}
)

type fakePlatform struct{}

func (fakePlatform) Name() string                      { return "fake" }
func (fakePlatform) UserHomeDir() (string, error)      { return "", nil }
func (fakePlatform) UserConfigDir() (string, error)    { return "", nil }
func (fakePlatform) UserCacheDir() (string, error)     { return "", nil }
func (fakePlatform) CommandPath(string) (string, bool) { return "", false }
func (fakePlatform) OpenPath(string) error             { return nil }
func (fakePlatform) OpenURL(string) error              { return nil }

type fakeWorkspaceService struct{}

func (fakeWorkspaceService) Discover(string) (workspace.Workspace, error) {
	return workspace.Workspace{}, nil
}

func (fakeWorkspaceService) Init(string, workspace.InitOptions) (workspace.Workspace, error) {
	return workspace.Workspace{}, nil
}

func (fakeWorkspaceService) Validate(string) error { return nil }

type fakeConfigStore struct{}

func (fakeConfigStore) GlobalPath() (string, error)          { return "", nil }
func (fakeConfigStore) ProjectPath(string) (string, error)   { return "", nil }
func (fakeConfigStore) LoadGlobal() (config.Settings, error) { return nil, nil }
func (fakeConfigStore) LoadProject(string) (config.Settings, error) {
	return nil, nil
}
func (fakeConfigStore) SaveGlobal(config.Settings) error          { return nil }
func (fakeConfigStore) SaveProject(string, config.Settings) error { return nil }
func (fakeConfigStore) SetGlobal(string, config.Value) error      { return nil }
func (fakeConfigStore) SetProject(string, string, config.Value) error {
	return nil
}

type fakeStateStore struct{}

func (fakeStateStore) Get(context.Context, string, string) ([]byte, bool, error) {
	return nil, false, nil
}
func (fakeStateStore) Set(context.Context, string, string, []byte) error { return nil }
func (fakeStateStore) Delete(context.Context, string, string) error      { return nil }

type fakeWorkspaceMetaStore struct{}

func (fakeWorkspaceMetaStore) Get(context.Context, string) ([]byte, bool, error) {
	return nil, false, nil
}
func (fakeWorkspaceMetaStore) Set(context.Context, string, []byte) error { return nil }
func (fakeWorkspaceMetaStore) Delete(context.Context, string) error      { return nil }

type fakeArtifactIndex struct{}

func (fakeArtifactIndex) Get(context.Context, string) (artifacts.Artifact, bool, error) {
	return artifacts.Artifact{}, false, nil
}
func (fakeArtifactIndex) Put(context.Context, artifacts.Artifact) error { return nil }
func (fakeArtifactIndex) Delete(context.Context, string) error          { return nil }

type fakeSecretStore struct{}

func (fakeSecretStore) Get(string) (string, error)              { return "", nil }
func (fakeSecretStore) Set(string, string) error                { return nil }
func (fakeSecretStore) Delete(string) error                     { return nil }
func (fakeSecretStore) Status() ([]storage.SecretStatus, error) { return nil, nil }
func (fakeSecretStore) Availability() error                     { return nil }

type fakeAuditRecorder struct{}

func (fakeAuditRecorder) Record(context.Context, audit.Event) error { return nil }
