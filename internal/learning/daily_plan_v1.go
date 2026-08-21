package learning

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"
	"time"
)

const (
	DailyPlanPolicyVersion               = "daily-plan-v1"
	LegacyDailyPlanPolicyVersion         = "legacy-daily-plan/v0"
	DefaultDailyPlanWarmUpMinutes        = 5
	DefaultDailyPlanReinforcementMinutes = 10
	DefaultDailyPlanNewLearningMinutes   = 25
	DefaultDailyPlanMinimumNewMinutes    = 10
	DefaultDailyPlanBufferMinutes        = 5
)

type DailyPlanPolicy struct {
	WarmUpMinutes             int
	ReinforcementMinutes      int
	NewLearningMinutes        int
	MinimumNewLearningMinutes int
	BufferMinutes             int
	Version                   string
}

func DefaultDailyPlanPolicy() DailyPlanPolicy {
	return DailyPlanPolicy{
		WarmUpMinutes:             DefaultDailyPlanWarmUpMinutes,
		ReinforcementMinutes:      DefaultDailyPlanReinforcementMinutes,
		NewLearningMinutes:        DefaultDailyPlanNewLearningMinutes,
		MinimumNewLearningMinutes: DefaultDailyPlanMinimumNewMinutes,
		BufferMinutes:             DefaultDailyPlanBufferMinutes,
		Version:                   DailyPlanPolicyVersion,
	}
}

func (policy DailyPlanPolicy) Validate() error {
	if policy.Version != DailyPlanPolicyVersion {
		return fmt.Errorf("unsupported daily plan policy %q", policy.Version)
	}
	for _, candidate := range []struct {
		name  string
		value int
	}{
		{"warm-up minutes", policy.WarmUpMinutes},
		{"reinforcement minutes", policy.ReinforcementMinutes},
		{"new learning minutes", policy.NewLearningMinutes},
		{"minimum new learning minutes", policy.MinimumNewLearningMinutes},
	} {
		if candidate.value < 1 || candidate.value > 24*60 {
			return fmt.Errorf("daily plan %s must be within 1..1440", candidate.name)
		}
	}
	if policy.MinimumNewLearningMinutes > policy.NewLearningMinutes {
		return fmt.Errorf("daily plan minimum new learning time exceeds its target")
	}
	if policy.BufferMinutes < 0 || policy.BufferMinutes > 24*60 {
		return fmt.Errorf("daily plan buffer minutes must be within 0..1440")
	}
	return nil
}

// DailyPlanCurriculumConcept is the minimal ordered curriculum projection the
// planner needs. Every compact prerequisite requires mastery under the current
// resolved threshold.
type DailyPlanCurriculumConcept struct {
	ConceptID       ID
	Sequence        int
	PrerequisiteIDs []ID
}

func (concept DailyPlanCurriculumConcept) Validate() error {
	if err := concept.ConceptID.Validate(); err != nil {
		return fmt.Errorf("daily plan curriculum concept: %w", err)
	}
	if concept.Sequence < 0 {
		return fmt.Errorf("daily plan concept %q has a negative sequence", concept.ConceptID)
	}
	if err := validateIDs("daily plan concept prerequisites", concept.PrerequisiteIDs); err != nil {
		return err
	}
	for _, prerequisiteID := range concept.PrerequisiteIDs {
		if prerequisiteID == concept.ConceptID {
			return fmt.Errorf("daily plan concept %q requires itself", concept.ConceptID)
		}
	}
	return nil
}

type DailyPlanInput struct {
	StudentID            ID
	GoalID               ID
	CurriculumInstanceID ID
	Curriculum           CurriculumRef
	Timezone             string
	Date                 Timestamp
	GeneratedAt          Timestamp
	AvailableMinutes     int
	MasteryPolicy        ResolvedMasteryThreshold
	Concepts             []DailyPlanCurriculumConcept
	States               []InstanceConceptState
	ReviewsDue           []ReviewItem
	Retention            []RetentionState
	Mistakes             []Mistake
	History              []StudyEvent
	GenerationReason     DailyPlanGenerationReason
	Policy               DailyPlanPolicy
}

type dailyPlanReviewCandidate struct {
	item            ReviewItem
	strength        float64
	overdue         bool
	criticalOverdue bool
}

