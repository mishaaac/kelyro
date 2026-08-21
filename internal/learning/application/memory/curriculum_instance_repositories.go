package memory

import (
	"context"
	"errors"
	"sort"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

type curriculumDefinitionRepository struct{ store *Store }

func (repository curriculumDefinitionRepository) Install(ctx context.Context, curriculum learning.Curriculum) error {
	const operation = "install memory curriculum definition"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	fingerprint, err := learning.CurriculumFingerprint(curriculum)
	if err != nil {
		return application.Classify(application.ErrorInvalidState, operation, err)
	}
	key := curriculumKey{id: curriculum.Reference.ID, version: curriculum.Reference.Version}

	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if existing, exists := repository.store.curricula[key]; exists {
		if existing.fingerprint == fingerprint {
			return nil
		}
		return application.Classify(application.ErrorConflict, operation, errors.New("curriculum version is already installed with different content"))
	}

	fixture := curriculumFixture{
		concepts:    make(map[learning.ID]learning.Concept),
		modules:     make(map[learning.ID]learning.ID),
		fingerprint: fingerprint,
	}
	nodesByID := make(map[learning.ID]learning.CurriculumNode, len(curriculum.Nodes))
	for _, node := range curriculum.Nodes {
		nodesByID[node.ID] = node
	}
	for _, node := range curriculum.Nodes {
		fixture.outline = append(fixture.outline, learning.CurriculumOutlineNode{
			ID: node.ID, Type: node.Type, ParentID: cloneIDPointer(node.ParentID), Title: node.Title, Order: node.Order,
		})
		if node.Type != learning.CurriculumNodeConcept {
			continue
		}
		fixture.concepts[node.ID] = learning.Concept{ID: node.ID, TopicID: *node.ParentID, Title: node.Title}
		if moduleID, ok := curriculumModuleForConcept(nodesByID, node.ID); ok {
			fixture.modules[node.ID] = moduleID
		}
		for _, prerequisite := range node.Concept.Prerequisites {
			fixture.prerequisites = append(fixture.prerequisites, learning.Prerequisite{
				ConceptID: node.ID, RequiredConceptID: prerequisite.ConceptID,
			})
		}
	}
	fixture.planning = memoryPlanningConcepts(curriculum)
	repository.store.curricula[key] = fixture
	return nil
}

func cloneIDPointer(source *learning.ID) *learning.ID {
	if source == nil {
		return nil
	}
	value := *source
	return &value
}

func memoryPlanningConcepts(curriculum learning.Curriculum) []learning.DailyPlanCurriculumConcept {
	concepts := make([]learning.CurriculumNode, 0)
	nodesByID := make(map[learning.ID]learning.CurriculumNode, len(curriculum.Nodes))
	for _, node := range curriculum.Nodes {
		nodesByID[node.ID] = node
		if node.Type == learning.CurriculumNodeConcept {
			concepts = append(concepts, node)
		}
	}
	paths := make(map[learning.ID][]learning.CurriculumNode, len(concepts))
	for _, concept := range concepts {
		paths[concept.ID] = memoryCurriculumPath(nodesByID, concept)
	}
	sort.Slice(concepts, func(i, j int) bool {
		return curriculumPathBefore(paths[concepts[i].ID], paths[concepts[j].ID])
	})
	result := make([]learning.DailyPlanCurriculumConcept, 0, len(concepts))
	for sequence, node := range concepts {
		concept := learning.DailyPlanCurriculumConcept{ConceptID: node.ID, Sequence: sequence}
		for _, prerequisite := range node.Concept.Prerequisites {
			concept.PrerequisiteIDs = append(concept.PrerequisiteIDs, prerequisite.ConceptID)
		}
		sort.Slice(concept.PrerequisiteIDs, func(i, j int) bool { return concept.PrerequisiteIDs[i].String() < concept.PrerequisiteIDs[j].String() })
		result = append(result, concept)
	}
	return result
}

func curriculumPathBefore(leftPath, rightPath []learning.CurriculumNode) bool {
	for index := 0; index < len(leftPath) && index < len(rightPath); index++ {
		if leftPath[index].Order != rightPath[index].Order {
			return leftPath[index].Order < rightPath[index].Order
		}
		if leftPath[index].ID != rightPath[index].ID {
			return leftPath[index].ID.String() < rightPath[index].ID.String()
		}
	}
	return len(leftPath) < len(rightPath)
}

func memoryCurriculumPath(nodes map[learning.ID]learning.CurriculumNode, node learning.CurriculumNode) []learning.CurriculumNode {
	path := []learning.CurriculumNode{node}
	for node.ParentID != nil {
		parent, exists := nodes[*node.ParentID]
		if !exists {
			break
		}
		path = append(path, parent)
		node = parent
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}

func curriculumModuleForConcept(nodes map[learning.ID]learning.CurriculumNode, conceptID learning.ID) (learning.ID, bool) {
	node, ok := nodes[conceptID]
	for ok && node.ParentID != nil {
		node, ok = nodes[*node.ParentID]
		if ok && node.Type == learning.CurriculumNodeModule {
			return node.ID, true
		}
	}
	return learning.ID{}, false
}

type curriculumInstanceRepository struct{ store *Store }

func (repository curriculumInstanceRepository) Create(ctx context.Context, instance learning.CurriculumInstance) error {
	const operation = "create memory curriculum instance"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := instance.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.instances[instance.ID]; exists {
		return conflict(operation)
	}
	if _, exists := repository.store.students[instance.StudentID]; !exists {
		return application.Classify(application.ErrorInvalidState, operation, errors.New("student does not exist"))
	}
	goal, exists := repository.store.goals[instance.GoalID]
	if !exists || goal.StudentID != instance.StudentID {
		return application.Classify(application.ErrorInvalidState, operation, errors.New("learning goal does not belong to student"))
	}
	if _, exists := repository.store.curricula[curriculumKey{id: instance.Curriculum.ID, version: instance.Curriculum.Version}]; !exists {
		return application.Classify(application.ErrorInvalidState, operation, errors.New("curriculum definition does not exist"))
	}
	for _, existing := range repository.store.instances {
		if existing.StudentID == instance.StudentID && existing.GoalID == instance.GoalID && existing.Curriculum == instance.Curriculum {
			return conflict(operation)
		}
	}
	repository.store.instances[instance.ID] = instance
	return nil
}

func (repository curriculumInstanceRepository) Get(ctx context.Context, id learning.ID) (learning.CurriculumInstance, error) {
	const operation = "get memory curriculum instance"
	if err := contextError(operation, ctx); err != nil {
		return learning.CurriculumInstance{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	instance, exists := repository.store.instances[id]
	if !exists {
		return learning.CurriculumInstance{}, notFound(operation)
	}
	return instance, nil
}

func (repository curriculumInstanceRepository) ListByStudent(ctx context.Context, studentID learning.ID) ([]learning.CurriculumInstance, error) {
	const operation = "list memory curriculum instances"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	instances := make([]learning.CurriculumInstance, 0)
	for _, instance := range repository.store.instances {
		if instance.StudentID == studentID {
			instances = append(instances, instance)
		}
	}
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].CreatedAt == instances[j].CreatedAt {
			return instances[i].ID.String() < instances[j].ID.String()
		}
		return instances[i].CreatedAt.Before(instances[j].CreatedAt)
	})
	return instances, nil
}

type instanceConceptStateRepository struct{ store *Store }

func (repository instanceConceptStateRepository) Get(ctx context.Context, instanceID, conceptID learning.ID) (learning.InstanceConceptState, error) {
	const operation = "get memory instance concept state"
	if err := contextError(operation, ctx); err != nil {
		return learning.InstanceConceptState{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	state, exists := repository.store.instanceStates[instanceConceptKey{instance: instanceID, concept: conceptID}]
	if !exists {
		return learning.InstanceConceptState{}, notFound(operation)
	}
	return cloneInstanceConceptState(state), nil
}

func (repository instanceConceptStateRepository) ListByInstance(ctx context.Context, instanceID learning.ID) ([]learning.InstanceConceptState, error) {
	const operation = "list memory instance concept states"
	if err := contextError(operation, ctx); err != nil {
		return nil, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	states := make([]learning.InstanceConceptState, 0)
	for _, state := range repository.store.instanceStates {
		if state.CurriculumInstanceID == instanceID {
			states = append(states, cloneInstanceConceptState(state))
		}
	}
	sort.Slice(states, func(i, j int) bool { return states[i].ConceptID.String() < states[j].ConceptID.String() })
	return states, nil
}

func (repository instanceConceptStateRepository) Save(ctx context.Context, state learning.InstanceConceptState) error {
	const operation = "save memory instance concept state"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := state.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, operation, err)
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	instance, exists := repository.store.instances[state.CurriculumInstanceID]
	if !exists || instance.StudentID != state.StudentID {
		return application.Classify(application.ErrorInvalidState, operation, errors.New("curriculum instance does not belong to student"))
	}
	fixture := repository.store.curricula[curriculumKey{id: instance.Curriculum.ID, version: instance.Curriculum.Version}]
	if _, exists := fixture.concepts[state.ConceptID]; !exists {
		return application.Classify(application.ErrorInvalidState, operation, errors.New("concept does not belong to curriculum instance"))
	}
	repository.store.instanceStates[instanceConceptKey{instance: state.CurriculumInstanceID, concept: state.ConceptID}] = cloneInstanceConceptState(state)
	return nil
}
