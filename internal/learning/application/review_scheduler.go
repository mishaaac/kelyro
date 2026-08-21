package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

const skippedReviewDelay = 24 * time.Hour

type ReviewSchedulerOption func(*reviewSchedulerService)

func WithReviewSchedulerClock(now func() time.Time) ReviewSchedulerOption {
	return func(service *reviewSchedulerService) {
		if now != nil {
			service.now = now
		}
	}
}

type reviewSchedulerService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
}

func NewReviewSchedulerService(profiles ProfileService, unitOfWork UnitOfWork, options ...ReviewSchedulerOption) ReviewSchedulerService {
	service := &reviewSchedulerService{profiles: profiles, unitOfWork: unitOfWork, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *reviewSchedulerService) List(ctx context.Context, dueOnly bool) (ReviewQueueView, error) {
	const operation = "list scheduled reviews"
	student, generatedAt, err := service.studentAndTime(ctx, operation)
	if err != nil {
		return ReviewQueueView{}, err
	}
	var view ReviewQueueView
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		contexts, err := loadReviewConceptContexts(ctx, repositories, student.ID)
		if err != nil {
			return err
		}
		items, err := repositories.Reviews.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		byConcept := groupReviewItems(items)
		conceptIDs := make([]learning.ID, 0, len(contexts))
		for conceptID := range contexts {
			conceptIDs = append(conceptIDs, conceptID)
		}
		sort.Slice(conceptIDs, func(i, j int) bool { return conceptIDs[i].String() < conceptIDs[j].String() })
		calculated := make(map[learning.ID]learning.RetentionState, len(conceptIDs))
		for _, conceptID := range conceptIDs {
			state, err := recalculateRetention(ctx, repositories, student.ID, conceptID, generatedAt)
			if err != nil {
				return err
			}
			calculated[state.ConceptID] = state
			context := contexts[state.ConceptID]
			if err := projectReviewDue(ctx, repositories, context.instances, state); err != nil {
				return err
			}
			schedule, scheduled, err := learning.ScheduleReviewV1(learning.ReviewSchedulingInput{
				Concept: context.state, Retention: state, History: byConcept[state.ConceptID],
				CriticalPrerequisite: context.critical, ScheduledAt: generatedAt,
			})
			if err != nil {
				return Classify(ErrorInvalidState, operation, err)
			}
			if !scheduled {
				continue
			}
			if err := repositories.Reviews.SaveSchedule(ctx, schedule); err != nil {
				return err
			}
			pending := findPendingReview(byConcept[state.ConceptID])
			if pending != nil {
				var aligned learning.ReviewItem
				if pending.AlgorithmVersion == learning.ReviewSchedulerVersion {
					aligned, err = learning.AlignReviewItemV1(*pending, schedule)
				} else {
					aligned, err = learning.NewReviewItemV1(pending.ID, schedule, pending.CreatedAt)
				}
				if err != nil {
					return Classify(ErrorInvalidState, operation, err)
				}
				if aligned != *pending {
					if err := repositories.Reviews.UpdateItem(ctx, aligned); err != nil {
						return err
					}
				}
				continue
			}
			id, err := learning.NewReviewItemIDV1(student.ID, state.ConceptID, schedule.DueAt, len(byConcept[state.ConceptID]))
			if err != nil {
				return Classify(ErrorInvalidState, operation, err)
			}
			item, err := learning.NewReviewItemV1(id, schedule, generatedAt)
			if err != nil {
				return Classify(ErrorInvalidState, operation, err)
			}
			if err := repositories.Reviews.CreateItem(ctx, item); err != nil {
				return err
			}
		}
		items, err = repositories.Reviews.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		candidates := reviewQueueCandidates(items, calculated)
		queue, err := learning.BuildDueReviewQueueV1(student.Profile.Availability, candidates, generatedAt)
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		view = reviewQueueView(student, items, candidates, queue, dueOnly)
		return nil
	})
	if err != nil {
		return ReviewQueueView{}, repositoryError(operation, err)
	}
	return view, nil
}

