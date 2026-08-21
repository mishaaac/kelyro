package learning

import (
	"fmt"
	"sort"
	"time"
)

// AchievementConceptProgress is the minimum curriculum fact required to
// recalculate concept and module milestones.
type AchievementConceptProgress struct {
	ConceptID  ID
	MasteredAt *Timestamp
}

// AchievementModuleProgress groups the complete concept membership of one
// module in one curriculum instance. Missing mastery timestamps remain facts,
// not inferred failures.
type AchievementModuleProgress struct {
	StudentID            ID
	CurriculumInstanceID ID
	GoalID               ID
	Curriculum           CurriculumRef
	ModuleID             ID
	Concepts             []AchievementConceptProgress
}

func (progress AchievementModuleProgress) Validate() error {
	for _, field := range []struct {
		name string
		id   ID
	}{
		{"achievement module student", progress.StudentID},
		{"achievement module curriculum instance", progress.CurriculumInstanceID},
		{"achievement module goal", progress.GoalID},
		{"achievement module", progress.ModuleID},
	} {
		if err := field.id.Validate(); err != nil {
			return fmt.Errorf("%s: %w", field.name, err)
		}
	}
	if err := progress.Curriculum.Validate(); err != nil {
		return err
	}
	if len(progress.Concepts) == 0 {
		return fmt.Errorf("achievement module %q has no concepts", progress.ModuleID)
	}
	seen := make(map[ID]struct{}, len(progress.Concepts))
	for _, concept := range progress.Concepts {
		if err := concept.ConceptID.Validate(); err != nil {
			return fmt.Errorf("achievement module concept: %w", err)
		}
		if _, exists := seen[concept.ConceptID]; exists {
			return fmt.Errorf("achievement module %q contains duplicate concept %q", progress.ModuleID, concept.ConceptID)
		}
		seen[concept.ConceptID] = struct{}{}
		if err := validateOptionalTimestamp("achievement concept mastered at", concept.MasteredAt); err != nil {
			return err
		}
	}
	return nil
}

type AchievementEvaluationInput struct {
	StudentID    ID
	Timezone     string
	AsOf         Timestamp
	Sessions     []StudySession
	Events       []StudyEvent
	Reviews      []ReviewItem
	Modules      []AchievementModuleProgress
	StreakPolicy StreakPolicy
}

// EvaluateAchievementsV1 calculates every satisfied definition from durable
// learning facts. It returns candidates only; persistence and idempotency stay
// in the application service.
func EvaluateAchievementsV1(definitions []AchievementDefinition, input AchievementEvaluationInput) ([]Achievement, error) {
	if err := input.StudentID.Validate(); err != nil {
		return nil, fmt.Errorf("achievement student: %w", err)
	}
	if err := input.AsOf.Validate(); err != nil {
		return nil, fmt.Errorf("achievement as of: %w", err)
	}
	if input.StreakPolicy.Version == "" {
		input.StreakPolicy = DefaultStreakPolicy()
	}
	activeDays, err := CalculateActiveStudyDaysV1(StreakCalculationInput{
		StudentID: input.StudentID, Events: input.Events, Sessions: input.Sessions,
		Timezone: input.Timezone, AsOf: input.AsOf, Policy: input.StreakPolicy,
	})
	if err != nil {
		return nil, err
	}
	if err := validateAchievementFacts(input); err != nil {
		return nil, err
	}
	seenDefinitions := make(map[ID]struct{}, len(definitions))
	candidates := make([]Achievement, 0, len(definitions))
	for _, definition := range definitions {
		if err := definition.Validate(); err != nil {
			return nil, err
		}
		if _, exists := seenDefinitions[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate achievement definition %q", definition.ID)
		}
		seenDefinitions[definition.ID] = struct{}{}
		unlockedAt, context, satisfied := evaluateAchievementCriterion(definition, input, activeDays)
		if !satisfied {
			continue
		}
		id, err := NewID("achievement." + input.StudentID.String() + "." + definition.ID.String())
		if err != nil {
			return nil, err
		}
		candidate := Achievement{
			ID: id, StudentID: input.StudentID, Key: definition.ID, Name: definition.Title,
			Description: definition.Description, Criteria: definition.Criteria, Config: definition.Config,
			Hidden: definition.Hidden, DefinitionVersion: definition.Version,
			Status: AchievementUnlocked, UnlockedAt: &unlockedAt, Context: context,
			PolicyVersion: AchievementPolicyVersion,
		}
		if err := candidate.Validate(); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if *candidates[i].UnlockedAt == *candidates[j].UnlockedAt {
			return candidates[i].Key.String() < candidates[j].Key.String()
		}
		return candidates[i].UnlockedAt.Before(*candidates[j].UnlockedAt)
	})
	return candidates, nil
}

