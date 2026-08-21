package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

func (repository learningGoalRepository) Create(ctx context.Context, goal learning.LearningGoal) error {
	const operation = "create SQLite learning goal"
	if err := goal.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO learning_goals
(id, student_id, title, description, domain, target_outcome, starting_level, status, mastery_threshold,
 created_at, updated_at, activated_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		goal.ID.String(), goal.StudentID.String(), goal.Title, goal.Description, goal.Domain, goal.TargetOutcome,
		goal.StartingLevel, goal.Status, goal.MasteryThreshold.Value(), encodeTimestamp(goal.CreatedAt),
		encodeTimestamp(goal.UpdatedAt), encodeOptionalTimestamp(goal.ActivatedAt), encodeOptionalTimestamp(goal.CompletedAt))
	return classifyLearningError(operation, err)
}

func (repository learningGoalRepository) Get(ctx context.Context, id learning.ID) (learning.LearningGoal, error) {
	const operation = "get SQLite learning goal"
	if err := id.Validate(); err != nil {
		return learning.LearningGoal{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	goal, err := scanGoal(repository.executor.QueryRowContext(operationContext, `SELECT id, student_id, title, description, domain,
target_outcome, starting_level, status, mastery_threshold, created_at, updated_at, activated_at, completed_at
FROM learning_goals WHERE id = ?`, id.String()))
	if err != nil {
		return learning.LearningGoal{}, classifyLearningError(operation, err)
	}
	return goal, nil
}

func (repository learningGoalRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.LearningGoal, error) {
	const operation = "list SQLite learning goals"
	if err := studentID.Validate(); err != nil {
		return nil, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT id, student_id, title, description, domain, target_outcome,
starting_level, status, mastery_threshold, created_at, updated_at, activated_at, completed_at
FROM learning_goals WHERE student_id = ? ORDER BY created_at, id`, studentID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	goals := make([]learning.LearningGoal, 0)
	for rows.Next() {
		goal, err := scanGoal(rows)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		goals = append(goals, goal)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return goals, nil
}

func (repository learningGoalRepository) Update(ctx context.Context, goal learning.LearningGoal) error {
	const operation = "update SQLite learning goal"
	if err := goal.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	result, err := repository.executor.ExecContext(operationContext, `UPDATE learning_goals SET student_id = ?, title = ?, description = ?,
domain = ?, target_outcome = ?, starting_level = ?, status = ?, mastery_threshold = ?, created_at = ?, updated_at = ?,
activated_at = ?, completed_at = ? WHERE id = ?`, goal.StudentID.String(), goal.Title, goal.Description, goal.Domain,
		goal.TargetOutcome, goal.StartingLevel, goal.Status, goal.MasteryThreshold.Value(), encodeTimestamp(goal.CreatedAt),
		encodeTimestamp(goal.UpdatedAt), encodeOptionalTimestamp(goal.ActivatedAt), encodeOptionalTimestamp(goal.CompletedAt), goal.ID.String())
	if err == nil {
		err = requireAffected(result)
	}
	return classifyLearningError(operation, err)
}

type rowScanner interface{ Scan(...any) error }

func scanGoal(scanner rowScanner) (learning.LearningGoal, error) {
	var idValue, studentValue, title, description, domain, targetOutcome, startingLevel, status, createdValue, updatedValue string
	var thresholdValue float64
	var activatedValue, completedValue sql.NullString
	if err := scanner.Scan(&idValue, &studentValue, &title, &description, &domain, &targetOutcome, &startingLevel,
		&status, &thresholdValue, &createdValue, &updatedValue, &activatedValue, &completedValue); err != nil {
		return learning.LearningGoal{}, err
	}
	id, err := decodeID(idValue)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	studentID, err := decodeID(studentValue)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	threshold, err := learning.NewMasteryThreshold(thresholdValue)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	createdAt, err := decodeTimestamp(createdValue)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	updatedAt, err := decodeTimestamp(updatedValue)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	activatedAt, err := decodeOptionalTimestamp(activatedValue)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	completedAt, err := decodeOptionalTimestamp(completedValue)
	if err != nil {
		return learning.LearningGoal{}, err
	}
	goal := learning.LearningGoal{
		ID: id, StudentID: studentID, Title: title, Description: description, Domain: domain,
		TargetOutcome: targetOutcome, StartingLevel: learning.ExperienceLevel(startingLevel), Status: learning.GoalStatus(status),
		MasteryThreshold: threshold, CreatedAt: createdAt, UpdatedAt: updatedAt,
		ActivatedAt: activatedAt, CompletedAt: completedAt,
	}
	return goal, goal.Validate()
}

func (repository learningCurriculumRepository) Concept(ctx context.Context, reference learning.CurriculumRef, conceptID learning.ID) (learning.Concept, error) {
	const operation = "get SQLite curriculum concept"
	if err := reference.Validate(); err != nil {
		return learning.Concept{}, invalidLearning(operation, err)
	}
	if err := conceptID.Validate(); err != nil {
		return learning.Concept{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var idValue, topicValue, title string
	err := repository.executor.QueryRowContext(operationContext, `SELECT concept_id, parent_node_id, title FROM curriculum_nodes
WHERE curriculum_id = ? AND curriculum_version = ? AND concept_id = ? AND node_type = 'concept'`,
		reference.ID.String(), reference.Version, conceptID.String()).Scan(&idValue, &topicValue, &title)
	if err != nil {
		return learning.Concept{}, classifyLearningError(operation, err)
	}
	concept, err := decodeConcept(idValue, topicValue, title)
	if err != nil {
		return learning.Concept{}, corruptLearning(operation, err)
	}
	return concept, nil
}

func (repository learningCurriculumRepository) Concepts(ctx context.Context, reference learning.CurriculumRef) ([]learning.Concept, error) {
	const operation = "list SQLite curriculum concepts"
	if err := reference.Validate(); err != nil {
		return nil, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	if err := requireCurriculum(operationContext, repository.executor, reference); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	rows, err := repository.executor.QueryContext(operationContext, `SELECT concept_id, parent_node_id, title FROM curriculum_nodes
WHERE curriculum_id = ? AND curriculum_version = ? AND node_type = 'concept' ORDER BY concept_id`, reference.ID.String(), reference.Version)
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	concepts := make([]learning.Concept, 0)
	for rows.Next() {
		var idValue, topicValue, title string
		if err := rows.Scan(&idValue, &topicValue, &title); err != nil {
			return nil, corruptLearning(operation, err)
		}
		concept, err := decodeConcept(idValue, topicValue, title)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		concepts = append(concepts, concept)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return concepts, nil
}

func (repository learningCurriculumRepository) Prerequisites(ctx context.Context, reference learning.CurriculumRef, conceptID learning.ID) ([]learning.Prerequisite, error) {
	const operation = "list SQLite curriculum prerequisites"
	if err := reference.Validate(); err != nil {
		return nil, invalidLearning(operation, err)
	}
	if err := conceptID.Validate(); err != nil {
		return nil, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	if err := requireCurriculum(operationContext, repository.executor, reference); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	rows, err := repository.executor.QueryContext(operationContext, `SELECT concept_id, required_concept_id FROM curriculum_edges
WHERE curriculum_id = ? AND curriculum_version = ? AND concept_id = ? ORDER BY required_concept_id`, reference.ID.String(), reference.Version, conceptID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	items := make([]learning.Prerequisite, 0)
	for rows.Next() {
		var conceptValue, requiredValue string
		if err := rows.Scan(&conceptValue, &requiredValue); err != nil {
			return nil, corruptLearning(operation, err)
		}
		concept, err := decodeID(conceptValue)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		required, err := decodeID(requiredValue)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		item, err := learning.NewPrerequisite(concept, required)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return items, nil
}

func (repository learningCurriculumRepository) ModuleForConcept(ctx context.Context, reference learning.CurriculumRef, conceptID learning.ID) (learning.ID, error) {
	const operation = "get SQLite curriculum module for concept"
	if err := reference.Validate(); err != nil {
		return learning.ID{}, invalidLearning(operation, err)
	}
	if err := conceptID.Validate(); err != nil {
		return learning.ID{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var moduleValue string
	err := repository.executor.QueryRowContext(operationContext, `SELECT module.node_id
FROM curriculum_nodes AS concept
JOIN curriculum_nodes AS topic
  ON topic.curriculum_id=concept.curriculum_id AND topic.curriculum_version=concept.curriculum_version
 AND topic.node_id=concept.parent_node_id AND topic.node_type='topic'
JOIN curriculum_nodes AS lesson
  ON lesson.curriculum_id=topic.curriculum_id AND lesson.curriculum_version=topic.curriculum_version
 AND lesson.node_id=topic.parent_node_id AND lesson.node_type='lesson'
JOIN curriculum_nodes AS module
  ON module.curriculum_id=lesson.curriculum_id AND module.curriculum_version=lesson.curriculum_version
 AND module.node_id=lesson.parent_node_id AND module.node_type='module'
WHERE concept.curriculum_id=? AND concept.curriculum_version=? AND concept.concept_id=? AND concept.node_type='concept'`,
		reference.ID.String(), reference.Version, conceptID.String()).Scan(&moduleValue)
	if err != nil {
		return learning.ID{}, classifyLearningError(operation, err)
	}
	moduleID, err := decodeID(moduleValue)
	if err != nil {
		return learning.ID{}, corruptLearning(operation, err)
	}
	return moduleID, nil
}

func requireCurriculum(ctx context.Context, target executor, reference learning.CurriculumRef) error {
	var present int
	return target.QueryRowContext(ctx, "SELECT 1 FROM curriculum_instances WHERE id = ? AND version = ?", reference.ID.String(), reference.Version).Scan(&present)
}

func decodeConcept(idValue, topicValue, title string) (learning.Concept, error) {
	id, err := decodeID(idValue)
	if err != nil {
		return learning.Concept{}, err
	}
	topicID, err := decodeID(topicValue)
	if err != nil {
		return learning.Concept{}, err
	}
	concept := learning.Concept{ID: id, TopicID: topicID, Title: title}
	return concept, concept.Validate()
}

func (repository learningConceptRepository) Get(ctx context.Context, studentID, conceptID learning.ID) (learning.ConceptState, error) {
	const operation = "get SQLite concept state"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	state, err := scanConceptState(repository.executor.QueryRowContext(operationContext, `SELECT student_id, concept_id, exposure, mastery, introduced_at, updated_at
FROM student_concept_states WHERE student_id = ? AND concept_id = ?`, studentID.String(), conceptID.String()))
	if err != nil {
		return learning.ConceptState{}, classifyLearningError(operation, err)
	}
	return state, nil
}

func (repository learningConceptRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.ConceptState, error) {
	const operation = "list SQLite concept states"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT student_id, concept_id, exposure, mastery, introduced_at, updated_at
FROM student_concept_states WHERE student_id = ? ORDER BY concept_id`, studentID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	items := make([]learning.ConceptState, 0)
	for rows.Next() {
		item, err := scanConceptState(rows)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return items, nil
}

func (repository learningConceptRepository) Save(ctx context.Context, state learning.ConceptState) error {
	const operation = "save SQLite concept state"
	if err := state.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO student_concept_states
(student_id, concept_id, exposure, mastery, introduced_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(student_id, concept_id) DO UPDATE SET exposure=excluded.exposure, mastery=excluded.mastery, introduced_at=excluded.introduced_at, updated_at=excluded.updated_at`,
		state.StudentID.String(), state.ConceptID.String(), state.Exposure, state.Mastery.Value(), encodeOptionalTimestamp(state.IntroducedAt), encodeTimestamp(state.UpdatedAt))
	return classifyLearningError(operation, err)
}

func scanConceptState(scanner rowScanner) (learning.ConceptState, error) {
	var studentValue, conceptValue, exposure, updatedValue string
	var masteryValue float64
	var introducedValue sql.NullString
	if err := scanner.Scan(&studentValue, &conceptValue, &exposure, &masteryValue, &introducedValue, &updatedValue); err != nil {
		return learning.ConceptState{}, err
	}
	studentID, err := decodeID(studentValue)
	if err != nil {
		return learning.ConceptState{}, err
	}
	conceptID, err := decodeID(conceptValue)
	if err != nil {
		return learning.ConceptState{}, err
	}
	mastery, err := learning.NewMasteryScore(masteryValue)
	if err != nil {
		return learning.ConceptState{}, err
	}
	introducedAt, err := decodeOptionalTimestamp(introducedValue)
	if err != nil {
		return learning.ConceptState{}, err
	}
	updatedAt, err := decodeTimestamp(updatedValue)
	if err != nil {
		return learning.ConceptState{}, err
	}
	state := learning.ConceptState{StudentID: studentID, ConceptID: conceptID, Exposure: learning.ExposureState(exposure), Mastery: mastery, IntroducedAt: introducedAt, UpdatedAt: updatedAt}
	return state, state.Validate()
}

func (repository learningEvidenceRepository) Append(ctx context.Context, evidence learning.Evidence) error {
	const operation = "append SQLite evidence"
	if err := evidence.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO learning_evidence
(id, student_id, concept_id, evidence_type, source, score, observed_at, mastery_evidence_type, confidence, independence, difficulty, algorithm_version)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, evidence.ID.String(), evidence.StudentID.String(), evidence.ConceptID.String(),
		storageEvidenceCategory(evidence.Type), evidence.Source, evidence.Score.Value(), encodeTimestamp(evidence.ObservedAt), evidence.Type,
		evidence.Confidence, evidence.Independence, evidence.Difficulty, evidence.AlgorithmVersion)
	return classifyLearningError(operation, err)
}

func (repository learningEvidenceRepository) ListByConcept(ctx context.Context, studentID, conceptID learning.ID) ([]learning.Evidence, error) {
	const operation = "list SQLite evidence"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT id, student_id, concept_id, mastery_evidence_type, source, score, observed_at, confidence, independence, difficulty, algorithm_version FROM learning_evidence
WHERE student_id = ? AND concept_id = ? ORDER BY observed_at, id`, studentID.String(), conceptID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	items := make([]learning.Evidence, 0)
	for rows.Next() {
		item, err := scanEvidence(rows)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return items, nil
}

func scanEvidence(scanner rowScanner) (learning.Evidence, error) {
	var idValue, studentValue, conceptValue, kind, source, observedValue, algorithmVersion string
	var scoreValue, confidence, independence, difficulty float64
	if err := scanner.Scan(&idValue, &studentValue, &conceptValue, &kind, &source, &scoreValue, &observedValue, &confidence, &independence, &difficulty, &algorithmVersion); err != nil {
		return learning.Evidence{}, err
	}
	id, err := decodeID(idValue)
	if err != nil {
		return learning.Evidence{}, err
	}
	studentID, err := decodeID(studentValue)
	if err != nil {
		return learning.Evidence{}, err
	}
	conceptID, err := decodeID(conceptValue)
	if err != nil {
		return learning.Evidence{}, err
	}
	score, err := learning.NewMasteryScore(scoreValue)
	if err != nil {
		return learning.Evidence{}, err
	}
	observedAt, err := decodeTimestamp(observedValue)
	if err != nil {
		return learning.Evidence{}, err
	}
	item := learning.Evidence{
		ID: id, StudentID: studentID, ConceptID: conceptID, Type: learning.EvidenceType(kind), Source: source, Score: score,
		Confidence: confidence, Independence: independence, Difficulty: difficulty, ObservedAt: observedAt, AlgorithmVersion: algorithmVersion,
	}
	return item, item.Validate()
}

func storageEvidenceCategory(evidenceType learning.EvidenceType) learning.EvidenceType {
	switch evidenceType {
	case learning.EvidenceDiagnosticObjective, learning.EvidenceDiagnosticSelfReport:
		return "diagnostic"
	case learning.EvidenceKnowledgeCheck, learning.EvidencePracticeSuccess, learning.EvidencePracticeFailure:
		return "practice"
	case learning.EvidenceAssessment:
		return "assessment"
	case learning.EvidenceReviewRecall:
		return "review"
	case learning.EvidenceProject:
		return "observation"
	case learning.EvidenceManualImport:
		return "import"
	default:
		return evidenceType
	}
}

func (repository learningMistakeRepository) Create(ctx context.Context, mistake learning.Mistake) error {
	const operation = "create SQLite mistake"
	if err := mistake.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO mistakes
(id, student_id, concept_id, description, occurred_at, resolved_at, mistake_key, category, summary,
 first_seen_at, last_seen_at, occurrences, status, source_ref)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, mistake.ID.String(), mistake.StudentID.String(), mistake.ConceptID.String(),
		mistake.Summary, encodeTimestamp(mistake.FirstSeenAt), encodeOptionalTimestamp(mistake.ResolvedAt), string(mistake.Key), string(mistake.Category), mistake.Summary,
		encodeTimestamp(mistake.FirstSeenAt), encodeTimestamp(mistake.LastSeenAt), mistake.Occurrences,
		string(mistake.Status), mistake.SourceRef)
	return classifyLearningError(operation, err)
}

func (repository learningMistakeRepository) Get(ctx context.Context, studentID, id learning.ID) (learning.Mistake, error) {
	const operation = "get SQLite mistake"
	if err := validateMistakeIDs("student", studentID, "mistake", id); err != nil {
		return learning.Mistake{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	mistake, err := scanMistake(repository.executor.QueryRowContext(operationContext, mistakeSelect+" WHERE student_id = ? AND id = ?", studentID.String(), id.String()))
	if err != nil {
		return learning.Mistake{}, classifyLearningError(operation, err)
	}
	return mistake, nil
}

func (repository learningMistakeRepository) FindByKey(ctx context.Context, studentID, conceptID learning.ID, key learning.MistakeKey) (learning.Mistake, error) {
	const operation = "find SQLite mistake by key"
	if err := validateMistakeIDs("student", studentID, "concept", conceptID); err != nil {
		return learning.Mistake{}, invalidLearning(operation, err)
	}
	if err := key.Validate(); err != nil {
		return learning.Mistake{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	mistake, err := scanMistake(repository.executor.QueryRowContext(operationContext, mistakeSelect+" WHERE student_id = ? AND concept_id = ? AND mistake_key = ?", studentID.String(), conceptID.String(), string(key)))
	if err != nil {
		return learning.Mistake{}, classifyLearningError(operation, err)
	}
	return mistake, nil
}

func (repository learningMistakeRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.Mistake, error) {
	const operation = "list SQLite student mistakes"
	if err := studentID.Validate(); err != nil {
		return nil, invalidLearning(operation, err)
	}
	return repository.listMistakes(ctx, operation, mistakeSelect+" WHERE student_id = ? ORDER BY last_seen_at DESC, id", studentID.String())
}

func (repository learningMistakeRepository) ListByConcept(ctx context.Context, studentID, conceptID learning.ID) ([]learning.Mistake, error) {
	const operation = "list SQLite mistakes"
	if err := validateMistakeIDs("student", studentID, "concept", conceptID); err != nil {
		return nil, invalidLearning(operation, err)
	}
	return repository.listMistakes(ctx, operation, mistakeSelect+" WHERE student_id = ? AND concept_id = ? ORDER BY last_seen_at DESC, id", studentID.String(), conceptID.String())
}

const mistakeSelect = `SELECT id, student_id, concept_id, mistake_key, category, summary,
first_seen_at, last_seen_at, occurrences, status, source_ref, resolved_at FROM mistakes`

func (repository learningMistakeRepository) listMistakes(ctx context.Context, operation, query string, arguments ...any) ([]learning.Mistake, error) {
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, query, arguments...)
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	items := make([]learning.Mistake, 0)
	for rows.Next() {
		item, err := scanMistake(rows)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return items, nil
}

func (repository learningMistakeRepository) Update(ctx context.Context, mistake learning.Mistake) error {
	const operation = "update SQLite mistake"
	if err := mistake.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	result, err := repository.executor.ExecContext(operationContext, `UPDATE mistakes SET
resolved_at=?, last_seen_at=?, occurrences=?, status=?, source_ref=?
WHERE id=? AND student_id=? AND concept_id=? AND mistake_key=? AND category=? AND summary=? AND first_seen_at=?`,
		encodeOptionalTimestamp(mistake.ResolvedAt), encodeTimestamp(mistake.LastSeenAt), mistake.Occurrences, string(mistake.Status), mistake.SourceRef,
		mistake.ID.String(), mistake.StudentID.String(), mistake.ConceptID.String(), string(mistake.Key), string(mistake.Category), mistake.Summary,
		encodeTimestamp(mistake.FirstSeenAt))
	if err == nil {
		err = requireAffected(result)
	}
	return classifyLearningError(operation, err)
}

func scanMistake(scanner rowScanner) (learning.Mistake, error) {
	var idValue, studentValue, conceptValue, keyValue, categoryValue, summary, firstSeenValue, lastSeenValue, statusValue, sourceRef string
	var occurrences int
	var resolvedValue sql.NullString
	if err := scanner.Scan(&idValue, &studentValue, &conceptValue, &keyValue, &categoryValue, &summary, &firstSeenValue,
		&lastSeenValue, &occurrences, &statusValue, &sourceRef, &resolvedValue); err != nil {
		return learning.Mistake{}, err
	}
	id, err := decodeID(idValue)
	if err != nil {
		return learning.Mistake{}, err
	}
	studentID, err := decodeID(studentValue)
	if err != nil {
		return learning.Mistake{}, err
	}
	conceptID, err := decodeID(conceptValue)
	if err != nil {
		return learning.Mistake{}, err
	}
	firstSeenAt, err := decodeTimestamp(firstSeenValue)
	if err != nil {
		return learning.Mistake{}, err
	}
	lastSeenAt, err := decodeTimestamp(lastSeenValue)
	if err != nil {
		return learning.Mistake{}, err
	}
	resolvedAt, err := decodeOptionalTimestamp(resolvedValue)
	if err != nil {
		return learning.Mistake{}, err
	}
	item := learning.Mistake{
		ID: id, StudentID: studentID, ConceptID: conceptID, Key: learning.MistakeKey(keyValue),
		Category: learning.MistakeCategory(categoryValue), Summary: summary, FirstSeenAt: firstSeenAt,
		LastSeenAt: lastSeenAt, Occurrences: occurrences, Status: learning.MistakeStatus(statusValue),
		SourceRef: sourceRef, ResolvedAt: resolvedAt,
	}
	return item, item.Validate()
}

func (repository learningMistakeRepository) AppendEvent(ctx context.Context, event learning.MistakeEvent) error {
	const operation = "append SQLite mistake event"
	if err := event.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO mistake_events
(id, mistake_id, event_type, occurred_at, source_ref) VALUES (?, ?, ?, ?, ?)`, event.ID.String(), event.MistakeID.String(),
		string(event.Type), encodeTimestamp(event.OccurredAt), event.SourceRef)
	return classifyLearningError(operation, err)
}

func (repository learningMistakeRepository) ListEvents(ctx context.Context, mistakeID learning.ID) ([]learning.MistakeEvent, error) {
	const operation = "list SQLite mistake events"
	if err := mistakeID.Validate(); err != nil {
		return nil, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var present int
	if err := repository.executor.QueryRowContext(operationContext, "SELECT 1 FROM mistakes WHERE id = ?", mistakeID.String()).Scan(&present); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	rows, err := repository.executor.QueryContext(operationContext, `SELECT id, mistake_id, event_type, occurred_at, source_ref
FROM mistake_events WHERE mistake_id = ? ORDER BY occurred_at, id`, mistakeID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	items := make([]learning.MistakeEvent, 0)
	for rows.Next() {
		var idValue, ownerValue, typeValue, occurredValue, sourceRef string
		if err := rows.Scan(&idValue, &ownerValue, &typeValue, &occurredValue, &sourceRef); err != nil {
			return nil, corruptLearning(operation, err)
		}
		id, err := decodeID(idValue)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		owner, err := decodeID(ownerValue)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		occurredAt, err := decodeTimestamp(occurredValue)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		event, err := learning.NewMistakeEvent(id, owner, learning.MistakeEventType(typeValue), occurredAt, sourceRef)
		if err != nil {
			return nil, corruptLearning(operation, err)
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return items, nil
}

func validateMistakeIDs(firstName string, first learning.ID, secondName string, second learning.ID) error {
	if err := first.Validate(); err != nil {
		return fmt.Errorf("%s: %w", firstName, err)
	}
	if err := second.Validate(); err != nil {
		return fmt.Errorf("%s: %w", secondName, err)
	}
	return nil
}

func (repository learningRetentionRepository) Get(ctx context.Context, studentID, conceptID learning.ID) (learning.RetentionState, error) {
	const operation = "get SQLite retention state"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var studentValue, conceptValue, measuredValue, statusValue, algorithmVersion string
	var lastSuccessfulValue, lastPracticeValue, nextDueValue sql.NullString
	var strengthValue float64
	var reviewCount, successfulReviews, failedReviews int
	var stabilitySeconds int64
	err := repository.executor.QueryRowContext(operationContext, `SELECT student_id,concept_id,last_successful_recall,last_practice,
review_count,successful_reviews,failed_reviews,stability_estimate_seconds,strength,retention_status,next_due_at,measured_at,algorithm_version
FROM retention_state WHERE student_id=? AND concept_id=?`, studentID.String(), conceptID.String()).Scan(
		&studentValue, &conceptValue, &lastSuccessfulValue, &lastPracticeValue, &reviewCount, &successfulReviews,
		&failedReviews, &stabilitySeconds, &strengthValue, &statusValue, &nextDueValue, &measuredValue, &algorithmVersion)
	if err != nil {
		return learning.RetentionState{}, classifyLearningError(operation, err)
	}
	student, err := decodeID(studentValue)
	if err != nil {
		return learning.RetentionState{}, corruptLearning(operation, err)
	}
	concept, err := decodeID(conceptValue)
	if err != nil {
		return learning.RetentionState{}, corruptLearning(operation, err)
	}
	strength, err := learning.NewMasteryScore(strengthValue)
	if err != nil {
		return learning.RetentionState{}, corruptLearning(operation, err)
	}
	measured, err := decodeTimestamp(measuredValue)
	if err != nil {
		return learning.RetentionState{}, corruptLearning(operation, err)
	}
	lastSuccessful, err := decodeOptionalTimestamp(lastSuccessfulValue)
	if err != nil {
		return learning.RetentionState{}, corruptLearning(operation, err)
	}
	lastPractice, err := decodeOptionalTimestamp(lastPracticeValue)
	if err != nil {
		return learning.RetentionState{}, corruptLearning(operation, err)
	}
	nextDue, err := decodeOptionalTimestamp(nextDueValue)
	if err != nil {
		return learning.RetentionState{}, corruptLearning(operation, err)
	}
	if stabilitySeconds < 0 || stabilitySeconds > int64(90*24*time.Hour/time.Second) {
		return learning.RetentionState{}, corruptLearning(operation, fmt.Errorf("retention stability seconds are invalid"))
	}
	state := learning.RetentionState{
		StudentID: student, ConceptID: concept, LastSuccessfulRecall: lastSuccessful, LastPractice: lastPractice,
		ReviewCount: reviewCount, SuccessfulReviews: successfulReviews, FailedReviews: failedReviews,
		StabilityEstimate: time.Duration(stabilitySeconds) * time.Second, Strength: strength,
		Status: learning.RetentionStatus(statusValue), NextDueAt: nextDue, MeasuredAt: measured, AlgorithmVersion: algorithmVersion,
	}
	if err := state.Validate(); err != nil {
		return learning.RetentionState{}, corruptLearning(operation, err)
	}
	return state, nil
}

func (repository learningRetentionRepository) Save(ctx context.Context, state learning.RetentionState) error {
	const operation = "save SQLite retention state"
	if err := state.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO retention_state
(student_id,concept_id,last_successful_recall,last_practice,review_count,successful_reviews,failed_reviews,
 stability_estimate_seconds,strength,retention_status,next_due_at,measured_at,algorithm_version)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(student_id,concept_id) DO UPDATE SET
last_successful_recall=excluded.last_successful_recall,last_practice=excluded.last_practice,
review_count=excluded.review_count,successful_reviews=excluded.successful_reviews,failed_reviews=excluded.failed_reviews,
stability_estimate_seconds=excluded.stability_estimate_seconds,strength=excluded.strength,
retention_status=excluded.retention_status,next_due_at=excluded.next_due_at,measured_at=excluded.measured_at,
algorithm_version=excluded.algorithm_version`,
		state.StudentID.String(), state.ConceptID.String(), encodeOptionalTimestamp(state.LastSuccessfulRecall),
		encodeOptionalTimestamp(state.LastPractice), state.ReviewCount, state.SuccessfulReviews, state.FailedReviews,
		int64(state.StabilityEstimate/time.Second), state.Strength.Value(), state.Status, encodeOptionalTimestamp(state.NextDueAt),
		encodeTimestamp(state.MeasuredAt), state.AlgorithmVersion)
	return classifyLearningError(operation, err)
}
