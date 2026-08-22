package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

func (repository learningCurriculumDefinitionRepository) Install(ctx context.Context, curriculum learning.Curriculum) error {
	const operation = "install SQLite curriculum definition"
	fingerprint, err := learning.CurriculumFingerprint(curriculum)
	if err != nil {
		return invalidLearning(operation, err)
	}
	return repository.atomic(ctx, operation, func(ctx context.Context, target executor) error {
		var existingFingerprint string
		err := target.QueryRowContext(ctx, `SELECT fingerprint FROM curriculum_definition_fingerprints
WHERE curriculum_id = ? AND curriculum_version = ?`, curriculum.Reference.ID.String(), curriculum.Reference.Version).Scan(&existingFingerprint)
		if err == nil {
			if existingFingerprint == fingerprint {
				return nil
			}
			return application.Classify(application.ErrorConflict, operation, errors.New("curriculum version is already installed with different content"))
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var definitionExists int
		err = target.QueryRowContext(ctx, `SELECT 1 FROM curriculum_instances WHERE id = ? AND version = ?`,
			curriculum.Reference.ID.String(), curriculum.Reference.Version).Scan(&definitionExists)
		if err == nil {
			return application.Classify(application.ErrorConflict, operation, errors.New("curriculum version exists without an immutable definition fingerprint"))
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := target.ExecContext(ctx, `INSERT INTO curriculum_instances (id, version) VALUES (?, ?)`,
			curriculum.Reference.ID.String(), curriculum.Reference.Version); err != nil {
			return err
		}
		for _, node := range curriculum.Nodes {
			if node.Type == learning.CurriculumNodeConcept {
				if _, err := target.ExecContext(ctx, `INSERT INTO concept_registry (id) VALUES (?) ON CONFLICT(id) DO NOTHING`, node.ID.String()); err != nil {
					return err
				}
			}
		}
		for _, nodeType := range []learning.CurriculumNodeType{
			learning.CurriculumNodePhase,
			learning.CurriculumNodeModule,
			learning.CurriculumNodeLesson,
			learning.CurriculumNodeTopic,
			learning.CurriculumNodeConcept,
		} {
			for _, node := range curriculum.Nodes {
				if node.Type != nodeType {
					continue
				}
				var parentID any
				if node.ParentID != nil {
					parentID = node.ParentID.String()
				}
				var conceptID any
				if node.Type == learning.CurriculumNodeConcept {
					conceptID = node.ID.String()
				}
				if _, err := target.ExecContext(ctx, `INSERT INTO curriculum_nodes
(curriculum_id, curriculum_version, node_id, node_type, parent_node_id, concept_id, title, position)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, curriculum.Reference.ID.String(), curriculum.Reference.Version,
					node.ID.String(), string(node.Type), parentID, conceptID, node.Title, node.Order); err != nil {
					return err
				}
			}
		}
		for _, node := range curriculum.Nodes {
			if node.Type != learning.CurriculumNodeConcept {
				continue
			}
			for _, prerequisite := range node.Concept.Prerequisites {
				if _, err := target.ExecContext(ctx, `INSERT INTO curriculum_edges
(curriculum_id, curriculum_version, concept_id, required_concept_id) VALUES (?, ?, ?, ?)`,
					curriculum.Reference.ID.String(), curriculum.Reference.Version, node.ID.String(), prerequisite.ConceptID.String()); err != nil {
					return err
				}
			}
		}
		_, err = target.ExecContext(ctx, `INSERT INTO curriculum_definition_fingerprints
(curriculum_id, curriculum_version, fingerprint) VALUES (?, ?, ?)`, curriculum.Reference.ID.String(), curriculum.Reference.Version, fingerprint)
		return err
	})
}

func (repository learningCurriculumInstanceRepository) Create(ctx context.Context, instance learning.CurriculumInstance) error {
	const operation = "create SQLite curriculum instance"
	if err := instance.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	_, err := repository.executor.ExecContext(operationContext, `INSERT INTO learner_curriculum_instances
(id, student_id, goal_id, curriculum_id, curriculum_version, source_kind, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, instance.ID.String(), instance.StudentID.String(), instance.GoalID.String(),
		instance.Curriculum.ID.String(), instance.Curriculum.Version, string(instance.Source), string(instance.Status),
		encodeTimestamp(instance.CreatedAt), encodeTimestamp(instance.UpdatedAt))
	return classifyLearningError(operation, err)
}

func (repository learningCurriculumInstanceRepository) Get(ctx context.Context, id learning.ID) (learning.CurriculumInstance, error) {
	const operation = "get SQLite curriculum instance"
	if err := id.Validate(); err != nil {
		return learning.CurriculumInstance{}, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	instance, err := scanCurriculumInstance(repository.executor.QueryRowContext(operationContext, `SELECT id, student_id, goal_id,
curriculum_id, curriculum_version, source_kind, status, created_at, updated_at
FROM learner_curriculum_instances WHERE id = ?`, id.String()))
	if err != nil {
		return learning.CurriculumInstance{}, classifyLearningError(operation, err)
	}
	return instance, nil
}

func (repository learningCurriculumInstanceRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.CurriculumInstance, error) {
	const operation = "list SQLite curriculum instances"
	if err := studentID.Validate(); err != nil {
		return nil, invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT id, student_id, goal_id,
curriculum_id, curriculum_version, source_kind, status, created_at, updated_at
FROM learner_curriculum_instances WHERE student_id = ? ORDER BY created_at, id`, studentID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	instances := make([]learning.CurriculumInstance, 0)
	for rows.Next() {
		instance, scanErr := scanCurriculumInstance(rows)
		if scanErr != nil {
			return nil, corruptLearning(operation, scanErr)
		}
		instances = append(instances, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return instances, nil
}

func scanCurriculumInstance(row rowScanner) (learning.CurriculumInstance, error) {
	var id, studentID, goalID, curriculumID, version, source, status, createdAt, updatedAt string
	if err := row.Scan(&id, &studentID, &goalID, &curriculumID, &version, &source, &status, &createdAt, &updatedAt); err != nil {
		return learning.CurriculumInstance{}, err
	}
	decodedID, err := decodeID(id)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	decodedStudentID, err := decodeID(studentID)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	decodedGoalID, err := decodeID(goalID)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	decodedCurriculumID, err := decodeID(curriculumID)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	created, err := decodeTimestamp(createdAt)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	updated, err := decodeTimestamp(updatedAt)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	instance := learning.CurriculumInstance{
		ID: decodedID, StudentID: decodedStudentID, GoalID: decodedGoalID,
		Curriculum: learning.CurriculumRef{ID: decodedCurriculumID, Version: version},
		Source:     learning.CurriculumSourceKind(source), Status: learning.CurriculumInstanceStatus(status),
		CreatedAt: created, UpdatedAt: updated,
	}
	if err := instance.Validate(); err != nil {
		return learning.CurriculumInstance{}, err
	}
	return instance, nil
}

func (repository learningInstanceConceptStateRepository) Get(ctx context.Context, instanceID, conceptID learning.ID) (learning.InstanceConceptState, error) {
	const operation = "get SQLite instance concept state"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	state, err := scanInstanceConceptState(repository.executor.QueryRowContext(operationContext, `SELECT curriculum_instance_id,
student_id, concept_id, exposure, mastery, mastery_algorithm_version, progression_policy_version, first_seen_at, last_seen_at, mastered_at, review_due_at, manual_flags_json, updated_at
FROM learner_curriculum_concept_states WHERE curriculum_instance_id = ? AND concept_id = ?`, instanceID.String(), conceptID.String()))
	if err != nil {
		return learning.InstanceConceptState{}, classifyLearningError(operation, err)
	}
	return state, nil
}

func (repository learningInstanceConceptStateRepository) ListByInstance(ctx context.Context, instanceID learning.ID) ([]learning.InstanceConceptState, error) {
	const operation = "list SQLite instance concept states"
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	rows, err := repository.executor.QueryContext(operationContext, `SELECT curriculum_instance_id,
student_id, concept_id, exposure, mastery, mastery_algorithm_version, progression_policy_version, first_seen_at, last_seen_at, mastered_at, review_due_at, manual_flags_json, updated_at
FROM learner_curriculum_concept_states WHERE curriculum_instance_id = ? ORDER BY concept_id`, instanceID.String())
	if err != nil {
		return nil, classifyLearningError(operation, err)
	}
	defer rows.Close()
	states := make([]learning.InstanceConceptState, 0)
	for rows.Next() {
		state, scanErr := scanInstanceConceptState(rows)
		if scanErr != nil {
			return nil, corruptLearning(operation, scanErr)
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, classifyLearningError(operation, err)
	}
	return states, nil
}

func (repository learningInstanceConceptStateRepository) Save(ctx context.Context, state learning.InstanceConceptState) error {
	const operation = "save SQLite instance concept state"
	if err := state.Validate(); err != nil {
		return invalidLearning(operation, err)
	}
	manualFlags := state.ManualFlags
	if manualFlags == nil {
		manualFlags = []string{}
	}
	flags, err := json.Marshal(manualFlags)
	if err != nil {
		return invalidLearning(operation, err)
	}
	operationContext, cancel := context.WithTimeout(ctx, repository.timeout)
	defer cancel()
	masteryVersion, progressionVersion := state.MasteryAlgorithmVersion, state.ProgressionPolicyVersion
	if masteryVersion == "" && progressionVersion == "" {
		masteryVersion, progressionVersion = learning.UnversionedDerivedStateVersion, learning.UnversionedDerivedStateVersion
	}
	_, err = repository.executor.ExecContext(operationContext, `INSERT INTO learner_curriculum_concept_states
(curriculum_instance_id, student_id, concept_id, exposure, mastery, mastery_algorithm_version, progression_policy_version, first_seen_at, last_seen_at, mastered_at, review_due_at, manual_flags_json, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(curriculum_instance_id, concept_id) DO UPDATE SET
student_id = excluded.student_id, exposure = excluded.exposure, mastery = excluded.mastery,
mastery_algorithm_version = excluded.mastery_algorithm_version, progression_policy_version = excluded.progression_policy_version,
first_seen_at = excluded.first_seen_at, last_seen_at = excluded.last_seen_at, mastered_at = excluded.mastered_at,
review_due_at = excluded.review_due_at, manual_flags_json = excluded.manual_flags_json, updated_at = excluded.updated_at`,
		state.CurriculumInstanceID.String(), state.StudentID.String(), state.ConceptID.String(), string(state.Exposure), state.Mastery.Value(),
		masteryVersion, progressionVersion,
		encodeOptionalTimestamp(state.FirstSeenAt), encodeOptionalTimestamp(state.LastSeenAt), encodeOptionalTimestamp(state.MasteredAt),
		encodeOptionalTimestamp(state.ReviewDueAt), string(flags), encodeTimestamp(state.UpdatedAt))
	return classifyLearningError(operation, err)
}

func scanInstanceConceptState(row rowScanner) (learning.InstanceConceptState, error) {
	var instanceID, studentID, conceptID, exposure, masteryVersion, progressionVersion, flagsJSON, updatedAt string
	var mastery float64
	var firstSeenAt, lastSeenAt, masteredAt, reviewDueAt sql.NullString
	if err := row.Scan(&instanceID, &studentID, &conceptID, &exposure, &mastery, &masteryVersion, &progressionVersion, &firstSeenAt, &lastSeenAt,
		&masteredAt, &reviewDueAt, &flagsJSON, &updatedAt); err != nil {
		return learning.InstanceConceptState{}, err
	}
	decodedInstanceID, err := decodeID(instanceID)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	decodedStudentID, err := decodeID(studentID)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	decodedConceptID, err := decodeID(conceptID)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	decodedMastery, err := learning.NewMasteryScore(mastery)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	firstSeen, err := decodeOptionalTimestamp(firstSeenAt)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	lastSeen, err := decodeOptionalTimestamp(lastSeenAt)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	mastered, err := decodeOptionalTimestamp(masteredAt)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	reviewDue, err := decodeOptionalTimestamp(reviewDueAt)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	updated, err := decodeTimestamp(updatedAt)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	var flags []string
	if err := json.Unmarshal([]byte(flagsJSON), &flags); err != nil {
		return learning.InstanceConceptState{}, fmt.Errorf("decode manual flags: %w", err)
	}
	if flags == nil {
		flags = []string{}
	}
	state := learning.InstanceConceptState{
		CurriculumInstanceID: decodedInstanceID, StudentID: decodedStudentID, ConceptID: decodedConceptID,
		Exposure: learning.ExposureState(exposure), Mastery: decodedMastery,
		MasteryAlgorithmVersion: masteryVersion, ProgressionPolicyVersion: progressionVersion,
		FirstSeenAt: firstSeen, LastSeenAt: lastSeen, MasteredAt: mastered, ReviewDueAt: reviewDue,
		ManualFlags: flags, UpdatedAt: updated,
	}
	if err := state.Validate(); err != nil {
		return learning.InstanceConceptState{}, err
	}
	return state, nil
}