func (service *reviewSchedulerService) Postpone(ctx context.Context, itemID learning.ID, until learning.Timestamp) (learning.ReviewItem, error) {
	const operation = "postpone review"
	student, now, err := service.studentAndTime(ctx, operation)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	var postponed learning.ReviewItem
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		item, err := repositories.Reviews.GetItem(ctx, itemID)
		if err != nil {
			return err
		}
		if item.StudentID != student.ID {
			return Classify(ErrorNotFound, operation, errors.New("review not found"))
		}
		postponed, err = item.Postpone(until, now)
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		if err := repositories.Reviews.UpdateItem(ctx, postponed); err != nil {
			return err
		}
		schedule, err := repositories.Reviews.GetSchedule(ctx, student.ID, item.ConceptID)
		if err != nil {
			return err
		}
		schedule.DueAt, schedule.UpdatedAt = until, now
		return repositories.Reviews.SaveSchedule(ctx, schedule)
	})
	if err != nil {
		return learning.ReviewItem{}, repositoryError(operation, err)
	}
	return postponed, nil
}

func (service *reviewSchedulerService) Skip(ctx context.Context, itemID learning.ID) (learning.ReviewItem, error) {
	const operation = "skip review"
	student, now, err := service.studentAndTime(ctx, operation)
	if err != nil {
		return learning.ReviewItem{}, err
	}
	var skipped learning.ReviewItem
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		item, err := repositories.Reviews.GetItem(ctx, itemID)
		if err != nil {
			return err
		}
		if item.StudentID != student.ID {
			return Classify(ErrorNotFound, operation, errors.New("review not found"))
		}
		skipped, err = item.Skip(now)
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		if err := repositories.Reviews.UpdateItem(ctx, skipped); err != nil {
			return err
		}
		schedule, err := repositories.Reviews.GetSchedule(ctx, student.ID, item.ConceptID)
		if err != nil {
			return err
		}
		baseDue := item.DueAt
		if now.After(baseDue) {
			baseDue = now
		}
		deferredDue, _ := learning.NewTimestamp(baseDue.Time().Add(skippedReviewDelay))
		original := schedule
		original.DueAt, original.UpdatedAt = item.DueAt, now
		items, err := repositories.Reviews.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		id, err := learning.NewReviewItemIDV1(student.ID, item.ConceptID, deferredDue, len(items))
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		next, err := learning.NewReviewItemV1(id, original, now)
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		next, err = next.Postpone(deferredDue, now)
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		if err := repositories.Reviews.CreateItem(ctx, next); err != nil {
			return err
		}
		schedule.DueAt, schedule.UpdatedAt = deferredDue, now
		return repositories.Reviews.SaveSchedule(ctx, schedule)
	})
	if err != nil {
		return learning.ReviewItem{}, repositoryError(operation, err)
	}
	return skipped, nil
}

