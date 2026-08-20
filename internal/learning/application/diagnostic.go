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

type DiagnosticOption func(*diagnosticService)

func WithDiagnosticClock(now func() time.Time) DiagnosticOption {
	return func(service *diagnosticService) { service.now = now }
}

func WithDiagnosticAttemptIDGenerator(generate func() (learning.ID, error)) DiagnosticOption {
	return func(service *diagnosticService) { service.attemptID = generate }
}

func WithDiagnosticEvidenceIDGenerator(generate func() (learning.ID, error)) DiagnosticOption {
	return func(service *diagnosticService) { service.evidenceID = generate }
}

type diagnosticService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
	attemptID  func() (learning.ID, error)
	evidenceID func() (learning.ID, error)
}

func NewDiagnosticService(profiles ProfileService, unitOfWork UnitOfWork, options ...DiagnosticOption) DiagnosticService {
	service := &diagnosticService{profiles: profiles, unitOfWork: unitOfWork, now: time.Now,
		attemptID:  func() (learning.ID, error) { return randomDiagnosticID("diagnostic-attempt") },
		evidenceID: func() (learning.ID, error) { return randomDiagnosticID("evidence.diagnostic") },
	}
	for _, configure := range options {
		if configure != nil {
			configure(service)
		}
	}
	return service
}

func (service *diagnosticService) Start(ctx context.Context, instanceID learning.ID, diagnostic learning.Diagnostic) (DiagnosticView, error) {
	const operation = "start diagnostic"
	if err := instanceID.Validate(); err != nil {
		return DiagnosticView{}, invalid(operation, err)
	}
	if err := diagnostic.Validate(); err != nil {
		return DiagnosticView{}, invalid(operation, err)
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return DiagnosticView{}, err
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return DiagnosticView{}, err
	}
	id, err := service.generateID(operation, "attempt", service.attemptID)
	if err != nil {
		return DiagnosticView{}, err
	}
	created, err := learning.NewDiagnosticAttempt(id, student.ID, instanceID, diagnostic, timestamp)
	if err != nil {
		return DiagnosticView{}, invalid(operation, err)
	}
	attempt := created
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		if err := validateDiagnosticInstance(ctx, operation, repositories, student.ID, instanceID, diagnostic); err != nil {
			return err
		}
		existing, findErr := repositories.Diagnostics.Find(ctx, student.ID, instanceID, diagnostic.Reference)
		if findErr == nil {
			attempt = existing
			return nil
		}
		if !errors.Is(findErr, ErrNotFound) {
			return findErr
		}
		return repositories.Diagnostics.Create(ctx, created)
	})
	if err != nil {
		return DiagnosticView{}, err
	}
	return diagnosticView(diagnostic, attempt, operation)
}

func (service *diagnosticService) Resume(ctx context.Context, attemptID learning.ID, diagnostic learning.Diagnostic) (DiagnosticView, error) {
	const operation = "resume diagnostic"
	attempt, err := service.load(ctx, operation, attemptID, diagnostic)
	if err != nil {
		return DiagnosticView{}, err
	}
	return diagnosticView(diagnostic, attempt, operation)
}

