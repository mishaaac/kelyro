package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestTemporalClassificationV1TransfersClaimStatusToConceptsAndLessons(t *testing.T) {
	t.Parallel()
	request, concepts := temporalClassificationFixture(t)
	report, err := NewTemporalClassificationV1().Classify(context.Background(), request)
	if err != nil {
		t.Fatalf("Classify() error = %v", err)
	}
	if report.AlgorithmVersion != curriculum.TemporalClassificationVersionV1 || len(report.Classifications) != len(concepts)+1 {
		t.Fatalf("temporal report = %+v", report)
	}
	current := temporalClassificationFor(t, report, curriculum.TemporalTargetConcept, concepts[0].ID.String())
	if current.Status != curriculum.ConceptCurrent || !current.PrimaryRecommendation || current.Separated || current.ContextOnly {
		t.Fatalf("current classification = %+v", current)
	}
	experimental := temporalClassificationFor(t, report, curriculum.TemporalTargetConcept, concepts[1].ID.String())
	if experimental.Status != curriculum.ConceptExperimental || !experimental.Separated || experimental.PrimaryRecommendation {
		t.Fatalf("experimental classification = %+v", experimental)
	}
	historical := temporalClassificationFor(t, report, curriculum.TemporalTargetConcept, concepts[2].ID.String())
	if historical.Status != curriculum.ConceptHistorical || !historical.ContextOnly || historical.PrimaryRecommendation || historical.DeclaredStatus != curriculum.ConceptCurrent {
		t.Fatalf("historical classification = %+v", historical)
	}
	legacy := temporalClassificationFor(t, report, curriculum.TemporalTargetConcept, concepts[3].ID.String())
	if legacy.Status != curriculum.ConceptLegacy || !legacy.ContextOnly || !legacy.ContextualUse {
		t.Fatalf("legacy classification = %+v", legacy)
	}
	deprecated := temporalClassificationFor(t, report, curriculum.TemporalTargetConcept, concepts[4].ID.String())
	if deprecated.Status != curriculum.ConceptDeprecated || deprecated.PrimaryRecommendation || deprecated.Separated || deprecated.ContextOnly {
		t.Fatalf("deprecated classification = %+v", deprecated)
	}
	preview := temporalClassificationFor(t, report, curriculum.TemporalTargetConcept, concepts[5].ID.String())
	if preview.Status != curriculum.ConceptPreview || !preview.Separated || preview.PrimaryRecommendation {
		t.Fatalf("preview classification = %+v", preview)
	}
	lesson := temporalClassificationFor(t, report, curriculum.TemporalTargetLesson, "lesson.mixed")
	if lesson.Status != curriculum.ConceptExperimental || !lesson.Separated || len(lesson.EvidenceRefs) != 2 {
		t.Fatalf("lesson classification = %+v", lesson)
	}

	reordered := request
	reordered.Concepts = append([]curriculum.Concept(nil), request.Concepts...)
	for left, right := 0, len(reordered.Concepts)-1; left < right; left, right = left+1, right-1 {
		reordered.Concepts[left], reordered.Concepts[right] = reordered.Concepts[right], reordered.Concepts[left]
	}
	reordered.Lessons[0].ConceptIDs = []curriculum.ConceptID{concepts[1].ID, concepts[0].ID}
	repeated, err := NewTemporalClassificationV1().Classify(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(report, repeated) {
		t.Fatalf("reordered temporal classification differs: %+v / %+v / %v", report, repeated, err)
	}
}

func temporalClassificationFixture(t *testing.T) (TemporalClassificationRequest, []curriculum.Concept) {
	t.Helper()
	evidence, _ := decompositionEvidence(t)
	currentSource := evidence.SourceAuthority[0].SourceID
	historicalSource := curriculumID(t, "source.temporal.historical")
	evidence.SourceAuthority = append(evidence.SourceAuthority, curriculum.EvidenceSourceAuthority{
		SourceID: historicalSource, Role: "historical", TemporalScope: "historical",
	})
	claims := []curriculum.CurriculumEvidenceClaim{
		{ID: curriculumID(t, "claim.temporal.current"), Statement: "Use the current mechanism.", Kind: curriculum.EvidenceClaimBehavior, Scope: "current mechanism", Status: curriculum.ConceptCurrent, Confidence: .95, SourceIDs: []curriculum.ID{currentSource}},
		{ID: curriculumID(t, "claim.temporal.experimental"), Statement: "The feature is experimental.", Kind: curriculum.EvidenceClaimBehavior, Scope: "experimental feature", Status: curriculum.ConceptExperimental, Confidence: .9, SourceIDs: []curriculum.ID{currentSource}},
		{ID: curriculumID(t, "claim.temporal.historical"), Statement: "The historical mechanism behaved differently.", Kind: curriculum.EvidenceClaimBehavior, Scope: "historical mechanism", Status: curriculum.ConceptCurrent, Confidence: .9, SourceIDs: []curriculum.ID{historicalSource}},
		{ID: curriculumID(t, "claim.temporal.legacy"), Statement: "The legacy mechanism remains in maintenance.", Kind: curriculum.EvidenceClaimCompatibility, Scope: "legacy mechanism", Status: curriculum.ConceptLegacy, Confidence: .9, SourceIDs: []curriculum.ID{currentSource}},
		{ID: curriculumID(t, "claim.temporal.deprecated"), Statement: "The old mechanism is deprecated.", Kind: curriculum.EvidenceClaimDeprecation, Scope: "deprecated mechanism", Status: curriculum.ConceptCurrent, Confidence: .95, SourceIDs: []curriculum.ID{currentSource}},
		{ID: curriculumID(t, "claim.temporal.preview"), Statement: "The feature is available as preview.", Kind: curriculum.EvidenceClaimBehavior, Scope: "preview feature", Status: curriculum.ConceptPreview, Confidence: .9, SourceIDs: []curriculum.ID{currentSource}},
	}
	evidence.Claims = claims
	if err := evidence.Validate(); err != nil {
		t.Fatal(err)
	}
	concepts := make([]curriculum.Concept, len(claims))
	for index, claim := range claims {
		concepts[index] = graphConcept(t, "concept.temporal."+string(rune('a'+index)), false)
		concepts[index].Status = curriculum.ConceptCurrent
		concepts[index].EvidenceRefs = []curriculum.EvidenceRef{{BundleID: evidence.Bundle.ID, ClaimID: claim.ID}}
	}
	lesson := curriculum.LessonTemporalInput{
		Lesson:     curriculum.LessonSpec{ID: curriculumID(t, "lesson.mixed"), ModuleID: curriculumID(t, "module.temporal"), Title: "Mixed status", Description: "Current and experimental material.", Order: 0},
		ConceptIDs: []curriculum.ConceptID{concepts[0].ID, concepts[1].ID}, DeclaredStatus: curriculum.ConceptCurrent,
	}
	return TemporalClassificationRequest{
		Concepts: concepts, ContextualConceptIDs: []curriculum.ConceptID{concepts[2].ID, concepts[3].ID},
		Lessons: []curriculum.LessonTemporalInput{lesson}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	}, concepts
}

func temporalClassificationFor(t *testing.T, result curriculum.TemporalClassificationResult, kind curriculum.TemporalTargetKind, id string) curriculum.TemporalClassification {
	t.Helper()
	for _, classification := range result.Classifications {
		if classification.TargetKind == kind && classification.TargetID.String() == id {
			return classification
		}
	}
	t.Fatalf("missing temporal classification %s/%s", kind, id)
	return curriculum.TemporalClassification{}
}