func (service *reviewSchedulerService) RecordOutcome(ctx context.Context, itemID learning.ID, score learning.MasteryScore) (ReviewOutcomeUpdate, error) {
	const operation = "record review outcome"
	student, now, err := service.studentAndTime(ctx, operation)
	if err != nil {
		return ReviewOutcomeUpdate{}, err
	}
	if err := score.Validate(); err != nil {
		return ReviewOutcomeUpdate{}, invalid(operation, err)
	}
	var update ReviewOutcomeUpdate
	err = service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		item, err := repositories.Reviews.GetItem(ctx, itemID)
		if err != nil {
			return err
		}
		if item.StudentID != student.ID {
			return Classify(ErrorNotFound, operation, errors.New("review not found"))
		}
		if item.Status == learning.ReviewCompleted {
			if item.Score != nil && item.Score.Value() == score.Value() {
				update.Completed = item
				if state, stateErr := repositories.Retention.Get(ctx, student.ID, item.ConceptID); stateErr == nil {
					update.Retention = state
				}
				if next, nextErr := repositories.Reviews.PendingByConcept(ctx, student.ID, item.ConceptID); nextErr == nil {
					update.Next = &next
				}
				return nil
			}
			return Classify(ErrorConflict, operation, errors.New("review already completed with another score"))
		}
		completed, err := item.Complete(score, now)
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		if err := repositories.Reviews.UpdateItem(ctx, completed); err != nil {
			return err
		}
		evidenceID, _ := learning.NewID("evidence." + item.ID.String())
		evidence, err := learning.NewEvidenceWithMetadata(evidenceID, student.ID, item.ConceptID, learning.EvidenceReviewRecall,
			"review:"+item.ID.String(), score, learning.EvidenceMetadata{Confidence: 1, Independence: 1, Difficulty: .5,
				AlgorithmVersion: learning.ReviewSchedulerVersion}, now)
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		if err := repositories.Evidence.Append(ctx, evidence); err != nil {
			return err
		}
		retention, err := recalculateRetention(ctx, repositories, student.ID, item.ConceptID, now)
		if err != nil {
			return err
		}
		contexts, err := loadReviewConceptContexts(ctx, repositories, student.ID)
		if err != nil {
			return err
		}
		concept, exists := contexts[item.ConceptID]
		if !exists {
			return Classify(ErrorInvalidState, operation, errors.New("review concept is not introduced in an active curriculum"))
		}
		if err := projectReviewDue(ctx, repositories, concept.instances, retention); err != nil {
			return err
		}
		history, err := repositories.Reviews.ListByStudent(ctx, student.ID)
		if err != nil {
			return err
		}
		conceptHistory := groupReviewItems(history)[item.ConceptID]
		schedule, scheduled, err := learning.ScheduleReviewV1(learning.ReviewSchedulingInput{
			Concept: concept.state, Retention: retention, History: conceptHistory,
			CriticalPrerequisite: concept.critical, ScheduledAt: now,
		})
		if err != nil {
			return Classify(ErrorInvalidState, operation, fmt.Errorf("build next review schedule: %w", err))
		}
		if !scheduled {
			return Classify(ErrorInvalidState, operation, errors.New("completed review did not produce a next schedule"))
		}
		if err := repositories.Reviews.SaveSchedule(ctx, schedule); err != nil {
			return err
		}
		nextID, err := learning.NewReviewItemIDV1(student.ID, item.ConceptID, schedule.DueAt, len(conceptHistory))
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		next, err := learning.NewReviewItemV1(nextID, schedule, now)
		if err != nil {
			return Classify(ErrorInvalidState, operation, err)
		}
		if err := repositories.Reviews.CreateItem(ctx, next); err != nil {
			return err
		}
		if err := recordStudyEvent(ctx, repositories.History, student.ID, learning.StudyEventReviewCompleted,
			item.ID, now, nil, nil, &item.ConceptID); err != nil {
			return err
		}
		update = ReviewOutcomeUpdate{Completed: completed, Next: &next, Retention: retention}
		return nil
	})
	if err != nil {
		return ReviewOutcomeUpdate{}, repositoryError(operation, err)
	}
	return update, nil
}

func (service *reviewSchedulerService) studentAndTime(ctx context.Context, operation string) (learning.Student, learning.Timestamp, error) {
	if service == nil || service.profiles == nil || service.unitOfWork == nil || service.now == nil {
		return learning.Student{}, learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("review scheduler dependencies are not configured"))
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.Student{}, learning.Timestamp{}, err
	}
	now, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Student{}, learning.Timestamp{}, invalid(operation, err)
	}
	return student, now, nil
}

type reviewConceptContext struct {
	state     learning.ConceptState
	critical  bool
	instances []learning.InstanceConceptState
}

func loadReviewConceptContexts(ctx context.Context, repositories Repositories, studentID learning.ID) (map[learning.ID]reviewConceptContext, error) {
	instances, err := repositories.CurriculumInstances.ListByStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	sort.Slice(instances, func(i, j int) bool { return instances[i].ID.String() < instances[j].ID.String() })
	contexts := make(map[learning.ID]reviewConceptContext)
	critical := make(map[learning.ID]bool)
	for _, instance := range instances {
		if instance.Status != learning.CurriculumInstanceActive {
			continue
		}
		concepts, err := repositories.Curricula.Concepts(ctx, instance.Curriculum)
		if err != nil {
			return nil, err
		}
		for _, concept := range concepts {
			prerequisites, err := repositories.Curricula.Prerequisites(ctx, instance.Curriculum, concept.ID)
			if err != nil {
				return nil, err
			}
			for _, prerequisite := range prerequisites {
				critical[prerequisite.RequiredConceptID] = true
			}
		}
		states, err := repositories.InstanceConceptStates.ListByInstance(ctx, instance.ID)
		if err != nil {
			return nil, err
		}
		for _, state := range states {
			if state.Exposure == learning.ExposureNotSeen {
				continue
			}
			context := contexts[state.ConceptID]
			context.instances = append(context.instances, state)
			if context.state.StudentID.String() == "" || state.UpdatedAt.After(context.state.UpdatedAt) {
				context.state = state.ConceptState()
			}
			contexts[state.ConceptID] = context
		}
	}
	for conceptID, context := range contexts {
		context.critical = critical[conceptID]
		contexts[conceptID] = context
	}
	return contexts, nil
}

