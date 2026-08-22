package application

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type RecalculationRequest struct {
	DryRun   bool
	BackupID string
}

type AlgorithmVersionSummary struct {
	Mastery   []string
	Retention []string
	DailyPlan []string
}

type RecalculationImpact struct {
	DryRun                 bool
	Applied                bool
	BackupID               string
	CalculatedAt           learning.Timestamp
	Previous               AlgorithmVersionSummary
	Target                 AlgorithmVersionSummary
	EvidenceRecords        int
	ConceptsScanned        int
	ConceptStatesChanged   int
	RetentionStatesChanged int
	ReviewSchedulesChanged int
	ReviewItemsChanged     int
	DailyPlansChanged      int
}

type MaintenanceRecalculationService interface {
	Recalculate(context.Context, RecalculationRequest) (RecalculationImpact, error)
}

type MaintenanceRecalculationOption func(*maintenanceRecalculationService)

func WithMaintenanceRecalculationClock(now func() time.Time) MaintenanceRecalculationOption {
	return func(service *maintenanceRecalculationService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithLearningAlgorithmSuite(suite LearningAlgorithmSuite) MaintenanceRecalculationOption {
	return func(service *maintenanceRecalculationService) { service.algorithms = suite }
}

type maintenanceRecalculationService struct {
	profiles   ProfileService
	mastery    MasteryPolicyService
	unitOfWork UnitOfWork
	now        func() time.Time
	algorithms LearningAlgorithmSuite
}

func NewMaintenanceRecalculationService(profiles ProfileService, mastery MasteryPolicyService, unitOfWork UnitOfWork, options ...MaintenanceRecalculationOption) MaintenanceRecalculationService {
	service := &maintenanceRecalculationService{
		profiles: profiles, mastery: mastery, unitOfWork: unitOfWork, now: time.Now,
		algorithms: DefaultLearningAlgorithmSuite(),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *maintenanceRecalculationService) Recalculate(ctx context.Context, request RecalculationRequest) (RecalculationImpact, error) {
	const operation = "recalculate versioned learning state"
	if service == nil || service.profiles == nil || service.mastery == nil || service.unitOfWork == nil || service.now == nil {
		return RecalculationImpact{}, Classify(ErrorUnavailable, operation, errors.New("maintenance recalculation dependencies are not configured"))
	}
	if err := service.algorithms.Validate(); err != nil {
		return RecalculationImpact{}, invalid(operation, err)
	}
	if !request.DryRun && strings.TrimSpace(request.BackupID) == "" {
		return RecalculationImpact{}, invalid(operation, errors.New("backup id is required before applying recalculation"))
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return RecalculationImpact{}, repositoryError(operation, err)
	}
	threshold, err := service.mastery.Show(ctx, nil)
	if err != nil {
		return RecalculationImpact{}, repositoryError(operation, err)
	}
	calculatedAt, err := learning.NewTimestamp(service.now())
	if err != nil {
		return RecalculationImpact{}, invalid(operation, err)
	}
	impact := RecalculationImpact{
		DryRun: request.DryRun, Applied: !request.DryRun, BackupID: request.BackupID, CalculatedAt: calculatedAt,
		Target: AlgorithmVersionSummary{
			Mastery: []string{service.algorithms.Mastery.Version()}, Retention: []string{service.algorithms.Retention.Version()},
			DailyPlan: []string{service.algorithms.DailyPlan.Version()},
		},
	}

	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		if err := requireMaintenanceRepositories(operation, repositories); err != nil {
			return err
		}
		return service.recalculate(ctx, repositories, student, threshold, calculatedAt, request.DryRun, &impact)
	})
	if err != nil {
		return RecalculationImpact{}, repositoryError(operation, err)
	}
	impact.Previous.Mastery = sortedVersions(impact.Previous.Mastery)
	impact.Previous.Retention = sortedVersions(impact.Previous.Retention)
	impact.Previous.DailyPlan = sortedVersions(impact.Previous.DailyPlan)
	return impact, nil
}

func requireMaintenanceRepositories(operation string, repositories Repositories) error {
	if repositories.Goals == nil || repositories.CurriculumInstances == nil || repositories.Curricula == nil ||
		repositories.InstanceConceptStates == nil || repositories.Evidence == nil || repositories.Retention == nil ||
		repositories.Reviews == nil || repositories.DailyPlans == nil || repositories.Mistakes == nil || repositories.History == nil {
		return Classify(ErrorUnavailable, operation, errors.New("maintenance repositories are not configured"))
	}
	return nil
}

type recalculationData struct {
	instances         []learning.CurriculumInstance
	states            map[learning.ID][]learning.InstanceConceptState
	evidence          []learning.Evidence
	evidenceByConcept map[learning.ID][]learning.Evidence
	retention         map[learning.ID]learning.RetentionState
	reviews           []learning.ReviewItem
	goals             []learning.LearningGoal
}

func (service *maintenanceRecalculationService) recalculate(ctx context.Context, repositories Repositories, student learning.Student,
	threshold learning.ResolvedMasteryThreshold, calculatedAt learning.Timestamp, dryRun bool, impact *RecalculationImpact) error {
	data, err := loadRecalculationData(ctx, repositories, student.ID)
	if err != nil {
		return err
	}
	if err := addMissingEvidenceStates(ctx, repositories, &data); err != nil {
		return err
	}
	impact.EvidenceRecords = len(data.evidence)
	for _, states := range data.states {
		for _, state := range states {
			impact.Previous.Mastery = appendVersion(impact.Previous.Mastery, state.MasteryAlgorithmVersion)
		}
	}
	for _, state := range data.retention {
		impact.Previous.Retention = appendVersion(impact.Previous.Retention, state.AlgorithmVersion)
	}

	conceptIDs := recalculationConceptIDs(data)
	impact.ConceptsScanned = len(conceptIDs)
	instanceByID := make(map[learning.ID]learning.CurriculumInstance, len(data.instances))
	for _, instance := range data.instances {
		instanceByID[instance.ID] = instance
	}
	projectedRetention := make(map[learning.ID]learning.RetentionState, len(conceptIDs))
	for _, conceptID := range conceptIDs {
		items := data.evidenceByConcept[conceptID]
		mastery, err := service.algorithms.Mastery.Calculate(student.ID, conceptID, items)
		if err != nil {
			return Classify(ErrorInvalidState, "calculate maintenance mastery", err)
		}
		if mastery.PolicyVersion != service.algorithms.Mastery.Version() {
			return Classify(ErrorInvalidState, "calculate maintenance mastery", errors.New("mastery result version does not match configured algorithm"))
		}
		calculation, err := service.algorithms.Retention.Calculate(mastery, items, calculatedAt)
		if err != nil {
			return Classify(ErrorInvalidState, "calculate maintenance retention", err)
		}
		if calculation.State.AlgorithmVersion != service.algorithms.Retention.Version() {
			return Classify(ErrorInvalidState, "calculate maintenance retention", errors.New("retention result version does not match configured algorithm"))
		}
		projectedRetention[conceptID] = calculation.State
		if previous, exists := data.retention[conceptID]; !exists || !reflect.DeepEqual(previous, calculation.State) {
			impact.RetentionStatesChanged++
			if !dryRun {
				if err := repositories.Retention.Save(ctx, calculation.State); err != nil {
					return err
				}
			}
		}

		for instanceID, states := range data.states {
			for index, previous := range states {
				if previous.ConceptID != conceptID {
					continue
				}
				progression, err := learning.ApplyProgressionV1(previous, mastery, threshold, instanceByID[instanceID].CreatedAt, calculatedAt)
				if err != nil {
					return Classify(ErrorInvalidState, "apply maintenance progression", err)
				}
				projected := progression.State
				retentionProjection, err := learning.ApplyRetentionV1(projected, calculation.State)
				if err != nil {
					return Classify(ErrorInvalidState, "apply maintenance retention", err)
				}
				projected = retentionProjection.State
				if instanceStateEquivalent(previous, projected) {
					projected.UpdatedAt = previous.UpdatedAt
				}
				data.states[instanceID][index] = projected
				if !reflect.DeepEqual(previous, projected) {
					impact.ConceptStatesChanged++
					if !dryRun {
						if err := repositories.InstanceConceptStates.Save(ctx, projected); err != nil {
							return err
						}
					}
				}
			}
		}
	}

	projectedReviews, err := recalculateReviewSchedules(ctx, repositories, student.ID, calculatedAt, data, projectedRetention, dryRun, impact)
	if err != nil {
		return err
	}
	return service.recalculateDailyPlan(ctx, repositories, student, threshold, calculatedAt, data, projectedRetention, projectedReviews, dryRun, impact)
}

func addMissingEvidenceStates(ctx context.Context, repositories Repositories, data *recalculationData) error {
	for _, instance := range data.instances {
		planning, err := repositories.Curricula.PlanningConcepts(ctx, instance.Curriculum)
		if err != nil {
			return err
		}
		present := make(map[learning.ID]struct{}, len(data.states[instance.ID]))
		for _, state := range data.states[instance.ID] {
			present[state.ConceptID] = struct{}{}
		}
		for _, concept := range planning {
			if _, exists := present[concept.ConceptID]; exists || len(data.evidenceByConcept[concept.ConceptID]) == 0 {
				continue
			}
			state, err := learning.NewInstanceConceptState(instance, concept.ConceptID, instance.CreatedAt)
			if err != nil {
				return Classify(ErrorInvalidState, "prepare missing maintenance concept state", err)
			}
			data.states[instance.ID] = append(data.states[instance.ID], state)
		}
	}
	return nil
}

func loadRecalculationData(ctx context.Context, repositories Repositories, studentID learning.ID) (recalculationData, error) {
	data := recalculationData{states: make(map[learning.ID][]learning.InstanceConceptState), evidenceByConcept: make(map[learning.ID][]learning.Evidence), retention: make(map[learning.ID]learning.RetentionState)}
	var err error
	data.instances, err = repositories.CurriculumInstances.ListByStudent(ctx, studentID)
	if err != nil {
		return data, err
	}
	for _, instance := range data.instances {
		states, loadErr := repositories.InstanceConceptStates.ListByInstance(ctx, instance.ID)
		if loadErr != nil {
			return data, loadErr
		}
		data.states[instance.ID] = states
	}
	data.evidence, err = repositories.Evidence.ListByStudent(ctx, studentID)
	if err != nil {
		return data, err
	}
	for _, item := range data.evidence {
		data.evidenceByConcept[item.ConceptID] = append(data.evidenceByConcept[item.ConceptID], item)
	}
	retention, err := repositories.Retention.ListByStudent(ctx, studentID)
	if err != nil {
		return data, err
	}
	for _, item := range retention {
		data.retention[item.ConceptID] = item
	}
	data.reviews, err = repositories.Reviews.ListByStudent(ctx, studentID)
	if err != nil {
		return data, err
	}
	data.goals, err = repositories.Goals.ListByStudent(ctx, studentID)
	return data, err
}

func recalculationConceptIDs(data recalculationData) []learning.ID {
	known := make(map[learning.ID]struct{})
	for conceptID := range data.evidenceByConcept {
		known[conceptID] = struct{}{}
	}
	for conceptID := range data.retention {
		known[conceptID] = struct{}{}
	}
	for _, states := range data.states {
		for _, state := range states {
			known[state.ConceptID] = struct{}{}
		}
	}
	result := make([]learning.ID, 0, len(known))
	for conceptID := range known {
		result = append(result, conceptID)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func instanceStateEquivalent(left, right learning.InstanceConceptState) bool {
	right.UpdatedAt = left.UpdatedAt
	return reflect.DeepEqual(left, right)
}

func appendVersion(versions []string, version string) []string {
	if strings.TrimSpace(version) == "" {
		version = learning.UnversionedDerivedStateVersion
	}
	for _, existing := range versions {
		if existing == version {
			return versions
		}
	}
	return append(versions, version)
}

func sortedVersions(versions []string) []string {
	result := append([]string(nil), versions...)
	sort.Strings(result)
	return result
}
