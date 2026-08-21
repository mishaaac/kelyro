package learning

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const (
	RetentionAlgorithmVersion       = "retention-v1"
	LegacyRetentionAlgorithmVersion = "legacy-retention/v0"
	RetentionRecallSuccessThreshold = .7
)

type RetentionStatus string

const (
	RetentionFresh     RetentionStatus = "fresh"
	RetentionStable    RetentionStatus = "stable"
	RetentionWeakening RetentionStatus = "weakening"
	RetentionDue       RetentionStatus = "due"
	RetentionOverdue   RetentionStatus = "overdue"
	RetentionUnknown   RetentionStatus = "unknown"
)

func (status RetentionStatus) Valid() bool {
	switch status {
	case RetentionFresh, RetentionStable, RetentionWeakening, RetentionDue, RetentionOverdue, RetentionUnknown:
		return true
	default:
		return false
	}
}

// RetentionState is a point-in-time, replaceable estimate. Strength predicts
// recall at MeasuredAt; it never overwrites evidence-derived mastery.
type RetentionState struct {
	StudentID            ID
	ConceptID            ID
	LastSuccessfulRecall *Timestamp
	LastPractice         *Timestamp
	ReviewCount          int
	SuccessfulReviews    int
	FailedReviews        int
	StabilityEstimate    time.Duration
	Strength             MasteryScore
	Status               RetentionStatus
	NextDueAt            *Timestamp
	MeasuredAt           Timestamp
	AlgorithmVersion     string
}

func (state RetentionState) Validate() error {
	if err := state.StudentID.Validate(); err != nil {
		return fmt.Errorf("retention state student: %w", err)
	}
	if err := state.ConceptID.Validate(); err != nil {
		return fmt.Errorf("retention state concept: %w", err)
	}
	if err := state.Strength.Validate(); err != nil {
		return fmt.Errorf("retention state: %w", err)
	}
	if err := state.MeasuredAt.Validate(); err != nil {
		return fmt.Errorf("retention measured at: %w", err)
	}
	if err := validateOptionalTimestamp("retention last successful recall", state.LastSuccessfulRecall); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("retention last practice", state.LastPractice); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("retention next due at", state.NextDueAt); err != nil {
		return err
	}
	if !state.Status.Valid() {
		return fmt.Errorf("retention status %q is invalid", state.Status)
	}
	if state.AlgorithmVersion != RetentionAlgorithmVersion && state.AlgorithmVersion != LegacyRetentionAlgorithmVersion {
		return fmt.Errorf("unsupported retention algorithm %q", state.AlgorithmVersion)
	}
	if state.ReviewCount < 0 || state.SuccessfulReviews < 0 || state.FailedReviews < 0 ||
		state.ReviewCount != state.SuccessfulReviews+state.FailedReviews {
		return fmt.Errorf("retention review counts are inconsistent")
	}
	if state.StabilityEstimate < 0 || state.StabilityEstimate%time.Second != 0 {
		return fmt.Errorf("retention stability must be a non-negative whole-second duration")
	}
	if state.LastSuccessfulRecall != nil && state.LastSuccessfulRecall.After(state.MeasuredAt) {
		return fmt.Errorf("retention successful recall is after measurement")
	}
	if state.LastPractice != nil && state.LastPractice.After(state.MeasuredAt) {
		return fmt.Errorf("retention practice is after measurement")
	}
	if state.LastSuccessfulRecall != nil && state.LastPractice == nil {
		return fmt.Errorf("retention successful recall requires practice history")
	}
	if state.LastSuccessfulRecall != nil && state.LastPractice.Before(*state.LastSuccessfulRecall) {
		return fmt.Errorf("retention successful recall is after last practice")
	}
	if state.AlgorithmVersion == LegacyRetentionAlgorithmVersion {
		if state.Status != RetentionUnknown || state.LastSuccessfulRecall != nil || state.LastPractice != nil ||
			state.ReviewCount != 0 || state.StabilityEstimate != 0 || state.NextDueAt != nil {
			return fmt.Errorf("legacy retention snapshot must remain unknown")
		}
		return nil
	}
	if state.Status == RetentionUnknown {
		if state.LastSuccessfulRecall != nil || state.LastPractice != nil || state.ReviewCount != 0 ||
			state.StabilityEstimate != 0 || state.NextDueAt != nil || state.Strength.Value() != 0 {
			return fmt.Errorf("unknown retention cannot contain a recall estimate")
		}
		return nil
	}
	if state.LastPractice == nil || state.StabilityEstimate <= 0 || state.StabilityEstimate > 90*24*time.Hour || state.NextDueAt == nil {
		return fmt.Errorf("known retention requires practice, positive stability, and next due time")
	}
	if !state.NextDueAt.Time().Equal(state.LastPractice.Time().Add(state.StabilityEstimate)) {
		return fmt.Errorf("retention next due time is inconsistent with stability")
	}
	dueElapsed := state.MeasuredAt.Time().Sub(state.NextDueAt.Time())
	switch state.Status {
	case RetentionFresh, RetentionStable, RetentionWeakening:
		if dueElapsed >= 0 {
			return fmt.Errorf("retention status %q cannot be at or after due time", state.Status)
		}
	case RetentionDue:
		if dueElapsed < 0 || dueElapsed > state.StabilityEstimate {
			return fmt.Errorf("due retention is outside its due window")
		}
	case RetentionOverdue:
		if dueElapsed <= state.StabilityEstimate {
			return fmt.Errorf("overdue retention has not crossed its boundary")
		}
	}
	return nil
}

