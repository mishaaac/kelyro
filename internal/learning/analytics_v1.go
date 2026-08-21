package learning

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const (
	LearningAnalyticsPolicyVersion = "learning-analytics-v1"
	DefaultAnalyticsRankingLimit   = 5
	DefaultAnalyticsPaceWeeks      = 4
)

type AnalyticsCountMetric struct {
	Value       int
	Description string
}

func (metric AnalyticsCountMetric) Validate(name string) error {
	if metric.Value < 0 {
		return fmt.Errorf("analytics %s cannot be negative", name)
	}
	return requireText("analytics "+name+" description", metric.Description)
}

type AnalyticsDurationMetric struct {
	Value       time.Duration
	Description string
}

func (metric AnalyticsDurationMetric) Validate(name string) error {
	if metric.Value < 0 {
		return fmt.Errorf("analytics %s cannot be negative", name)
	}
	return requireText("analytics "+name+" description", metric.Description)
}

type AnalyticsScoreMetric struct {
	Value       *MasteryScore
	Description string
}

func (metric AnalyticsScoreMetric) Validate(name string) error {
	if metric.Value != nil {
		if err := metric.Value.Validate(); err != nil {
			return fmt.Errorf("analytics %s: %w", name, err)
		}
	}
	return requireText("analytics "+name+" description", metric.Description)
}

type AnalyticsRateMetric struct {
	Value       float64
	Unit        string
	Description string
}

func (metric AnalyticsRateMetric) Validate(name string) error {
	if math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) || metric.Value < 0 {
		return fmt.Errorf("analytics %s rate is invalid", name)
	}
	if err := requireText("analytics "+name+" unit", metric.Unit); err != nil {
		return err
	}
	return requireText("analytics "+name+" description", metric.Description)
}

type AnalyticsConceptMastery struct {
	CurriculumInstanceID ID
	ConceptID            ID
	Mastery              MasteryScore
}

func (item AnalyticsConceptMastery) Validate() error {
	if err := item.CurriculumInstanceID.Validate(); err != nil {
		return fmt.Errorf("analytics concept curriculum instance: %w", err)
	}
	if err := item.ConceptID.Validate(); err != nil {
		return fmt.Errorf("analytics concept: %w", err)
	}
	return item.Mastery.Validate()
}

type AnalyticsConceptRanking struct {
	Concepts    []AnalyticsConceptMastery
	Description string
}

type AnalyticsProgress struct {
	ConceptsIntroduced AnalyticsCountMetric
	ConceptsLearning   AnalyticsCountMetric
	ConceptsMastered   AnalyticsCountMetric
	ReviewsDue         AnalyticsCountMetric
}

type AnalyticsTime struct {
	Today AnalyticsDurationMetric
	Week  AnalyticsDurationMetric
	Month AnalyticsDurationMetric
	Total AnalyticsDurationMetric
}

type AnalyticsMastery struct {
	AverageKnown AnalyticsScoreMetric
	Strongest    AnalyticsConceptRanking
	Weakest      AnalyticsConceptRanking
}

type AnalyticsRetention struct {
	Fresh   AnalyticsCountMetric
	Due     AnalyticsCountMetric
	Overdue AnalyticsCountMetric
}

type AnalyticsActivity struct {
	ActiveDays    AnalyticsCountMetric
	CurrentStreak AnalyticsCountMetric
	LongestStreak AnalyticsCountMetric
}

type AnalyticsPace struct {
	WindowStart             LocalDate
	WindowEndExclusive      LocalDate
	WindowWeeks             int
	ConceptsMasteredPerWeek AnalyticsRateMetric
	StudyMinutesPerWeek     AnalyticsRateMetric
}

// LearningAnalyticsSnapshot is a point-in-time calculation from durable
// learning facts. It is not an authority for any underlying metric.
type LearningAnalyticsSnapshot struct {
	StudentID     ID
	CapturedAt    Timestamp
	Timezone      string
	Progress      AnalyticsProgress
	Time          AnalyticsTime
	Mastery       AnalyticsMastery
	Retention     AnalyticsRetention
	Activity      AnalyticsActivity
	Pace          AnalyticsPace
	PolicyVersion string
}

