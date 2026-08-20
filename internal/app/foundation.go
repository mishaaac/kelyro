package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/learning"
)

// FoundationCheck is one presentation-independent health result shown by the
// Foundation interface. Detail is intentionally safe for display to a user.
type FoundationCheck struct {
	Name   string
	OK     bool
	Detail string
}

// FoundationSnapshot is the application state required by the first TUI. It
// deliberately contains no Bubble Tea or terminal concepts.
type FoundationSnapshot struct {
	WorkspaceName string
	WorkspaceRoot string
	Checks        []FoundationCheck
	Settings      config.Settings
	LearningPath  bool
	Diagnostics   doctor.Report
}

// LoadFoundation gathers the independently testable state consumed by the
// terminal interface. A discovered workspace is required, while configuration
// and database failures are returned as individual diagnostic checks so the UI
// can still render useful recovery information.
func (service *Service) LoadFoundation(ctx context.Context, command Command) (FoundationSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return FoundationSnapshot{}, err
	}

	found, err := service.discoverWorkspace(command)
	if err != nil {
		return FoundationSnapshot{}, err
	}

	snapshot := FoundationSnapshot{
		WorkspaceName: filepath.Base(found.Root),
		WorkspaceRoot: found.Root,
		Checks: []FoundationCheck{{
			Name: "Workspace initialized",
			OK:   true,
		}},
		Settings: config.Defaults(),
	}

	configCheck := FoundationCheck{Name: "Configuration loaded"}
	if service.configs == nil {
		configCheck.Detail = "configuration store is unavailable"
	} else {
		settings, loadErr := service.resolvedConfigForWorkspace(found.Root, command.ConfigOverrides)
		if loadErr != nil {
			configCheck.Detail = loadErr.Error()
		} else {
			snapshot.Settings = settings
			configCheck.OK = true
			if configuredName, ok := settings[config.KeyWorkspaceName].StringField(); ok && strings.TrimSpace(configuredName) != "" {
				snapshot.WorkspaceName = configuredName
			}
		}
	}

	databaseCheck := FoundationCheck{Name: "Database healthy"}
	if service.artifactStores == nil {
		databaseCheck.Detail = "workspace database is unavailable"
	} else {
		store, openErr := service.artifactStores.Open(ctx, found.Root)
		if openErr != nil {
			databaseCheck.Detail = openErr.Error()
		} else if closeErr := store.Close(); closeErr != nil {
			databaseCheck.Detail = fmt.Sprintf("close workspace database: %v", closeErr)
		} else {
			databaseCheck.OK = true
		}
	}

	// Keep the Home order aligned with the product specification.
	snapshot.Checks = append(snapshot.Checks, databaseCheck, configCheck)
	if service.profiles != nil {
		store, openErr := service.profiles.Open(ctx, found.Root)
		if openErr == nil {
			if store.Setup() != nil {
				if setup, setupErr := store.Setup().Show(ctx); setupErr == nil {
					snapshot.LearningPath = setup.Setup.Status == learning.SetupCompleted
				}
			}
			_ = store.Close()
		}
	}
	snapshot.Diagnostics = service.doctorReport(ctx, command, found)
	return snapshot, nil
}
