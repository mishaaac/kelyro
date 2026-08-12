package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/session"
)

// ResumeSession begins a workspace session and returns the last safe context.
func (service *Service) ResumeSession(ctx context.Context, command Command) (result session.Resume, resultErr error) {
	store, err := service.openSessionStore(ctx, command)
	if err != nil {
		return session.Resume{}, err
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, closeErr)
		}
	}()
	return store.Resume(ctx)
}

// CheckpointSession persists a meaningful TUI transition without coupling the
// application to presentation details or writing on every keypress.
func (service *Service) CheckpointSession(ctx context.Context, command Command, state session.State) (resultErr error) {
	store, err := service.openSessionStore(ctx, command)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, closeErr)
		}
	}()
	return store.Checkpoint(ctx, state)
}

// CompleteSession stores the final resumable state and clears the incomplete
// session marker after normal quit or Ctrl+C.
func (service *Service) CompleteSession(ctx context.Context, command Command, state session.State) (resultErr error) {
	store, err := service.openSessionStore(ctx, command)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, closeErr)
		}
	}()
	return store.Complete(ctx, state)
}

func (service *Service) openSessionStore(ctx context.Context, command Command) (session.Store, error) {
	if service.sessionStores == nil {
		return nil, fmt.Errorf("workspace session store is unavailable")
	}
	workspace, err := service.discoverWorkspace(command)
	if err != nil {
		return nil, err
	}
	store, err := service.sessionStores.Open(ctx, workspace.Root)
	if err != nil {
		return nil, fmt.Errorf("open workspace session store: %w", err)
	}
	return store, nil
}