func (snapshot LearningAnalyticsSnapshot) Validate() error {
	if err := snapshot.StudentID.Validate(); err != nil {
		return fmt.Errorf("learning analytics student: %w", err)
	}
	if err := snapshot.CapturedAt.Validate(); err != nil {
		return fmt.Errorf("learning analytics captured at: %w", err)
	}
	if _, err := time.LoadLocation(snapshot.Timezone); err != nil {
		return fmt.Errorf("learning analytics timezone: %w", err)
	}
	if snapshot.PolicyVersion != LearningAnalyticsPolicyVersion {
		return fmt.Errorf("learning analytics policy %q is unsupported", snapshot.PolicyVersion)
	}
	counts := []struct {
		name   string
		metric AnalyticsCountMetric
	}{
		{"concepts introduced", snapshot.Progress.ConceptsIntroduced},
		{"concepts learning", snapshot.Progress.ConceptsLearning},
		{"concepts mastered", snapshot.Progress.ConceptsMastered},
		{"reviews due", snapshot.Progress.ReviewsDue},
		{"retention fresh", snapshot.Retention.Fresh},
		{"retention due", snapshot.Retention.Due},
		{"retention overdue", snapshot.Retention.Overdue},
		{"active days", snapshot.Activity.ActiveDays},
		{"current streak", snapshot.Activity.CurrentStreak},
		{"longest streak", snapshot.Activity.LongestStreak},
	}
	for _, candidate := range counts {
		if err := candidate.metric.Validate(candidate.name); err != nil {
			return err
		}
	}
	if snapshot.Progress.ConceptsLearning.Value > snapshot.Progress.ConceptsIntroduced.Value ||
		snapshot.Progress.ConceptsMastered.Value > snapshot.Progress.ConceptsIntroduced.Value {
		return fmt.Errorf("learning or mastered concepts cannot exceed introduced concepts")
	}
	if snapshot.Activity.CurrentStreak.Value > snapshot.Activity.LongestStreak.Value ||
		snapshot.Activity.LongestStreak.Value > snapshot.Activity.ActiveDays.Value {
		return fmt.Errorf("analytics streak counts are inconsistent")
	}
	durations := []struct {
		name   string
		metric AnalyticsDurationMetric
	}{
		{"time today", snapshot.Time.Today}, {"time week", snapshot.Time.Week},
		{"time month", snapshot.Time.Month}, {"time total", snapshot.Time.Total},
	}
	for _, candidate := range durations {
		if err := candidate.metric.Validate(candidate.name); err != nil {
			return err
		}
	}
	if snapshot.Time.Today.Value > snapshot.Time.Week.Value || snapshot.Time.Today.Value > snapshot.Time.Month.Value ||
		snapshot.Time.Week.Value > snapshot.Time.Total.Value || snapshot.Time.Month.Value > snapshot.Time.Total.Value {
		return fmt.Errorf("analytics time windows are inconsistent")
	}
	if err := snapshot.Mastery.AverageKnown.Validate("average known mastery"); err != nil {
		return err
	}
	if err := validateAnalyticsRanking("strongest concepts", snapshot.Mastery.Strongest, true); err != nil {
		return err
	}
	if err := validateAnalyticsRanking("weakest concepts", snapshot.Mastery.Weakest, false); err != nil {
		return err
	}
	if snapshot.Mastery.AverageKnown.Value == nil &&
		(len(snapshot.Mastery.Strongest.Concepts) != 0 || len(snapshot.Mastery.Weakest.Concepts) != 0) {
		return fmt.Errorf("analytics unknown mastery cannot contain rankings")
	}
	if snapshot.Pace.WindowWeeks < 1 || snapshot.Pace.WindowWeeks > 52 {
		return fmt.Errorf("analytics pace window must be within 1..52 weeks")
	}
	if err := snapshot.Pace.WindowStart.Validate(); err != nil {
		return fmt.Errorf("analytics pace start: %w", err)
	}
	if err := snapshot.Pace.WindowEndExclusive.Validate(); err != nil {
		return fmt.Errorf("analytics pace end: %w", err)
	}
	if snapshot.Pace.WindowStart >= snapshot.Pace.WindowEndExclusive {
		return fmt.Errorf("analytics pace window is empty")
	}
	if err := snapshot.Pace.ConceptsMasteredPerWeek.Validate("concepts mastered per week"); err != nil {
		return err
	}
	if err := snapshot.Pace.StudyMinutesPerWeek.Validate("study minutes per week"); err != nil {
		return err
	}
	return nil
}

