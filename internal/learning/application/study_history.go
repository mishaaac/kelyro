package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

type StudyHistoryOption func(*studyHistoryService)

func WithStudyHistoryClock(now func() time.Time) StudyHistoryOption {
	return func(service *studyHistoryService) {
		if now != nil {
			service.now = now
		}
	}
}

type studyHistoryService struct {
	profiles   ProfileService
	unitOfWork UnitOfWork
	now        func() time.Time
}

func NewStudyHistoryService(profiles ProfileService, unitOfWork UnitOfWork, options ...StudyHistoryOption) StudyHistoryService {
	service := &studyHistoryService{profiles: profiles, unitOfWork: unitOfWork, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *studyHistoryService) List(ctx context.Context, period learning.StudyPeriod) (StudyHistoryView, error) {
	const operation = "list study history"
	if !period.Valid() || (period != learning.StudyPeriodAll && period != learning.StudyPeriodToday) {
		return StudyHistoryView{}, invalid(operation, fmt.Errorf("history period %q is unsupported", period))
	}
	student, now, location, err := service.context(ctx, operation)
	if err != nil {
		return StudyHistoryView{}, err
	}
	view := StudyHistoryView{Period: period, Timezone: student.Profile.Timezone}
	if period == learning.StudyPeriodToday {
		from, to, windowErr := learning.StudyWindow(period, now.Time(), location)
		if windowErr != nil {
			return StudyHistoryView{}, invalid(operation, windowErr)
		}
		view.From, view.To = &from, &to
	}
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		var listErr error
		view.Events, listErr = repositories.History.ListByStudent(ctx, student.ID, view.From, view.To)
		return listErr
	})
	return view, err
}

func (service *studyHistoryService) Time(ctx context.Context) (StudyTimeSummary, error) {
	const operation = "calculate study time"
	student, now, location, err := service.context(ctx, operation)
	if err != nil {
		return StudyTimeSummary{}, err
	}
	windows := make(map[learning.StudyPeriod][2]learning.Timestamp, 3)
	for _, period := range []learning.StudyPeriod{learning.StudyPeriodToday, learning.StudyPeriodWeek, learning.StudyPeriodMonth} {
		from, to, windowErr := learning.StudyWindow(period, now.Time(), location)
		if windowErr != nil {
			return StudyTimeSummary{}, invalid(operation, windowErr)
		}
		windows[period] = [2]learning.Timestamp{from, to}
	}
	summary := StudyTimeSummary{
		Timezone: student.Profile.Timezone, GeneratedAt: now,
		PolicyVersion: learning.TimeTrackingPolicyVersion,
	}
	err = service.withRepositories(ctx, operation, func(repositories Repositories) error {
		sessions, listErr := repositories.StudySessions.ListByStudent(ctx, student.ID)
		if listErr != nil {
			return listErr
		}
		events, listErr := repositories.History.ListByStudent(ctx, student.ID, nil, nil)
		if listErr != nil {
			return listErr
		}
		instances, listErr := repositories.CurriculumInstances.ListByStudent(ctx, student.ID)
		if listErr != nil {
			return listErr
		}
		instanceByID := make(map[learning.ID]learning.CurriculumInstance, len(instances))
		for _, instance := range instances {
			instanceByID[instance.ID] = instance
		}
		conceptBreakdown := make(map[learning.ID]learning.StudyTimeBreakdown)
		moduleBreakdown := make(map[learning.ID]learning.StudyTimeBreakdown)
		for _, session := range sessions {
			anchor := session.LastActivityAt
			if session.EndedAt != nil {
				anchor = *session.EndedAt
			}
			if anchor.After(now) {
				continue
			}
			summary.Total += session.ActiveDuration
			summary.TotalSessions++
			if containsStudyAnchor(anchor, windows[learning.StudyPeriodToday]) {
				summary.Today += session.ActiveDuration
				summary.TodaySessions++
			}
			if containsStudyAnchor(anchor, windows[learning.StudyPeriodWeek]) {
				summary.Week += session.ActiveDuration
				summary.WeekSessions++
			}
			if containsStudyAnchor(anchor, windows[learning.StudyPeriodMonth]) {
				summary.Month += session.ActiveDuration
				summary.MonthSessions++
			}
			concepts := sessionConcepts(session, anchor, events)
			if len(concepts) == 1 {
				addStudyBreakdown(conceptBreakdown, concepts[0], session.ActiveDuration)
			}
			instance, exists := instanceByID[session.CurriculumInstanceID]
			if !exists {
				continue
			}
			modules := make(map[learning.ID]struct{})
			allConceptsMapped := len(concepts) > 0
			for _, conceptID := range concepts {
				moduleID, moduleErr := repositories.Curricula.ModuleForConcept(ctx, instance.Curriculum, conceptID)
				if moduleErr != nil {
					if errors.Is(moduleErr, ErrNotFound) {
						allConceptsMapped = false
						break
					}
					return moduleErr
				}
				modules[moduleID] = struct{}{}
			}
			if allConceptsMapped && len(modules) == 1 {
				for moduleID := range modules {
					addStudyBreakdown(moduleBreakdown, moduleID, session.ActiveDuration)
				}
			}
		}
		summary.ByConcept = studyBreakdowns(conceptBreakdown)
		summary.ByModule = studyBreakdowns(moduleBreakdown)
		return nil
	})
	return summary, err
}

