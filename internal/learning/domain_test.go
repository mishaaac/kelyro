package learning

import "testing"

func TestStudentAndGoalConstructorsEnforceRequiredState(t *testing.T) {
	t.Parallel()

	studentID := mustID(t, "student.ada")
	createdAt := mustTimestamp(t, 10)
	profile := StudentProfile{
		DisplayName:  "Ada",
		Experience:   ExperienceBeginner,
		Preferences:  []StudyPreference{PreferenceTheoryFirst, PreferencePractice},
		Availability: Availability{WeeklyMinutes: 180, PreferredDays: []int{1, 3, 5}},
	}
	student, err := NewStudent(studentID, profile, createdAt)
	if err != nil {
		t.Fatalf("NewStudent() error = %v", err)
	}
	if student.CreatedAt != student.UpdatedAt {
		t.Fatal("new student timestamps differ")
	}

	goal, err := NewLearningGoal(
		mustID(t, "goal.statistics"), studentID, "Understand descriptive statistics",
		mustThreshold(t, 0.8), createdAt,
	)
	if err != nil {
		t.Fatalf("NewLearningGoal() error = %v", err)
	}
	if goal.Status != GoalDraft {
		t.Fatalf("new goal status = %q, want draft", goal.Status)
	}

	goal.Status = GoalStatus("unknown")
	if err := goal.Validate(); err == nil {
		t.Fatal("LearningGoal.Validate() accepted invalid status")
	}
	profile.Experience = ExperienceLevel("expert-ish")
	if _, err := NewStudent(studentID, profile, createdAt); err == nil {
		t.Fatal("NewStudent() accepted invalid experience level")
	}
}

func TestCurriculumVocabularyUsesStableIDsAndValidHierarchy(t *testing.T) {
	t.Parallel()

	curriculum := CurriculumRef{ID: mustID(t, "curriculum.statistics"), Version: "2026.08.1"}
	phaseID := mustID(t, "phase.foundations")
	moduleID := mustID(t, "module.descriptive")
	lessonID := mustID(t, "lesson.central-tendency")
	topicID := mustID(t, "topic.mean")
	conceptID := mustID(t, "concept.arithmetic-mean")
	structures := []struct {
		name     string
		validate func() error
	}{
		{"curriculum", curriculum.Validate},
		{"phase", Phase{ID: phaseID, Title: "Foundations", ModuleIDs: []ID{moduleID}}.Validate},
		{"module", Module{ID: moduleID, PhaseID: phaseID, Title: "Descriptive statistics", LessonIDs: []ID{lessonID}}.Validate},
		{"lesson", Lesson{ID: lessonID, ModuleID: moduleID, Title: "Central tendency", TopicIDs: []ID{topicID}}.Validate},
		{"topic", Topic{ID: topicID, LessonID: lessonID, Title: "Mean", ConceptIDs: []ID{conceptID}}.Validate},
		{"concept", Concept{ID: conceptID, TopicID: topicID, Title: "Arithmetic mean"}.Validate},
	}
	for _, structure := range structures {
		if err := structure.validate(); err != nil {
			t.Errorf("%s.Validate() error = %v", structure.name, err)
		}
	}

	duplicate := Phase{ID: phaseID, Title: "Foundations", ModuleIDs: []ID{moduleID, moduleID}}
	if err := duplicate.Validate(); err == nil {
		t.Fatal("Phase.Validate() accepted duplicate module IDs")
	}

	if _, err := NewPrerequisite(conceptID, conceptID); err == nil {
		t.Fatal("NewPrerequisite() accepted self dependency")
	}
	if _, err := NewPrerequisite(conceptID, mustID(t, "concept.number-sense")); err != nil {
		t.Fatalf("NewPrerequisite() valid dependency error = %v", err)
	}
}

func TestConceptStateKeepsExposureSeparateFromMastery(t *testing.T) {
	t.Parallel()

	introducedAt := mustTimestamp(t, 10)
	state := ConceptState{
		StudentID: mustID(t, "student.ada"), ConceptID: mustID(t, "concept.arithmetic-mean"),
		Exposure: ExposureIntroduced, Mastery: mustScore(t, 0),
		IntroducedAt: &introducedAt, UpdatedAt: introducedAt,
	}
	if err := state.Validate(); err != nil {
		t.Fatalf("ConceptState.Validate() error = %v", err)
	}
	state.Exposure = ExposureState("complete")
	if err := state.Validate(); err == nil {
		t.Fatal("ConceptState.Validate() accepted invalid exposure state")
	}

	notSeen := state
	notSeen.Exposure = ExposureNotSeen
	notSeen.IntroducedAt = &introducedAt
	if err := notSeen.Validate(); err == nil {
		t.Fatal("ConceptState.Validate() accepted not-seen state with introduction")
	}
}

