package learning

import (
	"crypto/sha256"
	"fmt"
	"sort"
)

type ReviewSchedulingInput struct {
	Concept              ConceptState
	Retention            RetentionState
	History              []ReviewItem
	CriticalPrerequisite bool
	ScheduledAt          Timestamp
}

// ScheduleReviewV1 returns false when the concept has not been introduced or
// retention has no recall estimate. An explicit postponement always wins over
// an older retention due instant.
func ScheduleReviewV1(input ReviewSchedulingInput) (ReviewSchedule, bool, error) {
	if err := input.Concept.Validate(); err != nil {
		return ReviewSchedule{}, false, fmt.Errorf("review scheduler concept: %w", err)
	}
	if err := input.Retention.Validate(); err != nil {
		return ReviewSchedule{}, false, fmt.Errorf("review scheduler retention: %w", err)
	}
	if err := input.ScheduledAt.Validate(); err != nil {
		return ReviewSchedule{}, false, fmt.Errorf("review scheduler time: %w", err)
	}
	if input.Concept.StudentID != input.Retention.StudentID || input.Concept.ConceptID != input.Retention.ConceptID {
		return ReviewSchedule{}, false, fmt.Errorf("review scheduler concept and retention mismatch")
	}
	if input.ScheduledAt.Before(input.Retention.MeasuredAt) {
		return ReviewSchedule{}, false, fmt.Errorf("review scheduler time precedes retention measurement")
	}
	seen := make(map[ID]struct{}, len(input.History))
	var pending *ReviewItem
	var latestCompleted *ReviewItem
	for index := range input.History {
		item := input.History[index]
		if err := item.Validate(); err != nil {
			return ReviewSchedule{}, false, fmt.Errorf("review scheduler history %q: %w", item.ID, err)
		}
		if item.StudentID != input.Concept.StudentID || item.ConceptID != input.Concept.ConceptID {
			return ReviewSchedule{}, false, fmt.Errorf("review scheduler history %q belongs to another concept", item.ID)
		}
		if _, exists := seen[item.ID]; exists {
			return ReviewSchedule{}, false, fmt.Errorf("review scheduler history contains duplicate %q", item.ID)
		}
		seen[item.ID] = struct{}{}
		if item.Status == ReviewPending {
			if pending != nil {
				return ReviewSchedule{}, false, fmt.Errorf("review scheduler history contains multiple pending items")
			}
			copy := item
			pending = &copy
		}
		if item.Status == ReviewCompleted && item.AlgorithmVersion == ReviewSchedulerVersion {
			if latestCompleted == nil || item.CompletedAt.After(*latestCompleted.CompletedAt) ||
				(*item.CompletedAt == *latestCompleted.CompletedAt && item.ID.String() > latestCompleted.ID.String()) {
				copy := item
				latestCompleted = &copy
			}
		}
	}
	if input.Concept.Exposure == ExposureNotSeen || input.Retention.Status == RetentionUnknown || input.Retention.NextDueAt == nil {
		return ReviewSchedule{}, false, nil
	}

	reviewType := ReviewQuickRecall
	if input.Retention.Strength.Value() < .5 || (latestCompleted != nil && latestCompleted.Outcome == ReviewOutcomeFailure) {
		reviewType = ReviewDeep
	} else if input.Retention.Strength.Value() < .8 {
		reviewType = ReviewStandard
	}
	dueAt := *input.Retention.NextDueAt
	if pending != nil && pending.PostponeCount > 0 && pending.DueAt.After(dueAt) {
		dueAt = pending.DueAt
	}
	schedule, err := NewReviewScheduleV1(input.Concept.StudentID, input.Concept.ConceptID, *input.Concept.IntroducedAt,
		dueAt, reviewType, input.CriticalPrerequisite, input.ScheduledAt)
	if err != nil {
		return ReviewSchedule{}, false, err
	}
	return schedule, true, nil
}

// AlignReviewItemV1 applies a recalculated schedule while preserving a later
// explicit postponement on the pending item.
func AlignReviewItemV1(item ReviewItem, schedule ReviewSchedule) (ReviewItem, error) {
	if err := item.Validate(); err != nil {
		return ReviewItem{}, err
	}
	if err := schedule.Validate(); err != nil {
		return ReviewItem{}, err
	}
	if item.Status != ReviewPending || item.AlgorithmVersion != ReviewSchedulerVersion ||
		item.StudentID != schedule.StudentID || item.ConceptID != schedule.ConceptID {
		return ReviewItem{}, fmt.Errorf("only a matching pending v1 review can align to a schedule")
	}
	if item.PostponeCount == 0 || schedule.DueAt.After(item.DueAt) {
		item.DueAt = schedule.DueAt
	}
	item.Type = schedule.Type
	item.EstimatedMinutes = schedule.EstimatedMinutes
	item.CriticalPrerequisite = schedule.CriticalPrerequisite
	return item, item.Validate()
}

