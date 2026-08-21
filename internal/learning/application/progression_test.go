package application_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestProgressionServiceAtomicallyMastersAndDerivesDependentUnlocks(t *testing.T) {
	t.Parallel()
	fixture := newProgressionServiceFixture(t)
	ctx := context.Background()
	evidence := progressionEvidence(t, "master", fixture.student.ID, testID(t, "concept.a"), .8, fixture.evidenceAt)

	update, err := fixture.service.RecordEvidence(ctx, fixture.instance.ID, evidence, nil)
	if err != nil {
		t.Fatal(err)
	}
	if update.Progression.State.Exposure != learning.ExposureMastered || !update.Progression.ThresholdMet || len(update.Dependents) != 2 {
		t.Fatalf("RecordEvidence() = %+v", update)
	}
	byID := map[string]application.DependentProgression{}
	for _, dependent := range update.Dependents {
		byID[dependent.Decision.ConceptID.String()] = dependent
	}
	if byID["concept.b"].Decision.CanIntroduce || byID["concept.b"].NewlyEligible || len(byID["concept.b"].Decision.Checks) != 2 {
		t.Fatalf("multiple-prerequisite dependent = %+v", byID["concept.b"])
	}
	if !byID["concept.c"].Decision.CanIntroduce || !byID["concept.c"].NewlyEligible || byID["concept.c"].WasEligible {
		t.Fatalf("newly unlocked dependent = %+v", byID["concept.c"])
	}
	items, err := fixture.repositories.Evidence.ListByConcept(ctx, fixture.student.ID, evidence.ConceptID)
	if err != nil || len(items) != 1 || items[0].ID != evidence.ID {
		t.Fatalf("persisted evidence = (%+v, %v)", items, err)
	}
	state, err := fixture.repositories.InstanceConceptStates.Get(ctx, fixture.instance.ID, evidence.ConceptID)
	if err != nil || state.Exposure != learning.ExposureMastered || state.Mastery.Value() != .8 {
		t.Fatalf("persisted state = (%+v, %v)", state, err)
	}
	states, err := fixture.repositories.InstanceConceptStates.ListByInstance(ctx, fixture.instance.ID)
	if err != nil || len(states) != 1 {
		t.Fatalf("derived unlock persisted extra state = (%+v, %v)", states, err)
	}
	history, err := fixture.repositories.History.ListByStudent(ctx, fixture.student.ID, nil, nil)
	if err != nil || len(history) != 3 {
		t.Fatalf("progression history = (%+v, %v)", history, err)
	}
	types := map[learning.StudyEventType]bool{}
	for _, event := range history {
		types[event.Type] = true
		if event.SourceID != evidence.ID || event.CurriculumInstanceID == nil || *event.CurriculumInstanceID != fixture.instance.ID || event.ConceptID == nil || *event.ConceptID != evidence.ConceptID {
			t.Fatalf("progression event scope = %+v", event)
		}
	}
	for _, want := range []learning.StudyEventType{learning.StudyEventEvidenceRecorded, learning.StudyEventConceptIntroduced, learning.StudyEventConceptMastered} {
		if !types[want] {
			t.Fatalf("progression history missing %s: %+v", want, history)
		}
	}
}

