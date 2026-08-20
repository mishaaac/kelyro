package memory

import (
	"context"

	"github.com/mishaaac/kelyro/internal/learning"
)

type masteryThresholdRepository struct{ store *Store }

func (repository masteryThresholdRepository) Get(ctx context.Context, studentID learning.ID) (learning.MasteryThresholdSettings, error) {
	if err := contextError("get memory mastery threshold", ctx); err != nil {
		return learning.MasteryThresholdSettings{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	settings, exists := repository.store.mastery[studentID]
	if !exists {
		return learning.MasteryThresholdSettings{}, notFound("get memory mastery threshold")
	}
	return cloneMasterySettings(settings), nil
}

func (repository masteryThresholdRepository) Save(ctx context.Context, settings learning.MasteryThresholdSettings) error {
	if err := contextError("save memory mastery threshold", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.mastery[settings.StudentID] = cloneMasterySettings(settings)
	return nil
}

func cloneMasterySettings(settings learning.MasteryThresholdSettings) learning.MasteryThresholdSettings {
	if settings.WorkspaceOverride != nil {
		copy := *settings.WorkspaceOverride
		settings.WorkspaceOverride = &copy
	}
	return settings
}