type dailyPlanWeakness struct {
	concept       DailyPlanCurriculumConcept
	state         *InstanceConceptState
	occurrences   int
	lastStudiedAt *Timestamp
}

// BuildAdaptiveDailyPlanV1 selects work only. I-05 remains responsible for
// constructing and assessing the concrete lesson, practice, or review.
func BuildAdaptiveDailyPlanV1(input DailyPlanInput) (DailyPlan, error) {
	prepared, err := validateDailyPlanInput(input)
	if err != nil {
		return DailyPlan{}, err
	}
	fingerprint, err := dailyPlanFingerprintV1(prepared)
	if err != nil {
		return DailyPlan{}, err
	}
	planID, err := dailyPlanIDV1(prepared.StudentID, prepared.GoalID, prepared.Date)
	if err != nil {
		return DailyPlan{}, err
	}
	buffer := 0
	if prepared.AvailableMinutes >= prepared.Policy.MinimumNewLearningMinutes+prepared.Policy.BufferMinutes {
		buffer = prepared.Policy.BufferMinutes
	}
	plan := DailyPlan{
		ID: planID, StudentID: prepared.StudentID, GoalID: prepared.GoalID,
		CurriculumInstanceID: prepared.CurriculumInstanceID, Date: prepared.Date, CreatedAt: prepared.GeneratedAt,
		Timezone: prepared.Timezone, AvailableMinutes: prepared.AvailableMinutes, BufferMinutes: buffer,
		GenerationReason: prepared.GenerationReason, SourceFingerprint: fingerprint, PolicyVersion: prepared.Policy.Version,
	}
	budget := prepared.AvailableMinutes - buffer
	usedConcepts := make(map[ID]struct{})
	consumedReviews := make(map[ID]struct{})
	states := dailyPlanStatesByConcept(prepared.States)
	mistakes := dailyPlanMistakesByConcept(prepared.Mistakes)
	lastStudied := dailyPlanLastStudied(prepared.History)
	reviews := dailyPlanDueReviews(prepared.ReviewsDue, prepared.Retention, prepared.GeneratedAt)

	if len(reviews) > 0 && reviews[0].criticalOverdue && prepared.Policy.WarmUpMinutes <= budget {
		candidate := reviews[0]
		plan.Items = append(plan.Items, newDailyPlanItem(plan.ID, DailyPlanRoleWarmUp,
			DailyPlanCriticalOverduePrerequisite, DailyPlanReview, candidate.item.ConceptID,
			prepared.Policy.WarmUpMinutes, len(plan.Items),
			fmt.Sprintf("Critical prerequisite %s is overdue; begin with a short recall before other work.", candidate.item.ConceptID)))
		plan.PlannedMinutes += prepared.Policy.WarmUpMinutes
		usedConcepts[candidate.item.ConceptID] = struct{}{}
		consumedReviews[candidate.item.ID] = struct{}{}
	}

	for _, candidate := range reviews {
		if _, consumed := consumedReviews[candidate.item.ID]; consumed {
			continue
		}
		if _, selected := usedConcepts[candidate.item.ConceptID]; selected {
			continue
		}
		if plan.PlannedMinutes+candidate.item.EstimatedMinutes > budget {
			continue
		}
		plan.Items = append(plan.Items, newDailyPlanItem(plan.ID, DailyPlanRoleReview,
			DailyPlanImportantDueReview, DailyPlanReview, candidate.item.ConceptID,
			candidate.item.EstimatedMinutes, len(plan.Items),
			fmt.Sprintf("Review %s is due; it is scheduled before new material to protect retention.", candidate.item.ConceptID)))
		plan.PlannedMinutes += candidate.item.EstimatedMinutes
		usedConcepts[candidate.item.ConceptID] = struct{}{}
	}

	frontier, frontierPresent, blocking := dailyPlanFrontier(prepared.Concepts, states, prepared.MasteryPolicy.Requirement)
	workExists := len(reviews) > 0 || frontierPresent
	if len(blocking) > 0 {
		weaknesses := make([]dailyPlanWeakness, 0, len(blocking))
		for _, concept := range blocking {
			weaknesses = append(weaknesses, dailyPlanWeaknessFor(concept, states, mistakes, lastStudied))
		}
		sortDailyPlanWeaknesses(weaknesses)
		for _, weakness := range weaknesses {
			if _, selected := usedConcepts[weakness.concept.ConceptID]; selected {
				continue
			}
			if plan.PlannedMinutes+prepared.Policy.ReinforcementMinutes > budget {
				break
			}
			plan.Items = append(plan.Items, newDailyPlanItem(plan.ID, DailyPlanRoleReinforcement,
				DailyPlanBlockingWeakness, DailyPlanPractice, weakness.concept.ConceptID,
				prepared.Policy.ReinforcementMinutes, len(plan.Items),
				fmt.Sprintf("Concept %s is below the current mastery requirement and blocks the next curriculum step.", weakness.concept.ConceptID)))
			plan.PlannedMinutes += prepared.Policy.ReinforcementMinutes
			usedConcepts[weakness.concept.ConceptID] = struct{}{}
			break
		}
	} else if frontierPresent {
		remaining := budget - plan.PlannedMinutes
		if remaining >= prepared.Policy.MinimumNewLearningMinutes {
			minutes := prepared.Policy.NewLearningMinutes
			if minutes > remaining {
				minutes = remaining
			}
			plan.Items = append(plan.Items, newDailyPlanItem(plan.ID, DailyPlanRoleNewLearning,
				DailyPlanNextEligibleConcept, DailyPlanLearn, frontier.ConceptID, minutes, len(plan.Items),
				fmt.Sprintf("Concept %s is the next unseen curriculum concept whose prerequisites are satisfied.", frontier.ConceptID)))
			plan.PlannedMinutes += minutes
			usedConcepts[frontier.ConceptID] = struct{}{}
		}
	}

	optional := dailyPlanOptionalWeaknesses(prepared.Concepts, states, mistakes, lastStudied, usedConcepts)
	if len(optional) > 0 {
		workExists = true
	}
	if plan.PlannedMinutes+prepared.Policy.ReinforcementMinutes <= budget && len(optional) > 0 {
		weakness := optional[0]
		plan.Items = append(plan.Items, newDailyPlanItem(plan.ID, DailyPlanRoleReinforcement,
			DailyPlanOptionalExtraPractice, DailyPlanPractice, weakness.concept.ConceptID,
			prepared.Policy.ReinforcementMinutes, len(plan.Items),
			fmt.Sprintf("Concept %s has an unresolved mistake and is selected for optional extra practice.", weakness.concept.ConceptID)))
		plan.PlannedMinutes += prepared.Policy.ReinforcementMinutes
	}

	hasNewLearning := false
	for _, item := range plan.Items {
		hasNewLearning = hasNewLearning || item.Role == DailyPlanRoleNewLearning
	}
	switch {
	case hasNewLearning:
		plan.Status = DailyPlanReady
	case len(plan.Items) > 0:
		plan.Status = DailyPlanReviewOnly
	case workExists:
		plan.Status = DailyPlanTimeLimited
	default:
		plan.Status = DailyPlanNothingUrgent
	}
	return plan, plan.Validate()
}

