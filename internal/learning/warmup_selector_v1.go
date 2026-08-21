package learning

import (
	"fmt"
	"sort"
	"strings"
)

const (
	WarmUpSelectorVersion = "warm-up-selector-v1"
	warmUpItemMinutes     = 5
	warmUpMaximumMinutes  = 15
	warmUpBudgetDivisor   = 3
)

type WarmUpReason string

const (
	WarmUpPrerequisiteReviewDue       WarmUpReason = "prerequisite_review_due"
	WarmUpPrerequisiteRepeatedMistake WarmUpReason = "prerequisite_repeated_mistake"
	WarmUpReviewDue                   WarmUpReason = "review_due"
	WarmUpRepeatedMistake             WarmUpReason = "repeated_mistake"
)

func (reason WarmUpReason) Valid() bool {
	switch reason {
	case WarmUpPrerequisiteReviewDue, WarmUpPrerequisiteRepeatedMistake, WarmUpReviewDue, WarmUpRepeatedMistake:
		return true
	default:
		return false
	}
}

func (reason WarmUpReason) Priority() int {
	switch reason {
	case WarmUpPrerequisiteReviewDue:
		return 1
	case WarmUpPrerequisiteRepeatedMistake:
		return 2
	case WarmUpReviewDue:
		return 3
	case WarmUpRepeatedMistake:
		return 4
	default:
		return 0
	}
}

// WarmUpLessonCandidate identifies the new lesson context supplied by a
// curriculum consumer. Prerequisites are stable concept IDs, not presentation
// hierarchy or generated exercise content.
type WarmUpLessonCandidate struct {
	Curriculum             CurriculumRef
	LessonID               ID
	PrerequisiteConceptIDs []ID
}

func (candidate WarmUpLessonCandidate) Validate() error {
	if err := candidate.Curriculum.Validate(); err != nil {
		return err
	}
	if err := candidate.LessonID.Validate(); err != nil {
		return fmt.Errorf("warm-up lesson: %w", err)
	}
	seen := make(map[ID]struct{}, len(candidate.PrerequisiteConceptIDs))
	for _, conceptID := range candidate.PrerequisiteConceptIDs {
		if err := conceptID.Validate(); err != nil {
			return fmt.Errorf("warm-up prerequisite: %w", err)
		}
		if _, exists := seen[conceptID]; exists {
			return fmt.Errorf("warm-up lesson contains duplicate prerequisite %q", conceptID)
		}
		seen[conceptID] = struct{}{}
	}
	return nil
}

type WarmUpSelectionInput struct {
	StudentID        ID
	Lesson           WarmUpLessonCandidate
	Concepts         []Concept
	DueReviews       []ReviewItem
	Mistakes         []Mistake
	RecentConceptIDs []ID
	AvailableMinutes int
	GeneratedAt      Timestamp
}

type WarmUpItem struct {
	Concept          Concept
	Reason           WarmUpReason
	Priority         int
	EstimatedMinutes int
	Explanation      string
}

func (item WarmUpItem) Validate() error {
	if err := item.Concept.Validate(); err != nil {
		return err
	}
	if !item.Reason.Valid() || item.Priority != item.Reason.Priority() {
		return fmt.Errorf("warm-up reason %q and priority %d are inconsistent", item.Reason, item.Priority)
	}
	if item.EstimatedMinutes != warmUpItemMinutes {
		return fmt.Errorf("warm-up item must take %d minutes", warmUpItemMinutes)
	}
	if strings.TrimSpace(item.Explanation) == "" {
		return fmt.Errorf("warm-up explanation is empty")
	}
	return nil
}

type WarmUpPlan struct {
	LessonID         ID
	Items            []WarmUpItem
	BudgetMinutes    int
	UsedMinutes      int
	AvailableMinutes int
	GeneratedAt      Timestamp
	AlgorithmVersion string
}

func (plan WarmUpPlan) Validate() error {
	if err := plan.LessonID.Validate(); err != nil {
		return fmt.Errorf("warm-up plan lesson: %w", err)
	}
	if plan.AvailableMinutes < 0 || plan.AvailableMinutes > 24*60 {
		return fmt.Errorf("warm-up available minutes must be within 0..1440")
	}
	if plan.BudgetMinutes != warmUpBudget(plan.AvailableMinutes) {
		return fmt.Errorf("warm-up budget does not match policy")
	}
	if plan.UsedMinutes < 0 || plan.UsedMinutes > plan.BudgetMinutes || plan.UsedMinutes >= plan.AvailableMinutes && plan.UsedMinutes > 0 {
		return fmt.Errorf("warm-up used minutes do not preserve new-content time")
	}
	if plan.AlgorithmVersion != WarmUpSelectorVersion {
		return fmt.Errorf("unsupported warm-up selector %q", plan.AlgorithmVersion)
	}
	if err := plan.GeneratedAt.Validate(); err != nil {
		return fmt.Errorf("warm-up generated at: %w", err)
	}
	seen := make(map[ID]struct{}, len(plan.Items))
	minutes := 0
	for _, item := range plan.Items {
		if err := item.Validate(); err != nil {
			return err
		}
		if _, exists := seen[item.Concept.ID]; exists {
			return fmt.Errorf("warm-up plan contains duplicate concept %q", item.Concept.ID)
		}
		seen[item.Concept.ID] = struct{}{}
		minutes += item.EstimatedMinutes
	}
	if minutes != plan.UsedMinutes {
		return fmt.Errorf("warm-up used minutes do not match items")
	}
	return nil
}

