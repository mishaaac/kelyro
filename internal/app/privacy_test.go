package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/logging"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceNetworkGateUsesResolvedPolicyAndLogsDenials(t *testing.T) {
	t.Parallel()
	root := filepath.Join(string(filepath.Separator), "private student workspace")
	logs := &recordingLogFactory{}
	service := NewService(
		&recordingWorkspaceService{discovered: workspace.Workspace{Root: root}},
		func() (string, error) { return filepath.Join(root, "lesson"), nil },
	).WithConfig(&recordingConfigStore{
		global: config.Settings{config.KeyAllowNetwork: config.BoolValue(false)},
		project: config.Settings{
			config.KeyAllowNetwork:   config.BoolValue(true),
			config.KeyAllowAIContent: config.BoolValue(false),
		},
	}).WithLogging(logs)

	gate, err := service.NetworkGate(Command{})
	if err != nil {
		t.Fatalf("NetworkGate() error = %v", err)
	}
	if err := gate.Authorize(context.Background(), privacy.Request{
		Operation: "update.check", Purpose: privacy.ExternalResource,
	}); err != nil {
		t.Fatalf("Authorize(external) error = %v, want project opt-in", err)
	}
	err = gate.Authorize(context.Background(), privacy.Request{
		Operation: "ai.generate", Purpose: privacy.AIContent,
	})
	if !errors.Is(err, privacy.ErrNetworkBlocked) {
		t.Fatalf("Authorize(ai) error = %v, want ErrNetworkBlocked", err)
	}

	if logs.logger == nil || len(logs.logger.entries) != 1 {
		t.Fatalf("privacy log entries = %+v", logs.logger)
	}
	entry := logs.logger.entries[0]
	if entry.Level != logging.Warn || entry.Operation != "ai.generate" || entry.Component != "privacy" || entry.ErrorCategory != "privacy" {
		t.Fatalf("privacy log entry = %+v", entry)
	}
	if entry.Fields["decision"] != "blocked" || entry.Fields["purpose"] != string(privacy.AIContent) {
		t.Fatalf("privacy log fields = %+v", entry.Fields)
	}
	for _, value := range entry.Fields {
		if value == root {
			t.Fatal("privacy log fields contain a workspace path")
		}
	}
}

func TestServiceNetworkGateHonorsExplicitPrivacyOverrides(t *testing.T) {
	t.Parallel()
	service := NewService(&recordingWorkspaceService{}, func() (string, error) { return "/outside", nil }).
		WithConfig(&recordingConfigStore{})

	gate, err := service.NetworkGate(Command{ConfigOverrides: config.Settings{
		config.KeyAllowNetwork:   config.BoolValue(true),
		config.KeyAllowTelemetry: config.BoolValue(true),
	}})
	if err != nil {
		t.Fatalf("NetworkGate() error = %v", err)
	}
	if err := gate.Authorize(context.Background(), privacy.Request{
		Operation: "telemetry.send", Purpose: privacy.UsageTelemetry,
	}); err != nil {
		t.Fatalf("Authorize(telemetry) error = %v, want explicit opt-in", err)
	}
}