// RetentionCalculation keeps the inputs and factors used by retention-v1
// inspectable without asking callers to reverse-engineer RetentionState.
type RetentionCalculation struct {
	State              RetentionState
	Mastery            MasteryScore
	MasteryKnown       bool
	Difficulty         float64
	DifficultyFactor   float64
	ReviewFactor       float64
	RecentResultFactor float64
}

type RetentionProgression struct {
	State        InstanceConceptState
	StateChanged bool
}

// ApplyRetentionV1 projects a durable retention estimate onto an already
// mastered instance state. Due means "check recall", not "mastery was lost".
func ApplyRetentionV1(state InstanceConceptState, retention RetentionState) (RetentionProgression, error) {
	if err := state.Validate(); err != nil {
		return RetentionProgression{}, fmt.Errorf("retention progression state: %w", err)
	}
	if err := retention.Validate(); err != nil {
		return RetentionProgression{}, fmt.Errorf("retention progression estimate: %w", err)
	}
	if state.StudentID != retention.StudentID || state.ConceptID != retention.ConceptID {
		return RetentionProgression{}, fmt.Errorf("retention progression owner or concept mismatch")
	}
	result := RetentionProgression{State: state}
	if retention.AlgorithmVersion != RetentionAlgorithmVersion || retention.Status == RetentionUnknown ||
		(state.Exposure != ExposureMastered && state.Exposure != ExposureReviewDue) {
		return result, nil
	}
	if retention.MeasuredAt.Before(state.UpdatedAt) {
		return RetentionProgression{}, fmt.Errorf("retention measurement precedes instance concept update")
	}
	result.State.ReviewDueAt = cloneTimestamp(retention.NextDueAt)
	if retention.Status == RetentionDue || retention.Status == RetentionOverdue {
		result.State.Exposure = ExposureReviewDue
	} else if result.State.Exposure == ExposureReviewDue {
		result.State.Exposure = ExposureMastered
	}
	result.StateChanged = result.State.Exposure != state.Exposure || !optionalTimestampEqual(result.State.ReviewDueAt, state.ReviewDueAt)
	if result.StateChanged {
		result.State.UpdatedAt = retention.MeasuredAt
	}
	if err := result.State.Validate(); err != nil {
		return RetentionProgression{}, fmt.Errorf("retention progression result: %w", err)
	}
	return result, nil
}