type warmUpCandidate struct {
	concept       Concept
	reason        WarmUpReason
	dueAt         *Timestamp
	occurrences   int
	lastMistakeAt *Timestamp
	recentIndex   int
	recent        bool
}

// SelectWarmUpV1 chooses concepts only. I-05 remains responsible for creating
// and executing any concrete recall exercise.
func SelectWarmUpV1(input WarmUpSelectionInput) (WarmUpPlan, error) {
	if err := input.StudentID.Validate(); err != nil {
		return WarmUpPlan{}, fmt.Errorf("warm-up student: %w", err)
	}
	if err := input.Lesson.Validate(); err != nil {
		return WarmUpPlan{}, err
	}
	if err := input.GeneratedAt.Validate(); err != nil {
		return WarmUpPlan{}, fmt.Errorf("warm-up generated at: %w", err)
	}
	if input.AvailableMinutes < 0 || input.AvailableMinutes > 24*60 {
		return WarmUpPlan{}, fmt.Errorf("warm-up available minutes must be within 0..1440")
	}

	concepts := make(map[ID]Concept, len(input.Concepts))
	for _, concept := range input.Concepts {
		if err := concept.Validate(); err != nil {
			return WarmUpPlan{}, fmt.Errorf("warm-up concept: %w", err)
		}
		if _, exists := concepts[concept.ID]; exists {
			return WarmUpPlan{}, fmt.Errorf("warm-up contains duplicate concept %q", concept.ID)
		}
		concepts[concept.ID] = concept
	}
	prerequisites := make(map[ID]struct{}, len(input.Lesson.PrerequisiteConceptIDs))
	for _, conceptID := range input.Lesson.PrerequisiteConceptIDs {
		if _, exists := concepts[conceptID]; !exists {
			return WarmUpPlan{}, fmt.Errorf("warm-up prerequisite %q is absent from the candidate curriculum", conceptID)
		}
		prerequisites[conceptID] = struct{}{}
	}
	recent := make(map[ID]int, len(input.RecentConceptIDs))
	for index, conceptID := range input.RecentConceptIDs {
		if err := conceptID.Validate(); err != nil {
			return WarmUpPlan{}, fmt.Errorf("recent warm-up concept: %w", err)
		}
		if _, exists := recent[conceptID]; exists {
			return WarmUpPlan{}, fmt.Errorf("recent warm-up concepts contain duplicate %q", conceptID)
		}
		recent[conceptID] = index
	}

	due := make(map[ID]Timestamp, len(input.DueReviews))
	for _, review := range input.DueReviews {
		if err := review.Validate(); err != nil {
			return WarmUpPlan{}, fmt.Errorf("warm-up due review %q: %w", review.ID, err)
		}
		if review.StudentID != input.StudentID || review.Status != ReviewPending || review.DueAt.After(input.GeneratedAt) {
			return WarmUpPlan{}, fmt.Errorf("warm-up review %q is not a due review for the student", review.ID)
		}
		if _, exists := concepts[review.ConceptID]; !exists {
			return WarmUpPlan{}, fmt.Errorf("warm-up review %q references a concept outside the candidate curriculum", review.ID)
		}
		if _, exists := due[review.ConceptID]; exists {
			return WarmUpPlan{}, fmt.Errorf("warm-up contains multiple due reviews for concept %q", review.ConceptID)
		}
		due[review.ConceptID] = review.DueAt
	}

	type mistakeSignal struct {
		occurrences int
		lastSeenAt  Timestamp
	}
	repeated := make(map[ID]mistakeSignal)
	for _, mistake := range input.Mistakes {
		if err := mistake.Validate(); err != nil {
			return WarmUpPlan{}, fmt.Errorf("warm-up mistake %q: %w", mistake.ID, err)
		}
		if mistake.StudentID != input.StudentID {
			return WarmUpPlan{}, fmt.Errorf("warm-up mistake %q belongs to another student", mistake.ID)
		}
		if _, exists := concepts[mistake.ConceptID]; !exists {
			return WarmUpPlan{}, fmt.Errorf("warm-up mistake %q references a concept outside the candidate curriculum", mistake.ID)
		}
		if mistake.Status == MistakeResolved || mistake.Occurrences < 2 {
			continue
		}
		signal := repeated[mistake.ConceptID]
		signal.occurrences += mistake.Occurrences
		if signal.lastSeenAt.Time().IsZero() || mistake.LastSeenAt.After(signal.lastSeenAt) {
			signal.lastSeenAt = mistake.LastSeenAt
		}
		repeated[mistake.ConceptID] = signal
	}

	ids := make(map[ID]struct{}, len(due)+len(repeated))
	for conceptID := range due {
		ids[conceptID] = struct{}{}
	}
	for conceptID := range repeated {
		ids[conceptID] = struct{}{}
	}
	candidates := make([]warmUpCandidate, 0, len(ids))
	for conceptID := range ids {
		_, prerequisite := prerequisites[conceptID]
		dueAt, reviewDue := due[conceptID]
		mistake, repeatedMistake := repeated[conceptID]
		reason := WarmUpRepeatedMistake
		switch {
		case prerequisite && reviewDue:
			reason = WarmUpPrerequisiteReviewDue
		case prerequisite && repeatedMistake:
			reason = WarmUpPrerequisiteRepeatedMistake
		case reviewDue:
			reason = WarmUpReviewDue
		}
		candidate := warmUpCandidate{concept: concepts[conceptID], reason: reason, occurrences: mistake.occurrences}
		if reviewDue {
			candidate.dueAt = &dueAt
		}
		if repeatedMistake {
			lastSeenAt := mistake.lastSeenAt
			candidate.lastMistakeAt = &lastSeenAt
		}
		candidate.recentIndex, candidate.recent = recent[conceptID]
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.reason.Priority() != right.reason.Priority() {
			return left.reason.Priority() < right.reason.Priority()
		}
		if left.recent != right.recent {
			return !left.recent
		}
		if left.recent && left.recentIndex != right.recentIndex {
			return left.recentIndex > right.recentIndex
		}
		if left.dueAt != nil && right.dueAt != nil && *left.dueAt != *right.dueAt {
			return left.dueAt.Before(*right.dueAt)
		}
		if left.occurrences != right.occurrences {
			return left.occurrences > right.occurrences
		}
		if left.lastMistakeAt != nil && right.lastMistakeAt != nil && *left.lastMistakeAt != *right.lastMistakeAt {
			return left.lastMistakeAt.After(*right.lastMistakeAt)
		}
		return left.concept.ID.String() < right.concept.ID.String()
	})

	plan := WarmUpPlan{
		LessonID: input.Lesson.LessonID, BudgetMinutes: warmUpBudget(input.AvailableMinutes),
		AvailableMinutes: input.AvailableMinutes, GeneratedAt: input.GeneratedAt, AlgorithmVersion: WarmUpSelectorVersion,
	}
	for _, candidate := range candidates {
		if plan.UsedMinutes+warmUpItemMinutes > plan.BudgetMinutes {
			break
		}
		plan.Items = append(plan.Items, WarmUpItem{
			Concept: candidate.concept, Reason: candidate.reason, Priority: candidate.reason.Priority(),
			EstimatedMinutes: warmUpItemMinutes, Explanation: warmUpExplanation(candidate, input.Lesson.LessonID),
		})
		plan.UsedMinutes += warmUpItemMinutes
	}
	return plan, plan.Validate()
}

func warmUpBudget(availableMinutes int) int {
	budget := availableMinutes / warmUpBudgetDivisor
	if budget > warmUpMaximumMinutes {
		budget = warmUpMaximumMinutes
	}
	return budget - budget%warmUpItemMinutes
}

func warmUpExplanation(candidate warmUpCandidate, lessonID ID) string {
	var explanation string
	switch candidate.reason {
	case WarmUpPrerequisiteReviewDue:
		explanation = fmt.Sprintf("Prerequisite for lesson %s with a review due.", lessonID)
	case WarmUpPrerequisiteRepeatedMistake:
		explanation = fmt.Sprintf("Prerequisite for lesson %s with a repeated unresolved mistake.", lessonID)
	case WarmUpReviewDue:
		explanation = "Review is due for this concept."
	case WarmUpRepeatedMistake:
		explanation = "A repeated unresolved mistake needs a short recall."
	}
	if candidate.dueAt != nil && candidate.occurrences >= 2 && candidate.reason != WarmUpPrerequisiteRepeatedMistake {
		explanation = strings.TrimSuffix(explanation, ".") + "; it also has a repeated unresolved mistake."
	}
	return explanation
}
