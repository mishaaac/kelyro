package learning

import (
	"fmt"
	"sort"
	"time"
)

const (
	StreakPolicyVersion            = "streak-v1"
	LegacyStreakPolicyVersion      = "legacy-streak/v0"
	DefaultStreakMinimumActiveTime = 10 * time.Minute
	localDateLayout                = "2006-01-02"
)

type LocalDate string

func NewLocalDate(value string) (LocalDate, error) {
	date := LocalDate(value)
	return date, date.Validate()
}

func LocalDateFromTime(value time.Time, location *time.Location) LocalDate {
	return LocalDate(value.In(location).Format(localDateLayout))
}

func (date LocalDate) Validate() error {
	parsed, err := time.Parse(localDateLayout, string(date))
	if err != nil || parsed.Format(localDateLayout) != string(date) {
		return fmt.Errorf("local date %q is invalid", date)
	}
	return nil
}

func (date LocalDate) String() string { return string(date) }

type StreakPolicy struct {
	MinimumActiveTime time.Duration
	Version           string
}

func DefaultStreakPolicy() StreakPolicy {
	return StreakPolicy{MinimumActiveTime: DefaultStreakMinimumActiveTime, Version: StreakPolicyVersion}
}

func (policy StreakPolicy) Validate() error {
	if policy.Version != StreakPolicyVersion {
		return fmt.Errorf("unsupported streak policy %q", policy.Version)
	}
	if policy.MinimumActiveTime < time.Minute || policy.MinimumActiveTime > 24*time.Hour || policy.MinimumActiveTime%time.Minute != 0 {
		return fmt.Errorf("streak minimum active time must be whole minutes within 1m..24h")
	}
	return nil
}

type StreakCalculationInput struct {
	StudentID ID
	Events    []StudyEvent
	Sessions  []StudySession
	Timezone  string
	AsOf      Timestamp
	Policy    StreakPolicy
}

type streakDaySignal struct {
	duration    time.Duration
	latest      Timestamp
	qualifiedAt *Timestamp
}

// ActiveStudyDay is one qualifying local calendar day under streak-v1. Latest
// is the historical instant at which that day's currently known qualification
// was last observed.
type ActiveStudyDay struct {
	Date        LocalDate
	QualifiedAt Timestamp
	Latest      Timestamp
}

