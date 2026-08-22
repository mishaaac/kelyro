package memory

import (
	"context"
	"errors"
	"sort"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

type studentRepository struct{ store *Store }

func (repository studentRepository) Create(ctx context.Context, student learning.Student) error {
	if err := contextError("create memory student", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.students[student.ID]; exists {
		return conflict("create memory student")
	}
	repository.store.students[student.ID] = cloneStudent(student)
	return nil
}

func (repository studentRepository) Get(ctx context.Context, id learning.ID) (learning.Student, error) {
	if err := contextError("get memory student", ctx); err != nil {
		return learning.Student{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	student, exists := repository.store.students[id]
	if !exists {
		return learning.Student{}, notFound("get memory student")
	}
	return cloneStudent(student), nil
}

func (repository studentRepository) Update(ctx context.Context, student learning.Student) error {
	if err := contextError("update memory student", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.students[student.ID]; !exists {
		return notFound("update memory student")
	}
	repository.store.students[student.ID] = cloneStudent(student)
	return nil
}

type goalRepository struct{ store *Store }

func (repository goalRepository) Create(ctx context.Context, goal learning.LearningGoal) error {
	if err := contextError("create memory goal", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.goals[goal.ID]; exists {
		return conflict("create memory goal")
	}
	if goal.Status == learning.GoalActive && repository.hasOtherActive(goal.StudentID, goal.ID) {
		return conflict("create memory goal")
	}
	repository.store.goals[goal.ID] = cloneGoal(goal)
	return nil
}

func (repository goalRepository) Get(ctx context.Context, id learning.ID) (learning.LearningGoal, error) {
	if err := contextError("get memory goal", ctx); err != nil {
		return learning.LearningGoal{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	goal, exists := repository.store.goals[id]
	if !exists {
		return learning.LearningGoal{}, notFound("get memory goal")
	}
	return cloneGoal(goal), nil
}

func (repository goalRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.LearningGoal, error) {
	if err := contextError("list memory goals", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	goals := make([]learning.LearningGoal, 0)
	for _, goal := range repository.store.goals {
		if goal.StudentID == studentID {
			goals = append(goals, cloneGoal(goal))
		}
	}
	sort.Slice(goals, func(i, j int) bool {
		if goals[i].CreatedAt == goals[j].CreatedAt {
			return goals[i].ID.String() < goals[j].ID.String()
		}
		return goals[i].CreatedAt.Before(goals[j].CreatedAt)
	})
	return goals, nil
}

func (repository goalRepository) Update(ctx context.Context, goal learning.LearningGoal) error {
	if err := contextError("update memory goal", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.goals[goal.ID]; !exists {
		return notFound("update memory goal")
	}
	if goal.Status == learning.GoalActive && repository.hasOtherActive(goal.StudentID, goal.ID) {
		return conflict("update memory goal")
	}
	repository.store.goals[goal.ID] = cloneGoal(goal)
	return nil
}

func (repository goalRepository) hasOtherActive(studentID, except learning.ID) bool {
	for id, goal := range repository.store.goals {
		if id != except && goal.StudentID == studentID && goal.Status == learning.GoalActive {
			return true
		}
	}
	return false
}

type curriculumRepository struct{ store *Store }

func (repository curriculumRepository) Concept(ctx context.Context, reference learning.CurriculumRef, conceptID learning.ID) (learning.Concept, error) {
	if err := contextError("get memory curriculum concept", ctx); err != nil {
		return learning.Concept{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	fixture, exists := repository.store.curricula[curriculumKey{id: reference.ID, version: reference.Version}]
	if !exists {
		return learning.Concept{}, notFound("get memory curriculum")
	}
	concept, exists := fixture.concepts[conceptID]
	if !exists {
		return learning.Concept{}, notFound("get memory curriculum concept")
	}
	return concept, nil
}

func (repository curriculumRepository) Concepts(ctx context.Context, reference learning.CurriculumRef) ([]learning.Concept, error) {
	if err := contextError("list memory curriculum concepts", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	fixture, exists := repository.store.curricula[curriculumKey{id: reference.ID, version: reference.Version}]
	if !exists {
		return nil, notFound("list memory curriculum concepts")
	}
	concepts := make([]learning.Concept, 0, len(fixture.concepts))
	for _, concept := range fixture.concepts {
		concepts = append(concepts, concept)
	}
	sort.Slice(concepts, func(i, j int) bool { return concepts[i].ID.String() < concepts[j].ID.String() })
	return concepts, nil
}

func (repository curriculumRepository) Outline(ctx context.Context, reference learning.CurriculumRef) ([]learning.CurriculumOutlineNode, error) {
	if err := contextError("list memory curriculum outline", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	fixture, exists := repository.store.curricula[curriculumKey{id: reference.ID, version: reference.Version}]
	if !exists {
		return nil, notFound("list memory curriculum outline")
	}
	return cloneCurriculumOutline(fixture.outline), nil
}

func cloneCurriculumOutline(source []learning.CurriculumOutlineNode) []learning.CurriculumOutlineNode {
	result := make([]learning.CurriculumOutlineNode, len(source))
	for index, node := range source {
		result[index] = node
		result[index].ParentID = cloneIDPointer(node.ParentID)
	}
	return result
}

func (repository curriculumRepository) PlanningConcepts(ctx context.Context, reference learning.CurriculumRef) ([]learning.DailyPlanCurriculumConcept, error) {
	if err := contextError("list memory curriculum planning concepts", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	fixture, exists := repository.store.curricula[curriculumKey{id: reference.ID, version: reference.Version}]
	if !exists {
		return nil, notFound("list memory curriculum planning concepts")
	}
	return clonePlanningConcepts(fixture.planning), nil
}

func clonePlanningConcepts(source []learning.DailyPlanCurriculumConcept) []learning.DailyPlanCurriculumConcept {
	result := make([]learning.DailyPlanCurriculumConcept, len(source))
	for index, concept := range source {
		result[index] = concept
		result[index].PrerequisiteIDs = append([]learning.ID(nil), concept.PrerequisiteIDs...)
	}
	return result
}

func (repository curriculumRepository) Prerequisites(ctx context.Context, reference learning.CurriculumRef, conceptID learning.ID) ([]learning.Prerequisite, error) {
	if err := contextError("list memory prerequisites", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	fixture, exists := repository.store.curricula[curriculumKey{id: reference.ID, version: reference.Version}]
	if !exists {
		return nil, notFound("list memory prerequisites")
	}
	prerequisites := make([]learning.Prerequisite, 0)
	for _, prerequisite := range fixture.prerequisites {
		if prerequisite.ConceptID == conceptID {
			prerequisites = append(prerequisites, prerequisite)
		}
	}
	sort.Slice(prerequisites, func(i, j int) bool {
		return prerequisites[i].RequiredConceptID.String() < prerequisites[j].RequiredConceptID.String()
	})
	return prerequisites, nil
}

func (repository curriculumRepository) ModuleForConcept(ctx context.Context, reference learning.CurriculumRef, conceptID learning.ID) (learning.ID, error) {
	if err := contextError("get memory concept module", ctx); err != nil {
		return learning.ID{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	fixture, exists := repository.store.curricula[curriculumKey{id: reference.ID, version: reference.Version}]
	if !exists {
		return learning.ID{}, notFound("get memory concept module")
	}
	moduleID, exists := fixture.modules[conceptID]
	if !exists {
		return learning.ID{}, notFound("get memory concept module")
	}
	return moduleID, nil
}

type conceptRepository struct{ store *Store }

func (repository conceptRepository) Get(ctx context.Context, studentID, conceptID learning.ID) (learning.ConceptState, error) {
	if err := contextError("get memory concept state", ctx); err != nil {
		return learning.ConceptState{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	state, exists := repository.store.concepts[studentConceptKey{student: studentID, concept: conceptID}]
	if !exists {
		return learning.ConceptState{}, notFound("get memory concept state")
	}
	return cloneConceptState(state), nil
}

func (repository conceptRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.ConceptState, error) {
	if err := contextError("list memory concept states", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	states := make([]learning.ConceptState, 0)
	for _, state := range repository.store.concepts {
		if state.StudentID == studentID {
			states = append(states, cloneConceptState(state))
		}
	}
	sort.Slice(states, func(i, j int) bool { return states[i].ConceptID.String() < states[j].ConceptID.String() })
	return states, nil
}

func (repository conceptRepository) Save(ctx context.Context, state learning.ConceptState) error {
	if err := contextError("save memory concept state", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.concepts[studentConceptKey{student: state.StudentID, concept: state.ConceptID}] = cloneConceptState(state)
	return nil
}

type evidenceRepository struct{ store *Store }

func (repository evidenceRepository) Append(ctx context.Context, evidence learning.Evidence) error {
	if err := contextError("append memory evidence", ctx); err != nil {
		return err
	}
	if err := evidence.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "append memory evidence", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.evidence[evidence.ID]; exists {
		return conflict("append memory evidence")
	}
	repository.store.evidence[evidence.ID] = evidence
	return nil
}

func (repository evidenceRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.Evidence, error) {
	if err := contextError("list memory student evidence", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.Evidence, 0)
	for _, evidence := range repository.store.evidence {
		if evidence.StudentID == studentID {
			items = append(items, evidence)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ConceptID != items[j].ConceptID {
			return items[i].ConceptID.String() < items[j].ConceptID.String()
		}
		if items[i].ObservedAt == items[j].ObservedAt {
			return items[i].ID.String() < items[j].ID.String()
		}
		return items[i].ObservedAt.Before(items[j].ObservedAt)
	})
	return items, nil
}

func (repository evidenceRepository) ListByConcept(ctx context.Context, studentID, conceptID learning.ID) ([]learning.Evidence, error) {
	if err := contextError("list memory evidence", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.Evidence, 0)
	for _, evidence := range repository.store.evidence {
		if evidence.StudentID == studentID && evidence.ConceptID == conceptID {
			items = append(items, evidence)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ObservedAt == items[j].ObservedAt {
			return items[i].ID.String() < items[j].ID.String()
		}
		return items[i].ObservedAt.Before(items[j].ObservedAt)
	})
	return items, nil
}

type mistakeRepository struct{ store *Store }

func (repository mistakeRepository) Create(ctx context.Context, mistake learning.Mistake) error {
	if err := contextError("create memory mistake", ctx); err != nil {
		return err
	}
	if err := mistake.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "create memory mistake", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.mistakes[mistake.ID]; exists {
		return conflict("create memory mistake")
	}
	if _, exists := repository.store.students[mistake.StudentID]; !exists {
		return application.Classify(application.ErrorInvalidState, "create memory mistake", errors.New("student does not exist"))
	}
	if !repository.conceptKnownLocked(mistake.ConceptID) {
		return application.Classify(application.ErrorInvalidState, "create memory mistake", errors.New("concept does not exist"))
	}
	for _, existing := range repository.store.mistakes {
		if existing.StudentID == mistake.StudentID && existing.ConceptID == mistake.ConceptID && existing.Key == mistake.Key {
			return conflict("create memory mistake")
		}
	}
	repository.store.mistakes[mistake.ID] = cloneMistake(mistake)
	return nil
}

func (repository mistakeRepository) Get(ctx context.Context, studentID, id learning.ID) (learning.Mistake, error) {
	if err := contextError("get memory mistake", ctx); err != nil {
		return learning.Mistake{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	mistake, exists := repository.store.mistakes[id]
	if !exists || mistake.StudentID != studentID {
		return learning.Mistake{}, notFound("get memory mistake")
	}
	return cloneMistake(mistake), nil
}

func (repository mistakeRepository) FindByKey(ctx context.Context, studentID, conceptID learning.ID, key learning.MistakeKey) (learning.Mistake, error) {
	if err := contextError("find memory mistake by key", ctx); err != nil {
		return learning.Mistake{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	for _, mistake := range repository.store.mistakes {
		if mistake.StudentID == studentID && mistake.ConceptID == conceptID && mistake.Key == key {
			return cloneMistake(mistake), nil
		}
	}
	return learning.Mistake{}, notFound("find memory mistake by key")
}

func (repository mistakeRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.Mistake, error) {
	if err := contextError("list memory student mistakes", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.Mistake, 0)
	for _, mistake := range repository.store.mistakes {
		if mistake.StudentID == studentID {
			items = append(items, cloneMistake(mistake))
		}
	}
	sortMistakes(items)
	return items, nil
}

func (repository mistakeRepository) ListByConcept(ctx context.Context, studentID, conceptID learning.ID) ([]learning.Mistake, error) {
	if err := contextError("list memory mistakes", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.Mistake, 0)
	for _, mistake := range repository.store.mistakes {
		if mistake.StudentID == studentID && mistake.ConceptID == conceptID {
			items = append(items, cloneMistake(mistake))
		}
	}
	sortMistakes(items)
	return items, nil
}

func (repository mistakeRepository) Update(ctx context.Context, mistake learning.Mistake) error {
	if err := contextError("update memory mistake", ctx); err != nil {
		return err
	}
	if err := mistake.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "update memory mistake", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	existing, exists := repository.store.mistakes[mistake.ID]
	if !exists {
		return notFound("update memory mistake")
	}
	if existing.StudentID != mistake.StudentID || existing.ConceptID != mistake.ConceptID || existing.Key != mistake.Key ||
		existing.Category != mistake.Category || existing.Summary != mistake.Summary || existing.FirstSeenAt != mistake.FirstSeenAt {
		return application.Classify(application.ErrorInvalidState, "update memory mistake", errors.New("mistake identity and classification are immutable"))
	}
	repository.store.mistakes[mistake.ID] = cloneMistake(mistake)
	return nil
}

func (repository mistakeRepository) AppendEvent(ctx context.Context, event learning.MistakeEvent) error {
	if err := contextError("append memory mistake event", ctx); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "append memory mistake event", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.mistakes[event.MistakeID]; !exists {
		return application.Classify(application.ErrorInvalidState, "append memory mistake event", errors.New("mistake does not exist"))
	}
	if _, exists := repository.store.mistakeEvents[event.ID]; exists {
		return conflict("append memory mistake event")
	}
	repository.store.mistakeEvents[event.ID] = event
	return nil
}

func (repository mistakeRepository) ListEvents(ctx context.Context, mistakeID learning.ID) ([]learning.MistakeEvent, error) {
	if err := contextError("list memory mistake events", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	if _, exists := repository.store.mistakes[mistakeID]; !exists {
		return nil, notFound("list memory mistake events")
	}
	items := make([]learning.MistakeEvent, 0)
	for _, event := range repository.store.mistakeEvents {
		if event.MistakeID == mistakeID {
			items = append(items, event)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].OccurredAt == items[j].OccurredAt {
			return items[i].ID.String() < items[j].ID.String()
		}
		return items[i].OccurredAt.Before(items[j].OccurredAt)
	})
	return items, nil
}

func (repository mistakeRepository) conceptKnownLocked(conceptID learning.ID) bool {
	for _, fixture := range repository.store.curricula {
		if _, exists := fixture.concepts[conceptID]; exists {
			return true
		}
	}
	return false
}

func sortMistakes(items []learning.Mistake) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].LastSeenAt == items[j].LastSeenAt {
			return items[i].ID.String() < items[j].ID.String()
		}
		return items[i].LastSeenAt.After(items[j].LastSeenAt)
	})
}

type retentionRepository struct{ store *Store }

func (repository retentionRepository) Get(ctx context.Context, studentID, conceptID learning.ID) (learning.RetentionState, error) {
	if err := contextError("get memory retention state", ctx); err != nil {
		return learning.RetentionState{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	state, exists := repository.store.retention[studentConceptKey{student: studentID, concept: conceptID}]
	if !exists {
		return learning.RetentionState{}, notFound("get memory retention state")
	}
	return cloneRetentionState(state), nil
}

func (repository retentionRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.RetentionState, error) {
	if err := contextError("list memory retention states", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	states := make([]learning.RetentionState, 0)
	for key, state := range repository.store.retention {
		if key.student == studentID {
			states = append(states, cloneRetentionState(state))
		}
	}
	sort.Slice(states, func(i, j int) bool { return states[i].ConceptID.String() < states[j].ConceptID.String() })
	return states, nil
}

func (repository retentionRepository) Save(ctx context.Context, state learning.RetentionState) error {
	if err := contextError("save memory retention state", ctx); err != nil {
		return err
	}
	if err := state.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "save memory retention state", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.retention[studentConceptKey{student: state.StudentID, concept: state.ConceptID}] = cloneRetentionState(state)
	return nil
}

type sessionRepository struct{ store *Store }

func (repository sessionRepository) Append(ctx context.Context, session learning.LearningSession) error {
	if err := contextError("append memory session", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.sessions[session.ID]; exists {
		return conflict("append memory session")
	}
	repository.store.sessions[session.ID] = cloneSession(session)
	return nil
}

func (repository sessionRepository) Get(ctx context.Context, id learning.ID) (learning.LearningSession, error) {
	if err := contextError("get memory session", ctx); err != nil {
		return learning.LearningSession{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	session, exists := repository.store.sessions[id]
	if !exists {
		return learning.LearningSession{}, notFound("get memory session")
	}
	return cloneSession(session), nil
}

func (repository sessionRepository) ListByGoal(ctx context.Context, studentID, goalID learning.ID) ([]learning.LearningSession, error) {
	if err := contextError("list memory sessions", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	sessions := make([]learning.LearningSession, 0)
	for _, session := range repository.store.sessions {
		if session.StudentID == studentID && session.GoalID == goalID {
			sessions = append(sessions, cloneSession(session))
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].StartedAt == sessions[j].StartedAt {
			return sessions[i].ID.String() < sessions[j].ID.String()
		}
		return sessions[i].StartedAt.Before(sessions[j].StartedAt)
	})
	return sessions, nil
}

type studySessionRepository struct{ store *Store }

func (repository studySessionRepository) Create(ctx context.Context, session learning.StudySession) error {
	if err := contextError("create memory study session", ctx); err != nil {
		return err
	}
	if err := session.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "create memory study session", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.studySessions[session.ID]; exists || repository.activeLocked(session.StudentID) != nil {
		return conflict("create memory study session")
	}
	repository.store.studySessions[session.ID] = cloneStudySession(session)
	return nil
}

func (repository studySessionRepository) Get(ctx context.Context, id learning.ID) (learning.StudySession, error) {
	if err := contextError("get memory study session", ctx); err != nil {
		return learning.StudySession{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	session, exists := repository.store.studySessions[id]
	if !exists {
		return learning.StudySession{}, notFound("get memory study session")
	}
	return cloneStudySession(session), nil
}

func (repository studySessionRepository) ActiveByStudent(ctx context.Context, studentID learning.ID) (learning.StudySession, error) {
	if err := contextError("get active memory study session", ctx); err != nil {
		return learning.StudySession{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	if active := repository.activeLocked(studentID); active != nil {
		return cloneStudySession(*active), nil
	}
	return learning.StudySession{}, notFound("get active memory study session")
}

func (repository studySessionRepository) ListByGoal(ctx context.Context, studentID, goalID learning.ID) ([]learning.StudySession, error) {
	if err := contextError("list memory study sessions", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	sessions := make([]learning.StudySession, 0)
	for _, session := range repository.store.studySessions {
		if session.StudentID == studentID && session.GoalID == goalID {
			sessions = append(sessions, cloneStudySession(session))
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].StartedAt == sessions[j].StartedAt {
			return sessions[i].ID.String() < sessions[j].ID.String()
		}
		return sessions[i].StartedAt.Before(sessions[j].StartedAt)
	})
	return sessions, nil
}

func (repository studySessionRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.StudySession, error) {
	if err := contextError("list memory study sessions by student", ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	sessions := make([]learning.StudySession, 0)
	for _, session := range repository.store.studySessions {
		if session.StudentID == studentID {
			sessions = append(sessions, cloneStudySession(session))
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].StartedAt == sessions[j].StartedAt {
			return sessions[i].ID.String() < sessions[j].ID.String()
		}
		return sessions[i].StartedAt.Before(sessions[j].StartedAt)
	})
	return sessions, nil
}

func (repository studySessionRepository) Update(ctx context.Context, session learning.StudySession) error {
	if err := contextError("update memory study session", ctx); err != nil {
		return err
	}
	if err := session.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "update memory study session", err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.studySessions[session.ID]; !exists {
		return notFound("update memory study session")
	}
	if session.Status == learning.StudySessionActive {
		for id, candidate := range repository.store.studySessions {
			if id != session.ID && candidate.StudentID == session.StudentID && candidate.Status == learning.StudySessionActive {
				return conflict("update memory study session")
			}
		}
	}
	repository.store.studySessions[session.ID] = cloneStudySession(session)
	return nil
}

func (repository studySessionRepository) activeLocked(studentID learning.ID) *learning.StudySession {
	for _, session := range repository.store.studySessions {
		if session.StudentID == studentID && session.Status == learning.StudySessionActive {
			copy := session
			return &copy
		}
	}
	return nil
}
