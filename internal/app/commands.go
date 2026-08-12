package app

import (
	"context"
	"fmt"
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
	Action    Action
	Workspace string
}

// Result contains presentation-independent output from a Foundation action.
type Result struct {
	Message string
}

// FoundationService executes the operations currently exposed by the CLI.
type FoundationService interface {
	Execute(ctx context.Context, command Command) (Result, error)
}

// BootstrapService makes the Foundation command surface usable before the
// corresponding application services are implemented in later steps.
type BootstrapService struct{}

// Execute returns an explicit placeholder for each reserved Foundation action.
func (BootstrapService) Execute(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	switch command.Action {
	case ActionTUI:
		return Result{Message: "Kelyro TUI bootstrap: interactive mode is not implemented yet."}, nil
	case ActionInit:
		return Result{Message: "kelyro init: workspace initialization is not implemented yet."}, nil
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
