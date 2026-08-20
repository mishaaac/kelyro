package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type LearnerSetupOption func(*learnerSetupService)

func WithLearnerSetupClock(now func() time.Time) LearnerSetupOption {
	return func(service *learnerSetupService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithDevelopmentSetupReset(allowed bool) LearnerSetupOption {
	return func(service *learnerSetupService) { service.developmentReset = allowed }
}

type learnerSetupService struct {
	profiles         ProfileService
	onboarding       OnboardingService
	instances        CurriculumInstanceService
	diagnostics      DiagnosticService
	unitOfWork       UnitOfWork
	curriculum       learning.Curriculum
	diagnostic       learning.Diagnostic
	now              func() time.Time
	developmentReset bool
}

func NewLearnerSetupService(profiles ProfileService, onboarding OnboardingService, instances CurriculumInstanceService,
	diagnostics DiagnosticService, unitOfWork UnitOfWork, curriculum learning.Curriculum, diagnostic learning.Diagnostic,
	options ...LearnerSetupOption,
) LearnerSetupService {
	service := &learnerSetupService{profiles: profiles, onboarding: onboarding, instances: instances, diagnostics: diagnostics,
		unitOfWork: unitOfWork, curriculum: curriculum, diagnostic: diagnostic, now: time.Now}
	for _, configure := range options {
		if configure != nil {
			configure(service)
		}
	}
	return service
}

func (service *learnerSetupService) Show(ctx context.Context) (LearnerSetupView, error) {
	const operation = "show learner setup"
	student, err := service.student(ctx, operation)
	if err != nil {
		return LearnerSetupView{}, err
	}
	setup, err := service.loadOrCreate(ctx, operation, student.ID)
	if err != nil {
		return LearnerSetupView{}, err
	}
	if setup.Status == learning.SetupInitializing {
		if _, err := service.finalize(ctx, operation, setup); err != nil {
			return LearnerSetupView{}, err
		}
		setup, err = service.load(ctx, operation, student.ID)
		if err != nil {
			return LearnerSetupView{}, err
		}
	}
	if setup.Status == learning.SetupAwaitingDiagnostic {
		diagnostic, resumeErr := service.diagnostics.Resume(ctx, *setup.DiagnosticAttemptID, service.diagnostic)
		if resumeErr != nil {
			return LearnerSetupView{}, resumeErr
		}
		if diagnostic.Attempt.Status != learning.DiagnosticInProgress {
			if _, err := service.finalize(ctx, operation, setup); err != nil {
				return LearnerSetupView{}, err
			}
			setup, err = service.load(ctx, operation, student.ID)
			if err != nil {
				return LearnerSetupView{}, err
			}
		}
	}
	return service.view(ctx, operation, setup)
}

func (service *learnerSetupService) Start(ctx context.Context) (LearnerSetupView, error) {
	view, err := service.Show(ctx)
	if err != nil {
		return LearnerSetupView{}, err
	}
	if view.Setup.Status != learning.SetupAwaitingOnboarding {
		return view, nil
	}
	onboarding, err := service.onboarding.Start(ctx)
	if err != nil {
		return LearnerSetupView{}, err
	}
	view.Onboarding = &onboarding
	return view, nil
}

func (service *learnerSetupService) SubmitOnboarding(ctx context.Context, answer string) (LearnerSetupView, error) {
	const operation = "submit learner setup onboarding answer"
	view, err := service.requireStatus(ctx, operation, learning.SetupAwaitingOnboarding)
	if err != nil {
		return LearnerSetupView{}, err
	}
	onboarding, err := service.onboarding.Submit(ctx, answer)
	if err != nil {
		return LearnerSetupView{}, err
	}
	view.Onboarding = &onboarding
	return view, nil
}

func (service *learnerSetupService) Back(ctx context.Context) (LearnerSetupView, error) {
	const operation = "go back in learner setup"
	view, err := service.requireStatus(ctx, operation, learning.SetupAwaitingOnboarding)
	if err != nil {
		return LearnerSetupView{}, err
	}
	onboarding, err := service.onboarding.Back(ctx)
	if err != nil {
		return LearnerSetupView{}, err
	}
	view.Onboarding = &onboarding
	return view, nil
}

func (service *learnerSetupService) Cancel(ctx context.Context) (LearnerSetupView, error) {
	const operation = "cancel learner setup onboarding"
	view, err := service.requireStatus(ctx, operation, learning.SetupAwaitingOnboarding)
	if err != nil {
		return LearnerSetupView{}, err
	}
	onboarding, err := service.onboarding.Cancel(ctx)
	if err != nil {
		return LearnerSetupView{}, err
	}
	view.Onboarding = &onboarding
	return view, nil
}

func (service *learnerSetupService) Confirm(ctx context.Context) (LearnerSetupView, error) {
	const operation = "confirm learner setup"
	view, err := service.Show(ctx)
	if err != nil {
		return LearnerSetupView{}, err
	}
	if view.Setup.Status != learning.SetupAwaitingOnboarding {
		return view, nil
	}
	confirmation, err := service.onboarding.Confirm(ctx)
	if err != nil {
		return LearnerSetupView{}, err
	}
	instance, err := service.findOrCreateInstance(ctx, confirmation.Goal)
	if err != nil {
		return LearnerSetupView{}, err
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return LearnerSetupView{}, err
	}
	setup := view.Setup
	optIn := confirmation.View.Answers[OnboardingDiagnosticOptInQuestion] == "yes"
	if optIn {
		diagnostic, startErr := service.diagnostics.Start(ctx, instance.ID, service.diagnostic)
		if startErr != nil {
			return LearnerSetupView{}, startErr
		}
		setup, err = setup.AwaitDiagnostic(instance.ID, diagnostic.Attempt.ID, timestamp)
	} else {
		setup, err = setup.BeginInitialization(instance.ID, nil, false, timestamp)
	}
	if err != nil {
		return LearnerSetupView{}, invalid(operation, err)
	}
	if err := service.save(ctx, operation, setup); err != nil {
		return LearnerSetupView{}, err
	}
	if !optIn {
		if _, err := service.finalize(ctx, operation, setup); err != nil {
			return LearnerSetupView{}, err
		}
	}
	return service.Show(ctx)
}

func (service *learnerSetupService) SubmitDiagnostic(ctx context.Context, answers []string) (LearnerSetupView, error) {
	const operation = "submit learner setup diagnostic answer"
	view, err := service.requireStatus(ctx, operation, learning.SetupAwaitingDiagnostic)
	if err != nil {
		return LearnerSetupView{}, err
	}
	diagnostic, err := service.diagnostics.Submit(ctx, *view.Setup.DiagnosticAttemptID, service.diagnostic, answers)
	if err != nil {
		return LearnerSetupView{}, err
	}
	if diagnostic.Attempt.Status != learning.DiagnosticInProgress {
		if _, err := service.finalize(ctx, operation, view.Setup); err != nil {
			return LearnerSetupView{}, err
		}
	}
	return service.Show(ctx)
}

func (service *learnerSetupService) SkipDiagnostic(ctx context.Context) (LearnerSetupView, error) {
	const operation = "skip learner setup diagnostic"
	view, err := service.requireStatus(ctx, operation, learning.SetupAwaitingDiagnostic)
	if err != nil {
		return LearnerSetupView{}, err
	}
	if _, err := service.diagnostics.Skip(ctx, *view.Setup.CurriculumInstanceID, service.diagnostic); err != nil {
		return LearnerSetupView{}, err
	}
	if _, err := service.finalize(ctx, operation, view.Setup); err != nil {
		return LearnerSetupView{}, err
	}
	return service.Show(ctx)
}

func (service *learnerSetupService) ResetDevelopment(ctx context.Context) (LearnerSetupView, error) {
	const operation = "reset learner setup"
	if !service.developmentReset {
		return LearnerSetupView{}, Classify(ErrorInvalidState, operation, errors.New("setup reset is available only in development or demo builds"))
	}
	student, err := service.student(ctx, operation)
	if err != nil {
		return LearnerSetupView{}, err
	}
	if err := service.withRepositories(ctx, operation, func(repositories Repositories) error {
		return repositories.Setup.ResetDevelopment(ctx, student.ID)
	}); err != nil {
		return LearnerSetupView{}, err
	}
	return service.Start(ctx)
}

func (service *learnerSetupService) finalize(ctx context.Context, operation string, expected learning.LearnerSetup) (learning.LearnerSetup, error) {
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return learning.LearnerSetup{}, err
	}
	completed := expected
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		setup, getErr := repositories.Setup.Get(ctx, expected.StudentID)
		if getErr != nil {
			return getErr
		}
		if setup.Status == learning.SetupCompleted {
			completed = setup
			return nil
		}
		if setup.CurriculumInstanceID == nil {
			return Classify(ErrorInvalidState, operation, errors.New("setup has no curriculum instance"))
		}
		instance, getErr := repositories.CurriculumInstances.Get(ctx, *setup.CurriculumInstanceID)
		if getErr != nil {
			return getErr
		}
		if instance.StudentID != setup.StudentID || instance.Curriculum != service.curriculum.Reference {
			return Classify(ErrorInvalidState, operation, errors.New("setup curriculum instance does not match fixture"))
		}
		if setup.DiagnosticOptIn {
			if setup.DiagnosticAttemptID == nil {
				return Classify(ErrorInvalidState, operation, errors.New("setup has no diagnostic attempt"))
			}
			attempt, getErr := repositories.Diagnostics.Get(ctx, *setup.DiagnosticAttemptID)
			if getErr != nil {
				return getErr
			}
			if attempt.CurriculumInstanceID != instance.ID || attempt.Status == learning.DiagnosticInProgress {
				return Classify(ErrorInvalidState, operation, errors.New("diagnostic is not terminal for setup curriculum"))
			}
		}
		initializing, transitionErr := setup.BeginInitialization(instance.ID, setup.DiagnosticAttemptID, setup.DiagnosticOptIn, timestamp)
		if transitionErr != nil {
			return Classify(ErrorInvalidState, operation, transitionErr)
		}
		concepts, listErr := repositories.Curricula.Concepts(ctx, instance.Curriculum)
		if listErr != nil {
			return listErr
		}
		for _, concept := range concepts {
			if _, getErr := repositories.InstanceConceptStates.Get(ctx, instance.ID, concept.ID); getErr == nil {
				continue
			} else if !errors.Is(getErr, ErrNotFound) {
				return getErr
			}
			state, createErr := learning.NewInstanceConceptState(instance, concept.ID, timestamp)
			if createErr != nil {
				return Classify(ErrorInvalidState, operation, createErr)
			}
			if saveErr := repositories.InstanceConceptStates.Save(ctx, state); saveErr != nil {
				return saveErr
			}
		}
		completed, transitionErr = initializing.Complete(timestamp)
		if transitionErr != nil {
			return Classify(ErrorInvalidState, operation, transitionErr)
		}
		return repositories.Setup.Save(ctx, completed)
	})
	return completed, err
}