func validateAnalyticsRanking(name string, ranking AnalyticsConceptRanking, strongest bool) error {
	if err := requireText("analytics "+name+" description", ranking.Description); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(ranking.Concepts))
	for index, item := range ranking.Concepts {
		if err := item.Validate(); err != nil {
			return err
		}
		key := analyticsConceptKey(item.CurriculumInstanceID, item.ConceptID)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("analytics %s contains duplicate concept %q", name, item.ConceptID)
		}
		seen[key] = struct{}{}
		if index == 0 {
			continue
		}
		previous := ranking.Concepts[index-1]
		if analyticsConceptBefore(item, previous, strongest) {
			return fmt.Errorf("analytics %s are not deterministically sorted", name)
		}
	}
	return nil
}

type LearningAnalyticsPolicy struct {
	RankingLimit int
	PaceWeeks    int
	Streak       StreakPolicy
	Version      string
}

func DefaultLearningAnalyticsPolicy() LearningAnalyticsPolicy {
	return LearningAnalyticsPolicy{
		RankingLimit: DefaultAnalyticsRankingLimit, PaceWeeks: DefaultAnalyticsPaceWeeks,
		Streak: DefaultStreakPolicy(), Version: LearningAnalyticsPolicyVersion,
	}
}

func (policy LearningAnalyticsPolicy) Validate() error {
	if policy.Version != LearningAnalyticsPolicyVersion {
		return fmt.Errorf("unsupported learning analytics policy %q", policy.Version)
	}
	if policy.RankingLimit < 1 || policy.RankingLimit > 100 {
		return fmt.Errorf("analytics ranking limit must be within 1..100")
	}
	if policy.PaceWeeks < 1 || policy.PaceWeeks > 52 {
		return fmt.Errorf("analytics pace weeks must be within 1..52")
	}
	return policy.Streak.Validate()
}

type LearningAnalyticsInput struct {
	StudentID       ID
	Timezone        string
	AsOf            Timestamp
	ConceptStates   []InstanceConceptState
	RetentionStates []RetentionState
	Reviews         []ReviewItem
	Sessions        []StudySession
	Events          []StudyEvent
	Policy          LearningAnalyticsPolicy
}