func validateAchievementFacts(input AchievementEvaluationInput) error {
	reviewIDs := make(map[ID]struct{}, len(input.Reviews))
	for _, review := range input.Reviews {
		if err := review.Validate(); err != nil {
			return fmt.Errorf("achievement review %q: %w", review.ID, err)
		}
		if review.StudentID != input.StudentID {
			return fmt.Errorf("achievement review %q belongs to another student", review.ID)
		}
		if _, exists := reviewIDs[review.ID]; exists {
			return fmt.Errorf("achievement contains duplicate review %q", review.ID)
		}
		reviewIDs[review.ID] = struct{}{}
		if review.CompletedAt != nil && review.CompletedAt.After(input.AsOf) {
			return fmt.Errorf("achievement review %q completes after calculation time", review.ID)
		}
	}
	seenModules := make(map[string]struct{}, len(input.Modules))
	for _, module := range input.Modules {
		if err := module.Validate(); err != nil {
			return err
		}
		if module.StudentID != input.StudentID {
			return fmt.Errorf("achievement module %q belongs to another student", module.ModuleID)
		}
		key := module.CurriculumInstanceID.String() + "\x00" + module.ModuleID.String()
		if _, exists := seenModules[key]; exists {
			return fmt.Errorf("duplicate achievement module %q in instance %q", module.ModuleID, module.CurriculumInstanceID)
		}
		seenModules[key] = struct{}{}
		for _, concept := range module.Concepts {
			if concept.MasteredAt != nil && concept.MasteredAt.After(input.AsOf) {
				return fmt.Errorf("achievement concept %q is mastered after calculation time", concept.ConceptID)
			}
		}
	}
	return nil
}