func (service *learnerSetupService) view(ctx context.Context, operation string, setup learning.LearnerSetup) (LearnerSetupView, error) {
	view := LearnerSetupView{Setup: setup}
	if setup.CurriculumInstanceID != nil {
		instance, err := service.instances.Get(ctx, *setup.CurriculumInstanceID)
		if err != nil {
			return LearnerSetupView{}, err
		}
		view.Instance = &instance
	}
	switch setup.Status {
	case learning.SetupAwaitingOnboarding:
		onboarding, err := service.onboarding.Show(ctx)
		if err != nil {
			return LearnerSetupView{}, err
		}
		view.Onboarding = &onboarding
	case learning.SetupAwaitingDiagnostic:
		diagnostic, err := service.diagnostics.Resume(ctx, *setup.DiagnosticAttemptID, service.diagnostic)
		if err != nil {
			return LearnerSetupView{}, err
		}
		view.Diagnostic = &diagnostic
	case learning.SetupInitializing:
		return LearnerSetupView{}, Classify(ErrorInvalidState, operation, errors.New("setup initialization did not complete"))
	}
	return view, nil
}

func (service *learnerSetupService) findOrCreateInstance(ctx context.Context, goal learning.LearningGoal) (learning.CurriculumInstance, error) {
	instances, err := service.instances.List(ctx)
	if err != nil {
		return learning.CurriculumInstance{}, err
	}
	for _, instance := range instances {
		if instance.GoalID == goal.ID && instance.Curriculum == service.curriculum.Reference {
			return instance, nil
		}
	}
	return service.instances.Create(ctx, goal.ID, service.curriculum, learning.CurriculumSourceFixture)
}