func validateDailyPlanInput(input DailyPlanInput) (DailyPlanInput, error) {
	for _, candidate := range []struct {
		name string
		id   ID
	}{
		{"student", input.StudentID}, {"goal", input.GoalID}, {"curriculum instance", input.CurriculumInstanceID},
	} {
		if err := candidate.id.Validate(); err != nil {
			return DailyPlanInput{}, fmt.Errorf("daily plan %s: %w", candidate.name, err)
		}
	}
	if err := input.Curriculum.Validate(); err != nil {
		return DailyPlanInput{}, err
	}
	if err := input.Date.Validate(); err != nil {
		return DailyPlanInput{}, fmt.Errorf("daily plan date: %w", err)
	}
	if err := input.GeneratedAt.Validate(); err != nil {
		return DailyPlanInput{}, fmt.Errorf("daily plan generated at: %w", err)
	}
	location, err := time.LoadLocation(input.Timezone)
	if err != nil {
		return DailyPlanInput{}, fmt.Errorf("daily plan timezone: %w", err)
	}
	localDate := input.Date.Time().In(location)
	if localDate.Hour() != 0 || localDate.Minute() != 0 || localDate.Second() != 0 || localDate.Nanosecond() != 0 ||
		LocalDateFromTime(input.Date.Time(), location) != LocalDateFromTime(input.GeneratedAt.Time(), location) {
		return DailyPlanInput{}, fmt.Errorf("daily plan date and generation time are not the same local day")
	}
	if input.AvailableMinutes < 0 || input.AvailableMinutes > 24*60 {
		return DailyPlanInput{}, fmt.Errorf("daily plan available minutes must be within 0..1440")
	}
	if err := input.MasteryPolicy.Validate(); err != nil {
		return DailyPlanInput{}, err
	}
	if err := input.Policy.Validate(); err != nil {
		return DailyPlanInput{}, err
	}
	if !input.GenerationReason.Valid() {
		return DailyPlanInput{}, fmt.Errorf("daily plan generation reason %q is invalid", input.GenerationReason)
	}
	if len(input.Concepts) == 0 {
		return DailyPlanInput{}, fmt.Errorf("daily plan curriculum has no concepts")
	}
	prepared := input
	prepared.Concepts = append([]DailyPlanCurriculumConcept(nil), input.Concepts...)
	sort.Slice(prepared.Concepts, func(i, j int) bool {
		if prepared.Concepts[i].Sequence != prepared.Concepts[j].Sequence {
			return prepared.Concepts[i].Sequence < prepared.Concepts[j].Sequence
		}
		return prepared.Concepts[i].ConceptID.String() < prepared.Concepts[j].ConceptID.String()
	})
	known := make(map[ID]struct{}, len(prepared.Concepts))
	sequences := make(map[int]struct{}, len(prepared.Concepts))
	for index := range prepared.Concepts {
		concept := &prepared.Concepts[index]
		concept.PrerequisiteIDs = append([]ID(nil), concept.PrerequisiteIDs...)
		sort.Slice(concept.PrerequisiteIDs, func(i, j int) bool { return concept.PrerequisiteIDs[i].String() < concept.PrerequisiteIDs[j].String() })
		if err := concept.Validate(); err != nil {
			return DailyPlanInput{}, err
		}
		if _, exists := known[concept.ConceptID]; exists {
			return DailyPlanInput{}, fmt.Errorf("daily plan contains duplicate curriculum concept %q", concept.ConceptID)
		}
		if _, exists := sequences[concept.Sequence]; exists {
			return DailyPlanInput{}, fmt.Errorf("daily plan contains duplicate curriculum sequence %d", concept.Sequence)
		}
		known[concept.ConceptID], sequences[concept.Sequence] = struct{}{}, struct{}{}
	}
	for _, concept := range prepared.Concepts {
		for _, prerequisiteID := range concept.PrerequisiteIDs {
			if _, exists := known[prerequisiteID]; !exists {
				return DailyPlanInput{}, fmt.Errorf("daily plan prerequisite %q is outside the curriculum", prerequisiteID)
			}
		}
	}
	if err := validateDailyPlanFacts(prepared, known); err != nil {
		return DailyPlanInput{}, err
	}
	return prepared, nil
}

