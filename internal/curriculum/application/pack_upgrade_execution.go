package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/backup"
)

const PackUpgradePolicyVersionV1 = "pack-upgrade-policy-v1"

type BackupRetentionProvider func(context.Context, string) (int, error)

// PackUpgradeExecutorV1 adds recoverable side effects to the read-only planner.
// Candidate artifacts are immutable and already installed; activation is the
// final workspace reference change after Student Core migration succeeds.
type PackUpgradeExecutorV1 struct {
	preview    *PackUpgradePlannerV1
	packs      PackInstallService
	backups    backup.Service
	migration  StudentCurriculumMigrationService
	audits     audit.WorkspaceStoreFactory
	retention  BackupRetentionProvider
	appVersion string
}

func NewPackUpgradeExecutorV1(preview *PackUpgradePlannerV1, packs PackInstallService, backups backup.Service,
	migration StudentCurriculumMigrationService, audits audit.WorkspaceStoreFactory, retention BackupRetentionProvider,
	appVersion string) *PackUpgradeExecutorV1 {
	return &PackUpgradeExecutorV1{
		preview: preview, packs: packs, backups: backups, migration: migration, audits: audits,
		retention: retention, appVersion: strings.TrimSpace(appVersion),
	}
}

func (service *PackUpgradeExecutorV1) Upgrade(ctx context.Context, request PackUpgradeRequest) (PackUpgradeResult, error) {
	const operation = "upgrade Learning Pack"
	if err := service.requireDependencies(operation); err != nil {
		return PackUpgradeResult{}, err
	}
	previewRequest := request
	previewRequest.DryRun = true
	result, err := service.preview.Upgrade(ctx, previewRequest)
	if err != nil {
		return PackUpgradeResult{}, err
	}
	if request.DryRun {
		return result, nil
	}
	retention, err := service.retention(ctx, request.WorkspaceRoot)
	if err != nil {
		return PackUpgradeResult{}, ExternalError(operation, fmt.Errorf("resolve backup retention: %w", err))
	}
	backupInfo, err := service.backups.Create(ctx, request.WorkspaceRoot, backup.CreateOptions{
		Reason:    fmt.Sprintf("pack-upgrade-%s-%s-to-%s", request.PackID, result.Current.Manifest.Version.String(), result.Candidate.Manifest.Version.String()),
		Retention: retention,
	})
	if err != nil {
		return PackUpgradeResult{}, ExternalError(operation, fmt.Errorf("create pre-upgrade backup: %w", err))
	}
	result.BackupID = backupInfo.ID
	if !request.Confirmed {
		return result, Invalid(operation, fmt.Errorf("explicit confirmation is required; backup %s is ready", backupInfo.ID))
	}

	migrationResult, err := service.migration.Apply(ctx, StudentCurriculumMigrationRequest{
		WorkspaceRoot: request.WorkspaceRoot, BackupID: backupInfo.ID,
		Old: result.Current.Curriculum, New: result.Candidate.Curriculum, Plan: result.MigrationPlan,
	})
	if err != nil {
		return PackUpgradeResult{}, service.recover(ctx, request, result, backupInfo.ID, "student_migration", err)
	}
	if strings.TrimSpace(migrationResult.ProjectionVersion) == "" {
		return PackUpgradeResult{}, service.recover(ctx, request, result, backupInfo.ID, "student_migration", fmt.Errorf("migration adapter returned no projection version"))
	}
	_, err = service.packs.Activate(ctx, PackActivateRequest{
		WorkspaceRoot: request.WorkspaceRoot, PackID: request.PackID, Version: result.Candidate.Manifest.Version, MigrationAuthorized: true,
	})
	if err != nil {
		return PackUpgradeResult{}, service.recover(ctx, request, result, backupInfo.ID, "activation", err)
	}
	if err := service.migration.CheckIntegrity(ctx, request.WorkspaceRoot); err != nil {
		return PackUpgradeResult{}, service.recover(ctx, request, result, backupInfo.ID, "integrity_check", err)
	}
	if err := service.recordAudit(ctx, request.WorkspaceRoot, audit.Event{
		Name: "pack.upgrade.completed", Actor: audit.ActorUser, Subject: request.PackID.String(), AppVersion: service.appVersion,
		Metadata: map[string]string{
			"from_version": result.Current.Manifest.Version.String(), "to_version": result.Candidate.Manifest.Version.String(),
			"migration_plan_id": result.MigrationPlan.ID.String(), "backup_id": backupInfo.ID,
			"source_instances_archived": strconv.Itoa(migrationResult.SourceInstancesArchived),
			"target_instances_created":  strconv.Itoa(migrationResult.TargetInstancesCreated),
			"concept_states_preserved":  strconv.Itoa(migrationResult.ConceptStatesPreserved),
			"unknown_states_created":    strconv.Itoa(migrationResult.UnknownConceptStatesCreated),
			"projection_version":        migrationResult.ProjectionVersion,
			"policy_version":            PackUpgradePolicyVersionV1,
		},
	}); err != nil {
		return PackUpgradeResult{}, service.recover(ctx, request, result, backupInfo.ID, "audit", err)
	}
	result.Applied = true
	return result, nil
}

func (service *PackUpgradeExecutorV1) recover(ctx context.Context, request PackUpgradeRequest, result PackUpgradeResult, backupID, stage string, cause error) error {
	_, restoreErr := service.backups.Restore(ctx, request.WorkspaceRoot, backupID)
	recovered := restoreErr == nil
	auditErr := service.recordAudit(ctx, request.WorkspaceRoot, audit.Event{
		Name: "pack.upgrade.failed", Actor: audit.ActorUser, Subject: request.PackID.String(), AppVersion: service.appVersion,
		Metadata: map[string]string{
			"from_version": result.Current.Manifest.Version.String(), "to_version": result.Candidate.Manifest.Version.String(),
			"migration_plan_id": result.MigrationPlan.ID.String(), "backup_id": backupID, "failed_stage": stage,
			"recovered": strconv.FormatBool(recovered), "policy_version": PackUpgradePolicyVersionV1,
		},
	})
	base := fmt.Errorf("pack upgrade failed during %s; backup %s recovery=%t: %w", stage, backupID, recovered, cause)
	if restoreErr != nil || auditErr != nil {
		return ExternalError("recover Learning Pack upgrade", errors.Join(base, restoreErr, auditErr))
	}
	return ExternalError("recover Learning Pack upgrade", base)
}

func (service *PackUpgradeExecutorV1) recordAudit(ctx context.Context, workspaceRoot string, event audit.Event) (err error) {
	store, err := service.audits.Open(ctx, workspaceRoot)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, store.Close()) }()
	return store.Record(ctx, event)
}

func (service *PackUpgradeExecutorV1) requireDependencies(operation string) error {
	if service == nil || service.preview == nil || service.packs == nil || service.backups == nil || service.migration == nil || service.audits == nil || service.retention == nil {
		return Classify(ErrorUnavailable, operation, fmt.Errorf("pack upgrade executor is not configured"))
	}
	if service.appVersion == "" {
		return Classify(ErrorUnavailable, operation, fmt.Errorf("application version is not configured"))
	}
	return nil
}

var _ PackUpgradeService = (*PackUpgradeExecutorV1)(nil)
