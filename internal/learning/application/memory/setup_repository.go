package memory

import (
	"context"

	"github.com/mishaaac/kelyro/internal/learning"
)

type learnerSetupRepository struct{ store *Store }

func (repository learnerSetupRepository) Get(ctx context.Context, studentID learning.ID) (learning.LearnerSetup, error) {
	if err := contextError("get memory learner setup", ctx); err != nil {
		return learning.LearnerSetup{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	setup, exists := repository.store.setups[studentID]
	if !exists {
		return learning.LearnerSetup{}, notFound("get memory learner setup")
	}
	return cloneLearnerSetup(setup), nil
}

func (repository learnerSetupRepository) Save(ctx context.Context, setup learning.LearnerSetup) error {
	if err := contextError("save memory learner setup", ctx); err != nil {
		return err
	}
	if err := setup.Validate(); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if current, exists := repository.store.setups[setup.StudentID]; exists {
		if current.CreatedAt != setup.CreatedAt || setup.UpdatedAt.Before(current.UpdatedAt) {
			return conflict("save memory learner setup")
		}
	}
	repository.store.setups[setup.StudentID] = cloneLearnerSetup(setup)
	return nil
}

func (repository learnerSetupRepository) ResetDevelopment(ctx context.Context, studentID learning.ID) error {
	if err := contextError("reset memory learner setup", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	setup, exists := repository.store.setups[studentID]
	if exists && setup.CurriculumInstanceID != nil {
		instanceID := *setup.CurriculumInstanceID
		for attemptID, attempt := range repository.store.diagnostics {
			if attempt.StudentID != studentID || attempt.CurriculumInstanceID != instanceID {
				continue
			}
			for _, observation := range attempt.Observations {
				delete(repository.store.evidence, observation.EvidenceID)
			}
			delete(repository.store.diagnostics, attemptID)
		}
		for key := range repository.store.instanceStates {
			if key.instance == instanceID {
				delete(repository.store.instanceStates, key)
			}
		}
		delete(repository.store.instances, instanceID)
	}
	delete(repository.store.onboarding, studentID)
	delete(repository.store.setups, studentID)
	return nil
}