// CalculateRetentionV1 estimates recall from objective recall-bearing evidence.
// Self-report and manual imports can affect mastery but do not establish a
// retention clock by themselves.
func CalculateRetentionV1(mastery MasteryCalculation, items []Evidence, measuredAt Timestamp) (RetentionCalculation, error) {
	if err := mastery.Validate(); err != nil {
		return RetentionCalculation{}, fmt.Errorf("retention mastery: %w", err)
	}
	if err := measuredAt.Validate(); err != nil {
		return RetentionCalculation{}, fmt.Errorf("retention measurement: %w", err)
	}
	calculation := RetentionCalculation{Mastery: mastery.Score, MasteryKnown: mastery.Known}
	state := RetentionState{
		StudentID: mastery.StudentID, ConceptID: mastery.ConceptID, Strength: mastery.Score,
		Status: RetentionUnknown, MeasuredAt: measuredAt, AlgorithmVersion: RetentionAlgorithmVersion,
	}
	canonical := append([]Evidence(nil), items...)
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].ObservedAt == canonical[j].ObservedAt {
			return canonical[i].ID.String() < canonical[j].ID.String()
		}
		return canonical[i].ObservedAt.Before(canonical[j].ObservedAt)
	})
	seen := make(map[ID]struct{}, len(canonical))
	var latest *Evidence
	for index := range canonical {
		item := canonical[index]
		if err := item.Validate(); err != nil {
			return RetentionCalculation{}, fmt.Errorf("retention evidence %q: %w", item.ID, err)
		}
		if item.StudentID != mastery.StudentID || item.ConceptID != mastery.ConceptID {
			return RetentionCalculation{}, fmt.Errorf("retention evidence %q belongs to another student or concept", item.ID)
		}
		if item.ObservedAt.After(measuredAt) {
			return RetentionCalculation{}, fmt.Errorf("retention evidence %q occurs after measurement", item.ID)
		}
		if _, exists := seen[item.ID]; exists {
			return RetentionCalculation{}, fmt.Errorf("retention contains duplicate evidence %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		if !retentionRecallBearing(item.Type) {
			continue
		}
		copy := item
		latest = &copy
		state.LastPractice = cloneTimestamp(&item.ObservedAt)
		if item.Score.Value() >= RetentionRecallSuccessThreshold {
			state.LastSuccessfulRecall = cloneTimestamp(&item.ObservedAt)
		}
		if item.Type == EvidenceReviewRecall {
			state.ReviewCount++
			if item.Score.Value() >= RetentionRecallSuccessThreshold {
				state.SuccessfulReviews++
			} else {
				state.FailedReviews++
			}
		}
	}
	if !mastery.Known || latest == nil {
		state.Strength = MasteryScore{}
		calculation.State = state
		return calculation, state.Validate()
	}

	calculation.Difficulty = latest.Difficulty
	calculation.DifficultyFactor = 1.25 - .5*latest.Difficulty
	calculation.ReviewFactor = clampFloat(1+.5*float64(state.SuccessfulReviews)-.25*float64(state.FailedReviews), .5, 4)
	calculation.RecentResultFactor = 1
	if latest.Score.Value() < RetentionRecallSuccessThreshold {
		calculation.RecentResultFactor = .25
	} else if latest.Type == EvidenceReviewRecall {
		calculation.RecentResultFactor = 1.25
	}
	stabilityDays := (1 + 6*mastery.Score.Value()) * calculation.DifficultyFactor * calculation.ReviewFactor * calculation.RecentResultFactor
	state.StabilityEstimate = durationSeconds(stabilityDays * 24 * float64(time.Hour))
	state.StabilityEstimate = clampDuration(state.StabilityEstimate, 6*time.Hour, 90*24*time.Hour)
	due, _ := NewTimestamp(state.LastPractice.Time().Add(state.StabilityEstimate))
	state.NextDueAt = &due
	elapsed := measuredAt.Time().Sub(state.LastPractice.Time())
	retained := mastery.Score.Value() * math.Exp(-float64(elapsed)/float64(state.StabilityEstimate))
	state.Strength, _ = NewMasteryScore(clampFloat(retained, 0, 1))
	state.Status = retentionStatusAt(*latest, state, elapsed)
	calculation.State = state
	if err := state.Validate(); err != nil {
		return RetentionCalculation{}, err
	}
	return calculation, nil
}

func retentionRecallBearing(evidenceType EvidenceType) bool {
	switch evidenceType {
	case EvidenceDiagnosticObjective, EvidenceKnowledgeCheck, EvidencePracticeSuccess, EvidencePracticeFailure,
		EvidenceAssessment, EvidenceProject, EvidenceReviewRecall:
		return true
	default:
		return false
	}
}

