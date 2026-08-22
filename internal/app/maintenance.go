package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/config"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
)

func (service *Service) executeMaintenance(ctx context.Context, command Command) (result Result, err error) {
	if command.MaintenanceOperation != "recalculate" {
		return Result{}, fmt.Errorf("unsupported maintenance operation %q", command.MaintenanceOperation)
	}
	if service.profiles == nil {
		return Result{}, errors.New("student core store is unavailable")
	}
	found, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}

	backupID := ""
	if !command.MaintenanceDryRun {
		if service.backups == nil {
			return Result{}, errors.New("workspace backup service is required for learning-state recalculation")
		}
		if service.configs == nil {
			return Result{}, errors.New("configuration store is unavailable")
		}
		settings, loadErr := service.resolvedConfigForWorkspace(found.Root, command.ConfigOverrides)
		if loadErr != nil {
			return Result{}, loadErr
		}
		retentionValue, ok := settings[config.KeyBackupRetention].NumberField()
		if !ok {
			return Result{}, errors.New("backup retention configuration is invalid")
		}
		created, createErr := service.backups.Create(ctx, found.Root, backup.CreateOptions{
			Reason: "learning-algorithm-recalculation", Retention: int(retentionValue),
		})
		if createErr != nil {
			return Result{}, fmt.Errorf("backup before learning-state recalculation: %w", createErr)
		}
		backupID = created.ID
		if auditErr := service.recordAudit(ctx, found.Root, audit.Event{
			Name: "backup.created", Actor: audit.ActorUser, Subject: created.ID,
			Metadata: map[string]string{"reason": created.Reason, "files": strconv.Itoa(created.FileCount)},
		}); auditErr != nil {
			return Result{}, auditErr
		}
	}

	store, err := service.profiles.Open(ctx, found.Root)
	if err != nil {
		return Result{}, fmt.Errorf("open student core: %w", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	if store.Maintenance() == nil {
		return Result{}, errors.New("learning-state maintenance service is unavailable")
	}
	impact, err := store.Maintenance().Recalculate(ctx, learningapp.RecalculationRequest{
		DryRun: command.MaintenanceDryRun, BackupID: backupID,
	})
	if err != nil {
		return Result{}, err
	}
	if !command.MaintenanceDryRun {
		if auditErr := service.recordAudit(ctx, found.Root, audit.Event{
			Name: "learning.recalculation.completed", Actor: audit.ActorUser, Subject: backupID,
			Metadata: map[string]string{
				"backup_id": backupID,
				"mastery":   strings.Join(impact.Target.Mastery, ","), "retention": strings.Join(impact.Target.Retention, ","),
				"daily_plan":               strings.Join(impact.Target.DailyPlan, ","),
				"concept_states_changed":   strconv.Itoa(impact.ConceptStatesChanged),
				"retention_states_changed": strconv.Itoa(impact.RetentionStatesChanged),
				"review_schedules_changed": strconv.Itoa(impact.ReviewSchedulesChanged),
				"review_items_changed":     strconv.Itoa(impact.ReviewItemsChanged),
				"daily_plans_changed":      strconv.Itoa(impact.DailyPlansChanged),
			},
		}); auditErr != nil {
			return Result{}, auditErr
		}
	}
	result.Maintenance = &impact
	return result, nil
}