// CalculateLearningAnalyticsV1 rebuilds an explainable snapshot from primary
// learning state and history. It performs no writes and no forecasting.
func CalculateLearningAnalyticsV1(input LearningAnalyticsInput) (LearningAnalyticsSnapshot, error) {
	if err := input.StudentID.Validate(); err != nil {
		return LearningAnalyticsSnapshot{}, fmt.Errorf("learning analytics student: %w", err)
	}
	if err := input.AsOf.Validate(); err != nil {
		return LearningAnalyticsSnapshot{}, fmt.Errorf("learning analytics as of: %w", err)
	}
	if err := input.Policy.Validate(); err != nil {
		return LearningAnalyticsSnapshot{}, err
	}
	location, err := time.LoadLocation(input.Timezone)
	if err != nil {
		return LearningAnalyticsSnapshot{}, fmt.Errorf("learning analytics timezone: %w", err)
	}
	streak, err := CalculateStreakV1(StreakCalculationInput{
		StudentID: input.StudentID, Events: input.Events, Sessions: input.Sessions,
		Timezone: input.Timezone, AsOf: input.AsOf, Policy: input.Policy.Streak,
	})
	if err != nil {
		return LearningAnalyticsSnapshot{}, err
	}
	if err := validateLearningAnalyticsFacts(input); err != nil {
		return LearningAnalyticsSnapshot{}, err
	}
	windows, paceStart, paceEnd, paceStartDate, paceEndDate, err := analyticsWindows(input.AsOf, location, input.Policy.PaceWeeks)
	if err != nil {
		return LearningAnalyticsSnapshot{}, err
	}
	snapshot := newLearningAnalyticsSnapshot(input, streak, paceStartDate, paceEndDate)
	known := make([]AnalyticsConceptMastery, 0, len(input.ConceptStates))
	masteredInPace := 0
	for _, state := range input.ConceptStates {
		if state.Exposure == ExposureNotSeen {
			continue
		}
		snapshot.Progress.ConceptsIntroduced.Value++
		switch state.Exposure {
		case ExposureIntroduced, ExposureLearning, ExposurePracticing:
			snapshot.Progress.ConceptsLearning.Value++
		case ExposureMastered, ExposureReviewDue:
			snapshot.Progress.ConceptsMastered.Value++
		}
		item := AnalyticsConceptMastery{CurriculumInstanceID: state.CurriculumInstanceID, ConceptID: state.ConceptID, Mastery: state.Mastery}
		known = append(known, item)
		if state.MasteredAt != nil && containsAnalyticsInstant(*state.MasteredAt, paceStart, paceEnd) {
			masteredInPace++
		}
	}
	if len(known) > 0 {
		// Sum in stable identity order so floating-point addition cannot make the
		// result depend on repository iteration order.
		sort.Slice(known, func(i, j int) bool {
			return analyticsConceptKey(known[i].CurriculumInstanceID, known[i].ConceptID) <
				analyticsConceptKey(known[j].CurriculumInstanceID, known[j].ConceptID)
		})
		masteryTotal := 0.0
		for _, item := range known {
			masteryTotal += item.Mastery.Value()
		}
		average, _ := NewMasteryScore(masteryTotal / float64(len(known)))
		snapshot.Mastery.AverageKnown.Value = &average
		snapshot.Mastery.Strongest.Concepts = analyticsRanking(known, input.Policy.RankingLimit, true)
		snapshot.Mastery.Weakest.Concepts = analyticsRanking(known, input.Policy.RankingLimit, false)
	}
	for _, review := range input.Reviews {
		if review.Status == ReviewPending && !review.DueAt.After(input.AsOf) {
			snapshot.Progress.ReviewsDue.Value++
		}
	}
	for _, state := range input.RetentionStates {
		if state.AlgorithmVersion == LegacyRetentionAlgorithmVersion || state.Status == RetentionUnknown || state.NextDueAt == nil {
			continue
		}
		dueElapsed := input.AsOf.Time().Sub(state.NextDueAt.Time())
		switch {
		case dueElapsed < 0:
			snapshot.Retention.Fresh.Value++
		case dueElapsed <= state.StabilityEstimate:
			snapshot.Retention.Due.Value++
		default:
			snapshot.Retention.Overdue.Value++
		}
	}
	paceStudyTime := time.Duration(0)
	for _, session := range input.Sessions {
		anchor := analyticsSessionAnchor(session)
		snapshot.Time.Total.Value += session.ActiveDuration
		if containsAnalyticsInstant(anchor, windows[StudyPeriodToday][0], windows[StudyPeriodToday][1]) {
			snapshot.Time.Today.Value += session.ActiveDuration
		}
		if containsAnalyticsInstant(anchor, windows[StudyPeriodWeek][0], windows[StudyPeriodWeek][1]) {
			snapshot.Time.Week.Value += session.ActiveDuration
		}
		if containsAnalyticsInstant(anchor, windows[StudyPeriodMonth][0], windows[StudyPeriodMonth][1]) {
			snapshot.Time.Month.Value += session.ActiveDuration
		}
		if containsAnalyticsInstant(anchor, paceStart, paceEnd) {
			paceStudyTime += session.ActiveDuration
		}
	}
	snapshot.Pace.ConceptsMasteredPerWeek.Value = float64(masteredInPace) / float64(input.Policy.PaceWeeks)
	snapshot.Pace.StudyMinutesPerWeek.Value = paceStudyTime.Minutes() / float64(input.Policy.PaceWeeks)
	return snapshot, snapshot.Validate()
}