func (service *learnerSetupService) requireStatus(ctx context.Context, operation string, status learning.LearnerSetupStatus) (LearnerSetupView, error) {
	view, err := service.Show(ctx)
	if err != nil {
		return LearnerSetupView{}, err
	}
	if view.Setup.Status != status {
		return LearnerSetupView{}, Classify(ErrorInvalidState, operation, fmt.Errorf("setup status is %q, want %q", view.Setup.Status, status))
	}
	return view, nil
}

func (service *learnerSetupService) loadOrCreate(ctx context.Context, operation string, studentID learning.ID) (learning.LearnerSetup, error) {
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return learning.LearnerSetup{}, err
	}
	var setup learning.LearnerSetup
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		loaded, getErr := repositories.Setup.Get(ctx, studentID)
		if getErr == nil {
			setup = loaded
			return nil
		}
		if !errors.Is(getErr, ErrNotFound) {
			return getErr
		}
		created, createErr := learning.NewLearnerSetup(studentID, timestamp)
		if createErr != nil {
			return Classify(ErrorInvalidState, operation, createErr)
		}
		if saveErr := repositories.Setup.Save(ctx, created); saveErr != nil {
			return saveErr
		}
		setup = created
		return nil
	})
	return setup, err
}

func (service *learnerSetupService) load(ctx context.Context, operation string, studentID learning.ID) (learning.LearnerSetup, error) {
	var setup learning.LearnerSetup
	err := service.withRepositories(ctx, operation, func(repositories Repositories) error {
		loaded, getErr := repositories.Setup.Get(ctx, studentID)
		setup = loaded
		return getErr
	})
	return setup, err
}

