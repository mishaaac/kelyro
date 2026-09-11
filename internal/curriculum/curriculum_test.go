package curriculum

import (
	"strings"
	"testing"
	"time"
)

func TestCurriculumDefinitionValidatesRelationalInvariants(t *testing.T) {
	t.Parallel()

	if err := validCurriculumDefinition(t).Validate(); err != nil {
		t.Fatalf("valid definition rejected: %v", err)
	}

	tests := []struct {
		name string
		want string
		edit func(*CurriculumDefinition)
	}{
		{
			name: "duplicate concept",
			want: "duplicate concept id",
			edit: func(value *CurriculumDefinition) { value.Concepts = append(value.Concepts, value.Concepts[0]) },
		},
		{
			name: "self prerequisite",
			want: "cannot require itself",
			edit: func(value *CurriculumDefinition) {
				value.Prerequisites[0].RequiredConceptID = value.Prerequisites[0].ConceptID
			},
		},
		{
			name: "dangling prerequisite",
			want: "missing required concept",
			edit: func(value *CurriculumDefinition) {
				value.Prerequisites[0].RequiredConceptID = mustConceptID(t, "concept.missing")
			},
		},
		{
			name: "dangling hierarchy parent",
			want: "missing phase",
			edit: func(value *CurriculumDefinition) { value.Modules[0].PhaseID = mustID(t, "phase.missing") },
		},
		{
			name: "dangling hierarchy concept",
			want: "references missing concept",
			edit: func(value *CurriculumDefinition) { value.Topics[0].ConceptIDs[0] = mustConceptID(t, "concept.missing") },
		},
		{
			name: "concept omitted from hierarchy",
			want: "missing from curriculum hierarchy",
			edit: func(value *CurriculumDefinition) { value.Topics[0].ConceptIDs = value.Topics[0].ConceptIDs[:1] },
		},
		{
			name: "duplicate hierarchy identity",
			want: "duplicate curriculum node id",
			edit: func(value *CurriculumDefinition) { value.Modules[0].ID = value.Phases[0].ID },
		},
		{
			name: "duplicate sibling order",
			want: "share order",
			edit: func(value *CurriculumDefinition) {
				value.Modules = append(value.Modules, Module{ID: mustID(t, "module.two"), PhaseID: value.Phases[0].ID, Title: "Two", Description: "Second module", Order: 0})
			},
		},
		{
			name: "invalid concept status",
			want: "invalid concept status",
			edit: func(value *CurriculumDefinition) { value.Concepts[0].Status = "future" },
		},
		{
			name: "matrix references missing outcome",
			want: "references missing outcome",
			edit: func(value *CurriculumDefinition) {
				value.Competencies.Competencies[0].OutcomeID = mustID(t, "outcome.missing")
			},
		},
		{
			name: "competency references missing concept",
			want: "references missing concept",
			edit: func(value *CurriculumDefinition) {
				value.Competencies.Competencies[0].ConceptRefs[0] = mustConceptID(t, "concept.missing")
			},
		},
		{
			name: "vocabulary references missing concept",
			want: "vocabulary term",
			edit: func(value *CurriculumDefinition) {
				value.Vocabulary.Terms[0].UsedBy[0] = mustConceptID(t, "concept.missing")
			},
		},
		{
			name: "source references required",
			want: "source bundle references are required",
			edit: func(value *CurriculumDefinition) { value.SourceBundles = nil },
		},
		{
			name: "evidence bundle undeclared",
			want: "evidence references undeclared source bundle",
			edit: func(value *CurriculumDefinition) {
				value.Concepts[0].EvidenceRefs[0].BundleID = mustID(t, "bundle.missing")
			},
		},
		{
			name: "non UTC timestamp",
			want: "timestamp is not UTC",
			edit: func(value *CurriculumDefinition) {
				value.CreatedAt = Timestamp{value: time.Date(2026, 9, 9, 12, 0, 0, 0, time.FixedZone("local", -5*60*60))}
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			definition := validCurriculumDefinition(t)
			test.edit(&definition)
			err := definition.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestFixturePolicyAllowsDefinitionWithoutSourceBundles(t *testing.T) {
	t.Parallel()

	definition := validCurriculumDefinition(t)
	definition.SourcePolicy = SourceReferencesOptionalForFixture
	definition.SourceBundles = nil
	for index := range definition.Goal.Outcomes {
		definition.Goal.Outcomes[index].EvidenceRefs = nil
	}
	for index := range definition.Competencies.Competencies {
		definition.Competencies.Competencies[index].EvidenceRefs = nil
	}
	for index := range definition.Concepts {
		definition.Concepts[index].EvidenceRefs = nil
	}
	for index := range definition.Prerequisites {
		definition.Prerequisites[index].EvidenceRefs = nil
	}
	for index := range definition.CoverageRequirements {
		definition.CoverageRequirements[index].EvidenceRefs = nil
	}
	if err := definition.Validate(); err != nil {
		t.Fatalf("fixture definition rejected: %v", err)
	}
}

func validCurriculumDefinition(t *testing.T) CurriculumDefinition {
	t.Helper()
	bundle := testSourceBundleRef(t)
	evidence := EvidenceRef{BundleID: bundle.ID, ClaimID: mustID(t, "claim.core")}
	goalID := mustID(t, "goal.backend")
	outcomeID := mustID(t, "outcome.explain")
	rootConcept := mustConceptID(t, "concept.foundation")
	useConcept := mustConceptID(t, "concept.application")
	curriculumID := mustCurriculumID(t, "curriculum.backend")
	createdAt, err := NewTimestamp(time.Date(2026, time.September, 9, 18, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	version, err := NewCurriculumVersion("2026.09.09.1")
	if err != nil {
		t.Fatal(err)
	}
	return CurriculumDefinition{
		ID: curriculumID, Version: version, Title: "Backend", Description: "A source-backed curriculum.",
		Goal: LearningGoalSpec{
			ID: goalID, Title: "Backend role", Description: "Prepare for backend work.", Domain: "software",
			Outcomes: []GoalOutcome{{ID: outcomeID, Statement: "Explain and apply foundations.", Category: OutcomeKnowledge, Capability: OutcomeCapabilityExplain, EvidenceRefs: []EvidenceRef{evidence}}},
			Scope:    []string{"foundations", "application"}, Exclusions: []string{"exercise runtime"},
		},
		Competencies: CompetencyMatrix{
			Version: "competency-matrix-v1", GoalID: goalID,
			Competencies: []Competency{{
				ID: mustID(t, "competency.apply"), Area: "application", OutcomeID: outcomeID,
				ExpectedLevel: CompetencyApply, EvidenceRefs: []EvidenceRef{evidence}, ConceptRefs: []ConceptID{useConcept},
			}},
		},
		Concepts: []Concept{
			{ID: rootConcept, Title: "Foundation", Definition: "A foundational idea.", Version: "1", Atomicity: AtomicityAtomic, Difficulty: DifficultyFoundational, Status: ConceptCurrent, Foundational: true, EvidenceRefs: []EvidenceRef{evidence}},
			{ID: useConcept, Title: "Application", Definition: "Applying the foundational idea.", Version: "1", Atomicity: AtomicityAtomic, Difficulty: DifficultyIntermediate, Status: ConceptCurrent, EvidenceRefs: []EvidenceRef{evidence}},
		},
		Prerequisites: []Prerequisite{{ConceptID: useConcept, RequiredConceptID: rootConcept, Kind: PrerequisiteHard, EvidenceRefs: []EvidenceRef{evidence}}},
		Vocabulary:    VocabularyGraph{Terms: []VocabularyTerm{{Term: "foundation", CanonicalConceptID: rootConcept, IntroducedBy: rootConcept, UsedBy: []ConceptID{useConcept}, Aliases: []string{"base"}, Scope: "curriculum"}}},
		Phases:        []Phase{{ID: mustID(t, "phase.one"), Title: "Phase", Description: "First phase", Order: 0}},
		Modules:       []Module{{ID: mustID(t, "module.one"), PhaseID: mustID(t, "phase.one"), Title: "Module", Description: "First module", Order: 0}},
		Lessons:       []LessonSpec{{ID: mustID(t, "lesson.one"), ModuleID: mustID(t, "module.one"), Title: "Lesson", Description: "First lesson", Order: 0}},
		Topics:        []TopicSpec{{ID: mustID(t, "topic.one"), LessonID: mustID(t, "lesson.one"), Title: "Topic", Description: "First topic", Order: 0, ConceptIDs: []ConceptID{rootConcept, useConcept}}},
		CoverageRequirements: []CoverageRequirement{{
			ID: mustID(t, "coverage.apply"), Dimension: CoverageCompetency, TargetKind: CoverageTargetCompetency,
			TargetID: mustID(t, "competency.apply"), Description: "Application competency is represented.", EvidenceRefs: []EvidenceRef{evidence},
		}},
		SourcePolicy: SourceReferencesRequired, SourceBundles: []SourceBundleRef{bundle}, CreatedAt: createdAt,
	}
}

func testSourceBundleRef(t *testing.T) SourceBundleRef {
	t.Helper()
	verifiedAt, err := NewTimestamp(time.Date(2026, time.September, 9, 17, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return SourceBundleRef{
		ID: mustID(t, "bundle.core"), ContentHash: "sha256:" + strings.Repeat("a", 64),
		AlgorithmVersion: "source-bundle-v1", VerifiedAt: verifiedAt,
	}
}

func mustID(t *testing.T, value string) ID {
	t.Helper()
	id, err := NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustConceptID(t *testing.T, value string) ConceptID {
	t.Helper()
	id, err := NewConceptID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustCurriculumID(t *testing.T, value string) CurriculumID {
	t.Helper()
	id, err := NewCurriculumID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