func validateDailyPlanFacts(input DailyPlanInput, known map[ID]struct{}) error {
	stateIDs := make(map[ID]struct{}, len(input.States))
	for _, state := range input.States {
		if err := state.Validate(); err != nil {
			return fmt.Errorf("daily plan concept state: %w", err)
		}
		if state.StudentID != input.StudentID || state.CurriculumInstanceID != input.CurriculumInstanceID {
			return fmt.Errorf("daily plan concept %q belongs to another student or instance", state.ConceptID)
		}
		if _, exists := known[state.ConceptID]; !exists {
			return fmt.Errorf("daily plan concept state %q is outside the curriculum", state.ConceptID)
		}
		if _, exists := stateIDs[state.ConceptID]; exists {
			return fmt.Errorf("daily plan contains duplicate concept state %q", state.ConceptID)
		}
		stateIDs[state.ConceptID] = struct{}{}
		if state.UpdatedAt.After(input.GeneratedAt) {
			return fmt.Errorf("daily plan concept state %q is from the future", state.ConceptID)
		}
	}
	retentionIDs := make(map[ID]struct{}, len(input.Retention))
	for _, state := range input.Retention {
		if err := state.Validate(); err != nil {
			return fmt.Errorf("daily plan retention %q: %w", state.ConceptID, err)
		}
		if state.StudentID != input.StudentID || state.MeasuredAt.After(input.GeneratedAt) {
			return fmt.Errorf("daily plan retention %q has invalid ownership or time", state.ConceptID)
		}
		if _, exists := known[state.ConceptID]; !exists {
			continue
		}
		if _, exists := retentionIDs[state.ConceptID]; exists {
			return fmt.Errorf("daily plan contains duplicate retention %q", state.ConceptID)
		}
		retentionIDs[state.ConceptID] = struct{}{}
	}
	reviewIDs := make(map[ID]struct{}, len(input.ReviewsDue))
	for _, review := range input.ReviewsDue {
		if err := review.Validate(); err != nil {
			return fmt.Errorf("daily plan review %q: %w", review.ID, err)
		}
		if review.StudentID != input.StudentID || review.Status != ReviewPending || review.DueAt.After(input.GeneratedAt) || review.CreatedAt.After(input.GeneratedAt) {
			return fmt.Errorf("daily plan review %q is not currently due for the student", review.ID)
		}
		if _, exists := known[review.ConceptID]; !exists {
			return fmt.Errorf("daily plan review %q is outside the curriculum", review.ID)
		}
		if _, exists := reviewIDs[review.ID]; exists {
			return fmt.Errorf("daily plan contains duplicate review %q", review.ID)
		}
		reviewIDs[review.ID] = struct{}{}
	}
	mistakeIDs := make(map[ID]struct{}, len(input.Mistakes))
	for _, mistake := range input.Mistakes {
		if err := mistake.Validate(); err != nil {
			return fmt.Errorf("daily plan mistake %q: %w", mistake.ID, err)
		}
		if mistake.StudentID != input.StudentID || mistake.LastSeenAt.After(input.GeneratedAt) ||
			(mistake.ResolvedAt != nil && mistake.ResolvedAt.After(input.GeneratedAt)) {
			return fmt.Errorf("daily plan mistake %q has invalid ownership or time", mistake.ID)
		}
		if _, exists := known[mistake.ConceptID]; !exists {
			continue
		}
		if _, exists := mistakeIDs[mistake.ID]; exists {
			return fmt.Errorf("daily plan contains duplicate mistake %q", mistake.ID)
		}
		mistakeIDs[mistake.ID] = struct{}{}
	}
	eventIDs := make(map[ID]struct{}, len(input.History))
	for _, event := range input.History {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("daily plan history %q: %w", event.ID, err)
		}
		if event.StudentID != input.StudentID || event.OccurredAt.After(input.GeneratedAt) {
			return fmt.Errorf("daily plan history %q has invalid ownership or time", event.ID)
		}
		if _, exists := eventIDs[event.ID]; exists {
			return fmt.Errorf("daily plan contains duplicate history event %q", event.ID)
		}
		eventIDs[event.ID] = struct{}{}
	}
	return nil
}