func (service *studyHistoryService) context(ctx context.Context, operation string) (learning.Student, learning.Timestamp, *time.Location, error) {
	if service == nil || service.profiles == nil || service.unitOfWork == nil || service.now == nil {
		return learning.Student{}, learning.Timestamp{}, nil, Classify(ErrorUnavailable, operation, errors.New("study history dependencies are not configured"))
	}
	student, err := service.profiles.Show(ctx)
	if err != nil {
		return learning.Student{}, learning.Timestamp{}, nil, err
	}
	now, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Student{}, learning.Timestamp{}, nil, invalid(operation, err)
	}
	location, err := time.LoadLocation(student.Profile.Timezone)
	if err != nil {
		return learning.Student{}, learning.Timestamp{}, nil, invalid(operation, err)
	}
	return student, now, location, nil
}

func (service *studyHistoryService) withRepositories(ctx context.Context, operation string, work func(Repositories) error) error {
	err := service.unitOfWork.WithinTransaction(ctx, func(repositories Repositories) error {
		if repositories.History == nil || repositories.StudySessions == nil || repositories.CurriculumInstances == nil || repositories.Curricula == nil {
			return Classify(ErrorUnavailable, operation, errors.New("study history repositories are not configured"))
		}
		return work(repositories)
	})
	return repositoryError(operation, err)
}

func containsStudyAnchor(anchor learning.Timestamp, window [2]learning.Timestamp) bool {
	return !anchor.Before(window[0]) && anchor.Before(window[1])
}

func sessionConcepts(session learning.StudySession, anchor learning.Timestamp, events []learning.StudyEvent) []learning.ID {
	unique := make(map[learning.ID]struct{})
	for _, event := range events {
		if event.CurriculumInstanceID == nil || *event.CurriculumInstanceID != session.CurriculumInstanceID || event.ConceptID == nil ||
			event.OccurredAt.Before(session.StartedAt) || event.OccurredAt.After(anchor) {
			continue
		}
		unique[*event.ConceptID] = struct{}{}
	}
	items := make([]learning.ID, 0, len(unique))
	for id := range unique {
		items = append(items, id)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].String() < items[j].String() })
	return items
}

func addStudyBreakdown(items map[learning.ID]learning.StudyTimeBreakdown, id learning.ID, duration time.Duration) {
	item := items[id]
	item.ID = id
	item.Duration += duration
	item.Sessions++
	items[id] = item
}

func studyBreakdowns(items map[learning.ID]learning.StudyTimeBreakdown) []learning.StudyTimeBreakdown {
	result := make([]learning.StudyTimeBreakdown, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	learning.SortStudyTimeBreakdowns(result)
	return result
}