func retentionStatusAt(latest Evidence, state RetentionState, elapsed time.Duration) RetentionStatus {
	if !state.MeasuredAt.Before(*state.NextDueAt) {
		if state.MeasuredAt.Time().Sub(state.NextDueAt.Time()) > state.StabilityEstimate {
			return RetentionOverdue
		}
		return RetentionDue
	}
	if latest.Score.Value() < RetentionRecallSuccessThreshold {
		return RetentionWeakening
	}
	freshWindow := clampDuration(state.StabilityEstimate/4, 6*time.Hour, 24*time.Hour)
	if elapsed <= freshWindow {
		return RetentionFresh
	}
	if state.Strength.Value() >= RetentionRecallSuccessThreshold {
		return RetentionStable
	}
	return RetentionWeakening
}

func durationSeconds(nanoseconds float64) time.Duration {
	return time.Duration(math.Round(nanoseconds/float64(time.Second))) * time.Second
}

func clampDuration(value, minimum, maximum time.Duration) time.Duration {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func clampFloat(value, minimum, maximum float64) float64 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func optionalTimestampEqual(left, right *Timestamp) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Time().Equal(right.Time())
}

// ReviewSchedule states when a concept should next be reviewed. Imported
// schedules may predate Kelyro's own concept introduction record.
type ReviewSchedule struct {
	StudentID    ID
	ConceptID    ID
	IntroducedAt *Timestamp
	DueAt        Timestamp
	Imported     bool
}

func NewReviewSchedule(studentID, conceptID ID, introducedAt *Timestamp, dueAt Timestamp, imported bool) (ReviewSchedule, error) {
	schedule := ReviewSchedule{
		StudentID: studentID, ConceptID: conceptID, IntroducedAt: introducedAt,
		DueAt: dueAt, Imported: imported,
	}
	return schedule, schedule.Validate()
}

func (schedule ReviewSchedule) Validate() error {
	if err := schedule.StudentID.Validate(); err != nil {
		return fmt.Errorf("review schedule student: %w", err)
	}
	if err := schedule.ConceptID.Validate(); err != nil {
		return fmt.Errorf("review schedule concept: %w", err)
	}
	if err := validateOptionalTimestamp("review concept introduced at", schedule.IntroducedAt); err != nil {
		return err
	}
	if err := schedule.DueAt.Validate(); err != nil {
		return fmt.Errorf("review due at: %w", err)
	}
	if schedule.IntroducedAt == nil && !schedule.Imported {
		return fmt.Errorf("review requires concept introduction unless imported")
	}
	if schedule.IntroducedAt != nil && schedule.DueAt.Before(*schedule.IntroducedAt) && !schedule.Imported {
		return fmt.Errorf("review cannot be due before concept introduction unless imported")
	}
	return nil
}

type ReviewStatus string

const (
	ReviewPending   ReviewStatus = "pending"
	ReviewCompleted ReviewStatus = "completed"
	ReviewSkipped   ReviewStatus = "skipped"
)

func (status ReviewStatus) Valid() bool {
	switch status {
	case ReviewPending, ReviewCompleted, ReviewSkipped:
		return true
	default:
		return false
	}
}

type ReviewItem struct {
	ID          ID
	StudentID   ID
	ConceptID   ID
	DueAt       Timestamp
	Status      ReviewStatus
	CompletedAt *Timestamp
}

func (item ReviewItem) Validate() error {
	if err := item.ID.Validate(); err != nil {
		return fmt.Errorf("review item: %w", err)
	}
	if err := item.StudentID.Validate(); err != nil {
		return fmt.Errorf("review item student: %w", err)
	}
	if err := item.ConceptID.Validate(); err != nil {
		return fmt.Errorf("review item concept: %w", err)
	}
	if err := item.DueAt.Validate(); err != nil {
		return fmt.Errorf("review item due at: %w", err)
	}
	if !item.Status.Valid() {
		return fmt.Errorf("review status %q is invalid", item.Status)
	}
	if err := validateOptionalTimestamp("review completed at", item.CompletedAt); err != nil {
		return err
	}
	if item.Status == ReviewCompleted && item.CompletedAt == nil {
		return fmt.Errorf("completed review is missing completion timestamp")
	}
	if item.Status != ReviewCompleted && item.CompletedAt != nil {
		return fmt.Errorf("non-completed review cannot have completion timestamp")
	}
	return nil
}
