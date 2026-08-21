package learning

import (
	"fmt"
	"time"
)

const (
	StudySessionPolicyVersion      = "study-session-v1"
	DefaultStudySessionIdleTimeout = 15 * time.Minute
)

type StudySessionStatus string

const (
	StudySessionActive      StudySessionStatus = "active"
	StudySessionCompleted   StudySessionStatus = "completed"
	StudySessionInterrupted StudySessionStatus = "interrupted"
	StudySessionRecovered   StudySessionStatus = "recovered"
)

func (status StudySessionStatus) Valid() bool {
	switch status {
	case StudySessionActive, StudySessionCompleted, StudySessionInterrupted, StudySessionRecovered:
		return true
	default:
		return false
	}
}

// StudySession records a bounded period of intentional study. LastActivityAt
// advances only for meaningful educational activity, never for keystrokes or
// app-level navigation. IdleTimeout is captured at start so an open session
// keeps the policy under which it was created.
type StudySession struct {
	ID                   ID
	StudentID            ID
	GoalID               ID
	CurriculumInstanceID ID
	StartedAt            Timestamp
	EndedAt              *Timestamp
	LastActivityAt       Timestamp
	Status               StudySessionStatus
	ActiveDuration       time.Duration
	ActivityCount        int
	PolicyVersion        string
	IdleTimeout          time.Duration
}

func NewStudySession(id, studentID, goalID, curriculumInstanceID ID, startedAt Timestamp, idleTimeout time.Duration) (StudySession, error) {
	session := StudySession{
		ID: id, StudentID: studentID, GoalID: goalID, CurriculumInstanceID: curriculumInstanceID,
		StartedAt: startedAt, LastActivityAt: startedAt, Status: StudySessionActive,
		PolicyVersion: StudySessionPolicyVersion, IdleTimeout: idleTimeout,
	}
	return session, session.Validate()
}

func (session StudySession) Validate() error {
	for _, field := range []struct {
		name string
		id   ID
	}{
		{name: "study session", id: session.ID},
		{name: "study session student", id: session.StudentID},
		{name: "study session goal", id: session.GoalID},
		{name: "study session curriculum instance", id: session.CurriculumInstanceID},
	} {
		if err := field.id.Validate(); err != nil {
			return fmt.Errorf("%s: %w", field.name, err)
		}
	}
	if err := session.StartedAt.Validate(); err != nil {
		return fmt.Errorf("study session start: %w", err)
	}
	if err := session.LastActivityAt.Validate(); err != nil {
		return fmt.Errorf("study session last activity: %w", err)
	}
	if session.LastActivityAt.Before(session.StartedAt) {
		return fmt.Errorf("study session last activity precedes start")
	}
	if !session.Status.Valid() {
		return fmt.Errorf("study session status %q is invalid", session.Status)
	}
	if session.PolicyVersion != StudySessionPolicyVersion {
		return fmt.Errorf("study session policy version %q is unsupported", session.PolicyVersion)
	}
	if session.IdleTimeout <= 0 {
		return fmt.Errorf("study session idle timeout must be positive")
	}
	if session.ActiveDuration < 0 {
		return fmt.Errorf("study session active duration cannot be negative")
	}
	if session.ActivityCount < 0 {
		return fmt.Errorf("study session activity count cannot be negative")
	}
	if session.Status == StudySessionActive {
		if session.EndedAt != nil {
			return fmt.Errorf("active study session cannot have an end")
		}
		if session.ActiveDuration > session.LastActivityAt.Time().Sub(session.StartedAt.Time()) {
			return fmt.Errorf("active study duration exceeds observed session range")
		}
		return nil
	}
	if session.EndedAt == nil {
		return fmt.Errorf("terminal study session has no end")
	}
	if err := session.EndedAt.Validate(); err != nil {
		return fmt.Errorf("study session end: %w", err)
	}
	if session.EndedAt.Before(session.StartedAt) {
		return fmt.Errorf("study session end precedes start")
	}
	if session.EndedAt.Before(session.LastActivityAt) {
		return fmt.Errorf("study session end precedes last activity")
	}
	if session.ActiveDuration > session.EndedAt.Time().Sub(session.StartedAt.Time()) {
		return fmt.Errorf("active study duration exceeds session range")
	}
	return nil
}

// RecordActivity counts one meaningful educational action and accumulates at
// most one idle window since the previous action.
func (session StudySession) RecordActivity(observedAt Timestamp) (StudySession, error) {
	updated, err := session.accumulate(observedAt)
	if err != nil {
		return StudySession{}, err
	}
	updated.LastActivityAt = observedAt
	updated.ActivityCount++
	return updated, updated.Validate()
}

func (session StudySession) Complete(endedAt Timestamp) (StudySession, error) {
	return session.finish(StudySessionCompleted, endedAt, endedAt)
}

func (session StudySession) Interrupt(endedAt Timestamp) (StudySession, error) {
	return session.finish(StudySessionInterrupted, endedAt, endedAt)
}

// Recover closes an abandoned active session at one idle window after its
// last meaningful activity. Callers must first establish that it is stale.
func (session StudySession) Recover(observedAt Timestamp) (StudySession, error) {
	stale, err := session.IsStale(observedAt)
	if err != nil {
		return StudySession{}, err
	}
	if !stale {
		return StudySession{}, fmt.Errorf("study session is not stale")
	}
	endedAt, err := NewTimestamp(session.LastActivityAt.Time().Add(session.IdleTimeout))
	if err != nil {
		return StudySession{}, err
	}
	return session.finish(StudySessionRecovered, endedAt, endedAt)
}

func (session StudySession) IsStale(observedAt Timestamp) (bool, error) {
	if err := session.requireActive(observedAt); err != nil {
		return false, err
	}
	return observedAt.Time().Sub(session.LastActivityAt.Time()) > session.IdleTimeout, nil
}

func (session StudySession) finish(status StudySessionStatus, endedAt, accumulateUntil Timestamp) (StudySession, error) {
	updated, err := session.accumulate(accumulateUntil)
	if err != nil {
		return StudySession{}, err
	}
	updated.Status = status
	updated.EndedAt = &endedAt
	return updated, updated.Validate()
}

func (session StudySession) accumulate(observedAt Timestamp) (StudySession, error) {
	if err := session.requireActive(observedAt); err != nil {
		return StudySession{}, err
	}
	elapsed := observedAt.Time().Sub(session.LastActivityAt.Time())
	if elapsed > session.IdleTimeout {
		elapsed = session.IdleTimeout
	}
	session.ActiveDuration += elapsed
	return session, nil
}

func (session StudySession) requireActive(observedAt Timestamp) error {
	if err := session.Validate(); err != nil {
		return err
	}
	if session.Status != StudySessionActive {
		return fmt.Errorf("study session is %q, want active", session.Status)
	}
	if err := observedAt.Validate(); err != nil {
		return fmt.Errorf("study session observation: %w", err)
	}
	if observedAt.Before(session.LastActivityAt) {
		return fmt.Errorf("study session observation precedes last activity")
	}
	return nil
}
