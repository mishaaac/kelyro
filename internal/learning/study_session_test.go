package learning

import (
	"testing"
	"time"
)

func TestStudySessionLifecycleAccumulatesBoundedActiveTime(t *testing.T) {
	started := studySessionTimestamp(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))
	session, err := NewStudySession(mustID(t, "session.study"), mustID(t, "student.primary"), mustID(t, "goal.study"), mustID(t, "instance.study"), started, 10*time.Minute)
	if err != nil {
		t.Fatalf("NewStudySession() error = %v", err)
	}

	session, err = session.RecordActivity(studySessionTimestamp(t, started.Time().Add(4*time.Minute)))
	if err != nil {
		t.Fatalf("RecordActivity(first) error = %v", err)
	}
	session, err = session.RecordActivity(studySessionTimestamp(t, started.Time().Add(34*time.Minute)))
	if err != nil {
		t.Fatalf("RecordActivity(idle) error = %v", err)
	}
	session, err = session.Complete(studySessionTimestamp(t, started.Time().Add(39*time.Minute)))
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if session.Status != StudySessionCompleted || session.ActiveDuration != 19*time.Minute || session.ActivityCount != 2 {
		t.Fatalf("completed session = %+v", session)
	}
}

func TestStudySessionCrashRecoveryCapsIdle(t *testing.T) {
	started := studySessionTimestamp(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))
	session, _ := NewStudySession(mustID(t, "session.recovery"), mustID(t, "student.primary"), mustID(t, "goal.study"), mustID(t, "instance.study"), started, 15*time.Minute)
	session, _ = session.RecordActivity(studySessionTimestamp(t, started.Time().Add(5*time.Minute)))

	if stale, err := session.IsStale(studySessionTimestamp(t, started.Time().Add(20*time.Minute))); err != nil || stale {
		t.Fatalf("IsStale(at boundary) = (%t, %v), want false", stale, err)
	}
	recovered, err := session.Recover(studySessionTimestamp(t, started.Time().Add(time.Hour)))
	if err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if recovered.Status != StudySessionRecovered || recovered.EndedAt == nil || recovered.EndedAt.Time() != started.Time().Add(20*time.Minute) || recovered.ActiveDuration != 20*time.Minute {
		t.Fatalf("recovered session = %+v", recovered)
	}
}

func TestStudySessionRejectsInvalidTransitions(t *testing.T) {
	started := studySessionTimestamp(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))
	session, _ := NewStudySession(mustID(t, "session.invalid"), mustID(t, "student.primary"), mustID(t, "goal.study"), mustID(t, "instance.study"), started, time.Minute)
	completed, err := session.Complete(started)
	if err != nil {
		t.Fatalf("Complete(at start) error = %v", err)
	}
	if completed.ActiveDuration != 0 {
		t.Fatalf("active duration = %s, want zero", completed.ActiveDuration)
	}
	if _, err := completed.RecordActivity(studySessionTimestamp(t, started.Time().Add(time.Second))); err == nil {
		t.Fatal("RecordActivity() accepted a completed session")
	}
	if _, err := session.RecordActivity(studySessionTimestamp(t, started.Time().Add(-time.Second))); err == nil {
		t.Fatal("RecordActivity() accepted time before start")
	}
	if _, err := session.Recover(studySessionTimestamp(t, started.Time().Add(time.Minute))); err == nil {
		t.Fatal("Recover() accepted a session at the idle boundary")
	}
}

func TestStudySessionCanBeInterruptedWithoutCompletingLearning(t *testing.T) {
	started := studySessionTimestamp(t, time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC))
	session, _ := NewStudySession(mustID(t, "session.interrupted"), mustID(t, "student.primary"), mustID(t, "goal.study"), mustID(t, "instance.study"), started, 10*time.Minute)
	interrupted, err := session.Interrupt(studySessionTimestamp(t, started.Time().Add(3*time.Minute)))
	if err != nil {
		t.Fatalf("Interrupt() error = %v", err)
	}
	if interrupted.Status != StudySessionInterrupted || interrupted.ActiveDuration != 3*time.Minute || interrupted.ActivityCount != 0 {
		t.Fatalf("interrupted session = %+v", interrupted)
	}
}

func studySessionTimestamp(t *testing.T, value time.Time) Timestamp {
	t.Helper()
	timestamp, err := NewTimestamp(value)
	if err != nil {
		t.Fatalf("NewTimestamp() error = %v", err)
	}
	return timestamp
}
