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

type StudySessionOption func(*studySessionLifecycleService)

func WithStudySessionClock(now func() time.Time) StudySessionOption {
	return func(service *studySessionLifecycleService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithStudySessionIDGenerator(generate func() (learning.ID, error)) StudySessionOption {
	return func(service *studySessionLifecycleService) {
		if generate != nil {
			service.generateID = generate
		}
	}
}

func WithStudySessionIdleTimeout(timeout time.Duration) StudySessionOption {
	return func(service *studySessionLifecycleService) { service.idleTimeout = timeout }
}

type studySessionLifecycleService struct {
	profiles    ProfileService
	unitOfWork  UnitOfWork
	now         func() time.Time
	generateID  func() (learning.ID, error)
	idleTimeout time.Duration
}

func NewStudySessionLifecycleService(profiles ProfileService, unitOfWork UnitOfWork, options ...StudySessionOption) StudySessionLifecycleService {
	service := &studySessionLifecycleService{
		profiles: profiles, unitOfWork: unitOfWork, now: time.Now,
		generateID: randomStudySessionID, idleTimeout: learning.DefaultStudySessionIdleTimeout,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *studySessionLifecycleService) Start(ctx context.Context, goalID, curriculumInstanceID learning.ID) (learning.StudySession, error) {
	const operation = "start study session"
	if err := validatePair("goal", goalID, "curriculum instance", curriculumInstanceID); err != nil {
		return learning.StudySession{}, invalid(operation, err)
	}
	student, timestamp, err := service.context(ctx, operation)
	if err != nil {
		return learning.StudySession{}, err
	}
	if service.generateID == nil {
		return learning.StudySession{}, Classify(ErrorUnavailable, operation, errors.New("study session id generator is not configured"))
	}
	id, err := service.generateID()
	if err != nil {
		return learning.StudySession{}, Classify(ErrorUnavailable, operation, fmt.Errorf("generate study session id: %w", err))
	}
	session, err := learning.NewStudySession(id, student.ID, goalID, curriculumInstanceID, timestamp, service.idleTimeout)
	if err != nil {
		return learning.StudySession{}, invalid(operation, err)
	}
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		if err := validateStudySessionScope(ctx, repositories, student.ID, goalID, curriculumInstanceID); err != nil {
			return err
		}
		active, activeErr := repositories.StudySessions.ActiveByStudent(ctx, student.ID)
		switch {
		case activeErr == nil:
			stale, staleErr := active.IsStale(timestamp)
			if staleErr != nil {
				return invalid(operation, staleErr)
			}
			if !stale {
				return Classify(ErrorConflict, operation, fmt.Errorf("study session %q is already active", active.ID))
			}
			recovered, recoverErr := active.Recover(timestamp)
			if recoverErr != nil {
				return invalid(operation, recoverErr)
			}
			if updateErr := repositories.StudySessions.Update(ctx, recovered); updateErr != nil {
				return updateErr
			}
		case !errors.Is(activeErr, ErrNotFound):
			return activeErr
		}
		return repositories.StudySessions.Create(ctx, session)
	})
	return session, err
}

func (service *studySessionLifecycleService) Current(ctx context.Context) (learning.StudySession, error) {
	const operation = "get current study session"
	student, _, err := service.context(ctx, operation)
	if err != nil {
		return learning.StudySession{}, err
	}
	var session learning.StudySession
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		var getErr error
		session, getErr = repositories.StudySessions.ActiveByStudent(ctx, student.ID)
		return getErr
	})
	return session, err
}

func (service *studySessionLifecycleService) RecordActivity(ctx context.Context) (learning.StudySession, error) {
	return service.transition(ctx, "record study session activity", func(session learning.StudySession, timestamp learning.Timestamp) (learning.StudySession, error) {
		return session.RecordActivity(timestamp)
	})
}

func (service *studySessionLifecycleService) Stop(ctx context.Context) (learning.StudySession, error) {
	return service.transitionWithEvent(ctx, "stop study session", learning.StudyEventSessionCompleted, func(session learning.StudySession, timestamp learning.Timestamp) (learning.StudySession, error) {
		return session.Complete(timestamp)
	})
}