func newLearningAnalyticsSnapshot(input LearningAnalyticsInput, streak Streak, paceStart, paceEnd LocalDate) LearningAnalyticsSnapshot {
	return LearningAnalyticsSnapshot{
		StudentID: input.StudentID, CapturedAt: input.AsOf, Timezone: input.Timezone, PolicyVersion: input.Policy.Version,
		Progress: AnalyticsProgress{
			ConceptsIntroduced: AnalyticsCountMetric{Description: "Concepts introduced counts studied concept states, including concepts later mastered."},
			ConceptsLearning:   AnalyticsCountMetric{Description: "Concepts learning counts introduced, learning, and practicing states that are not currently mastered."},
			ConceptsMastered:   AnalyticsCountMetric{Description: "Concepts mastered counts current mastered and review-due states; a due review does not erase mastery."},
			ReviewsDue:         AnalyticsCountMetric{Description: "Reviews due counts pending review items whose due time has arrived."},
		},
		Time: AnalyticsTime{
			Today: AnalyticsDurationMetric{Description: "Today is intentional active study time anchored in the current local calendar day."},
			Week:  AnalyticsDurationMetric{Description: "Week is intentional active study time since Monday in the profile timezone."},
			Month: AnalyticsDurationMetric{Description: "Month is intentional active study time in the current local calendar month."},
			Total: AnalyticsDurationMetric{Description: "Total is all intentional active study time recorded for the student."},
		},
		Mastery: AnalyticsMastery{
			AverageKnown: AnalyticsScoreMetric{Description: "Known mastery average excludes concepts you have not studied yet."},
			Strongest:    AnalyticsConceptRanking{Description: "Strongest concepts are known concepts ordered by higher mastery, then stable identity."},
			Weakest:      AnalyticsConceptRanking{Description: "Weakest concepts are known concepts ordered by lower mastery, then stable identity."},
		},
		Retention: AnalyticsRetention{
			Fresh:   AnalyticsCountMetric{Description: "Fresh retention counts known estimates whose next due time has not arrived."},
			Due:     AnalyticsCountMetric{Description: "Due retention counts estimates at or after due time but within one stability interval."},
			Overdue: AnalyticsCountMetric{Description: "Overdue retention counts estimates more than one stability interval past due."},
		},
		Activity: AnalyticsActivity{
			ActiveDays:    AnalyticsCountMetric{Value: streak.TotalActiveDays, Description: "Active days are distinct local dates qualified by streak-v1 meaningful activity rules."},
			CurrentStreak: AnalyticsCountMetric{Value: streak.CurrentDays, Description: "Current streak is the latest consecutive active-day run and is informational only."},
			LongestStreak: AnalyticsCountMetric{Value: streak.LongestDays, Description: "Longest streak is the longest historical consecutive active-day run."},
		},
		Pace: AnalyticsPace{
			WindowStart: paceStart, WindowEndExclusive: paceEnd, WindowWeeks: input.Policy.PaceWeeks,
			ConceptsMasteredPerWeek: AnalyticsRateMetric{Unit: "concepts/week", Description: "Concept mastery pace is first mastery observations in the rolling local-calendar window divided by its week count."},
			StudyMinutesPerWeek:     AnalyticsRateMetric{Unit: "minutes/week", Description: "Study pace is active minutes anchored in the rolling local-calendar window divided by its week count."},
		},
	}
}

