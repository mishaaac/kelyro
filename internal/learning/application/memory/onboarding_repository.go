package memory

import (
	"context"

	"github.com/mishaaac/kelyro/internal/learning"
)

type onboardingRepository struct{ store *Store }

func (repository onboardingRepository) Get(ctx context.Context, studentID learning.ID) (learning.OnboardingInterview, error) {
	if err := contextError("get memory onboarding", ctx); err != nil {
		return learning.OnboardingInterview{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	interview, exists := repository.store.onboarding[studentID]
	if !exists {
		return learning.OnboardingInterview{}, notFound("get memory onboarding")
	}
	return cloneOnboarding(interview), nil
}

func (repository onboardingRepository) Save(ctx context.Context, interview learning.OnboardingInterview) error {
	if err := contextError("save memory onboarding", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	repository.store.onboarding[interview.StudentID] = cloneOnboarding(interview)
	return nil
}

func cloneOnboarding(interview learning.OnboardingInterview) learning.OnboardingInterview {
	answers := interview.Answers
	interview.Answers = make(map[string]string, len(interview.Answers))
	for key, value := range answers {
		interview.Answers[key] = value
	}
	if interview.CompletedAt != nil {
		copy := *interview.CompletedAt
		interview.CompletedAt = &copy
	}
	if interview.CancelledAt != nil {
		copy := *interview.CancelledAt
		interview.CancelledAt = &copy
	}
	return interview
}
