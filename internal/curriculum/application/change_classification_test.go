package application

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/research"
)

func TestCurriculumChangeClassifierV1ClassifiesEveryStructuralDimension(t *testing.T) {
	t.Parallel()
	old := changeDefinitionFixture(t)
	newDefinition := nextDefinition(t, old, "2026.09.14.2")
	newDefinition.Title = "Updated HTTP curriculum"
	newDefinition.Concepts = append([]curriculum.Concept(nil), old.Concepts...)
	newDefinition.Concepts[0].Status = curriculum.ConceptDeprecated
	addedID, _ := curriculum.NewConceptID("concept.http.requests")
	added := old.Concepts[0]
	added.ID, added.Title, added.Definition = addedID, "HTTP requests", "An HTTP request asks a server to perform an operation."
	newDefinition.Concepts = append(newDefinition.Concepts, added)
	newDefinition.Competencies.Competencies = append([]curriculum.Competency(nil), old.Competencies.Competencies...)
	newDefinition.Competencies.Competencies[0].ConceptRefs = append(append([]curriculum.ConceptID(nil), old.Competencies.Competencies[0].ConceptRefs...), addedID)
	newDefinition.Topics = append([]curriculum.TopicSpec(nil), old.Topics...)
	newDefinition.Topics[0].ConceptIDs = append(append([]curriculum.ConceptID(nil), old.Topics[0].ConceptIDs...), addedID)
	newDefinition.Prerequisites = append([]curriculum.Prerequisite(nil), old.Prerequisites...)
	newDefinition.Prerequisites = append(newDefinition.Prerequisites, curriculum.Prerequisite{ConceptID: addedID, RequiredConceptID: old.Concepts[0].ID, Kind: curriculum.PrerequisiteHard, EvidenceRefs: old.Concepts[0].EvidenceRefs})
	newDefinition.SourceBundles = append([]curriculum.SourceBundleRef(nil), old.SourceBundles...)
	newDefinition.SourceBundles[0].ContentHash = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	environment := changeEnvironmentFixture(t, addedID)

	result, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), CurriculumChangeClassificationRequest{Old: old, New: newDefinition, NewEnvironment: &environment})
	if err != nil {
		t.Fatal(err)
	}
	want := []curriculum.CurriculumChangeKind{
		curriculum.ChangeConceptAdded, curriculum.ChangeEnvironmentChanged, curriculum.ChangeHierarchyChanged,
		curriculum.ChangeMetadataOnly, curriculum.ChangePrerequisiteChanged, curriculum.ChangeSourceRefresh,
		curriculum.ChangeStatusChanged,
	}
	actual := make([]curriculum.CurriculumChangeKind, len(result.Changes))
	for index, change := range result.Changes {
		actual[index] = change.Kind
	}
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if !reflect.DeepEqual(actual, want) || result.AlgorithmVersion != curriculum.CurriculumChangeClassifierVersionV1 {
		t.Fatalf("changes = %+v, want kinds %v", result.Changes, want)
	}
	if changeOfKind(t, result, curriculum.ChangeConceptAdded).Migration != curriculum.MigrationSafe ||
		changeOfKind(t, result, curriculum.ChangeStatusChanged).Migration != curriculum.MigrationRequiresStudentReview {
		t.Fatalf("unexpected migration classes: %+v", result.Changes)
	}
}

