package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCreatesListsAndRestoresBackups(t *testing.T) {
	t.Parallel()
	created := backup.Info{ID: "backup-1", CreatedAt: time.Now(), Reason: "manual", FileCount: 4}
	backups := &recordingBackupService{created: created, listed: []backup.Info{created}, restored: created}
	service := NewService(
		&recordingWorkspaceService{discovered: workspace.Workspace{Root: "/project"}},
		func() (string, error) { return "/project", nil },
	).WithConfig(&recordingConfigStore{
		global: config.Settings{config.KeyBackupRetention: config.NumberValue(7)},
	}).WithBackups(backups)

	result, err := service.Execute(context.Background(), Command{Action: ActionBackup, BackupOperation: "create"})
	if err != nil || result.Message != "Created backup backup-1" {
		t.Fatalf("Execute(create) = (%+v, %v)", result, err)
	}
	if backups.createRoot != "/project" || backups.options.Retention != 7 || backups.options.Reason != "manual" {
		t.Fatalf("Create() = root %q options %+v", backups.createRoot, backups.options)
	}

	result, err = service.Execute(context.Background(), Command{Action: ActionBackup, BackupOperation: "list"})
	if err != nil || len(result.Backups) != 1 || result.Backups[0].ID != "backup-1" {
		t.Fatalf("Execute(list) = (%+v, %v)", result, err)
	}

	_, err = service.Execute(context.Background(), Command{Action: ActionBackup, BackupOperation: "restore", BackupID: "backup-1"})
	if !errors.Is(err, backup.ErrConfirmation) || backups.restoreID != "" {
		t.Fatalf("unconfirmed restore error = %v, restore id = %q", err, backups.restoreID)
	}
	result, err = service.Execute(context.Background(), Command{
		Action: ActionBackup, BackupOperation: "restore", BackupID: "backup-1", BackupConfirmed: true,
	})
	if err != nil || result.Message != "Restored backup backup-1" || backups.restoreID != "backup-1" {
		t.Fatalf("Execute(restore) = (%+v, %v), restore id %q", result, err, backups.restoreID)
	}
}

type recordingBackupService struct {
	createRoot string
	options    backup.CreateOptions
	created    backup.Info
	listed     []backup.Info
	restored   backup.Info
	restoreID  string
}

func (service *recordingBackupService) Create(_ context.Context, root string, options backup.CreateOptions) (backup.Info, error) {
	service.createRoot, service.options = root, options
	return service.created, nil
}

func (service *recordingBackupService) List(context.Context, string) ([]backup.Info, error) {
	return service.listed, nil
}

func (service *recordingBackupService) Restore(_ context.Context, _ string, id string) (backup.Info, error) {
	service.restoreID = id
	return service.restored, nil
}

var _ backup.Service = (*recordingBackupService)(nil)