func validateLearningAnalyticsFacts(input LearningAnalyticsInput) error {
	concepts := make(map[string]struct{}, len(input.ConceptStates))
	for _, state := range input.ConceptStates {
		if err := state.Validate(); err != nil {
			return fmt.Errorf("learning analytics concept state: %w", err)
		}
		if state.StudentID != input.StudentID {
			return fmt.Errorf("learning analytics concept %q belongs to another student", state.ConceptID)
		}
		key := analyticsConceptKey(state.CurriculumInstanceID, state.ConceptID)
		if _, exists := concepts[key]; exists {
			return fmt.Errorf("learning analytics contains duplicate concept state %q", state.ConceptID)
		}
		concepts[key] = struct{}{}
		if state.UpdatedAt.After(input.AsOf) || (state.MasteredAt != nil && state.MasteredAt.After(input.AsOf)) {
			return fmt.Errorf("learning analytics concept %q is after calculation time", state.ConceptID)
		}
	}
	retention := make(map[ID]struct{}, len(input.RetentionStates))
	for _, state := range input.RetentionStates {
		if err := state.Validate(); err != nil {
			return fmt.Errorf("learning analytics retention %q: %w", state.ConceptID, err)
		}
		if state.StudentID != input.StudentID {
			return fmt.Errorf("learning analytics retention %q belongs to another student", state.ConceptID)
		}
		if _, exists := retention[state.ConceptID]; exists {
			return fmt.Errorf("learning analytics contains duplicate retention %q", state.ConceptID)
		}
		retention[state.ConceptID] = struct{}{}
		if state.MeasuredAt.After(input.AsOf) {
			return fmt.Errorf("learning analytics retention %q is measured after calculation time", state.ConceptID)
		}
	}
	reviews := make(map[ID]struct{}, len(input.Reviews))
	for _, review := range input.Reviews {
		if err := review.Validate(); err != nil {
			return fmt.Errorf("learning analytics review %q: %w", review.ID, err)
		}
		if review.StudentID != input.StudentID {
			return fmt.Errorf("learning analytics review %q belongs to another student", review.ID)
		}
		if _, exists := reviews[review.ID]; exists {
			return fmt.Errorf("learning analytics contains duplicate review %q", review.ID)
		}
		reviews[review.ID] = struct{}{}
		if review.CreatedAt.After(input.AsOf) {
			return fmt.Errorf("learning analytics review %q is created after calculation time", review.ID)
		}
		for name, timestamp := range map[string]*Timestamp{
			"completed": review.CompletedAt, "skipped": review.SkippedAt, "postponed": review.PostponedAt,
		} {
			if timestamp != nil && timestamp.After(input.AsOf) {
				return fmt.Errorf("learning analytics review %q is %s after calculation time", review.ID, name)
			}
		}
	}
	return nil
}

func analyticsWindows(asOf Timestamp, location *time.Location, paceWeeks int) (map[StudyPeriod][2]Timestamp, Timestamp, Timestamp, LocalDate, LocalDate, error) {
	windows := make(map[StudyPeriod][2]Timestamp, 3)
	for _, period := range []StudyPeriod{StudyPeriodToday, StudyPeriodWeek, StudyPeriodMonth} {
		start, end, err := StudyWindow(period, asOf.Time(), location)
		if err != nil {
			return nil, Timestamp{}, Timestamp{}, "", "", err
		}
		windows[period] = [2]Timestamp{start, end}
	}
	local := asOf.Time().In(location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	endLocal := today.AddDate(0, 0, 1)
	startLocal := endLocal.AddDate(0, 0, -7*paceWeeks)
	paceStart, err := NewTimestamp(startLocal)
	if err != nil {
		return nil, Timestamp{}, Timestamp{}, "", "", err
	}
	paceEnd, err := NewTimestamp(endLocal)
	if err != nil {
		return nil, Timestamp{}, Timestamp{}, "", "", err
	}
	return windows, paceStart, paceEnd, LocalDateFromTime(startLocal, location), LocalDateFromTime(endLocal, location), nil
}

func analyticsRanking(source []AnalyticsConceptMastery, limit int, strongest bool) []AnalyticsConceptMastery {
	items := append([]AnalyticsConceptMastery(nil), source...)
	sort.Slice(items, func(i, j int) bool { return analyticsConceptBefore(items[i], items[j], strongest) })
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func analyticsConceptBefore(left, right AnalyticsConceptMastery, strongest bool) bool {
	if left.Mastery.Value() != right.Mastery.Value() {
		if strongest {
			return left.Mastery.Value() > right.Mastery.Value()
		}
		return left.Mastery.Value() < right.Mastery.Value()
	}
	return analyticsConceptKey(left.CurriculumInstanceID, left.ConceptID) < analyticsConceptKey(right.CurriculumInstanceID, right.ConceptID)
}

func analyticsConceptKey(instanceID, conceptID ID) string {
	return instanceID.String() + "\x00" + conceptID.String()
}

func analyticsSessionAnchor(session StudySession) Timestamp {
	if session.EndedAt != nil {
		return *session.EndedAt
	}
	return session.LastActivityAt
}

func containsAnalyticsInstant(value, start, end Timestamp) bool {
	return !value.Before(start) && value.Before(end)
}