func dailyPlanStatesByConcept(states []InstanceConceptState) map[ID]InstanceConceptState {
	result := make(map[ID]InstanceConceptState, len(states))
	for _, state := range states {
		result[state.ConceptID] = state
	}
	return result
}

func dailyPlanMistakesByConcept(mistakes []Mistake) map[ID][]Mistake {
	result := make(map[ID][]Mistake)
	for _, mistake := range mistakes {
		if mistake.Status != MistakeResolved {
			result[mistake.ConceptID] = append(result[mistake.ConceptID], mistake)
		}
	}
	return result
}

func dailyPlanLastStudied(events []StudyEvent) map[ID]Timestamp {
	result := make(map[ID]Timestamp)
	for _, event := range events {
		if event.ConceptID == nil {
			continue
		}
		current, exists := result[*event.ConceptID]
		if !exists || event.OccurredAt.After(current) {
			result[*event.ConceptID] = event.OccurredAt
		}
	}
	return result
}

func dailyPlanFrontier(concepts []DailyPlanCurriculumConcept, states map[ID]InstanceConceptState, requirement MasteryRequirement) (DailyPlanCurriculumConcept, bool, []DailyPlanCurriculumConcept) {
	byID := make(map[ID]DailyPlanCurriculumConcept, len(concepts))
	for _, concept := range concepts {
		byID[concept.ConceptID] = concept
	}
	for _, concept := range concepts {
		state, present := states[concept.ConceptID]
		if present && requirement.SatisfiedBy(state.Mastery) {
			continue
		}
		if present && state.Exposure != ExposureNotSeen {
			return concept, true, []DailyPlanCurriculumConcept{concept}
		}
		blocking := make([]DailyPlanCurriculumConcept, 0)
		for _, prerequisiteID := range concept.PrerequisiteIDs {
			prerequisiteState, exists := states[prerequisiteID]
			if !exists || !requirement.SatisfiedBy(prerequisiteState.Mastery) {
				blocking = append(blocking, byID[prerequisiteID])
			}
		}
		return concept, true, blocking
	}
	return DailyPlanCurriculumConcept{}, false, nil
}