func (service *studySessionLifecycleService) Interrupt(ctx context.Context) (learning.StudySession, error) {
	return service.transition(ctx, "interrupt study session", func(session learning.StudySession, timestamp learning.Timestamp) (learning.StudySession, error) {
		return session.Interrupt(timestamp)
	})
}

// Recover leaves a recent active session open so a restarted process can
// continue it. Only a session beyond its captured idle timeout is finalized.
func (service *studySessionLifecycleService) Recover(ctx context.Context) (learning.StudySession, error) {
	return service.transition(ctx, "recover study session", func(session learning.StudySession, timestamp learning.Timestamp) (learning.StudySession, error) {
		stale, err := session.IsStale(timestamp)
		if err != nil || !stale {
			return session, err
		}
		return session.Recover(timestamp)
	})
}

func (service *studySessionLifecycleService) transition(ctx context.Context, operation string, apply func(learning.StudySession, learning.Timestamp) (learning.StudySession, error)) (learning.StudySession, error) {
	return service.transitionWithEvent(ctx, operation, "", apply)
}

func (service *studySessionLifecycleService) transitionWithEvent(ctx context.Context, operation string, eventType learning.StudyEventType, apply func(learning.StudySession, learning.Timestamp) (learning.StudySession, error)) (learning.StudySession, error) {
	student, timestamp, err := service.context(ctx, operation)
	if err != nil {
		return learning.StudySession{}, err
	}
	var updated learning.StudySession
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		active, getErr := repositories.StudySessions.ActiveByStudent(ctx, student.ID)
		if getErr != nil {
			return getErr
		}
		updated, getErr = apply(active, timestamp)
		if getErr != nil {
			return invalid(operation, getErr)
		}
		if updated == active {
			return nil
		}
		if updateErr := repositories.StudySessions.Update(ctx, updated); updateErr != nil {
			return updateErr
		}
		if eventType.Valid() {
			return recordStudyEvent(ctx, repositories.History, updated.StudentID, eventType, updated.ID, *updated.EndedAt,
				&updated.GoalID, &updated.CurriculumInstanceID, nil)
		}
		return nil
	})
	return updated, err
}

func (service *studySessionLifecycleService) context(ctx context.Context, operation string) (learning.Student, learning.Timestamp, error) {
	if service == nil || service.profiles == nil || service.unitOfWork == nil {
		return learning.Student{}, learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("study session dependencies are not configured"))
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.Student{}, learning.Timestamp{}, err
	}
	if service.now == nil {
		return learning.Student{}, learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("study session clock is not configured"))
	}
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Student{}, learning.Timestamp{}, Classify(ErrorUnavailable, operation, fmt.Errorf("read study session clock: %w", err))
	}
	return student, timestamp, nil
}

func (service *studySessionLifecycleService) withRepositories(ctx context.Context, operation string, work func(Repositories) error) error {
	err := service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		if repositories.StudySessions == nil || repositories.Goals == nil || repositories.CurriculumInstances == nil {
			return Classify(ErrorUnavailable, operation, errors.New("study session repositories are not configured"))
		}
		return work(repositories)
	})
	return repositoryError(operation, err)
}

func validateStudySessionScope(ctx context.Context, repositories Repositories, studentID, goalID, curriculumInstanceID learning.ID) error {
	goal, err := repositories.Goals.Get(ctx, goalID)
	if err != nil {
		return err
	}
	if goal.StudentID != studentID {
		return Classify(ErrorNotFound, "validate study session scope", errors.New("learning goal not found"))
	}
	if goal.Status != learning.GoalActive {
		return Classify(ErrorInvalidState, "validate study session scope", fmt.Errorf("learning goal is %q, want active", goal.Status))
	}
	instance, err := repositories.CurriculumInstances.Get(ctx, curriculumInstanceID)
	if err != nil {
		return err
	}
	if instance.StudentID != studentID || instance.GoalID != goalID {
		return Classify(ErrorNotFound, "validate study session scope", errors.New("curriculum instance not found for learning goal"))
	}
	if instance.Status != learning.CurriculumInstanceActive {
		return Classify(ErrorInvalidState, "validate study session scope", fmt.Errorf("curriculum instance is %q, want active", instance.Status))
	}
	return nil
}

func randomStudySessionID() (learning.ID, error) {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return learning.ID{}, err
	}
	return learning.NewID("session." + hex.EncodeToString(entropy[:]))
}