func TestCurriculumChangeClassifierV1RequiresExplicitSplitAndMergeMappings(t *testing.T) {
	t.Parallel()
	old := changeDefinitionFixture(t)
	newDefinition := splitDefinition(t, old, "2026.09.14.2")
	oldID := old.Concepts[0].ID
	newIDs := []curriculum.ConceptID{newDefinition.Concepts[0].ID, newDefinition.Concepts[1].ID}
	request := CurriculumChangeClassificationRequest{
		Old: old, New: newDefinition,
		IdentityMappings: []curriculum.ConceptIdentityMapping{{OldConceptIDs: []curriculum.ConceptID{oldID}, NewConceptIDs: newIDs, Rationale: "The original unit contained two independently assessable ideas."}},
	}
	unmapped, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), CurriculumChangeClassificationRequest{Old: old, New: newDefinition})
	if err != nil || hasChangeKind(unmapped, curriculum.ChangeConceptSplit) || !hasChangeKind(unmapped, curriculum.ChangeConceptAdded) || !hasChangeKind(unmapped, curriculum.ChangeConceptRemoved) {
		t.Fatalf("unmapped identity change = %+v, %v", unmapped, err)
	}
	result, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	split := changeOfKind(t, result, curriculum.ChangeConceptSplit)
	if split.Migration != curriculum.MigrationBreaking || hasChangeKind(result, curriculum.ChangeConceptAdded) || hasChangeKind(result, curriculum.ChangeConceptRemoved) {
		t.Fatalf("split classification = %+v", result.Changes)
	}

	reordered := request
	reordered.IdentityMappings = []curriculum.ConceptIdentityMapping{{OldConceptIDs: []curriculum.ConceptID{oldID}, NewConceptIDs: []curriculum.ConceptID{newIDs[1], newIDs[0]}, Rationale: request.IdentityMappings[0].Rationale}}
	repeated, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("reordered mapping changed result: %+v / %+v / %v", result, repeated, err)
	}

	merged := nextDefinition(t, old, "2026.09.14.3")
	mergeResult, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), CurriculumChangeClassificationRequest{
		Old: newDefinition, New: merged,
		IdentityMappings: []curriculum.ConceptIdentityMapping{{OldConceptIDs: newIDs, NewConceptIDs: []curriculum.ConceptID{oldID}, Rationale: "The replacement restores one explicit tracking identity."}},
	})
	if err != nil || changeOfKind(t, mergeResult, curriculum.ChangeConceptMerged).Migration != curriculum.MigrationBreaking {
		t.Fatalf("merge classification = %+v, %v", mergeResult, err)
	}
}

func TestCurriculumChangeClassifierV1ConsumesDriftImpactConservatively(t *testing.T) {
	t.Parallel()
	old := changeDefinitionFixture(t)
	newDefinition := nextDefinition(t, old, "2026.09.14.2")
	newDefinition.SourceBundles = append([]curriculum.SourceBundleRef(nil), old.SourceBundles...)
	newDefinition.SourceBundles[0].ContentHash = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	drift, impact := changeResearchSignals(t, old)
	result, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), CurriculumChangeClassificationRequest{Old: old, New: newDefinition, DriftReports: []research.DriftReport{drift}, ImpactReports: []research.ImpactReport{impact}})
	if err != nil {
		t.Fatal(err)
	}
	change := changeOfKind(t, result, curriculum.ChangeSourceRefresh)
	if change.Migration != curriculum.MigrationRequiresStudentReview || !reflect.DeepEqual(change.AffectedConcepts, []curriculum.ConceptID{old.Concepts[0].ID}) {
		t.Fatalf("source refresh = %+v", change)
	}
}

func TestCurriculumChangeClassifierV1ClassifiesDisplayOnlyMetadataAsSafe(t *testing.T) {
	t.Parallel()
	old := changeDefinitionFixture(t)
	newDefinition := nextDefinition(t, old, "2026.09.14.2")
	newDefinition.Description = "A clarified display description."
	result, err := NewCurriculumChangeClassifierV1().Classify(context.Background(), CurriculumChangeClassificationRequest{Old: old, New: newDefinition})
	if err != nil || len(result.Changes) != 1 || result.Changes[0].Kind != curriculum.ChangeMetadataOnly || result.Changes[0].Migration != curriculum.MigrationSafe {
		t.Fatalf("metadata classification = %+v, %v", result, err)
	}
}

