package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

func TestStudyHistoryFiltersLocalTodayAndOrdersNewestFirst(t *testing.T) {
	fixture := newStudySessionFixture(t)
	zone := "America/Lima"
	if _, err := fixture.profiles.Edit(fixture.ctx, application.ProfileChanges{Timezone: &zone}); err != nil {
		t.Fatal(err)
	}
	student, err := fixture.profiles.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	repository := fixture.store.Repositories().History
	before := mustStudyEvent(t, "history.before", student.ID, learning.StudyEventOnboardingCompleted, "source.before",
		time.Date(2026, 8, 21, 4, 59, 59, 0, time.UTC), nil, nil, nil)
	first := mustStudyEvent(t, "history.first", student.ID, learning.StudyEventEvidenceRecorded, "source.first",
		time.Date(2026, 8, 21, 5, 0, 0, 0, time.UTC), &fixture.goal.ID, &fixture.instance.ID, idPointer(t, "concept.a"))
	second := mustStudyEvent(t, "history.second", student.ID, learning.StudyEventConceptIntroduced, "source.second",
		time.Date(2026, 8, 21, 6, 0, 0, 0, time.UTC), &fixture.goal.ID, &fixture.instance.ID, idPointer(t, "concept.a"))
	for _, event := range []learning.StudyEvent{before, first, second} {
		if err := repository.Record(fixture.ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	service := application.NewStudyHistoryService(fixture.profiles, fixture.store,
		application.WithStudyHistoryClock(func() time.Time { return time.Date(2026, 8, 21, 14, 0, 0, 0, time.UTC) }))
	view, err := service.List(fixture.ctx, learning.StudyPeriodToday)
	if err != nil {
		t.Fatal(err)
	}
	if view.Timezone != zone || len(view.Events) != 2 || view.Events[0].ID != second.ID || view.Events[1].ID != first.ID {
		t.Fatalf("today history = %+v", view)
	}
	if got := view.From.Time(); !got.Equal(time.Date(2026, 8, 21, 5, 0, 0, 0, time.UTC)) {
		t.Fatalf("today start = %s", got)
	}
	all, err := service.List(fixture.ctx, learning.StudyPeriodAll)
	if err != nil || len(all.Events) != 3 {
		t.Fatalf("all history = (%+v, %v)", all, err)
	}
}

func TestStudyTimeUsesActiveDurationAndOnlyUnambiguousAttribution(t *testing.T) {
	fixture := newStudySessionFixture(t)
	student, err := fixture.profiles.Show(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	session, err := fixture.sessions.Start(fixture.ctx, fixture.goal.ID, fixture.instance.ID)
	if err != nil {
		t.Fatal(err)
	}
	fixture.now = fixture.now.Add(10 * time.Minute)
	if _, err := fixture.sessions.RecordActivity(fixture.ctx); err != nil {
		t.Fatal(err)
	}
	conceptID := testID(t, "concept.a")
	event := mustStudyEvent(t, "history.session.concept", student.ID, learning.StudyEventEvidenceRecorded, "source.session.concept",
		fixture.now, &fixture.goal.ID, &fixture.instance.ID, &conceptID)
	if err := fixture.store.Repositories().History.Record(fixture.ctx, event); err != nil {
		t.Fatal(err)
	}
	fixture.now = fixture.now.Add(5 * time.Minute)
	completed, err := fixture.sessions.Stop(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if completed.ActiveDuration != 15*time.Minute || completed.ID != session.ID {
		t.Fatalf("completed session = %+v", completed)
	}
	for _, historical := range []learning.StudySession{
		completedStudySession(t, "session.month", student.ID, fixture.goal.ID, fixture.instance.ID,
			time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC), 10*time.Minute),
		completedStudySession(t, "session.total", student.ID, fixture.goal.ID, fixture.instance.ID,
			time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC), 20*time.Minute),
	} {
		if err := fixture.store.Repositories().StudySessions.Create(fixture.ctx, historical); err != nil {
			t.Fatal(err)
		}
	}
	fixture.now = fixture.now.Add(time.Minute)
	service := application.NewStudyHistoryService(fixture.profiles, fixture.store,
		application.WithStudyHistoryClock(func() time.Time { return fixture.now }))
	summary, err := service.Time(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if summary.Today != 15*time.Minute || summary.Week != 15*time.Minute || summary.Month != 25*time.Minute || summary.Total != 45*time.Minute ||
		summary.TodaySessions != 1 || summary.WeekSessions != 1 || summary.MonthSessions != 2 || summary.TotalSessions != 3 ||
		summary.PolicyVersion != learning.TimeTrackingPolicyVersion {
		t.Fatalf("study time = %+v", summary)
	}
	if len(summary.ByConcept) != 1 || summary.ByConcept[0].ID != conceptID || summary.ByConcept[0].Duration != 15*time.Minute {
		t.Fatalf("concept breakdown = %+v", summary.ByConcept)
	}
	moduleID, err := fixture.store.Repositories().Curricula.ModuleForConcept(fixture.ctx, fixture.instance.Curriculum, conceptID)
	if err != nil || len(summary.ByModule) != 1 || summary.ByModule[0].ID != moduleID || summary.ByModule[0].Duration != 15*time.Minute {
		t.Fatalf("module breakdown = %+v, module=%s err=%v", summary.ByModule, moduleID, err)
	}
}

func completedStudySession(t *testing.T, id string, studentID, goalID, instanceID learning.ID, startedAt time.Time, duration time.Duration) learning.StudySession {
	t.Helper()
	started, err := learning.NewTimestamp(startedAt)
	if err != nil {
		t.Fatal(err)
	}
	session, err := learning.NewStudySession(testID(t, id), studentID, goalID, instanceID, started, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ended, err := learning.NewTimestamp(startedAt.Add(duration))
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.RecordActivity(ended)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.Complete(ended)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func mustStudyEvent(t *testing.T, id string, studentID learning.ID, eventType learning.StudyEventType, source string, occurred time.Time,
	goalID, instanceID, conceptID *learning.ID) learning.StudyEvent {
	t.Helper()
	timestamp, err := learning.NewTimestamp(occurred)
	if err != nil {
		t.Fatal(err)
	}
	event, err := learning.NewStudyEvent(testID(t, id), studentID, eventType, testID(t, source), timestamp, goalID, instanceID, conceptID)
	if err != nil {
		t.Fatal(err)
	}
	return event
}

func idPointer(t *testing.T, value string) *learning.ID {
	t.Helper()
	id := testID(t, value)
	return &id
}
