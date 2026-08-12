package app

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/logging"
	"github.com/mishaaac/kelyro/internal/privacy"
)

// NetworkGate resolves the current global/project/CLI privacy policy and
// returns the boundary future network-capable components must consult.
func (service *Service) NetworkGate(command Command) (privacy.NetworkGate, error) {
	if service.configs == nil {
		return nil, fmt.Errorf("configuration store is unavailable")
	}
	settings, err := service.resolvedConfig(command)
	if err != nil {
		return nil, err
	}
	policy, err := policyFromSettings(settings)
	if err != nil {
		return nil, err
	}
	recorder := &privacyLogRecorder{
		factory: service.loggers,
		root:    service.observabilityRoot(command),
		verbose: command.Verbose,
	}
	return privacy.NewNetworkGate(policy, recorder), nil
}

func policyFromSettings(settings config.Settings) (privacy.Policy, error) {
	allowNetwork, err := booleanSetting(settings, config.KeyAllowNetwork)
	if err != nil {
		return privacy.Policy{}, err
	}
	allowAIContent, err := booleanSetting(settings, config.KeyAllowAIContent)
	if err != nil {
		return privacy.Policy{}, err
	}
	allowTelemetry, err := booleanSetting(settings, config.KeyAllowTelemetry)
	if err != nil {
		return privacy.Policy{}, err
	}
	return privacy.Policy{
		AllowNetwork:        allowNetwork,
		AllowAIContent:      allowAIContent,
		AllowUsageTelemetry: allowTelemetry,
	}, nil
}

func booleanSetting(settings config.Settings, key string) (bool, error) {
	value, ok := settings[key]
	if !ok {
		return false, fmt.Errorf("privacy configuration %q is missing", key)
	}
	result, ok := value.BoolField()
	if !ok {
		return false, fmt.Errorf("privacy configuration %q is invalid", key)
	}
	return result, nil
}

type privacyLogRecorder struct {
	factory logging.WorkspaceFactory
	root    string
	verbose bool
}

func (recorder *privacyLogRecorder) RecordBlocked(ctx context.Context, event privacy.BlockedEvent) error {
	if recorder.factory == nil || recorder.root == "" {
		return nil
	}
	logger, err := recorder.factory.Open(recorder.root, recorder.verbose)
	if err != nil {
		return err
	}
	logErr := logger.Log(ctx, logging.Entry{
		Level:         logging.Warn,
		Message:       "network operation blocked by privacy policy",
		Operation:     event.Operation,
		Workspace:     recorder.root,
		Component:     "privacy",
		ErrorCategory: "privacy",
		Fields: map[string]string{
			"decision": "blocked",
			"purpose":  string(event.Purpose),
		},
	})
	if closeErr := logger.Close(); logErr == nil {
		logErr = closeErr
	}
	return logErr
}