type ReviewQueueCandidate struct {
	Item      ReviewItem
	Retention RetentionState
}

type ReviewQueueItem struct {
	Item     ReviewItem
	Strength MasteryScore
	Status   RetentionStatus
	Overdue  bool
	Critical bool
}

type ReviewQueue struct {
	Items            []ReviewQueueItem
	Deferred         []ReviewQueueItem
	BudgetMinutes    int
	UsedMinutes      int
	TotalDueMinutes  int
	GeneratedAt      Timestamp
	AlgorithmVersion string
}

// BuildDueReviewQueueV1 applies deterministic priority and a greedy daily time
// budget. A high-priority item that cannot fit is deferred while smaller later
// items may still use the remaining budget.
func BuildDueReviewQueueV1(availability Availability, candidates []ReviewQueueCandidate, generatedAt Timestamp) (ReviewQueue, error) {
	if err := availability.Validate(); err != nil {
		return ReviewQueue{}, fmt.Errorf("review queue availability: %w", err)
	}
	if err := generatedAt.Validate(); err != nil {
		return ReviewQueue{}, fmt.Errorf("review queue time: %w", err)
	}
	queue := ReviewQueue{BudgetMinutes: availability.DailyMinutes, GeneratedAt: generatedAt, AlgorithmVersion: ReviewSchedulerVersion}
	due := make([]ReviewQueueItem, 0, len(candidates))
	seen := make(map[ID]struct{}, len(candidates))
	for _, candidate := range candidates {
		if err := candidate.Item.Validate(); err != nil {
			return ReviewQueue{}, fmt.Errorf("review queue item %q: %w", candidate.Item.ID, err)
		}
		if err := candidate.Retention.Validate(); err != nil {
			return ReviewQueue{}, fmt.Errorf("review queue retention for %q: %w", candidate.Item.ID, err)
		}
		if candidate.Item.StudentID != candidate.Retention.StudentID || candidate.Item.ConceptID != candidate.Retention.ConceptID {
			return ReviewQueue{}, fmt.Errorf("review queue candidate %q owner or concept mismatch", candidate.Item.ID)
		}
		if _, exists := seen[candidate.Item.ID]; exists {
			return ReviewQueue{}, fmt.Errorf("review queue contains duplicate item %q", candidate.Item.ID)
		}
		seen[candidate.Item.ID] = struct{}{}
		if candidate.Item.Status != ReviewPending || candidate.Item.DueAt.After(generatedAt) {
			continue
		}
		item := ReviewQueueItem{
			Item: candidate.Item, Strength: candidate.Retention.Strength, Status: candidate.Retention.Status,
			Overdue: candidate.Retention.Status == RetentionOverdue, Critical: candidate.Item.CriticalPrerequisite,
		}
		due = append(due, item)
		queue.TotalDueMinutes += candidate.Item.EstimatedMinutes
	}
	sort.Slice(due, func(i, j int) bool {
		left, right := due[i], due[j]
		if left.Overdue != right.Overdue {
			return left.Overdue
		}
		if left.Strength.Value() != right.Strength.Value() {
			return left.Strength.Value() < right.Strength.Value()
		}
		if left.Critical != right.Critical {
			return left.Critical
		}
		if left.Item.DueAt != right.Item.DueAt {
			return left.Item.DueAt.Before(right.Item.DueAt)
		}
		return left.Item.ID.String() < right.Item.ID.String()
	})
	for _, item := range due {
		if queue.UsedMinutes+item.Item.EstimatedMinutes <= queue.BudgetMinutes {
			queue.Items = append(queue.Items, item)
			queue.UsedMinutes += item.Item.EstimatedMinutes
		} else {
			queue.Deferred = append(queue.Deferred, item)
		}
	}
	return queue, nil
}

func NewReviewItemIDV1(studentID, conceptID ID, dueAt Timestamp, generation int) (ID, error) {
	if err := studentID.Validate(); err != nil {
		return ID{}, err
	}
	if err := conceptID.Validate(); err != nil {
		return ID{}, err
	}
	if err := dueAt.Validate(); err != nil {
		return ID{}, err
	}
	if generation < 0 {
		return ID{}, fmt.Errorf("review generation cannot be negative")
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%s", studentID, conceptID,
		dueAt.Time().Format(timeLayoutRFC3339Nano), generation, ReviewSchedulerVersion)))
	return NewID(fmt.Sprintf("review.%x", digest[:16]))
}

const timeLayoutRFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
