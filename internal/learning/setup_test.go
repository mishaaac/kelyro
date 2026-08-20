package learning

import "testing"

func TestLearnerSetupLifecycleRequiresCompleteDownstreamState(t *testing.T) {
	t.Parallel()
	setup, err := NewLearnerSetup(mustID(t, "student.setup"), mustTimestamp(t, 10))
	if err != nil || setup.Status != SetupAwaitingOnboarding || setup.SetupCompletedAt != nil {
		t.Fatalf("NewLearnerSetup() = (%+v, %v)", setup, err)
	}
	instanceID := mustID(t, "instance.setup")
	attemptID := mustID(t, "attempt.setup")
	setup, err = setup.AwaitDiagnostic(instanceID, attemptID, mustTimestamp(t, 11))
	if err != nil || setup.Status != SetupAwaitingDiagnostic || !setup.DiagnosticOptIn {
		t.Fatalf("AwaitDiagnostic() = (%+v, %v)", setup, err)
	}
	setup, err = setup.BeginInitialization(instanceID, &attemptID, true, mustTimestamp(t, 12))
	if err != nil || setup.Status != SetupInitializing {
		t.Fatalf("BeginInitialization() = (%+v, %v)", setup, err)
	}
	setup, err = setup.Complete(mustTimestamp(t, 13))
	if err != nil || setup.Status != SetupCompleted || setup.SetupCompletedAt == nil {
		t.Fatalf("Complete() = (%+v, %v)", setup, err)
	}
}

func TestLearnerSetupSupportsDiagnosticOptOutAndRejectsInvalidCompletion(t *testing.T) {
	t.Parallel()
	setup, _ := NewLearnerSetup(mustID(t, "student.optout"), mustTimestamp(t, 10))
	if _, err := setup.Complete(mustTimestamp(t, 11)); err == nil {
		t.Fatal("Complete() accepted setup without curriculum")
	}
	instanceID := mustID(t, "instance.optout")
	setup, err := setup.BeginInitialization(instanceID, nil, false, mustTimestamp(t, 11))
	if err != nil {
		t.Fatal(err)
	}
	completed, err := setup.Complete(mustTimestamp(t, 12))
	if err != nil || completed.DiagnosticOptIn || completed.DiagnosticAttemptID != nil {
		t.Fatalf("opt-out complete = (%+v, %v)", completed, err)
	}
	broken := completed
	broken.SetupCompletedAt = nil
	if err := broken.Validate(); err == nil {
		t.Fatal("Validate() accepted completed setup without setup_completed_at")
	}
}
