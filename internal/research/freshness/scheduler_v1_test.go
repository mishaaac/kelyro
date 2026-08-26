package freshness

import (
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestScheduleV1TTLDeadlineDueAndNotDue(t *testing.T) {
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	tests := []struct {
		name string
		age  time.Duration
		due  bool
	}{
		{name: "due", age: 91 * 24 * time.Hour, due: true},
		{name: "not due", age: 10 * 24 * time.Hour, due: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			last := freshnessTimestamp(t, now.Time().Add(-test.age))
			assessment, err := NewModelV1(fixedClock{now}).Assess(Input{
				LastVerifiedAt: &last, ClaimType: research.ClaimBehavior, SourceKind: research.SourceOther,
			})
			if err != nil {
				t.Fatal(err)
			}
			schedule, err := ScheduleV1(SchedulingInput{Assessment: assessment})
			if err != nil {
				t.Fatal(err)
			}
			wantNext := last.Time().Add(90 * 24 * time.Hour)
			if !schedule.NextVerifyAt.Time().Equal(wantNext) || schedule.Reason != research.VerificationTTLExpired ||
				schedule.Priority != research.VerificationPriorityNormal || schedule.AlgorithmVersion != research.RefreshSchedulingAlgorithmV1 {
				t.Fatalf("ScheduleV1() = %+v", schedule)
			}
			if gotDue := !schedule.NextVerifyAt.After(now); gotDue != test.due {
				t.Fatalf("due = %v, want %v", gotDue, test.due)
			}
		})
	}
}

func TestScheduleV1ReleaseAndManualTriggersAreImmediatelyDue(t *testing.T) {
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	last := freshnessTimestamp(t, now.Time().Add(-time.Hour))
	releaseAssessment, err := NewModelV1(fixedClock{now}).Assess(Input{
		LastVerifiedAt: &last, ClaimType: research.ClaimBehavior, SourceKind: research.SourceOther, KnownNewRelease: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	release, err := ScheduleV1(SchedulingInput{Assessment: releaseAssessment})
	if err != nil {
		t.Fatal(err)
	}
	if !release.NextVerifyAt.Time().Equal(now.Time()) || release.Reason != research.VerificationNewRelease || release.Priority != research.VerificationPriorityHigh {
		t.Fatalf("release schedule = %+v", release)
	}

	regularAssessment, err := NewModelV1(fixedClock{now}).Assess(Input{
		LastVerifiedAt: &last, ClaimType: research.ClaimBehavior, SourceKind: research.SourceOther,
	})
	if err != nil {
		t.Fatal(err)
	}
	manual, err := ScheduleV1(SchedulingInput{Assessment: regularAssessment, ManualRequest: true})
	if err != nil {
		t.Fatal(err)
	}
	if !manual.NextVerifyAt.Time().Equal(now.Time()) || manual.Reason != research.VerificationManualRequest || manual.Priority != research.VerificationPriorityCritical {
		t.Fatalf("manual schedule = %+v", manual)
	}
}

func TestScheduleV1EventTriggerPrecedence(t *testing.T) {
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	last := freshnessTimestamp(t, now.Time().Add(-time.Hour))
	updated := freshnessTimestamp(t, now.Time().Add(-time.Minute))
	assessment, err := NewModelV1(fixedClock{now}).Assess(Input{
		LastVerifiedAt: &last, SourceUpdatedAt: &updated, KnownNewRelease: true,
		ClaimType: research.ClaimSecurity, SourceKind: research.SourceReleaseNotes,
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		input  SchedulingInput
		reason research.VerificationReason
		level  research.VerificationPriority
	}{
		{name: "source changed beats release", input: SchedulingInput{Assessment: assessment}, reason: research.VerificationSourceChanged, level: research.VerificationPriorityHigh},
		{name: "conflict beats source", input: SchedulingInput{Assessment: assessment, ConflictUnresolved: true}, reason: research.VerificationConflictUnresolved, level: research.VerificationPriorityHigh},
		{name: "security beats conflict", input: SchedulingInput{Assessment: assessment, ConflictUnresolved: true, SecuritySensitive: true}, reason: research.VerificationSecuritySensitive, level: research.VerificationPriorityCritical},
		{name: "manual beats all", input: SchedulingInput{Assessment: assessment, ConflictUnresolved: true, SecuritySensitive: true, ManualRequest: true}, reason: research.VerificationManualRequest, level: research.VerificationPriorityCritical},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ScheduleV1(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Reason != test.reason || got.Priority != test.level || !got.NextVerifyAt.Time().Equal(now.Time()) {
				t.Fatalf("ScheduleV1() = %+v", got)
			}
		})
	}
}

func TestScheduleV1RejectsUnknownFreshness(t *testing.T) {
	now := freshnessTimestamp(t, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC))
	assessment, err := NewModelV1(fixedClock{now}).Assess(Input{ClaimType: research.ClaimBehavior, SourceKind: research.SourceOther})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ScheduleV1(SchedulingInput{Assessment: assessment, ManualRequest: true}); err == nil {
		t.Fatal("ScheduleV1() accepted unknown freshness")
	}
}
