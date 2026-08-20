package application

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/learning"
)

type studentService struct{ repository StudentRepository }

func NewStudentService(repository StudentRepository) StudentService {
	return &studentService{repository: repository}
}

func (service *studentService) Create(ctx context.Context, student learning.Student) error {
	const operation = "create student"
	if err := student.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Create(ctx, student))
}

func (service *studentService) Get(ctx context.Context, id learning.ID) (learning.Student, error) {
	const operation = "get student"
	if err := id.Validate(); err != nil {
		return learning.Student{}, invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return learning.Student{}, err
	}
	student, err := service.repository.Get(ctx, id)
	return student, repositoryError(operation, err)
}

func (service *studentService) Update(ctx context.Context, student learning.Student) error {
	const operation = "update student"
	if err := student.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Update(ctx, student))
}

type goalService struct{ repository GoalRepository }

func NewGoalService(repository GoalRepository) GoalService {
	return &goalService{repository: repository}
}

func (service *goalService) Create(ctx context.Context, goal learning.LearningGoal) error {
	const operation = "create learning goal"
	if err := goal.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Create(ctx, goal))
}

func (service *goalService) Get(ctx context.Context, id learning.ID) (learning.LearningGoal, error) {
	const operation = "get learning goal"
	if err := id.Validate(); err != nil {
		return learning.LearningGoal{}, invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return learning.LearningGoal{}, err
	}
	goal, err := service.repository.Get(ctx, id)
	return goal, repositoryError(operation, err)
}

func (service *goalService) List(ctx context.Context, studentID learning.ID) ([]learning.LearningGoal, error) {
	const operation = "list learning goals"
	if err := studentID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return nil, err
	}
	goals, err := service.repository.ListByStudent(ctx, studentID)
	return goals, repositoryError(operation, err)
}

func (service *goalService) Update(ctx context.Context, goal learning.LearningGoal) error {
	const operation = "update learning goal"
	if err := goal.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Update(ctx, goal))
}

type progressService struct {
	concepts ConceptStateRepository
	evidence EvidenceRepository
	mistakes MistakeRepository
}

func NewProgressService(concepts ConceptStateRepository, evidence EvidenceRepository, mistakes MistakeRepository) ProgressService {
	return &progressService{concepts: concepts, evidence: evidence, mistakes: mistakes}
}

func (service *progressService) Concept(ctx context.Context, studentID, conceptID learning.ID) (ConceptProgress, error) {
	const operation = "get concept progress"
	if err := validatePair("student", studentID, "concept", conceptID); err != nil {
		return ConceptProgress{}, invalid(operation, err)
	}
	if err := requireRepository(operation, service.concepts); err != nil {
		return ConceptProgress{}, err
	}
	if err := requireRepository(operation, service.evidence); err != nil {
		return ConceptProgress{}, err
	}
	if err := requireRepository(operation, service.mistakes); err != nil {
		return ConceptProgress{}, err
	}

	state, err := service.concepts.Get(ctx, studentID, conceptID)
	if err != nil {
		return ConceptProgress{}, repositoryError(operation, err)
	}
	evidence, err := service.evidence.ListByConcept(ctx, studentID, conceptID)
	if err != nil {
		return ConceptProgress{}, repositoryError(operation, err)
	}
	mistakes, err := service.mistakes.ListByConcept(ctx, studentID, conceptID)
	if err != nil {
		return ConceptProgress{}, repositoryError(operation, err)
	}
	return ConceptProgress{State: state, Evidence: evidence, Mistakes: mistakes}, nil
}

func (service *progressService) RecordEvidence(ctx context.Context, evidence learning.Evidence) error {
	const operation = "record learning evidence"
	if err := evidence.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.evidence); err != nil {
		return err
	}
	return repositoryError(operation, service.evidence.Append(ctx, evidence))
}

func (service *progressService) SaveConceptState(ctx context.Context, state learning.ConceptState) error {
	const operation = "save concept state"
	if err := state.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.concepts); err != nil {
		return err
	}
	return repositoryError(operation, service.concepts.Save(ctx, state))
}

type sessionService struct{ repository SessionRepository }

func NewSessionService(repository SessionRepository) SessionService {
	return &sessionService{repository: repository}
}

