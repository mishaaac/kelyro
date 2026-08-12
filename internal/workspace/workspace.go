// Package workspace defines project-workspace concepts and service contracts.
package workspace

// Workspace identifies a validated Kelyro workspace.
type Workspace struct {
	Root string
}

// Service discovers, creates, and validates workspaces without prescribing a
// filesystem implementation.
type Service interface {
	Discover(startDir string) (Workspace, error)
	Init(root string) (Workspace, error)
	Validate(root string) error
}
