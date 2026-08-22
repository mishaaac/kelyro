package app

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/config"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesMaintenanceDryRunWithoutBackup(t *testing.T) {
	t.Parallel()
	maintenance := &fakeMaintenanceService{impact: learningapp.RecalculationImpact{
		DryRun: true, EvidenceRecords: 3, ConceptsScanned: 2,
	}}
	profiles := &fakeProfileStoreFactory{maintenance: maintenance}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: "/project"}}, nil).WithProfiles(profiles)

	result, err := service.Execute(context.Background(), Command{
		Action: ActionMaintenance, MaintenanceOperation: "recalculate", MaintenanceDryRun: true, Workspace: "/project",
	})
	if err != nil || result.Maintenance == nil || !result.Maintenance.DryRun {
		t.Fatalf("Execute(maintenance dry-run) = (%+v, %v)", result, err)
	}
	if maintenance.calls != 1 || !maintenance.request.DryRun || maintenance.request.BackupID != "" || profiles.closed != 1 {
		t.Fatalf("maintenance calls=%d request=%+v closes=%d", maintenance.calls, maintenance.request, profiles.closed)
	}
}

func TestServiceBacksUpAppliesAndAuditsMaintenance(t *testing.T) {
	t.Parallel()
	maintenance := &fakeMaintenanceService{impact: learningapp.RecalculationImpact{
		Target:               learningapp.AlgorithmVersionSummary{Mastery: []string{"mastery-v1"}, Retention: []string{"retention-v1"}, DailyPlan: []string{"daily-plan-v1"}},
		ConceptStatesChanged: 2, RetentionStatesChanged: 2, ReviewSchedulesChanged: 1, ReviewItemsChanged: 1, DailyPlansChanged: 1,
	}}
	backups := &recordingBackupService{created: backup.Info{ID: "backup-maintenance", Reason: "learning-algorithm-recalculation", FileCount: 3}}
	auditStore := &recordingAuditStore{}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: "/project"}}, nil).
		WithProfiles(&fakeProfileStoreFactory{maintenance: maintenance}).
		WithConfig(&recordingConfigStore{global: config.Settings{config.KeyBackupRetention: config.NumberValue(5)}}).
		WithBackups(backups).
		WithAudit(&recordingAuditFactory{store: auditStore})

	result, err := service.Execute(context.Background(), Command{
		Action: ActionMaintenance, MaintenanceOperation: "recalculate", Workspace: "/project",
	})
	if err != nil || result.Maintenance == nil {
		t.Fatalf("Execute(maintenance apply) = (%+v, %v)", result, err)
	}
	if backups.createRoot != "/project" || backups.options.Reason != "learning-algorithm-recalculation" || backups.options.Retention != 5 {
		t.Fatalf("backup call root=%q options=%+v", backups.createRoot, backups.options)
	}
	if maintenance.calls != 1 || maintenance.request.DryRun || maintenance.request.BackupID != "backup-maintenance" {
		t.Fatalf("maintenance request=%+v calls=%d", maintenance.request, maintenance.calls)
	}
	if len(auditStore.events) != 2 || auditStore.events[0].Name != "backup.created" ||
		auditStore.events[1].Name != "learning.recalculation.completed" || auditStore.events[1].Actor != audit.ActorUser ||
		auditStore.events[1].Metadata["backup_id"] != "backup-maintenance" {
		t.Fatalf("audit events=%+v", auditStore.events)
	}
}

func TestServiceDoesNotRecalculateWhenMaintenanceBackupFails(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("backup unavailable")
	maintenance := &fakeMaintenanceService{}
	service := NewService(&recordingWorkspaceService{discovered: workspace.Workspace{Root: "/project"}}, nil).
		WithProfiles(&fakeProfileStoreFactory{maintenance: maintenance}).
		WithConfig(&recordingConfigStore{global: config.Settings{config.KeyBackupRetention: config.NumberValue(5)}}).
		WithBackups(&recordingBackupService{createErr: wantErr})

	_, err := service.Execute(context.Background(), Command{Action: ActionMaintenance, MaintenanceOperation: "recalculate", Workspace: "/project"})
	if !errors.Is(err, wantErr) || maintenance.calls != 0 {
		t.Fatalf("backup failure error=%v maintenance calls=%d", err, maintenance.calls)
	}
}

type fakeMaintenanceService struct {
	impact  learningapp.RecalculationImpact
	err     error
	request learningapp.RecalculationRequest
	calls   int
}

func (service *fakeMaintenanceService) Recalculate(_ context.Context, request learningapp.RecalculationRequest) (learningapp.RecalculationImpact, error) {
	service.calls++
	service.request = request
	result := service.impact
	result.BackupID = request.BackupID
	result.DryRun = request.DryRun
	result.Applied = !request.DryRun
	return result, service.err
}