func changeDefinitionFixture(t *testing.T) curriculum.CurriculumDefinition {
	t.Helper()
	compiled, err := NewCurriculumCompilerV1().Compile(context.Background(), compilerFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	return compiled.Curriculum
}

func nextDefinition(t *testing.T, old curriculum.CurriculumDefinition, version string) curriculum.CurriculumDefinition {
	t.Helper()
	result := old
	result.Version, _ = curriculum.NewCurriculumVersion(version)
	return result
}

func splitDefinition(t *testing.T, old curriculum.CurriculumDefinition, version string) curriculum.CurriculumDefinition {
	t.Helper()
	result := nextDefinition(t, old, version)
	firstID, _ := curriculum.NewConceptID("concept.http.syntax")
	secondID, _ := curriculum.NewConceptID("concept.http.semantics")
	first, second := old.Concepts[0], old.Concepts[0]
	first.ID, first.Title, first.Definition = firstID, "HTTP syntax", "HTTP syntax defines the wire representation."
	second.ID, second.Title, second.Definition = secondID, "HTTP semantics", "HTTP semantics define request meaning."
	result.Concepts = []curriculum.Concept{first, second}
	result.Prerequisites = nil
	result.Vocabulary = curriculum.VocabularyGraph{}
	result.Competencies.Competencies = append([]curriculum.Competency(nil), old.Competencies.Competencies...)
	result.Competencies.Competencies[0].ConceptRefs = []curriculum.ConceptID{firstID, secondID}
	result.Topics = append([]curriculum.TopicSpec(nil), old.Topics...)
	result.Topics[0].ConceptIDs = []curriculum.ConceptID{firstID, secondID}
	return result
}

func changeEnvironmentFixture(t *testing.T, conceptID curriculum.ConceptID) curriculum.EnvironmentPack {
	t.Helper()
	return curriculum.EnvironmentPack{
		ID: curriculumID(t, "environment.http"), Version: packVersion(t, "1.0.0"), SchemaVersion: curriculum.EnvironmentPackSchemaVersionV1,
		SupportedPlatforms: []string{curriculum.EnvironmentPlatformLinux},
		Tools:              []curriculum.ToolRequirement{{ID: curriculumID(t, "tool.curl"), Purpose: "Inspect HTTP exchanges.", Level: curriculum.ToolRecommended, WhenNeeded: &conceptID, Platforms: []string{curriculum.EnvironmentPlatformLinux}}},
	}
}

func changeResearchSignals(t *testing.T, definition curriculum.CurriculumDefinition) (research.DriftReport, research.ImpactReport) {
	t.Helper()
	id := func(value string) research.ID { result, _ := research.NewID(value); return result }
	claim := func(value string) research.ClaimID { result, _ := research.NewClaimID(value); return result }
	timestamp, _ := research.NewTimestamp(time.Date(2026, 9, 14, 14, 0, 0, 0, time.UTC))
	confidence, _ := research.NewClaimConfidence(.95)
	bundleID := id(definition.SourceBundles[0].ID.String())
	newBundleID := bundleID
	drift := research.DriftReport{
		ID: id("drift.curriculum"), OldBundleID: bundleID, NewBundleID: &newBundleID, Type: research.DriftRecommendationChanged,
		Severity: research.SeverityCritical, AffectedClaims: []research.ClaimID{claim(definition.Concepts[0].EvidenceRefs[0].ClaimID.String())},
		OldEvidence: []research.ID{id("evidence.old")}, NewEvidence: []research.ID{id("evidence.new")}, Confidence: confidence,
		DetectedAt: timestamp, AlgorithmVersion: research.DriftAlgorithmV1,
	}
	impact := research.ImpactReport{
		ID: id("impact.curriculum"), DriftReportID: drift.ID, AffectedEvidenceIDs: []research.ID{id("evidence.old")},
		AffectedBundleIDs: []research.ID{bundleID}, AffectedClaimIDs: drift.AffectedClaims,
		FutureConceptRefs: []research.ID{id(definition.Concepts[0].ID.String())}, Severity: research.SeverityCritical,
		RecommendedAction: research.ActionManualReview, AssessedAt: timestamp, AlgorithmVersion: research.ImpactAnalysisAlgorithmV1,
	}
	return drift, impact
}

func changeOfKind(t *testing.T, result curriculum.CurriculumChangeClassification, kind curriculum.CurriculumChangeKind) curriculum.CurriculumChange {
	t.Helper()
	for _, change := range result.Changes {
		if change.Kind == kind {
			return change
		}
	}
	t.Fatalf("change %q missing from %+v", kind, result.Changes)
	return curriculum.CurriculumChange{}
}

func hasChangeKind(result curriculum.CurriculumChangeClassification, kind curriculum.CurriculumChangeKind) bool {
	for _, change := range result.Changes {
		if change.Kind == kind {
			return true
		}
	}
	return false
}