func TestProgressionServiceRelocksDependentWhenRecalculationFallsBelowThreshold(t *testing.T) {
	t.Parallel()
	fixture := newProgressionServiceFixture(t)
	ctx := context.Background()
	first := progressionEvidence(t, "master", fixture.student.ID, testID(t, "concept.a"), .8, fixture.evidenceAt)
	initial, err := fixture.service.RecordEvidence(ctx, fixture.instance.ID, first, nil)
	if err != nil {
		t.Fatal(err)
	}
	masteredAt := initial.Progression.State.MasteredAt
	fixture.now = fixture.now.Add(2 * time.Hour)
	second := progressionEvidence(t, "failure", fixture.student.ID, testID(t, "concept.a"), 0, fixture.evidenceAt.Add(time.Hour))

	update, err := fixture.service.RecordEvidence(ctx, fixture.instance.ID, second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if update.Progression.ThresholdMet || update.Progression.State.Exposure != learning.ExposurePracticing ||
		update.Progression.State.MasteredAt == nil || *update.Progression.State.MasteredAt != *masteredAt {
		t.Fatalf("demoted progression = %+v", update.Progression)
	}
	byID := map[string]application.DependentProgression{}
	for _, dependent := range update.Dependents {
		byID[dependent.Decision.ConceptID.String()] = dependent
	}
	if !byID["concept.c"].WasEligible || byID["concept.c"].Decision.CanIntroduce || byID["concept.c"].NewlyEligible {
		t.Fatalf("relocked dependent = %+v", byID["concept.c"])
	}
}

func TestProgressionServiceRecalculatesExistingEvidenceWithoutRewritingIt(t *testing.T) {
	t.Parallel()
	fixture := newProgressionServiceFixture(t)
	ctx := context.Background()
	evidence := progressionEvidence(t, "existing", fixture.student.ID, testID(t, "concept.a"), .8, fixture.evidenceAt)
	if err := fixture.repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}

	update, err := fixture.service.Recalculate(ctx, fixture.instance.ID, evidence.ConceptID, nil)
	if err != nil || !update.Progression.ThresholdMet || update.Progression.State.Exposure != learning.ExposureMastered {
		t.Fatalf("Recalculate() = (%+v, %v)", update, err)
	}
	items, err := fixture.repositories.Evidence.ListByConcept(ctx, fixture.student.ID, evidence.ConceptID)
	if err != nil || len(items) != 1 || items[0] != evidence {
		t.Fatalf("evidence after recalculation = (%+v, %v)", items, err)
	}
}

func TestProgressionServiceRollsBackDuplicateEvidenceWithoutChangingState(t *testing.T) {
	t.Parallel()
	fixture := newProgressionServiceFixture(t)
	ctx := context.Background()
	evidence := progressionEvidence(t, "duplicate", fixture.student.ID, testID(t, "concept.a"), .8, fixture.evidenceAt)
	first, err := fixture.service.RecordEvidence(ctx, fixture.instance.ID, evidence, nil)
	if err != nil {
		t.Fatal(err)
	}
	fixture.now = fixture.now.Add(time.Hour)
	if _, err := fixture.service.RecordEvidence(ctx, fixture.instance.ID, evidence, nil); !errors.Is(err, application.ErrConflict) {
		t.Fatalf("duplicate RecordEvidence() error = %v", err)
	}
	state, err := fixture.repositories.InstanceConceptStates.Get(ctx, fixture.instance.ID, evidence.ConceptID)
	if err != nil || !reflect.DeepEqual(state, first.Progression.State) {
		t.Fatalf("state after rollback = (%+v, %v), want %+v", state, err, first.Progression.State)
	}
}

func TestProgressionServiceRollsBackFutureEvidenceAndRejectsWrongOwner(t *testing.T) {
	t.Parallel()
	fixture := newProgressionServiceFixture(t)
	ctx := context.Background()
	conceptID := testID(t, "concept.a")
	future := progressionEvidence(t, "future", fixture.student.ID, conceptID, .8, fixture.now.Add(time.Hour))
	if _, err := fixture.service.RecordEvidence(ctx, fixture.instance.ID, future, nil); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("future RecordEvidence() error = %v", err)
	}
	items, err := fixture.repositories.Evidence.ListByConcept(ctx, fixture.student.ID, conceptID)
	if err != nil || len(items) != 0 {
		t.Fatalf("future evidence after rollback = (%+v, %v)", items, err)
	}
	states, err := fixture.repositories.InstanceConceptStates.ListByInstance(ctx, fixture.instance.ID)
	if err != nil || len(states) != 0 {
		t.Fatalf("future state after rollback = (%+v, %v)", states, err)
	}

	wrongOwner := progressionEvidence(t, "wrong-owner", testID(t, "student.other"), conceptID, .8, fixture.evidenceAt)
	if _, err := fixture.service.RecordEvidence(ctx, fixture.instance.ID, wrongOwner, nil); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("wrong-owner RecordEvidence() error = %v", err)
	}
}

type progressionServiceFixture struct {
	service      application.ProgressionService
	repositories application.Repositories
	student      learning.Student
	instance     learning.CurriculumInstance
	evidenceAt   time.Time
	now          time.Time
}

