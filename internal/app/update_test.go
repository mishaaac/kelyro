package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/logging"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/update"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceUpdateCheckIsOfflineAndNonFatalByDefault(t *testing.T) {
	t.Parallel()
	root := filepath.Join(string(filepath.Separator), "offline workspace")
	provider := &recordingReleaseProvider{release: update.Release{Version: "2.0.0"}, found: true}
	logs := &recordingLogFactory{}
	service := NewService(
		&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}},
		func() (string, error) { return root, nil },
	).WithConfig(&recordingConfigStore{}).
		WithLogging(logs).
		WithUpdates(update.New("1.0.0", provider, nil))

	result, err := service.Execute(context.Background(), Command{Action: ActionUpdate, UpdateOperation: "check"})
	if err != nil {
		t.Fatalf("Execute(update check) error = %v", err)
	}
	if result.Update == nil || result.Update.Status != update.Unavailable {
		t.Fatalf("Execute(update check) result = %+v", result)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want zero offline", provider.calls)
	}
	foundPrivacyLog := false
	for _, entry := range logs.logger.entries {
		if entry.Level == logging.Warn && entry.Operation == "update.check" && entry.Component == "privacy" {
			foundPrivacyLog = true
		}
	}
	if !foundPrivacyLog {
		t.Fatalf("privacy denial log not found: %+v", logs.logger.entries)
	}
}

func TestServiceUpdateCheckHonorsEnabledChannelAndReportsAvailable(t *testing.T) {
	t.Parallel()
	provider := &recordingReleaseProvider{release: update.Release{Version: "1.1.0-beta.1"}, found: true}
	service := NewService(&recordingWorkspaceService{}, func() (string, error) { return "/outside", nil }).
		WithConfig(&recordingConfigStore{global: config.Settings{
			config.KeyAllowNetwork:  config.BoolValue(true),
			config.KeyUpdateChannel: config.StringValue("prerelease"),
		}}).
		WithUpdates(update.New("1.0.0", provider, nil))

	result, err := service.Execute(context.Background(), Command{Action: ActionUpdate, UpdateOperation: "check"})
	if err != nil {
		t.Fatalf("Execute(update check) error = %v", err)
	}
	if result.Update == nil || result.Update.Status != update.UpdateAvailable || result.Update.Channel != update.Prerelease {
		t.Fatalf("Execute(update check) result = %+v", result)
	}
	if provider.calls != 1 || provider.channel != update.Prerelease {
		t.Fatalf("provider calls = %d channel = %s", provider.calls, provider.channel)
	}
}

func TestServiceUpdateCheckCanBeDisabledWithoutCallingChecker(t *testing.T) {
	t.Parallel()
	checker := &recordingUpdateChecker{}
	service := NewService(&recordingWorkspaceService{}, func() (string, error) { return "/outside", nil }).
		WithConfig(&recordingConfigStore{global: config.Settings{config.KeyUpdateCheck: config.BoolValue(false)}}).
		WithUpdates(checker)

	result, err := service.Execute(context.Background(), Command{Action: ActionUpdate, UpdateOperation: "check"})
	if err != nil || result.Message != "Update checks are disabled by updates.check." {
		t.Fatalf("Execute(disabled check) = %+v, %v", result, err)
	}
	if checker.calls != 0 {
		t.Fatalf("checker calls = %d, want zero", checker.calls)
	}
}

func TestServiceUpdateInstallRemainsSafelyUnsupported(t *testing.T) {
	t.Parallel()
	service := NewService(nil, nil)
	_, err := service.Execute(context.Background(), Command{Action: ActionUpdate, UpdateOperation: "install"})
	if !errors.Is(err, ErrUpdateUnsupported) {
		t.Fatalf("Execute(update install) error = %v, want ErrUpdateUnsupported", err)
	}
}

type recordingReleaseProvider struct {
	release update.Release
	found   bool
	err     error
	calls   int
	channel update.Channel
}

func (provider *recordingReleaseProvider) Latest(_ context.Context, channel update.Channel) (update.Release, bool, error) {
	provider.calls++
	provider.channel = channel
	return provider.release, provider.found, provider.err
}

type recordingUpdateChecker struct{ calls int }

func (checker *recordingUpdateChecker) Check(context.Context, update.Channel, privacy.NetworkGate) (update.Result, error) {
	checker.calls++
	return update.Result{}, nil
}
