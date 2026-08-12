package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/portability"
)

func (service *Service) executePortability(ctx context.Context, command Command) (Result, error) {
	if service.portability == nil {
		return Result{}, errors.New("workspace portability service is unavailable")
	}
	if command.Action == ActionExport {
		found, err := service.discoverWorkspace(command)
		if err != nil {
			return Result{}, err
		}
		report, err := service.portability.Export(ctx, found.Root, portability.ExportOptions{
			Mode: command.ExportMode, OutputPath: command.ExportOutput,
		})
		if err != nil {
			return Result{}, err
		}
		return Result{Portability: &report}, nil
	}

	destination := command.Workspace
	if destination == "" {
		if service.currentDirectory == nil {
			return Result{}, errors.New("current directory provider is unavailable")
		}
		var err error
		destination, err = service.currentDirectory()
		if err != nil {
			return Result{}, fmt.Errorf("find import destination: %w", err)
		}
		if service.workspaces != nil {
			if found, discoverErr := service.workspaces.Discover(destination); discoverErr == nil {
				destination = found.Root
			}
		}
	}
	report, err := service.portability.Import(ctx, portability.ImportOptions{
		ArchivePath: command.ImportArchive, Destination: destination,
		DryRun: command.ImportDryRun, Conflicts: command.ImportConflicts,
	})
	if err != nil {
		return Result{}, err
	}
	if !command.ImportDryRun && service.workspaces != nil && service.workspaces.Validate(report.Destination) == nil {
		if err := service.recordAudit(ctx, report.Destination, audit.Event{
			Name: "import.completed", Actor: audit.ActorUser, Subject: report.ArchivePath,
			Metadata: map[string]string{
				"mode": string(report.Mode), "files": strconv.Itoa(report.FileCount),
				"conflict_strategy": string(command.ImportConflicts),
			},
		}); err != nil {
			return Result{}, err
		}
	}
	return Result{Portability: &report}, nil
}