func newProgressionServiceFixture(t *testing.T) *progressionServiceFixture {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	profileAt := time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students), application.WithProfileClock(func() time.Time { return profileAt }))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	goalAt := profileAt.Add(time.Hour)
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(func() time.Time { return goalAt }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.progression"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Progression", "Test progression"))
	if err != nil {
		t.Fatal(err)
	}
	curriculum := progressionTestCurriculum(t)
	graph, err := learning.NewKnowledgeGraph(curriculum)
	if err != nil {
		t.Fatal(err)
	}
	instanceAt := goalAt.Add(time.Hour)
	instances := application.NewCurriculumInstanceService(profiles, store, application.WithCurriculumInstanceClock(func() time.Time { return instanceAt }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.progression"), nil }))
	instance, err := instances.Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	thresholds := application.NewMasteryPolicyService(profiles, repositories.Mastery,
		application.WithMasteryPolicyClock(func() time.Time { return instanceAt.Add(time.Hour) }))
	fixture := &progressionServiceFixture{
		repositories: repositories, student: student, instance: instance,
		evidenceAt: instanceAt.Add(2 * time.Hour), now: instanceAt.Add(3 * time.Hour),
	}
	fixture.service = application.NewProgressionService(graph, profiles, thresholds, store,
		application.WithProgressionClock(func() time.Time { return fixture.now }))
	return fixture
}

func progressionTestCurriculum(t *testing.T) learning.Curriculum {
	t.Helper()
	base := instanceTestCurriculum(t, "progression-v1")
	topicID := testID(t, "topic.fixture")
	conceptAID := testID(t, "concept.a")
	conceptDID := testID(t, "concept.d")
	status := learning.CurriculumStatusMetadata{State: learning.CurriculumNodeActive}
	nodes := append([]learning.CurriculumNode(nil), base.Nodes...)
	for index := range nodes {
		if nodes[index].ID == testID(t, "concept.b") {
			definition := *nodes[index].Concept
			definition.Prerequisites = append([]learning.ConceptPrerequisite(nil), definition.Prerequisites...)
			definition.Prerequisites = append(definition.Prerequisites, learning.ConceptPrerequisite{ConceptID: conceptDID, Requirement: learning.PrerequisiteIntroduced})
			nodes[index].Concept = &definition
		}
	}
	nodes = append(nodes,
		learning.CurriculumNode{
			ID: conceptDID, Type: learning.CurriculumNodeConcept, ParentID: &topicID, Title: "D", Description: "D.", Order: 2,
			Status: status, Version: base.Reference.Version, Concept: &learning.ConceptDefinition{
				Objectives: []string{"Understand D"}, Difficulty: learning.ConceptDifficultyFoundational,
				EstimatedEffortMinutes: 10, AssessmentExpectations: []string{"Explain D"},
			},
		},
		learning.CurriculumNode{
			ID: testID(t, "concept.c"), Type: learning.CurriculumNodeConcept, ParentID: &topicID, Title: "C", Description: "C.", Order: 3,
			Status: status, Version: base.Reference.Version, Concept: &learning.ConceptDefinition{
				Objectives: []string{"Apply C"}, Difficulty: learning.ConceptDifficultyIntermediate,
				EstimatedEffortMinutes: 15, AssessmentExpectations: []string{"Apply C"},
				Prerequisites: []learning.ConceptPrerequisite{{ConceptID: conceptAID, Requirement: learning.PrerequisiteMastered}},
			},
		},
	)
	curriculum, err := learning.NewCurriculum(base.ContractVersion, base.Reference, base.Title, base.Description, nodes)
	if err != nil {
		t.Fatal(err)
	}
	return curriculum
}

func progressionEvidence(t *testing.T, suffix string, studentID, conceptID learning.ID, score float64, occurredAt time.Time) learning.Evidence {
	t.Helper()
	timestamp, err := learning.NewTimestamp(occurredAt)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := learning.NewEvidenceWithMetadata(testID(t, "evidence."+suffix), studentID, conceptID, learning.EvidenceAssessment,
		"fixture/"+suffix, testScore(t, score), learning.EvidenceMetadata{
			Confidence: 1, Independence: 1, Difficulty: .5, AlgorithmVersion: "fixture-evaluator/v1",
		}, timestamp)
	if err != nil {
		t.Fatal(err)
	}
	return evidence
}
