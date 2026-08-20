package learning

import "fmt"

// LearnerSetupStatus is the durable orchestration state for the first learner
// path. It coordinates existing aggregates without replacing their lifecycles.
type LearnerSetupStatus string

const (
	SetupAwaitingOnboarding LearnerSetupStatus = "awaiting_onboarding"
	SetupAwaitingDiagnostic LearnerSetupStatus = "awaiting_diagnostic"
	SetupInitializing       LearnerSetupStatus = "initializing"
	SetupCompleted          LearnerSetupStatus = "completed"
)

func (status LearnerSetupStatus) Valid() bool {
	switch status {
	case SetupAwaitingOnboarding, SetupAwaitingDiagnostic, SetupInitializing, SetupCompleted:
		return true
	default:
		return false
	}
}

// LearnerSetup is a checkpoint for onboarding-to-curriculum orchestration.
// setup_completed_at is authoritative; onboarding completion alone never
// means the educational setup is complete.
type LearnerSetup struct {
	StudentID            ID
	Status               LearnerSetupStatus
	CurriculumInstanceID *ID
	DiagnosticAttemptID  *ID
	DiagnosticOptIn      bool
	CreatedAt            Timestamp
	UpdatedAt            Timestamp
	SetupCompletedAt     *Timestamp
}

func NewLearnerSetup(studentID ID, createdAt Timestamp) (LearnerSetup, error) {
	setup := LearnerSetup{StudentID: studentID, Status: SetupAwaitingOnboarding, CreatedAt: createdAt, UpdatedAt: createdAt}
	return setup, setup.Validate()
}

func (setup LearnerSetup) AwaitDiagnostic(instanceID, attemptID ID, at Timestamp) (LearnerSetup, error) {
	if setup.Status != SetupAwaitingOnboarding && setup.Status != SetupAwaitingDiagnostic {
		return LearnerSetup{}, fmt.Errorf("cannot await diagnostic from setup status %q", setup.Status)
	}
	setup.Status = SetupAwaitingDiagnostic
	setup.CurriculumInstanceID = cloneID(&instanceID)
	setup.DiagnosticAttemptID = cloneID(&attemptID)
	setup.DiagnosticOptIn = true
	setup.UpdatedAt = at
	return setup, setup.Validate()
}

func (setup LearnerSetup) BeginInitialization(instanceID ID, diagnosticAttemptID *ID, diagnosticOptIn bool, at Timestamp) (LearnerSetup, error) {
	if setup.Status != SetupAwaitingOnboarding && setup.Status != SetupAwaitingDiagnostic && setup.Status != SetupInitializing {
		return LearnerSetup{}, fmt.Errorf("cannot initialize from setup status %q", setup.Status)
	}
	if diagnosticOptIn && diagnosticAttemptID == nil {
		return LearnerSetup{}, fmt.Errorf("diagnostic opt-in requires an attempt")
	}
	setup.Status = SetupInitializing
	setup.CurriculumInstanceID = cloneID(&instanceID)
	setup.DiagnosticAttemptID = cloneID(diagnosticAttemptID)
	setup.DiagnosticOptIn = diagnosticOptIn
	setup.UpdatedAt = at
	return setup, setup.Validate()
}

func (setup LearnerSetup) Complete(at Timestamp) (LearnerSetup, error) {
	if setup.Status != SetupInitializing && setup.Status != SetupAwaitingDiagnostic {
		return LearnerSetup{}, fmt.Errorf("cannot complete setup from status %q", setup.Status)
	}
	setup.Status = SetupCompleted
	setup.UpdatedAt = at
	setup.SetupCompletedAt = &at
	return setup, setup.Validate()
}

func (setup LearnerSetup) Validate() error {
	if err := setup.StudentID.Validate(); err != nil {
		return fmt.Errorf("learner setup student: %w", err)
	}
	if !setup.Status.Valid() {
		return fmt.Errorf("learner setup status %q is invalid", setup.Status)
	}
	if setup.CurriculumInstanceID != nil {
		if err := setup.CurriculumInstanceID.Validate(); err != nil {
			return fmt.Errorf("learner setup curriculum instance: %w", err)
		}
	}
	if setup.DiagnosticAttemptID != nil {
		if err := setup.DiagnosticAttemptID.Validate(); err != nil {
			return fmt.Errorf("learner setup diagnostic attempt: %w", err)
		}
	}
	if err := setup.CreatedAt.Validate(); err != nil {
		return fmt.Errorf("learner setup created at: %w", err)
	}
	if err := setup.UpdatedAt.Validate(); err != nil {
		return fmt.Errorf("learner setup updated at: %w", err)
	}
	if setup.UpdatedAt.Before(setup.CreatedAt) {
		return fmt.Errorf("learner setup update precedes creation")
	}
	if err := validateOptionalTimestamp("learner setup completed at", setup.SetupCompletedAt); err != nil {
		return err
	}
	switch setup.Status {
	case SetupAwaitingOnboarding:
		if setup.CurriculumInstanceID != nil || setup.DiagnosticAttemptID != nil || setup.DiagnosticOptIn || setup.SetupCompletedAt != nil {
			return fmt.Errorf("onboarding setup cannot contain downstream state")
		}
	case SetupAwaitingDiagnostic:
		if setup.CurriculumInstanceID == nil || setup.DiagnosticAttemptID == nil || !setup.DiagnosticOptIn || setup.SetupCompletedAt != nil {
			return fmt.Errorf("diagnostic setup requires instance, opt-in, and attempt")
		}
	case SetupInitializing:
		if setup.CurriculumInstanceID == nil || setup.SetupCompletedAt != nil || (setup.DiagnosticOptIn && setup.DiagnosticAttemptID == nil) || (!setup.DiagnosticOptIn && setup.DiagnosticAttemptID != nil) {
			return fmt.Errorf("initializing setup has inconsistent curriculum or diagnostic state")
		}
	case SetupCompleted:
		if setup.CurriculumInstanceID == nil || setup.SetupCompletedAt == nil || (setup.DiagnosticOptIn && setup.DiagnosticAttemptID == nil) || (!setup.DiagnosticOptIn && setup.DiagnosticAttemptID != nil) {
			return fmt.Errorf("completed setup has inconsistent curriculum or diagnostic state")
		}
		if *setup.SetupCompletedAt != setup.UpdatedAt {
			return fmt.Errorf("setup completion timestamp must equal update timestamp")
		}
	}
	return nil
}

func cloneID(value *ID) *ID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
