package learning

import (
	"testing"
	"time"
)

func TestStudyEventValidatesStableScopes(t *testing.T) {
	at := studySessionTimestamp(t, time.Date(2026, 8, 21, 15, 0, 0, 0, time.UTC))
	goalID, instanceID, conceptID := mustID(t, "goal.history"), mustID(t, "instance.history"), mustID(t, "concept.history")
	event, err := NewStudyEvent(mustID(t, "history.evidence.1"), mustID(t, "student.primary"), StudyEventEvidenceRecorded,
		mustID(t, "evidence.1"), at, &goalID, &instanceID, &conceptID)
	if err != nil || event.Version != StudyHistoryVersion {
		t.Fatalf("NewStudyEvent() = (%+v, %v)", event, err)
	}
	event.GoalID = nil
	if err := event.Validate(); err == nil {
		t.Fatal("Validate() accepted curriculum instance without goal")
	}
}

func TestStudyEventRequiresItsEducationalScope(t *testing.T) {
	timestamp := studySessionTimestamp(t, time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	studentID, sourceID := mustID(t, "student.history"), mustID(t, "source.history")
	for _, eventType := range []StudyEventType{StudyEventDiagnosticCompleted,
		StudyEventConceptIntroduced, StudyEventEvidenceRecorded, StudyEventConceptMastered, StudyEventReviewCompleted} {
		if _, err := NewStudyEvent(mustID(t, "history."+string(eventType)), studentID, eventType, sourceID, timestamp, nil, nil, nil); err == nil {
			t.Fatalf("NewStudyEvent(%s) accepted missing scope", eventType)
		}
	}
}

func TestStudyWindowUsesLocalCalendarAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{name: "spring forward", now: time.Date(2026, 3, 8, 12, 0, 0, 0, location), want: 23 * time.Hour},
		{name: "fall back", now: time.Date(2026, 11, 1, 12, 0, 0, 0, location), want: 25 * time.Hour},
	} {
		t.Run(test.name, func(t *testing.T) {
			start, end, err := StudyWindow(StudyPeriodToday, test.now, location)
			if err != nil {
				t.Fatal(err)
			}
			if got := end.Time().Sub(start.Time()); got != test.want {
				t.Fatalf("window duration = %s, want %s", got, test.want)
			}
		})
	}
}

func TestStudyWindowWeekStartsMondayAndMonthUsesCalendar(t *testing.T) {
	location, _ := time.LoadLocation("America/Lima")
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, location)
	weekStart, weekEnd, err := StudyWindow(StudyPeriodWeek, now, location)
	if err != nil {
		t.Fatal(err)
	}
	if local := weekStart.Time().In(location); local.Weekday() != time.Monday || local.Day() != 17 || weekEnd.Time().In(location).Day() != 24 {
		t.Fatalf("week = %s..%s", weekStart.Time().In(location), weekEnd.Time().In(location))
	}
	monthStart, monthEnd, _ := StudyWindow(StudyPeriodMonth, now, location)
	if monthStart.Time().In(location).Day() != 1 || monthEnd.Time().In(location).Month() != time.September {
		t.Fatalf("month = %s..%s", monthStart.Time().In(location), monthEnd.Time().In(location))
	}
}
