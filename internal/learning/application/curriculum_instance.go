package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type CurriculumInstanceOption func(*curriculumInstanceService)

func WithCurriculumInstanceClock(now func() time.Time) CurriculumInstanceOption {
	return func(service *curriculumInstanceService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithCurriculumInstanceIDGenerator(generate func() (learning.ID, error)) CurriculumInstanceOption {
	return func(service *curriculumInstanceService) {
		if generate != nil {
			service.generateID = generate
		}
	}
}

type curriculumInstanceService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
	generateID func() (learning.ID, error)
}

func NewCurriculumInstanceService(profiles ProfileService, unitOfWork UnitOfWork, options ...CurriculumInstanceOption) CurriculumInstanceService {
	service := &curriculumInstanceService{
		profiles: profiles, unitOfWork: unitOfWork,
		now: time.Now, generateID: randomCurriculumInstanceID,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *curriculumInstanceService) Create(ctx context.Context, goalID learning.ID, curriculum learning.Curriculum, source learning.CurriculumSourceKind) (learning.CurriculumInstance, error) {
	const operation = "create curriculum instance"
	if err := curriculum.Validate(); err != nil {
		return learning.CurriculumInstance{}, invalid(operation, err)
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	if service.generateID == nil {
		return learning.CurriculumInstance{}, Classify(ErrorUnavailable, operation, errors.New("curriculum instance id generator is not configured"))
	}
	id, err := service.generateID()
	if err != nil {
		return learning.CurriculumInstance{}, Classify(ErrorUnavailable, operation, fmt.Errorf("generate curriculum instance id: %w", err))
	}
	instance, err := learning.NewCurriculumInstance(id, student.ID, goalID, curriculum.Reference, source, timestamp)
	if err != nil {
		return learning.CurriculumInstance{}, invalid(operation, err)
	}
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		goal, getErr := repositories.Goals.Get(ctx, goalID)
		if getErr != nil {
			return getErr
		}
		if goal.StudentID != student.ID {
			return Classify(ErrorInvalidState, operation, errors.New("learning goal belongs to another student"))
		}
		if goal.Status != learning.GoalActive {
			return Classify(ErrorInvalidState, operation, fmt.Errorf("learning goal must be active, got %q", goal.Status))
		}
		if err := repositories.Definitions.Install(ctx, curriculum); err != nil {
			return err
		}
		return repositories.CurriculumInstances.Create(ctx, instance)
	})
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	return instance, nil
}

func (service *curriculumInstanceService) Get(ctx context.Context, instanceID learning.ID) (learning.CurriculumInstance, error) {
	const operation = "get curriculum instance"
	student, err := service.student(ctx, operation)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	var instance learning.CurriculumInstance
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		loaded, getErr := repositories.CurriculumInstances.Get(ctx, instanceID)
		if getErr != nil {
			return getErr
		}
		if loaded.StudentID != student.ID {
			return Classify(ErrorNotFound, operation, errors.New("curriculum instance not found"))
		}
		instance = loaded
		return nil
	})
	return instance, err
}

func (service *curriculumInstanceService) List(ctx context.Context) ([]learning.CurriculumInstance, error) {
	const operation = "list curriculum instances"
	student, err := service.student(ctx, operation)
	if err != nil {
		return nil, err
	}
	var instances []learning.CurriculumInstance
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		listed, listErr := repositories.CurriculumInstances.ListByStudent(ctx, student.ID)
		instances = listed
		return listErr
	})
	return instances, err
}

