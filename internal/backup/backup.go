// Package backup defines format-independent workspace backup and restore
// contracts. Filesystem and database details belong to infrastructure adapters.
package backup

import (
	"context"
	"errors"
	"time"
)

const FormatVersion = 1

var (
	ErrCorrupt      = errors.New("backup is corrupt")
	ErrNotFound     = errors.New("backup not found")
	ErrConfirmation = errors.New("backup restore requires explicit confirmation")
)

// File records the integrity metadata for one allowlisted workspace file.
type File struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest is the durable, portable description stored with every backup.
type Manifest struct {
	FormatVersion          int       `json:"format_version"`
	ID                     string    `json:"id"`
	CreatedAt              time.Time `json:"created_at"`
	Reason                 string    `json:"reason"`
	AppVersion             string    `json:"app_version"`
	WorkspaceID            string    `json:"workspace_id"`
	WorkspaceSchemaVersion int       `json:"workspace_schema_version"`
	DatabaseSchemaVersion  int       `json:"database_schema_version"`
	Files                  []File    `json:"files"`
}

// Info is the presentation-safe summary returned by create and list.
type Info struct {
	ID                    string
	CreatedAt             time.Time
	Reason                string
	AppVersion            string
	DatabaseSchemaVersion int
	FileCount             int
	TotalSize             int64
}

// CreateOptions describes why a backup is needed and how many backups remain
// after successful creation.
type CreateOptions struct {
	Reason    string
	Retention int
}

// Validator checks a staged database without mutating or migrating it.
type Validator interface {
	Validate(ctx context.Context, databasePath string) (schemaVersion int, err error)
}

// RestoreReconciler preserves integrity metadata for workspace artifacts that
// intentionally remain outside a backup while a database snapshot is restored.
type RestoreReconciler interface {
	ReconcileUnbackedArtifacts(ctx context.Context, currentDatabasePath, restoredDatabasePath string) error
}

// Service protects the allowlisted machine-owned state of one workspace.
type Service interface {
	Create(ctx context.Context, workspaceRoot string, options CreateOptions) (Info, error)
	List(ctx context.Context, workspaceRoot string) ([]Info, error)
	Restore(ctx context.Context, workspaceRoot, id string) (Info, error)
}

func InfoFromManifest(manifest Manifest) Info {
	info := Info{
		ID: manifest.ID, CreatedAt: manifest.CreatedAt, Reason: manifest.Reason,
		AppVersion: manifest.AppVersion, DatabaseSchemaVersion: manifest.DatabaseSchemaVersion,
		FileCount: len(manifest.Files),
	}
	for _, file := range manifest.Files {
		info.TotalSize += file.Size
	}
	return info
}