func (service *sessionService) Record(ctx context.Context, session learning.LearningSession) error {
	const operation = "record learning session"
	if err := session.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Append(ctx, session))
}

func (service *sessionService) Get(ctx context.Context, id learning.ID) (learning.LearningSession, error) {
	const operation = "get learning session"
	if err := id.Validate(); err != nil {
		return learning.LearningSession{}, invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return learning.LearningSession{}, err
	}
	session, err := service.repository.Get(ctx, id)
	return session, repositoryError(operation, err)
}

func (service *sessionService) List(ctx context.Context, studentID, goalID learning.ID) ([]learning.LearningSession, error) {
	const operation = "list learning sessions"
	if err := validatePair("student", studentID, "goal", goalID); err != nil {
		return nil, invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return nil, err
	}
	sessions, err := service.repository.ListByGoal(ctx, studentID, goalID)
	return sessions, repositoryError(operation, err)
}

type reviewService struct{ repository ReviewRepository }

func NewReviewService(repository ReviewRepository) ReviewService {
	return &reviewService{repository: repository}
}

func (service *reviewService) SaveSchedule(ctx context.Context, schedule learning.ReviewSchedule) error {
	const operation = "save review schedule"
	if err := schedule.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.SaveSchedule(ctx, schedule))
}

func (service *reviewService) Create(ctx context.Context, item learning.ReviewItem) error {
	const operation = "create review item"
	if err := item.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.CreateItem(ctx, item))
}

func (service *reviewService) Update(ctx context.Context, item learning.ReviewItem) error {
	const operation = "update review item"
	if err := item.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.UpdateItem(ctx, item))
}

func (service *reviewService) Due(ctx context.Context, studentID learning.ID, asOf learning.Timestamp) ([]learning.ReviewItem, error) {
	const operation = "list due reviews"
	if err := studentID.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if err := asOf.Validate(); err != nil {
		return nil, invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return nil, err
	}
	items, err := service.repository.ListDue(ctx, studentID, asOf)
	return items, repositoryError(operation, err)
}

type analyticsService struct{ repository AnalyticsRepository }

func NewAnalyticsService(repository AnalyticsRepository) AnalyticsService {
	return &analyticsService{repository: repository}
}

func (service *analyticsService) Record(ctx context.Context, snapshot learning.AnalyticsSnapshot) error {
	const operation = "record analytics snapshot"
	if err := snapshot.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Append(ctx, snapshot))
}

func (service *analyticsService) Latest(ctx context.Context, studentID learning.ID) (learning.AnalyticsSnapshot, error) {
	const operation = "get latest analytics snapshot"
	if err := studentID.Validate(); err != nil {
		return learning.AnalyticsSnapshot{}, invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return learning.AnalyticsSnapshot{}, err
	}
	snapshot, err := service.repository.Latest(ctx, studentID)
	return snapshot, repositoryError(operation, err)
}

type dailyPlanService struct{ repository DailyPlanRepository }

func NewDailyPlanService(repository DailyPlanRepository) DailyPlanService {
	return &dailyPlanService{repository: repository}
}

func (service *dailyPlanService) Save(ctx context.Context, plan learning.DailyPlan) error {
	const operation = "save daily plan"
	if err := plan.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return err
	}
	return repositoryError(operation, service.repository.Save(ctx, plan))
}

func (service *dailyPlanService) ForDate(ctx context.Context, studentID, goalID learning.ID, date learning.Timestamp) (learning.DailyPlan, error) {
	const operation = "get daily plan"
	if err := validatePair("student", studentID, "goal", goalID); err != nil {
		return learning.DailyPlan{}, invalid(operation, err)
	}
	if err := date.Validate(); err != nil {
		return learning.DailyPlan{}, invalid(operation, err)
	}
	if err := requireRepository(operation, service.repository); err != nil {
		return learning.DailyPlan{}, err
	}
	plan, err := service.repository.ForDate(ctx, studentID, goalID, date)
	return plan, repositoryError(operation, err)
}

func validatePair(firstName string, first learning.ID, secondName string, second learning.ID) error {
	if err := first.Validate(); err != nil {
		return fmt.Errorf("%s: %w", firstName, err)
	}
	if err := second.Validate(); err != nil {
		return fmt.Errorf("%s: %w", secondName, err)
	}
	return nil
}