func evaluateAchievementCriterion(definition AchievementDefinition, input AchievementEvaluationInput, activeDays []ActiveStudyDay) (Timestamp, map[string]string, bool) {
	switch definition.Criteria {
	case AchievementFirstSession:
		sessions := sortedAchievementSessions(input.Sessions)
		for _, session := range sessions {
			if session.Status == StudySessionCompleted && session.ActivityCount > 0 && session.EndedAt != nil {
				return *session.EndedAt, map[string]string{"session_id": session.ID.String()}, true
			}
		}
	case AchievementFirstConceptMastered:
		var selected *AchievementConceptProgress
		var selectedInstance ID
		for _, module := range input.Modules {
			for index := range module.Concepts {
				concept := &module.Concepts[index]
				if concept.MasteredAt == nil || (selected != nil && !earlierAchievementConcept(*concept, module.CurriculumInstanceID, *selected, selectedInstance)) {
					continue
				}
				copy := *concept
				selected, selectedInstance = &copy, module.CurriculumInstanceID
			}
		}
		if selected != nil {
			return *selected.MasteredAt, map[string]string{"concept_id": selected.ConceptID.String(), "curriculum_instance_id": selectedInstance.String()}, true
		}
	case AchievementActiveDays:
		if len(activeDays) >= definition.Config.Count {
			day := activeDays[definition.Config.Count-1]
			return day.QualifiedAt, map[string]string{"active_days": achievementNumber(definition.Config.Count), "timezone": input.Timezone}, true
		}
	case AchievementStudyMinutes:
		threshold := time.Duration(definition.Config.Minutes) * time.Minute
		var total time.Duration
		for _, session := range sortedAchievementSessions(input.Sessions) {
			total += session.ActiveDuration
			if total >= threshold {
				anchor := session.LastActivityAt
				if session.EndedAt != nil {
					anchor = *session.EndedAt
				}
				return anchor, map[string]string{"studied_minutes": achievementNumber(definition.Config.Minutes)}, true
			}
		}
	case AchievementFirstReviewCompleted:
		var selected *ReviewItem
		for index := range input.Reviews {
			review := &input.Reviews[index]
			if review.Status != ReviewCompleted || review.CompletedAt == nil || (selected != nil && !earlierAchievementReview(*review, *selected)) {
				continue
			}
			copy := *review
			selected = &copy
		}
		if selected != nil {
			return *selected.CompletedAt, map[string]string{"review_id": selected.ID.String(), "concept_id": selected.ConceptID.String()}, true
		}
	case AchievementModuleMastered:
		var selected *AchievementModuleProgress
		var completedAt Timestamp
		for index := range input.Modules {
			module := &input.Modules[index]
			candidateAt, mastered := moduleMasteredAt(*module)
			if !mastered || (selected != nil && !earlierAchievementModule(*module, candidateAt, *selected, completedAt)) {
				continue
			}
			selected, completedAt = module, candidateAt
		}
		if selected != nil {
			return completedAt, map[string]string{
				"module_id": selected.ModuleID.String(), "curriculum_instance_id": selected.CurriculumInstanceID.String(),
				"goal_id": selected.GoalID.String(), "curriculum_id": selected.Curriculum.ID.String(),
				"curriculum_version": selected.Curriculum.Version,
			}, true
		}
	}
	return Timestamp{}, nil, false
}

func earlierAchievementConcept(candidate AchievementConceptProgress, candidateInstance ID, selected AchievementConceptProgress, selectedInstance ID) bool {
	if candidate.MasteredAt.Before(*selected.MasteredAt) {
		return true
	}
	if *candidate.MasteredAt != *selected.MasteredAt {
		return false
	}
	candidateKey := candidateInstance.String() + "\x00" + candidate.ConceptID.String()
	selectedKey := selectedInstance.String() + "\x00" + selected.ConceptID.String()
	return candidateKey < selectedKey
}

func earlierAchievementReview(candidate, selected ReviewItem) bool {
	if candidate.CompletedAt.Before(*selected.CompletedAt) {
		return true
	}
	return *candidate.CompletedAt == *selected.CompletedAt && candidate.ID.String() < selected.ID.String()
}

func earlierAchievementModule(candidate AchievementModuleProgress, candidateAt Timestamp, selected AchievementModuleProgress, selectedAt Timestamp) bool {
	if candidateAt.Before(selectedAt) {
		return true
	}
	if candidateAt != selectedAt {
		return false
	}
	candidateKey := candidate.CurriculumInstanceID.String() + "\x00" + candidate.ModuleID.String()
	selectedKey := selected.CurriculumInstanceID.String() + "\x00" + selected.ModuleID.String()
	return candidateKey < selectedKey
}

func sortedAchievementSessions(source []StudySession) []StudySession {
	sessions := append([]StudySession(nil), source...)
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
	return sessions
}

func moduleMasteredAt(module AchievementModuleProgress) (Timestamp, bool) {
	var completedAt Timestamp
	for _, concept := range module.Concepts {
		if concept.MasteredAt == nil {
			return Timestamp{}, false
		}
		if completedAt.Time().IsZero() || concept.MasteredAt.After(completedAt) {
			completedAt = *concept.MasteredAt
		}
	}
	return completedAt, true
}