func (service *curriculumInstanceService) State(ctx context.Context, instanceID, conceptID learning.ID) (learning.InstanceConceptState, error) {
	const operation = "get curriculum instance concept state"
	student, err := service.student(ctx, operation)
	if err != nil {
		return learning.InstanceConceptState{}, err
	}
	var state learning.InstanceConceptState
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		instance, getErr := repositories.CurriculumInstances.Get(ctx, instanceID)
		if getErr != nil {
			return getErr
		}
		if instance.StudentID != student.ID {
			return Classify(ErrorNotFound, operation, errors.New("curriculum instance not found"))
		}
		if _, getErr := repositories.Curricula.Concept(ctx, instance.Curriculum, conceptID); getErr != nil {
			return getErr
		}
		loaded, getErr := repositories.InstanceConceptStates.Get(ctx, instanceID, conceptID)
		if getErr == nil {
			state = loaded
			return nil
		}
		if !errors.Is(getErr, ErrNotFound) {
			return getErr
		}
		timestamp, timestampErr := service.timestamp(operation)
		if timestampErr != nil {
			return timestampErr
		}
		state, timestampErr = learning.NewInstanceConceptState(instance, conceptID, timestamp)
		if timestampErr != nil {
			return invalid(operation, timestampErr)
		}
		return repositories.InstanceConceptStates.Save(ctx, state)
	})
	return state, err
}

func (service *curriculumInstanceService) States(ctx context.Context, instanceID learning.ID) ([]learning.InstanceConceptState, error) {
	const operation = "list curriculum instance concept states"
	student, err := service.student(ctx, operation)
	if err != nil {
		return nil, err
	}
	var states []learning.InstanceConceptState
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		instance, getErr := repositories.CurriculumInstances.Get(ctx, instanceID)
		if getErr != nil {
			return getErr
		}
		if instance.StudentID != student.ID {
			return Classify(ErrorNotFound, operation, errors.New("curriculum instance not found"))
		}
		listed, listErr := repositories.InstanceConceptStates.ListByInstance(ctx, instanceID)
		states = listed
		return listErr
	})
	return states, err
}

func (service *curriculumInstanceService) SaveState(ctx context.Context, state learning.InstanceConceptState) error {
	const operation = "save curriculum instance concept state"
	if err := state.Validate(); err != nil {
		return invalid(operation, err)
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return err
	}
	return service.withRepositories(ctx, operation, func(repositories Repositories) error {
		instance, getErr := repositories.CurriculumInstances.Get(ctx, state.CurriculumInstanceID)
		if getErr != nil {
			return getErr
		}
		if instance.StudentID != student.ID || state.StudentID != student.ID {
			return Classify(ErrorInvalidState, operation, errors.New("instance concept state belongs to another student"))
		}
		if state.UpdatedAt.Before(instance.CreatedAt) || (state.FirstSeenAt != nil && state.FirstSeenAt.Before(instance.CreatedAt)) {
			return Classify(ErrorInvalidState, operation, errors.New("instance concept state precedes curriculum instance creation"))
		}
		if _, getErr := repositories.Curricula.Concept(ctx, instance.Curriculum, state.ConceptID); getErr != nil {
			return getErr
		}
		current, getErr := repositories.InstanceConceptStates.Get(ctx, instance.ID, state.ConceptID)
		if getErr == nil && state.UpdatedAt.Before(current.UpdatedAt) {
			return Classify(ErrorInvalidState, operation, errors.New("instance concept state update precedes persisted state"))
		}
		if getErr != nil && !errors.Is(getErr, ErrNotFound) {
			return getErr
		}
		return repositories.InstanceConceptStates.Save(ctx, state)
	})
}

func (service *curriculumInstanceService) student(ctx context.Context, operation string) (learning.Student, error) {
	if service == nil || service.profiles == nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, errors.New("profile service is not configured"))
	}
	return service.profiles.Show(ctx)
}

func (service *curriculumInstanceService) withRepositories(ctx context.Context, operation string, work func(Repositories) error) error {
	if service == nil || service.unitOfWork == nil {
		return Classify(ErrorUnavailable, operation, errors.New("learning transaction is not configured"))
	}
	return repositoryError(operation, service.unitOfWork.WithinTransaction(ctx, work))
}

func (service *curriculumInstanceService) timestamp(operation string) (learning.Timestamp, error) {
	if service == nil || service.now == nil {
		return learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("curriculum instance clock is not configured"))
	}
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Timestamp{}, invalid(operation, err)
	}
	return timestamp, nil
}

func randomCurriculumInstanceID() (learning.ID, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return learning.ID{}, err
	}
	return learning.NewID("curriculum-instance." + hex.EncodeToString(bytes))
}
