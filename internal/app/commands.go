package app

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/workspace"
)

// Action identifies a Foundation operation requested by a presentation
// adapter.
type Action string

const (
	ActionTUI    Action = "tui"
	ActionInit   Action = "init"
	ActionDoctor Action = "doctor"
	ActionConfig Action = "config"
	ActionStatus Action = "status"
	ActionOpen   Action = "open"
)

// Command contains presentation-independent input for a Foundation action.
type Command struct {
	Action      Action
	Workspace   string
	AllowNested bool
}

// Result contains presentation-independent output from a Foundation action.
type Result struct {
	Message string
}

// FoundationService executes the operations currently exposed by the CLI.
type FoundationService interface {
	Execute(ctx context.Context, command Command) (Result, error)
}

// Service coordinates implemented Foundation operations while retaining
// explicit placeholders for operations assigned to later steps.
type Service struct {
	workspaces       workspace.Service
	currentDirectory func() (string, error)
	bootstrap        BootstrapService
}

// NewService creates the application service with explicit infrastructure
// dependencies.
func NewService(workspaces workspace.Service, currentDirectory func() (string, error)) *Service {
	return &Service{workspaces: workspaces, currentDirectory: currentDirectory}
}

// Execute initializes workspaces and delegates future actions to placeholders.
func (service *Service) Execute(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if command.Action != ActionInit {
		return service.bootstrap.Execute(ctx, command)
	}
	if service.workspaces == nil {
		return Result{}, fmt.Errorf("workspace service is unavailable")
	}

	root := command.Workspace
	if root == "" {
		if service.currentDirectory == nil {
			return Result{}, fmt.Errorf("current directory provider is unavailable")
		}
		var err error
		root, err = service.currentDirectory()
		if err != nil {
			return Result{}, fmt.Errorf("find current directory: %w", err)
		}
	}

	created, err := service.workspaces.Init(root, workspace.InitOptions{AllowNested: command.AllowNested})
	if err != nil {
		return Result{}, err
	}

	return Result{Message: fmt.Sprintf("Kelyro workspace ready at %s", created.Root)}, nil
}

// BootstrapService provides explicit placeholders for Foundation operations
// assigned to later steps.
type BootstrapService struct{}

// Execute returns an explicit placeholder for each reserved Foundation action.
func (BootstrapService) Execute(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	switch command.Action {
	case ActionTUI:
		return Result{Message: "Kelyro TUI bootstrap: interactive mode is not implemented yet."}, nil
	case ActionDoctor:
		return Result{Message: "kelyro doctor: diagnostics are not implemented yet."}, nil
	case ActionConfig:
		return Result{Message: "kelyro config: configuration management is not implemented yet."}, nil
	case ActionStatus:
		return Result{Message: "kelyro status: workspace status is not implemented yet."}, nil
	case ActionOpen:
		return Result{Message: "kelyro open: workspace opening is not implemented yet."}, nil
	default:
		return Result{}, fmt.Errorf("unsupported Foundation action %q", command.Action)
	}
}