func dailyPlanWeaknessFor(concept DailyPlanCurriculumConcept, states map[ID]InstanceConceptState, mistakes map[ID][]Mistake, lastStudied map[ID]Timestamp) dailyPlanWeakness {
	weakness := dailyPlanWeakness{concept: concept}
	if state, exists := states[concept.ConceptID]; exists {
		copy := state
		weakness.state = &copy
	}
	for _, mistake := range mistakes[concept.ConceptID] {
		weakness.occurrences += mistake.Occurrences
	}
	if timestamp, exists := lastStudied[concept.ConceptID]; exists {
		copy := timestamp
		weakness.lastStudiedAt = &copy
	}
	return weakness
}

func sortDailyPlanWeaknesses(items []dailyPlanWeakness) {
	sort.Slice(items, func(i, j int) bool {
		left, right := items[i], items[j]
		if left.occurrences != right.occurrences {
			return left.occurrences > right.occurrences
		}
		leftScore, rightScore := 0.0, 0.0
		if left.state != nil {
			leftScore = left.state.Mastery.Value()
		}
		if right.state != nil {
			rightScore = right.state.Mastery.Value()
		}
		if leftScore != rightScore {
			return leftScore < rightScore
		}
		if left.lastStudiedAt == nil || right.lastStudiedAt == nil {
			if left.lastStudiedAt != right.lastStudiedAt {
				return left.lastStudiedAt == nil
			}
		} else if *left.lastStudiedAt != *right.lastStudiedAt {
			return left.lastStudiedAt.Before(*right.lastStudiedAt)
		}
		if left.concept.Sequence != right.concept.Sequence {
			return left.concept.Sequence < right.concept.Sequence
		}
		return left.concept.ConceptID.String() < right.concept.ConceptID.String()
	})
}

func dailyPlanOptionalWeaknesses(concepts []DailyPlanCurriculumConcept, states map[ID]InstanceConceptState, mistakes map[ID][]Mistake, lastStudied map[ID]Timestamp, used map[ID]struct{}) []dailyPlanWeakness {
	result := make([]dailyPlanWeakness, 0)
	for _, concept := range concepts {
		if _, selected := used[concept.ConceptID]; selected || len(mistakes[concept.ConceptID]) == 0 {
			continue
		}
		state, present := states[concept.ConceptID]
		if !present || state.Exposure == ExposureNotSeen {
			continue
		}
		result = append(result, dailyPlanWeaknessFor(concept, states, mistakes, lastStudied))
	}
	sortDailyPlanWeaknesses(result)
	return result
}

func dailyPlanDueReviews(reviews []ReviewItem, retention []RetentionState, generatedAt Timestamp) []dailyPlanReviewCandidate {
	retentionByConcept := make(map[ID]RetentionState, len(retention))
	for _, state := range retention {
		retentionByConcept[state.ConceptID] = state
	}
	result := make([]dailyPlanReviewCandidate, 0, len(reviews))
	for _, review := range reviews {
		candidate := dailyPlanReviewCandidate{item: review, strength: 1}
		if state, exists := retentionByConcept[review.ConceptID]; exists {
			candidate.strength = state.Strength.Value()
			if state.NextDueAt != nil && state.StabilityEstimate > 0 {
				overdueAt := state.NextDueAt.Time().Add(state.StabilityEstimate)
				candidate.overdue = generatedAt.Time().After(overdueAt)
			}
		}
		candidate.criticalOverdue = review.CriticalPrerequisite && candidate.overdue
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if left.criticalOverdue != right.criticalOverdue {
			return left.criticalOverdue
		}
		if left.overdue != right.overdue {
			return left.overdue
		}
		if left.item.CriticalPrerequisite != right.item.CriticalPrerequisite {
			return left.item.CriticalPrerequisite
		}
		if left.strength != right.strength {
			return left.strength < right.strength
		}
		if left.item.DueAt != right.item.DueAt {
			return left.item.DueAt.Before(right.item.DueAt)
		}
		return left.item.ID.String() < right.item.ID.String()
	})
	return result
}