func (service *diagnosticService) Submit(ctx context.Context, attemptID learning.ID, diagnostic learning.Diagnostic, answers []string) (DiagnosticView, error) {
	const operation = "submit diagnostic answer"
	if err := attemptID.Validate(); err != nil {
		return DiagnosticView{}, invalid(operation, err)
	}
	if err := diagnostic.Validate(); err != nil {
		return DiagnosticView{}, invalid(operation, err)
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return DiagnosticView{}, err
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return DiagnosticView{}, err
	}
	evidenceID, err := service.generateID(operation, "evidence", service.evidenceID)
	if err != nil {
		return DiagnosticView{}, err
	}
	var attempt learning.DiagnosticAttempt
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		loaded, getErr := repositories.Diagnostics.Get(ctx, attemptID)
		if getErr != nil {
			return getErr
		}
		if loaded.StudentID != student.ID {
			return Classify(ErrorNotFound, operation, errors.New("diagnostic attempt not found"))
		}
		if err := validateDiagnosticInstance(ctx, operation, repositories, student.ID, loaded.CurriculumInstanceID, diagnostic); err != nil {
			return err
		}
		item, nextErr := learning.NextDiagnosticItem(diagnostic, loaded)
		if nextErr != nil {
			return Classify(ErrorInvalidState, operation, nextErr)
		}
		if item == nil {
			return Classify(ErrorInvalidState, operation, errors.New("diagnostic has no unanswered item"))
		}
		score, evaluateErr := item.Evaluate(answers)
		if evaluateErr != nil {
			return Classify(ErrorInvalidState, operation, evaluateErr)
		}
		evidence, evidenceErr := learning.NewEvidence(evidenceID, student.ID, item.ConceptID, learning.EvidenceDiagnostic,
			diagnosticEvidenceSource(diagnostic, loaded, *item), score, timestamp)
		if evidenceErr != nil {
			return Classify(ErrorInvalidState, operation, evidenceErr)
		}
		updated, recordErr := loaded.Record(learning.DiagnosticObservation{ItemID: item.ID, ConceptID: item.ConceptID, Score: score, EvidenceID: evidence.ID, AnsweredAt: timestamp})
		if recordErr != nil {
			return Classify(ErrorInvalidState, operation, recordErr)
		}
		next, nextErr := learning.NextDiagnosticItem(diagnostic, updated)
		if nextErr != nil {
			return Classify(ErrorInvalidState, operation, nextErr)
		}
		if next == nil {
			updated, recordErr = updated.Complete(timestamp)
			if recordErr != nil {
				return Classify(ErrorInvalidState, operation, recordErr)
			}
		}
		if appendErr := repositories.Evidence.Append(ctx, evidence); appendErr != nil {
			return appendErr
		}
		if saveErr := repositories.Diagnostics.Save(ctx, updated); saveErr != nil {
			return saveErr
		}
		attempt = updated
		return nil
	})
	if err != nil {
		return DiagnosticView{}, err
	}
	return diagnosticView(diagnostic, attempt, operation)
}

func (service *diagnosticService) Skip(ctx context.Context, instanceID learning.ID, diagnostic learning.Diagnostic) (DiagnosticView, error) {
	const operation = "skip diagnostic"
	view, err := service.Start(ctx, instanceID, diagnostic)
	if err != nil {
		return DiagnosticView{}, err
	}
	if view.Attempt.Status == learning.DiagnosticSkipped || view.Attempt.Status == learning.DiagnosticCompleted {
		return view, nil
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return DiagnosticView{}, err
	}
	skipped, err := view.Attempt.Skip(timestamp)
	if err != nil {
		return DiagnosticView{}, Classify(ErrorInvalidState, operation, err)
	}
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error { return repositories.Diagnostics.Save(ctx, skipped) })
	if err != nil {
		return DiagnosticView{}, err
	}
	return diagnosticView(diagnostic, skipped, operation)
}

func (service *diagnosticService) Result(ctx context.Context, attemptID learning.ID, diagnostic learning.Diagnostic) (learning.DiagnosticResult, error) {
	const operation = "get diagnostic result"
	attempt, err := service.load(ctx, operation, attemptID, diagnostic)
	if err != nil {
		return learning.DiagnosticResult{}, err
	}
	result, err := learning.BuildDiagnosticResult(diagnostic, attempt)
	if err != nil {
		return learning.DiagnosticResult{}, Classify(ErrorInvalidState, operation, err)
	}
	return result, nil
}

