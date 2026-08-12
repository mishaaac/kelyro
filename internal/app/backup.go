package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/config"
)

func (service *Service) executeBackup(ctx context.Context, command Command) (Result, error) {
	if service.backups == nil {
		return Result{}, errors.New("workspace backup service is unavailable")
	}
	found, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}

	switch command.BackupOperation {
	case "create":
		if service.configs == nil {
			return Result{}, errors.New("configuration store is unavailable")
		}
		settings, err := service.resolvedConfigForWorkspace(found.Root, command.ConfigOverrides)
		if err != nil {
			return Result{}, err
		}
		retentionValue, ok := settings[config.KeyBackupRetention].NumberField()
		if !ok {
			return Result{}, errors.New("backup retention configuration is invalid")
		}
		created, err := service.backups.Create(ctx, found.Root, backup.CreateOptions{Reason: "manual", Retention: int(retentionValue)})
		if err != nil {
			return Result{}, err
		}
		if err := service.recordAudit(ctx, found.Root, audit.Event{
			Name: "backup.created", Actor: audit.ActorUser, Subject: created.ID,
			Metadata: map[string]string{"reason": created.Reason, "files": strconv.Itoa(created.FileCount)},
		}); err != nil {
			return Result{}, err
		}
		return Result{Message: fmt.Sprintf("Created backup %s", created.ID)}, nil
	case "list":
		backups, err := service.backups.List(ctx, found.Root)
		if err != nil {
			return Result{}, err
		}
		if backups == nil {
			backups = []backup.Info{}
		}
		return Result{Backups: backups}, nil
	case "restore":
		if !command.BackupConfirmed {
			return Result{}, backup.ErrConfirmation
		}
		restored, err := service.backups.Restore(ctx, found.Root, command.BackupID)
		if err != nil {
			return Result{}, err
		}
		if err := service.recordAudit(ctx, found.Root, audit.Event{
			Name: "backup.restored", Actor: audit.ActorUser, Subject: restored.ID,
			Metadata: map[string]string{"created_at": restored.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")},
		}); err != nil {
			return Result{}, err
		}
		return Result{Message: fmt.Sprintf("Restored backup %s", restored.ID)}, nil
	default:
		return Result{}, fmt.Errorf("unsupported backup operation %q", command.BackupOperation)
	}
}