func recalculateRetention(ctx context.Context, repositories Repositories, studentID, conceptID learning.ID, measuredAt learning.Timestamp) (learning.RetentionState, error) {
	items, err := repositories.Evidence.ListByConcept(ctx, studentID, conceptID)
	if err != nil {
		return learning.RetentionState{}, err
	}
	mastery, err := learning.CalculateMasteryV1(studentID, conceptID, items)
	if err != nil {
		return learning.RetentionState{}, Classify(ErrorInvalidState, "recalculate review retention", err)
	}
	calculation, err := learning.CalculateRetentionV1(mastery, items, measuredAt)
	if err != nil {
		return learning.RetentionState{}, Classify(ErrorInvalidState, "recalculate review retention", err)
	}
	if err := repositories.Retention.Save(ctx, calculation.State); err != nil {
		return learning.RetentionState{}, err
	}
	return calculation.State, nil
}

func projectReviewDue(ctx context.Context, repositories Repositories, states []learning.InstanceConceptState, retention learning.RetentionState) error {
	for _, state := range states {
		progression, err := learning.ApplyRetentionV1(state, retention)
		if err != nil {
			return Classify(ErrorInvalidState, "project scheduled review due", err)
		}
		if progression.StateChanged {
			if err := repositories.InstanceConceptStates.Save(ctx, progression.State); err != nil {
				return err
			}
		}
	}
	return nil
}

func groupReviewItems(items []learning.ReviewItem) map[learning.ID][]learning.ReviewItem {
	grouped := make(map[learning.ID][]learning.ReviewItem)
	for _, item := range items {
		grouped[item.ConceptID] = append(grouped[item.ConceptID], item)
	}
	return grouped
}

func findPendingReview(items []learning.ReviewItem) *learning.ReviewItem {
	for _, item := range items {
		if item.Status == learning.ReviewPending {
			copy := item
			return &copy
		}
	}
	return nil
}

func reviewQueueCandidates(items []learning.ReviewItem, states map[learning.ID]learning.RetentionState) []learning.ReviewQueueCandidate {
	candidates := make([]learning.ReviewQueueCandidate, 0)
	for _, item := range items {
		state, exists := states[item.ConceptID]
		if exists && item.Status == learning.ReviewPending {
			candidates = append(candidates, learning.ReviewQueueCandidate{Item: item, Retention: state})
		}
	}
	return candidates
}

func reviewQueueView(student learning.Student, items []learning.ReviewItem, candidates []learning.ReviewQueueCandidate, queue learning.ReviewQueue, dueOnly bool) ReviewQueueView {
	view := ReviewQueueView{Deferred: queue.Deferred, BudgetMinutes: queue.BudgetMinutes, UsedMinutes: queue.UsedMinutes,
		TotalDueMinutes: queue.TotalDueMinutes, Timezone: student.Profile.Timezone, GeneratedAt: queue.GeneratedAt,
		DueOnly: dueOnly, AlgorithmVersion: queue.AlgorithmVersion}
	for _, item := range items {
		if item.Status == learning.ReviewPending {
			view.Pending++
		}
	}
	if dueOnly {
		view.Items = queue.Items
		return view
	}
	for _, candidate := range candidates {
		view.Items = append(view.Items, learning.ReviewQueueItem{Item: candidate.Item, Strength: candidate.Retention.Strength,
			Status: candidate.Retention.Status, Overdue: candidate.Retention.Status == learning.RetentionOverdue,
			Critical: candidate.Item.CriticalPrerequisite})
	}
	sort.Slice(view.Items, func(i, j int) bool {
		if view.Items[i].Item.DueAt == view.Items[j].Item.DueAt {
			return view.Items[i].Item.ID.String() < view.Items[j].Item.ID.String()
		}
		return view.Items[i].Item.DueAt.Before(view.Items[j].Item.DueAt)
	})
	return view
}

var _ ReviewSchedulerService = (*reviewSchedulerService)(nil)