// CalculateStreakV1 rebuilds consistency from durable educational facts. It
// never changes mastery, progression, review eligibility, or unlock state.
func CalculateStreakV1(input StreakCalculationInput) (Streak, error) {
	activeDays, err := CalculateActiveStudyDaysV1(input)
	if err != nil {
		return Streak{}, err
	}
	location, _ := time.LoadLocation(input.Timezone)
	streak := Streak{
		StudentID: input.StudentID, TotalActiveDays: len(activeDays), Timezone: input.Timezone,
		MinimumActiveMinutes: int(input.Policy.MinimumActiveTime / time.Minute), PolicyVersion: input.Policy.Version,
	}
	if len(activeDays) == 0 {
		return streak, streak.Validate()
	}

	longest, run := 1, 1
	for index := 1; index < len(activeDays); index++ {
		if consecutiveLocalDates(activeDays[index-1].Date, activeDays[index].Date, location) {
			run++
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
	}
	last := activeDays[len(activeDays)-1]
	streak.LongestDays = longest
	streak.LastActiveLocalDate = &last.Date
	streak.LastStudyAt = &last.Latest
	today := LocalDateFromTime(input.AsOf.Time(), location)
	if last.Date == today || consecutiveLocalDates(last.Date, today, location) {
		streak.CurrentDays = run
	}
	return streak, streak.Validate()
}

// CalculateActiveStudyDaysV1 exposes the same source-of-truth day projection
// used by streaks so achievements cannot quietly redefine an active day.
func CalculateActiveStudyDaysV1(input StreakCalculationInput) ([]ActiveStudyDay, error) {
	if err := input.StudentID.Validate(); err != nil {
		return nil, fmt.Errorf("streak student: %w", err)
	}
	if err := input.AsOf.Validate(); err != nil {
		return nil, fmt.Errorf("streak as of: %w", err)
	}
	if err := input.Policy.Validate(); err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(input.Timezone)
	if err != nil {
		return nil, fmt.Errorf("streak timezone: %w", err)
	}

	days := make(map[LocalDate]streakDaySignal)
	eventIDs := make(map[ID]struct{}, len(input.Events))
	for _, event := range input.Events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("streak study event %q: %w", event.ID, err)
		}
		if event.StudentID != input.StudentID {
			return nil, fmt.Errorf("streak study event %q belongs to another student", event.ID)
		}
		if event.OccurredAt.After(input.AsOf) {
			return nil, fmt.Errorf("streak study event %q occurs after calculation time", event.ID)
		}
		if _, exists := eventIDs[event.ID]; exists {
			return nil, fmt.Errorf("streak contains duplicate study event %q", event.ID)
		}
		eventIDs[event.ID] = struct{}{}
		if !streakActivityEvent(event.Type) {
			continue
		}
		date := LocalDateFromTime(event.OccurredAt.Time(), location)
		signal := days[date]
		if signal.qualifiedAt == nil || event.OccurredAt.Before(*signal.qualifiedAt) {
			qualifiedAt := event.OccurredAt
			signal.qualifiedAt = &qualifiedAt
		}
		if signal.latest.Time().IsZero() || event.OccurredAt.After(signal.latest) {
			signal.latest = event.OccurredAt
		}
		days[date] = signal
	}

	sessionIDs := make(map[ID]struct{}, len(input.Sessions))
	sessions := append([]StudySession(nil), input.Sessions...)
	sort.Slice(sessions, func(i, j int) bool {
		left, right := sessions[i].LastActivityAt, sessions[j].LastActivityAt
		if sessions[i].EndedAt != nil {
			left = *sessions[i].EndedAt
		}
		if sessions[j].EndedAt != nil {
			right = *sessions[j].EndedAt
		}
		if left == right {
			return sessions[i].ID.String() < sessions[j].ID.String()
		}
		return left.Before(right)
	})
	for _, session := range sessions {
		if err := session.Validate(); err != nil {
			return nil, fmt.Errorf("streak study session %q: %w", session.ID, err)
		}
		if session.StudentID != input.StudentID {
			return nil, fmt.Errorf("streak study session %q belongs to another student", session.ID)
		}
		if _, exists := sessionIDs[session.ID]; exists {
			return nil, fmt.Errorf("streak contains duplicate study session %q", session.ID)
		}
		sessionIDs[session.ID] = struct{}{}
		anchor := session.LastActivityAt
		if session.EndedAt != nil {
			anchor = *session.EndedAt
		}
		if anchor.After(input.AsOf) {
			return nil, fmt.Errorf("streak study session %q ends after calculation time", session.ID)
		}
		date := LocalDateFromTime(anchor.Time(), location)
		signal := days[date]
		signal.duration += session.ActiveDuration
		if signal.duration >= input.Policy.MinimumActiveTime && (signal.qualifiedAt == nil || anchor.Before(*signal.qualifiedAt)) {
			qualifiedAt := anchor
			signal.qualifiedAt = &qualifiedAt
		}
		if signal.latest.Time().IsZero() || anchor.After(signal.latest) {
			signal.latest = anchor
		}
		days[date] = signal
	}

	active := make([]ActiveStudyDay, 0, len(days))
	for date, signal := range days {
		if signal.qualifiedAt != nil {
			active = append(active, ActiveStudyDay{Date: date, QualifiedAt: *signal.qualifiedAt, Latest: signal.latest})
		}
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Date < active[j].Date })
	return active, nil
}

func streakActivityEvent(eventType StudyEventType) bool {
	switch eventType {
	case StudyEventDiagnosticCompleted, StudyEventConceptIntroduced, StudyEventEvidenceRecorded,
		StudyEventConceptMastered, StudyEventReviewCompleted:
		return true
	default:
		return false
	}
}

func consecutiveLocalDates(left, right LocalDate, location *time.Location) bool {
	parsed, err := time.Parse(localDateLayout, left.String())
	if err != nil {
		return false
	}
	local := time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, location)
	return LocalDateFromTime(local.AddDate(0, 0, 1), location) == right
}
