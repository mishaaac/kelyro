package privacy

import (
	"context"
	"errors"
	"testing"
)

func TestNetworkGateDeniesByDefaultBeforeFakeNetworkIsCalled(t *testing.T) {
	t.Parallel()
	recorder := &recordingBlockedRecorder{}
	gate := NewNetworkGate(Policy{}, recorder)
	network := &fakeNetwork{}

	err := guardedCall(context.Background(), gate, network, Request{
		Operation: "update.check",
		Purpose:   ExternalResource,
	})
	if !errors.Is(err, ErrNetworkBlocked) {
		t.Fatalf("guardedCall() error = %v, want ErrNetworkBlocked", err)
	}
	if network.calls != 0 {
		t.Fatalf("fake network calls = %d, want 0", network.calls)
	}
	if len(recorder.events) != 1 || recorder.events[0].Operation != "update.check" {
		t.Fatalf("blocked events = %+v", recorder.events)
	}
}

func TestNetworkGateRequiresSpecificContentAndTelemetryOptIns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  Policy
		purpose Purpose
		allowed bool
	}{
		{name: "external allowed", policy: Policy{AllowNetwork: true}, purpose: ExternalResource, allowed: true},
		{name: "ai needs dedicated opt in", policy: Policy{AllowNetwork: true}, purpose: AIContent},
		{name: "ai needs network too", policy: Policy{AllowAIContent: true}, purpose: AIContent},
		{name: "ai fully allowed", policy: Policy{AllowNetwork: true, AllowAIContent: true}, purpose: AIContent, allowed: true},
		{name: "telemetry needs dedicated opt in", policy: Policy{AllowNetwork: true}, purpose: UsageTelemetry},
		{name: "telemetry needs network too", policy: Policy{AllowUsageTelemetry: true}, purpose: UsageTelemetry},
		{name: "telemetry fully allowed", policy: Policy{AllowNetwork: true, AllowUsageTelemetry: true}, purpose: UsageTelemetry, allowed: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := NewNetworkGate(test.policy, nil).Authorize(context.Background(), Request{
				Operation: "future.operation",
				Purpose:   test.purpose,
			})
			if test.allowed && err != nil {
				t.Fatalf("Authorize() error = %v, want allowed", err)
			}
			if !test.allowed && !errors.Is(err, ErrNetworkBlocked) {
				t.Fatalf("Authorize() error = %v, want ErrNetworkBlocked", err)
			}
		})
	}
}

func TestNetworkGateRejectsUnsafeMetadataAndPreservesCancellation(t *testing.T) {
	t.Parallel()
	recorder := &recordingBlockedRecorder{}
	gate := NewNetworkGate(Policy{}, recorder)

	for _, request := range []Request{
		{Operation: "/home/student/private", Purpose: ExternalResource},
		{Operation: "https://example.com", Purpose: ExternalResource},
		{Operation: "update.check", Purpose: Purpose("unknown")},
	} {
		if err := gate.Authorize(context.Background(), request); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("Authorize(%+v) error = %v, want ErrInvalidRequest", request, err)
		}
	}
	if len(recorder.events) != 0 {
		t.Fatalf("unsafe requests recorded = %+v", recorder.events)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := gate.Authorize(ctx, Request{Operation: "update.check", Purpose: ExternalResource}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Authorize(canceled) error = %v, want context.Canceled", err)
	}
}

func TestRecorderFailureDoesNotReplacePrivacyDenial(t *testing.T) {
	t.Parallel()
	gate := NewNetworkGate(Policy{}, failingBlockedRecorder{})
	err := gate.Authorize(context.Background(), Request{Operation: "update.check", Purpose: ExternalResource})
	if !errors.Is(err, ErrNetworkBlocked) {
		t.Fatalf("Authorize() error = %v, want ErrNetworkBlocked", err)
	}
}

type recordingBlockedRecorder struct{ events []BlockedEvent }

func (recorder *recordingBlockedRecorder) RecordBlocked(_ context.Context, event BlockedEvent) error {
	recorder.events = append(recorder.events, event)
	return nil
}

type failingBlockedRecorder struct{}

func (failingBlockedRecorder) RecordBlocked(context.Context, BlockedEvent) error {
	return errors.New("logger unavailable")
}

type fakeNetwork struct{ calls int }

func (network *fakeNetwork) Call() { network.calls++ }

func guardedCall(ctx context.Context, gate NetworkGate, network *fakeNetwork, request Request) error {
	if err := gate.Authorize(ctx, request); err != nil {
		return err
	}
	network.Call()
	return nil
}
