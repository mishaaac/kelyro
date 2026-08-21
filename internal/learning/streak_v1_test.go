package learning

import (
	"testing"
	"time"
)

func TestStreakV1DeduplicatesSameLocalDay(t *testing.T) {
	t.Parallel()
	input := streakInput(t, "UTC", time.Date(2026, 8, 21, 20, 0, 0, 0, time.UTC))
	input.Events = []StudyEvent{
		streakEvent(t, input.StudentID, "same.1", time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)),
		streakEvent(t, input.StudentID, "same.2", time.Date(2026, 8, 21, 18, 0, 0, 0, time.UTC)),
	}
	result, err := CalculateStreakV1(input)
	if err != nil || result.CurrentDays != 1 || result.LongestDays != 1 || result.TotalActiveDays != 1 ||
		result.LastActiveLocalDate == nil || result.LastActiveLocalDate.String() != "2026-08-21" {
		t.Fatalf("CalculateStreakV1() = (%+v, %v)", result, err)
	}
}

func TestStreakV1ExtendsNextDayAndResetsAfterSkippedDay(t *testing.T) {
	t.Parallel()
	input := streakInput(t, "UTC", time.Date(2026, 8, 22, 20, 0, 0, 0, time.UTC))
	input.Events = []StudyEvent{
		streakEvent(t, input.StudentID, "next.1", time.Date(2026, 8, 21, 9, 0, 0, 0, time.UTC)),
		streakEvent(t, input.StudentID, "next.2", time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC)),
	}
	result, err := CalculateStreakV1(input)
	if err != nil || result.CurrentDays != 2 || result.LongestDays != 2 {
		t.Fatalf("next day streak = (%+v, %v)", result, err)
	}

	input.AsOf = streakTimestamp(t, time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC))
	result, err = CalculateStreakV1(input)
	if err != nil || result.CurrentDays != 2 {
		t.Fatalf("yesterday grace streak = (%+v, %v)", result, err)
	}

	input.AsOf = streakTimestamp(t, time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))
	result, err = CalculateStreakV1(input)
	if err != nil || result.CurrentDays != 0 || result.LongestDays != 2 || result.TotalActiveDays != 2 {
		t.Fatalf("skipped day streak = (%+v, %v)", result, err)
	}
}

func TestStreakV1UsesProfileTimezoneWithoutPermanentDuplicateDays(t *testing.T) {
	t.Parallel()
	first := time.Date(2026, 8, 21, 4, 30, 0, 0, time.UTC)
	second := time.Date(2026, 8, 21, 5, 30, 0, 0, time.UTC)
	input := streakInput(t, "UTC", time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC))
	input.Events = []StudyEvent{streakEvent(t, input.StudentID, "timezone.1", first), streakEvent(t, input.StudentID, "timezone.2", second)}
	utc, err := CalculateStreakV1(input)
	if err != nil || utc.TotalActiveDays != 1 {
		t.Fatalf("UTC streak = (%+v, %v)", utc, err)
	}

	input.Timezone = "America/Lima"
	lima, err := CalculateStreakV1(input)
	if err != nil || lima.TotalActiveDays != 2 || lima.LongestDays != 2 {
		t.Fatalf("Lima streak = (%+v, %v)", lima, err)
	}

	input.Timezone = "UTC"
	again, err := CalculateStreakV1(input)
	if err != nil || again.TotalActiveDays != utc.TotalActiveDays || again.LongestDays != utc.LongestDays {
		t.Fatalf("timezone round trip = (%+v, %v), want %+v", again, err, utc)
	}
}

func TestStreakV1TreatsDSTCalendarDaysAsConsecutive(t *testing.T) {
	t.Parallel()
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, dates := range [][]time.Time{
		{
			time.Date(2026, 3, 7, 12, 0, 0, 0, location),
			time.Date(2026, 3, 8, 12, 0, 0, 0, location),
			time.Date(2026, 3, 9, 12, 0, 0, 0, location),
		},
		{
			time.Date(2026, 10, 31, 12, 0, 0, 0, location),
			time.Date(2026, 11, 1, 12, 0, 0, 0, location),
			time.Date(2026, 11, 2, 12, 0, 0, 0, location),
		},
	} {
		input := streakInput(t, location.String(), dates[2].Add(time.Hour))
		for index, date := range dates {
			input.Events = append(input.Events, streakEvent(t, input.StudentID, timeKey(date, index), date))
		}
		result, err := CalculateStreakV1(input)
		if err != nil || result.CurrentDays != 3 || result.LongestDays != 3 {
			t.Fatalf("DST dates %v = (%+v, %v)", dates, result, err)
		}
	}
}

func TestStreakV1TracksLongestRunAndSignificantTime(t *testing.T) {
	t.Parallel()
	input := streakInput(t, "UTC", time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC))
	for _, day := range []int{1, 2, 3, 8, 9} {
		input.Events = append(input.Events, streakEvent(t, input.StudentID, timeKey(time.Date(2026, 8, day, 9, 0, 0, 0, time.UTC), day),
			time.Date(2026, 8, day, 9, 0, 0, 0, time.UTC)))
	}
	input.Sessions = []StudySession{
		streakSession(t, input.StudentID, "short.1", time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC), 5*time.Minute),
		streakSession(t, input.StudentID, "short.2", time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC), 5*time.Minute),
	}
	result, err := CalculateStreakV1(input)
	if err != nil || result.CurrentDays != 3 || result.LongestDays != 3 || result.TotalActiveDays != 6 ||
		result.MinimumActiveMinutes != 10 || result.PolicyVersion != StreakPolicyVersion {
		t.Fatalf("longest/time streak = (%+v, %v)", result, err)
	}
}

func streakInput(t *testing.T, timezone string, asOf time.Time) StreakCalculationInput {
	t.Helper()
	return StreakCalculationInput{
		StudentID: streakID(t, "student.streak"), Timezone: timezone,
		AsOf: streakTimestamp(t, asOf), Policy: DefaultStreakPolicy(),
	}
}

func streakEvent(t *testing.T, studentID ID, suffix string, occurredAt time.Time) StudyEvent {
	t.Helper()
	goalID, instanceID, conceptID := streakID(t, "goal.streak"), streakID(t, "instance.streak"), streakID(t, "concept.streak")
	event, err := NewStudyEvent(streakID(t, "history.streak."+suffix), studentID, StudyEventEvidenceRecorded,
		streakID(t, "source.streak."+suffix), streakTimestamp(t, occurredAt), &goalID, &instanceID, &conceptID)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func streakSession(t *testing.T, studentID ID, suffix string, startedAt time.Time, duration time.Duration) StudySession {
	t.Helper()
	start := streakTimestamp(t, startedAt)
	end := streakTimestamp(t, startedAt.Add(duration))
	return StudySession{
		ID: streakID(t, "session.streak."+suffix), StudentID: studentID, GoalID: streakID(t, "goal.streak"),
		CurriculumInstanceID: streakID(t, "instance.streak"), StartedAt: start, EndedAt: &end, LastActivityAt: end,
		Status: StudySessionCompleted, ActiveDuration: duration, ActivityCount: 1,
		PolicyVersion: StudySessionPolicyVersion, IdleTimeout: DefaultStudySessionIdleTimeout,
	}
}

func streakID(t *testing.T, value string) ID {
	t.Helper()
	id, err := NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func streakTimestamp(t *testing.T, value time.Time) Timestamp {
	t.Helper()
	timestamp, err := NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func timeKey(value time.Time, suffix int) string {
	return value.Format("20060102") + "." + string(rune('a'+suffix%26))
}
