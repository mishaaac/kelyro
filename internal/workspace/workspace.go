// Package workspace defines project-workspace concepts and service contracts.
package workspace

import (
	"errors"
	"time"
)

const SchemaVersion = 1

var (
	// ErrNotFound means no Kelyro workspace exists at or above a path.
	ErrNotFound = errors.New("Kelyro workspace not found")
	// ErrInvalid means workspace internals or metadata are malformed.
	ErrInvalid = errors.New("invalid Kelyro workspace")
	// ErrNested means initialization would create a workspace inside another.
	ErrNested = errors.New("cannot initialize a nested Kelyro workspace")
)

// Metadata is the machine-owned identity stored for every workspace.
type Metadata struct {
	WorkspaceID   string    `json:"workspace_id"`
	SchemaVersion int       `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
	AppVersion    string    `json:"app_version"`
}

// Workspace identifies a validated Kelyro workspace.
type Workspace struct {
	Root     string
	Metadata Metadata
}

// InitOptions contains explicit exceptions to workspace initialization policy.
type InitOptions struct {
	AllowNested bool
}

// Service discovers, creates, and validates workspaces without prescribing a
// filesystem implementation.
type Service interface {
	Discover(startDir string) (Workspace, error)
	Init(root string, options InitOptions) (Workspace, error)
	Validate(root string) error
}