func newDailyPlanItem(planID ID, role DailyPlanItemRole, reason DailyPlanSelectionReason, itemType DailyPlanItemType, conceptID ID, minutes, position int, explanation string) DailyPlanItem {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%s", planID, role, reason, conceptID, position, DailyPlanPolicyVersion)))
	id, _ := NewID(fmt.Sprintf("plan-item.%x", digest[:16]))
	return DailyPlanItem{
		ID: id, Type: itemType, Role: role, Reason: reason, Explanation: explanation,
		ConceptIDs: []ID{conceptID}, EstimatedMinutes: minutes, Position: position,
	}
}

func dailyPlanIDV1(studentID, goalID ID, date Timestamp) (ID, error) {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%s", studentID, goalID,
		date.Time().Format(timeLayoutRFC3339Nano), DailyPlanPolicyVersion)))
	return NewID(fmt.Sprintf("plan.%x", digest[:16]))
}

func dailyPlanFingerprintV1(input DailyPlanInput) (string, error) {
	digest := sha256.New()
	dailyPlanHashValues(digest, input.StudentID, input.GoalID, input.CurriculumInstanceID,
		input.Curriculum.ID, input.Curriculum.Version, input.Timezone, input.Date.Time().Format(timeLayoutRFC3339Nano),
		input.AvailableMinutes, input.MasteryPolicy.Requirement.Threshold.Value(), input.MasteryPolicy.Source,
		input.Policy.Version, input.Policy.WarmUpMinutes, input.Policy.ReinforcementMinutes,
		input.Policy.NewLearningMinutes, input.Policy.MinimumNewLearningMinutes, input.Policy.BufferMinutes)
	states := dailyPlanStatesByConcept(input.States)
	lastStudied := dailyPlanLastStudied(input.History)
	for _, concept := range input.Concepts {
		dailyPlanHashValues(digest, "concept", concept.Sequence, concept.ConceptID)
		for _, prerequisiteID := range concept.PrerequisiteIDs {
			dailyPlanHashValues(digest, "prerequisite", prerequisiteID)
		}
		if state, exists := states[concept.ConceptID]; exists {
			dailyPlanHashValues(digest, "state", state.Exposure, state.Mastery.Value(), state.UpdatedAt.Time().Format(timeLayoutRFC3339Nano))
		} else {
			dailyPlanHashValues(digest, "state", "missing")
		}
		if timestamp, exists := lastStudied[concept.ConceptID]; exists {
			dailyPlanHashValues(digest, "last-studied", timestamp.Time().Format(timeLayoutRFC3339Nano))
		}
	}
	reviews := dailyPlanDueReviews(input.ReviewsDue, input.Retention, input.GeneratedAt)
	for _, review := range reviews {
		dailyPlanHashValues(digest, "review", review.item.ID, review.item.ConceptID, review.item.DueAt.Time().Format(timeLayoutRFC3339Nano),
			review.item.EstimatedMinutes, review.item.CriticalPrerequisite, review.strength, review.overdue)
	}
	mistakes := append([]Mistake(nil), input.Mistakes...)
	sort.Slice(mistakes, func(i, j int) bool { return mistakes[i].ID.String() < mistakes[j].ID.String() })
	for _, mistake := range mistakes {
		if mistake.Status == MistakeResolved {
			continue
		}
		dailyPlanHashValues(digest, "mistake", mistake.ID, mistake.ConceptID, mistake.Occurrences, mistake.Status,
			mistake.LastSeenAt.Time().Format(timeLayoutRFC3339Nano))
	}
	return "sha256:" + hex.EncodeToString(digest.Sum(nil)), nil
}

func dailyPlanHashValues(target hash.Hash, values ...any) {
	for _, value := range values {
		_, _ = fmt.Fprintf(target, "%v\x00", value)
	}
}
