package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/logging"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/workspace"
)

// Execute wraps application work with best-effort diagnostic logging. Logging
// failures never obscure the requested operation or add noise to normal CLI
// output.
func (service *Service) Execute(ctx context.Context, command Command) (Result, error) {
	root := service.observabilityRoot(command)
	logger := service.openLogger(root, command.Verbose)
	if logger != nil {
		_ = logger.Log(ctx, service.logEntry(command, root, logging.Debug, "operation started", nil))
	}

	result, err := service.execute(ctx, command)
	if root == "" {
		root = service.observabilityRoot(command)
		if logger == nil {
			logger = service.openLogger(root, command.Verbose)
		}
	}
	if logger != nil {
		if err != nil {
			_ = logger.Log(context.WithoutCancel(ctx), service.logEntry(command, root, logging.Error, err.Error(), err))
		} else {
			_ = logger.Log(context.WithoutCancel(ctx), service.logEntry(command, root, logging.Info, "operation completed", nil))
		}
		_ = logger.Close()
	}
	return result, err
}

func (service *Service) executeLogs(command Command) (Result, error) {
	if service.loggers == nil {
		return Result{}, errors.New("workspace logging is unavailable")
	}
	if command.LogOperation != "path" {
		return Result{}, fmt.Errorf("unsupported logs operation %q", command.LogOperation)
	}
	found, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	path, err := service.loggers.Path(found.Root)
	if err != nil {
		return Result{}, err
	}
	return Result{Message: path}, nil
}

func (service *Service) executeAudit(ctx context.Context, command Command) (Result, error) {
	if service.audits == nil {
		return Result{}, errors.New("workspace audit trail is unavailable")
	}
	found, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	store, err := service.audits.Open(ctx, found.Root)
	if err != nil {
		return Result{}, fmt.Errorf("open workspace audit trail: %w", err)
	}
	entries, listErr := store.List(ctx)
	if closeErr := store.Close(); closeErr != nil {
		listErr = errors.Join(listErr, closeErr)
	}
	if listErr != nil {
		return Result{}, listErr
	}
	if entries == nil {
		entries = []audit.Entry{}
	}
	return Result{Audit: entries}, nil
}

func (service *Service) recordAudit(ctx context.Context, root string, event audit.Event) error {
	if service.audits == nil {
		return nil
	}
	store, err := service.audits.Open(ctx, root)
	if err != nil {
		return fmt.Errorf("open workspace audit trail: %w", err)
	}
	recordErr := store.Record(ctx, event)
	if closeErr := store.Close(); closeErr != nil {
		recordErr = errors.Join(recordErr, closeErr)
	}
	if recordErr != nil {
		return fmt.Errorf("record %s audit event: %w", event.Name, recordErr)
	}
	return nil
}

func (service *Service) observabilityRoot(command Command) string {
	if service.workspaces == nil {
		return ""
	}
	start := command.Workspace
	if start == "" {
		if service.currentDirectory == nil {
			return ""
		}
		var err error
		start, err = service.currentDirectory()
		if err != nil {
			return ""
		}
	}
	found, err := service.workspaces.Discover(start)
	if err != nil {
		return ""
	}
	return found.Root
}

func (service *Service) openLogger(root string, verbose bool) logging.Logger {
	if service.loggers == nil || root == "" {
		return nil
	}
	logger, err := service.loggers.Open(root, verbose)
	if err != nil {
		return nil
	}
	return logger
}

func (service *Service) logEntry(command Command, root string, level logging.Level, message string, operationErr error) logging.Entry {
	operation := string(command.Action)
	suboperation := ""
	switch command.Action {
	case ActionConfig:
		suboperation = command.ConfigOperation
	case ActionSecrets:
		suboperation = command.SecretOperation
	case ActionLogs:
		suboperation = command.LogOperation
	case ActionBackup:
		suboperation = command.BackupOperation
	}
	if suboperation != "" {
		operation += "." + suboperation
	}
	entry := logging.Entry{
		Level: level, Message: message, Operation: operation, Workspace: root,
		Component: "application", Sensitive: []string{command.SecretValue},
	}
	if command.Verbose {
		entry.Fields = map[string]string{"action": string(command.Action)}
	}
	if operationErr != nil {
		entry.ErrorCategory = errorCategory(operationErr)
	}
	return entry
}

func errorCategory(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "canceled"
	case errors.Is(err, workspace.ErrNotFound), errors.Is(err, workspace.ErrInvalid), errors.Is(err, workspace.ErrNested):
		return "workspace"
	case errors.Is(err, privacy.ErrNetworkBlocked):
		return "privacy"
	case strings.Contains(strings.ToLower(err.Error()), "config"):
		return "configuration"
	case strings.Contains(strings.ToLower(err.Error()), "database"), strings.Contains(strings.ToLower(err.Error()), "sqlite"):
		return "storage"
	default:
		return "operation"
	}
}
