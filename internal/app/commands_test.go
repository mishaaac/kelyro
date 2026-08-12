package app

import (
	"context"
	"strings"
	"testing"
)

func TestBootstrapServiceSupportsFoundationActions(t *testing.T) {
	t.Parallel()

	for _, action := range []Action{
		ActionTUI,
		ActionInit,
		ActionDoctor,
		ActionConfig,
		ActionStatus,
		ActionOpen,
	} {
		t.Run(string(action), func(t *testing.T) {
			t.Parallel()

			result, err := (BootstrapService{}).Execute(context.Background(), Command{Action: action})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !strings.Contains(result.Message, "not implemented yet") {
				t.Errorf("Execute() message = %q, want explicit placeholder", result.Message)
			}
		})
	}
}

func TestBootstrapServiceRejectsUnknownAction(t *testing.T) {
	t.Parallel()

	_, err := (BootstrapService{}).Execute(context.Background(), Command{Action: "unknown"})
	if err == nil {
		t.Fatal("Execute() error = nil, want unsupported action error")
	}
}

func TestBootstrapServiceHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (BootstrapService{}).Execute(ctx, Command{Action: ActionStatus})
	if err != context.Canceled {
		t.Fatalf("Execute() error = %v, want %v", err, context.Canceled)
	}
}