func (service *diagnosticService) load(ctx context.Context, operation string, attemptID learning.ID, diagnostic learning.Diagnostic) (learning.DiagnosticAttempt, error) {
	if err := attemptID.Validate(); err != nil {
		return learning.DiagnosticAttempt{}, invalid(operation, err)
	}
	if err := diagnostic.Validate(); err != nil {
		return learning.DiagnosticAttempt{}, invalid(operation, err)
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return learning.DiagnosticAttempt{}, err
	}
	var attempt learning.DiagnosticAttempt
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		loaded, getErr := repositories.Diagnostics.Get(ctx, attemptID)
		if getErr != nil {
			return getErr
		}
		if loaded.StudentID != student.ID {
			return Classify(ErrorNotFound, operation, errors.New("diagnostic attempt not found"))
		}
		if err := validateDiagnosticInstance(ctx, operation, repositories, student.ID, loaded.CurriculumInstanceID, diagnostic); err != nil {
			return err
		}
		attempt = loaded
		return nil
	})
	return attempt, err
}

func validateDiagnosticInstance(ctx context.Context, operation string, repositories Repositories, studentID, instanceID learning.ID, diagnostic learning.Diagnostic) error {
	instance, err := repositories.CurriculumInstances.Get(ctx, instanceID)
	if err != nil {
		return err
	}
	if instance.StudentID != studentID {
		return Classify(ErrorNotFound, operation, errors.New("curriculum instance not found"))
	}
	if instance.Curriculum != diagnostic.Curriculum {
		return Classify(ErrorInvalidState, operation, errors.New("diagnostic curriculum does not match learner curriculum instance"))
	}
	for _, item := range diagnostic.Items() {
		if _, err := repositories.Curricula.Concept(ctx, instance.Curriculum, item.ConceptID); err != nil {
			return err
		}
	}
	return nil
}

func diagnosticView(diagnostic learning.Diagnostic, attempt learning.DiagnosticAttempt, operation string) (DiagnosticView, error) {
	item, err := learning.NextDiagnosticItem(diagnostic, attempt)
	if err != nil {
		return DiagnosticView{}, Classify(ErrorInvalidState, operation, err)
	}
	result, err := learning.BuildDiagnosticResult(diagnostic, attempt)
	if err != nil {
		return DiagnosticView{}, Classify(ErrorInvalidState, operation, err)
	}
	return DiagnosticView{Attempt: attempt, Item: item, Result: result}, nil
}

func (service *diagnosticService) student(ctx context.Context, operation string) (learning.Student, error) {
	if service == nil || service.profiles == nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, errors.New("profile service is not configured"))
	}
	return service.profiles.Show(ctx)
}

func (service *diagnosticService) withRepositories(ctx context.Context, operation string, work func(Repositories) error) error {
	if service == nil || service.unitOfWork == nil {
		return Classify(ErrorUnavailable, operation, errors.New("learning transaction is not configured"))
	}
	return repositoryError(operation, service.unitOfWork.WithinTransaction(ctx, work))
}

func (service *diagnosticService) timestamp(operation string) (learning.Timestamp, error) {
	if service == nil || service.now == nil {
		return learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("diagnostic clock is not configured"))
	}
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Timestamp{}, invalid(operation, err)
	}
	return timestamp, nil
}

func (service *diagnosticService) generateID(operation, kind string, generate func() (learning.ID, error)) (learning.ID, error) {
	if generate == nil {
		return learning.ID{}, Classify(ErrorUnavailable, operation, fmt.Errorf("diagnostic %s id generator is not configured", kind))
	}
	id, err := generate()
	if err != nil {
		return learning.ID{}, Classify(ErrorUnavailable, operation, fmt.Errorf("generate diagnostic %s id: %w", kind, err))
	}
	if err := id.Validate(); err != nil {
		return learning.ID{}, invalid(operation, err)
	}
	return id, nil
}

func diagnosticEvidenceSource(diagnostic learning.Diagnostic, attempt learning.DiagnosticAttempt, item learning.DiagnosticItem) string {
	return fmt.Sprintf("diagnostic/%s@%s/instance/%s/attempt/%s/item/%s/policy/%s", diagnostic.Reference.ID, diagnostic.Reference.Version, attempt.CurriculumInstanceID, attempt.ID, item.ID, diagnostic.ScoringVersion)
}

func randomDiagnosticID(prefix string) (learning.ID, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return learning.ID{}, err
	}
	return learning.NewID(prefix + "." + hex.EncodeToString(value))
}