func (service *learnerSetupService) save(ctx context.Context, operation string, setup learning.LearnerSetup) error {
	return service.withRepositories(ctx, operation, func(repositories Repositories) error { return repositories.Setup.Save(ctx, setup) })
}

func (service *learnerSetupService) student(ctx context.Context, operation string) (learning.Student, error) {
	if service == nil || service.profiles == nil || service.onboarding == nil || service.instances == nil || service.diagnostics == nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, errors.New("learner setup dependencies are not configured"))
	}
	if err := service.curriculum.Validate(); err != nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, fmt.Errorf("invalid setup curriculum: %w", err))
	}
	if err := service.diagnostic.Validate(); err != nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, fmt.Errorf("invalid setup diagnostic: %w", err))
	}
	if service.diagnostic.Curriculum != service.curriculum.Reference {
		return learning.Student{}, Classify(ErrorUnavailable, operation, errors.New("setup diagnostic does not match curriculum"))
	}
	return service.profiles.Show(ctx)
}

func (service *learnerSetupService) withRepositories(ctx context.Context, operation string, work func(Repositories) error) error {
	if service == nil || service.unitOfWork == nil {
		return Classify(ErrorUnavailable, operation, errors.New("learning transaction is not configured"))
	}
	return repositoryError(operation, service.unitOfWork.WithinTransaction(ctx, work))
}

func (service *learnerSetupService) timestamp(operation string) (learning.Timestamp, error) {
	if service == nil || service.now == nil {
		return learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("learner setup clock is not configured"))
	}
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Timestamp{}, invalid(operation, err)
	}
	return timestamp, nil
}
