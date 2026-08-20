package memory

import (
	"context"
	"sort"

	"github.com/mishaaac/kelyro/internal/learning"
)

type diagnosticRepository struct{ store *Store }

func (repository diagnosticRepository) Create(ctx context.Context, attempt learning.DiagnosticAttempt) error {
	if err := contextError("create memory diagnostic", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.diagnostics[attempt.ID]; exists {
		return conflict("create memory diagnostic")
	}
	for _, existing := range repository.store.diagnostics {
		if existing.StudentID == attempt.StudentID && existing.CurriculumInstanceID == attempt.CurriculumInstanceID && existing.Diagnostic == attempt.Diagnostic {
			return conflict("create memory diagnostic")
		}
	}
	repository.store.diagnostics[attempt.ID] = cloneDiagnosticAttempt(attempt)
	return nil
}

func (repository diagnosticRepository) Get(ctx context.Context, id learning.ID) (learning.DiagnosticAttempt, error) {
	if err := contextError("get memory diagnostic", ctx); err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	attempt, exists := repository.store.diagnostics[id]
	if !exists {
		return learning.DiagnosticAttempt{}, notFound("get memory diagnostic")
	}
	return cloneDiagnosticAttempt(attempt), nil
}

func (repository diagnosticRepository) Find(ctx context.Context, studentID, instanceID learning.ID, reference learning.DiagnosticRef) (learning.DiagnosticAttempt, error) {
	if err := contextError("find memory diagnostic", ctx); err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	repository.store.mu.RLock()
	defer repository.store.mu.RUnlock()
	items := make([]learning.DiagnosticAttempt, 0, 1)
	for _, attempt := range repository.store.diagnostics {
		if attempt.StudentID == studentID && attempt.CurriculumInstanceID == instanceID && attempt.Diagnostic == reference {
			items = append(items, cloneDiagnosticAttempt(attempt))
		}
	}
	if len(items) == 0 {
		return learning.DiagnosticAttempt{}, notFound("find memory diagnostic")
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID.String() < items[j].ID.String() })
	return items[0], nil
}

func (repository diagnosticRepository) Save(ctx context.Context, attempt learning.DiagnosticAttempt) error {
	if err := contextError("save memory diagnostic", ctx); err != nil {
		return err
	}
	repository.store.mu.Lock()
	defer repository.store.mu.Unlock()
	if _, exists := repository.store.diagnostics[attempt.ID]; !exists {
		return notFound("save memory diagnostic")
	}
	repository.store.diagnostics[attempt.ID] = cloneDiagnosticAttempt(attempt)
	return nil
}
