package sqlite

import (
	"context"
	"database/sql"

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
(id, student_id, concept_id, evidence_type, source, score, observed_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, evidence.ID.String(), evidence.StudentID.String(), evidence.ConceptID.String(), evidence.Type, evidence.Source, evidence.Score.Value(), encodeTimestamp(evidence.ObservedAt))
	return classifyLearningError(operation, err)
}

func (repository learningEvidenceRepository) ListByConcept(ctx context.Context, studentID, conceptID learning.ID) ([]learning.Evidence, error) {
	const operation = "list SQLite evidence"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT id, student_id, concept_id, evidence_type, source, score, observed_at FROM learning_evidence
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
	var idValue, studentValue, conceptValue, kind, source, observedValue string
	var scoreValue float64
	if err := scanner.Scan(&idValue, &studentValue, &conceptValue, &kind, &source, &scoreValue, &observedValue); err != nil {
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
	item := learning.Evidence{ID: id, StudentID: studentID, ConceptID: conceptID, Type: learning.EvidenceType(kind), Source: source, Score: score, ObservedAt: observedAt}
	return item, item.Validate()
}

func (repository learningMistakeRepository) Create(ctx context.Context, mistake learning.Mistake) error {
	const operation = "create SQLite mistake"
	if err := mistake.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO mistakes (id, student_id, concept_id, description, occurred_at, resolved_at) VALUES (?, ?, ?, ?, ?, ?)`, mistake.ID.String(), mistake.StudentID.String(), mistake.ConceptID.String(), mistake.Description, encodeTimestamp(mistake.OccurredAt), encodeOptionalTimestamp(mistake.ResolvedAt))
	return classifyLearningError(operation, err)
}

func (repository learningMistakeRepository) ListByConcept(ctx context.Context, studentID, conceptID learning.ID) ([]learning.Mistake, error) {
	const operation = "list SQLite mistakes"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT id, student_id, concept_id, description, occurred_at, resolved_at FROM mistakes WHERE student_id = ? AND concept_id = ? ORDER BY occurred_at, id`, studentID.String(), conceptID.String())
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
	result, err := repository.executor.ExecContext(operationContext, `UPDATE mistakes SET student_id=?, concept_id=?, description=?, occurred_at=?, resolved_at=? WHERE id=?`, mistake.StudentID.String(), mistake.ConceptID.String(), mistake.Description, encodeTimestamp(mistake.OccurredAt), encodeOptionalTimestamp(mistake.ResolvedAt), mistake.ID.String())
	if err == nil {
		err = requireAffected(result)
	}
	return classifyLearningError(operation, err)
}

func scanMistake(scanner rowScanner) (learning.Mistake, error) {
	var idValue, studentValue, conceptValue, description, occurredValue string
	var resolvedValue sql.NullString
	if err := scanner.Scan(&idValue, &studentValue, &conceptValue, &description, &occurredValue, &resolvedValue); err != nil {
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
	occurredAt, err := decodeTimestamp(occurredValue)
	if err != nil {
		return learning.Mistake{}, err
	}
	resolvedAt, err := decodeOptionalTimestamp(resolvedValue)
	if err != nil {
		return learning.Mistake{}, err
	}
	item := learning.Mistake{ID: id, StudentID: studentID, ConceptID: conceptID, Description: description, OccurredAt: occurredAt, ResolvedAt: resolvedAt}
	return item, item.Validate()
}

func (repository learningRetentionRepository) Get(ctx context.Context, studentID, conceptID learning.ID) (learning.RetentionState, error) {
	const operation = "get SQLite retention state"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	var studentValue, conceptValue, measuredValue string
	var strengthValue float64
	err := repository.executor.QueryRowContext(operationContext, "SELECT student_id, concept_id, strength, measured_at FROM retention_state WHERE student_id=? AND concept_id=?", studentID.String(), conceptID.String()).Scan(&studentValue, &conceptValue, &strengthValue, &measuredValue)
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
	state := learning.RetentionState{StudentID: student, ConceptID: concept, Strength: strength, MeasuredAt: measured}
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
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO retention_state (student_id,concept_id,strength,measured_at) VALUES (?,?,?,?) ON CONFLICT(student_id,concept_id) DO UPDATE SET strength=excluded.strength,measured_at=excluded.measured_at`, state.StudentID.String(), state.ConceptID.String(), state.Strength.Value(), encodeTimestamp(state.MeasuredAt))
	return classifyLearningError(operation, err)
}