func TestEvidenceAndMistakesRequireTraceableKnownConcepts(t *testing.T) {
	t.Parallel()

	studentID := mustID(t, "student.ada")
	conceptID := mustID(t, "concept.arithmetic-mean")
	topicID := mustID(t, "topic.mean")
	observedAt := mustTimestamp(t, 11)
	evidence, err := NewEvidence(
		mustID(t, "evidence.001"), studentID, conceptID, EvidencePractice,
		"fixture.practice.001", mustScore(t, 0.75), observedAt,
	)
	if err != nil {
		t.Fatalf("NewEvidence() error = %v", err)
	}
	if evidence.Source == "" || evidence.Type != EvidencePractice || evidence.ObservedAt != observedAt {
		t.Fatalf("evidence traceability fields = %+v", evidence)
	}
	evidence.Source = ""
	if err := evidence.Validate(); err == nil {
		t.Fatal("Evidence.Validate() accepted empty source")
	}

	mistake, err := NewMistake(
		mustID(t, "mistake.001"), studentID, conceptID,
		"Confused the arithmetic mean with the median", observedAt,
	)
	if err != nil {
		t.Fatalf("NewMistake() error = %v", err)
	}
	known := []Concept{{ID: conceptID, TopicID: topicID, Title: "Arithmetic mean"}}
	if err := ValidateMistakeConcept(mistake, known); err != nil {
		t.Fatalf("ValidateMistakeConcept() error = %v", err)
	}
	mistake.ConceptID = mustID(t, "concept.unknown")
	if err := ValidateMistakeConcept(mistake, known); err == nil {
		t.Fatal("ValidateMistakeConcept() accepted unknown concept")
	}
}

func TestLearningSessionRequiresStrictContainedTimeRanges(t *testing.T) {
	t.Parallel()

	startedAt := mustTimestamp(t, 10)
	endedAt := mustTimestamp(t, 12)
	activity := StudyActivity{
		ID: mustID(t, "activity.001"), ConceptIDs: []ID{mustID(t, "concept.arithmetic-mean")},
		Type: ActivityPractice, StartedAt: startedAt, EndedAt: mustTimestamp(t, 11),
	}
	if _, err := NewLearningSession(
		mustID(t, "session.001"), mustID(t, "student.ada"), mustID(t, "goal.statistics"),
		startedAt, endedAt, []StudyActivity{activity},
	); err != nil {
		t.Fatalf("NewLearningSession() error = %v", err)
	}

	if _, err := NewLearningSession(
		mustID(t, "session.invalid-range"), mustID(t, "student.ada"), mustID(t, "goal.statistics"),
		endedAt, startedAt, nil,
	); err == nil {
		t.Fatal("NewLearningSession() accepted reversed timestamps")
	}

	activity.EndedAt = mustTimestamp(t, 13)
	if _, err := NewLearningSession(
		mustID(t, "session.outside"), mustID(t, "student.ada"), mustID(t, "goal.statistics"),
		startedAt, endedAt, []StudyActivity{activity},
	); err == nil {
		t.Fatal("NewLearningSession() accepted activity outside session")
	}
}

func TestReviewScheduleRequiresIntroductionUnlessImported(t *testing.T) {
	t.Parallel()

	studentID := mustID(t, "student.ada")
	conceptID := mustID(t, "concept.arithmetic-mean")
	introducedAt := mustTimestamp(t, 11)
	earlier := mustTimestamp(t, 10)
	later := mustTimestamp(t, 12)

	if _, err := NewReviewSchedule(studentID, conceptID, &introducedAt, later, false); err != nil {
		t.Fatalf("NewReviewSchedule() error = %v", err)
	}
	if _, err := NewReviewSchedule(studentID, conceptID, &introducedAt, earlier, false); err == nil {
		t.Fatal("NewReviewSchedule() accepted due date before introduction")
	}
	if _, err := NewReviewSchedule(studentID, conceptID, nil, earlier, false); err == nil {
		t.Fatal("NewReviewSchedule() accepted missing introduction")
	}
	if _, err := NewReviewSchedule(studentID, conceptID, nil, earlier, true); err != nil {
		t.Fatalf("NewReviewSchedule() rejected explicit import: %v", err)
	}
}
