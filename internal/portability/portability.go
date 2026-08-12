// Package portability defines format-independent workspace export and import
// contracts. Archive and filesystem details belong to infrastructure adapters.
package portability

import (
	"context"
	"errors"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
)

const FormatVersion = 1

var (
	ErrMalformed = errors.New("portable archive is malformed")
	ErrConflict  = errors.New("portable import has conflicts")
)

// Mode controls whether an export contains only selected readable documents or
// also the allowlisted machine state needed to reopen the workspace elsewhere.
type Mode string

const (
	ModeHuman Mode = "human"
	ModeFull  Mode = "full"
)

func (mode Mode) Valid() bool { return mode == ModeHuman || mode == ModeFull }

// ConflictStrategy is the explicit policy applied to destination files whose
// contents differ from the archive.
type ConflictStrategy string

const (
	ConflictFail      ConflictStrategy = "fail"
	ConflictKeep      ConflictStrategy = "keep"
	ConflictOverwrite ConflictStrategy = "overwrite"
)

func (strategy ConflictStrategy) Valid() bool {
	return strategy == ConflictFail || strategy == ConflictKeep || strategy == ConflictOverwrite
}

// File records portable integrity and ownership metadata for one file.
type File struct {
	Path      string              `json:"path"`
	Size      int64               `json:"size"`
	SHA256    string              `json:"sha256"`
	Ownership artifacts.Ownership `json:"ownership"`
}

// Manifest is the versioned description embedded in each archive.
type Manifest struct {
	FormatVersion          int       `json:"format_version"`
	Mode                   Mode      `json:"mode"`
	CreatedAt              time.Time `json:"created_at"`
	AppVersion             string    `json:"app_version"`
	WorkspaceID            string    `json:"workspace_id"`
	WorkspaceSchemaVersion int       `json:"workspace_schema_version"`
	DatabaseSchemaVersion  int       `json:"database_schema_version,omitempty"`
	Files                  []File    `json:"files"`
}

// ExportOptions selects the archive mode and optional output path. An empty
// output path asks the adapter to generate a collision-resistant name.
type ExportOptions struct {
	Mode       Mode
	OutputPath string
}

// ImportOptions defines a validated archive extraction without relying on
// process-global CLI state.
type ImportOptions struct {
	ArchivePath string
	Destination string
	DryRun      bool
	Conflicts   ConflictStrategy
}

// Report describes an export or a fully preflighted import plan.
type Report struct {
	ArchivePath string
	Destination string
	Mode        Mode
	DryRun      bool
	FileCount   int
	TotalSize   int64
	Creates     []string
	Replaces    []string
	Skips       []string
	Conflicts   []string
}

// Validator checks an imported database without creating or migrating it.
type Validator interface {
	Validate(ctx context.Context, databasePath string) (schemaVersion int, err error)
}

// Service creates and imports self-describing portable archives.
type Service interface {
	Export(ctx context.Context, workspaceRoot string, options ExportOptions) (Report, error)
	Import(ctx context.Context, options ImportOptions) (Report, error)
}
