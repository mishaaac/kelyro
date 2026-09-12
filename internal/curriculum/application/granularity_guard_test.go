package application

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestGranularityGuardV1ForcesBroadSplitAndRejectsUnsafeMerge(t *testing.T) {
	t.Parallel()
	atomic := granularityConcept(t, "concept.atomic", curriculum.AtomicityAtomic)
	broad := granularityConcept(t, "concept.broad", curriculum.AtomicityNeedsSplit)
	tiny := granularityConcept(t, "concept.tiny", curriculum.AtomicityTooFragmented)
	broadMerge := atomicCriteria()
	broadMerge.IndependentlyAssessableParts = []string{"atomic", "tiny"}
	request := GranularityReviewRequest{
		Concepts: []curriculum.Concept{tiny, broad, atomic},
		MergeProposals: []curriculum.ConceptMergeProposal{
			{ID: curriculumID(t, "merge.safe"), ConceptIDs: []curriculum.ConceptID{tiny.ID, atomic.ID}, MergedCriteria: atomicCriteria()},
			{ID: curriculumID(t, "merge.unsafe"), ConceptIDs: []curriculum.ConceptID{atomic.ID, broad.ID}, MergedCriteria: broadMerge},
		},
		VisualGroups: []curriculum.VisualConceptGroup{{LessonID: curriculumID(t, "lesson.compact"), ConceptIDs: []curriculum.ConceptID{tiny.ID, atomic.ID}}},
	}

	result, err := NewGranularityGuardV1(curriculum.NewAtomicConceptPolicyV1()).Review(context.Background(), request)
	if err != nil {
		t.Fatalf("Review() error = %v", err)
	}
	if !reflect.DeepEqual(result.ForcedSplit, []curriculum.ConceptID{broad.ID}) {
		t.Fatalf("forced splits = %+v", result.ForcedSplit)
	}
	if len(result.MergeDecisions) != 2 || !result.MergeDecisions[0].Allowed || result.MergeDecisions[1].Allowed {
		t.Fatalf("merge decisions = %+v", result.MergeDecisions)
	}
	if len(result.SafeVisualGrouping) != 1 || !reflect.DeepEqual(result.SafeVisualGrouping[0].ConceptIDs, []curriculum.ConceptID{atomic.ID, tiny.ID}) {
		t.Fatalf("visual grouping = %+v", result.SafeVisualGrouping)
	}
	if len(result.Warnings) != 3 {
		t.Fatalf("warnings = %+v", result.Warnings)
	}
}

func TestGranularityGuardV1HasNoArtificialConceptOrVisualGroupLimit(t *testing.T) {
	t.Parallel()
	const size = 5000
	concepts := make([]curriculum.Concept, 0, size)
	conceptIDs := make([]curriculum.ConceptID, 0, size)
	for index := 0; index < size; index++ {
		concept := granularityConcept(t, fmt.Sprintf("concept.large.%05d", index), curriculum.AtomicityAtomic)
		concepts = append(concepts, concept)
		conceptIDs = append(conceptIDs, concept.ID)
	}
	result, err := NewGranularityGuardV1(curriculum.NewAtomicConceptPolicyV1()).Review(context.Background(), GranularityReviewRequest{
		Concepts:     concepts,
		VisualGroups: []curriculum.VisualConceptGroup{{LessonID: curriculumID(t, "lesson.large"), ConceptIDs: conceptIDs}},
	})
	if err != nil {
		t.Fatalf("Review() large fixture error = %v", err)
	}
	if len(result.SafeVisualGrouping) != 1 || len(result.SafeVisualGrouping[0].ConceptIDs) != size || len(result.ForcedSplit) != 0 {
		t.Fatalf("large result sizes = groups:%d concepts:%d splits:%d", len(result.SafeVisualGrouping), len(result.SafeVisualGrouping[0].ConceptIDs), len(result.ForcedSplit))
	}
}

func TestGranularityGuardV1IsDeterministicWithoutMutatingProposals(t *testing.T) {
	t.Parallel()
	first := granularityConcept(t, "concept.z", curriculum.AtomicityAtomic)
	second := granularityConcept(t, "concept.a", curriculum.AtomicityAtomic)
	request := GranularityReviewRequest{
		Concepts: []curriculum.Concept{first, second},
		MergeProposals: []curriculum.ConceptMergeProposal{{
			ID: curriculumID(t, "merge.values"), ConceptIDs: []curriculum.ConceptID{first.ID, second.ID}, MergedCriteria: atomicCriteria(),
		}},
	}
	original := append([]curriculum.ConceptID(nil), request.MergeProposals[0].ConceptIDs...)
	guard := NewGranularityGuardV1(curriculum.NewAtomicConceptPolicyV1())
	result, err := guard.Review(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := guard.Review(context.Background(), request)
	if err != nil || !reflect.DeepEqual(result, repeated) {
		t.Fatalf("repeated review differs: %+v / %+v / %v", result, repeated, err)
	}
	if !reflect.DeepEqual(request.MergeProposals[0].ConceptIDs, original) {
		t.Fatal("guard mutated merge proposal")
	}
}

func granularityConcept(t *testing.T, rawID string, atomicity curriculum.Atomicity) curriculum.Concept {
	t.Helper()
	id, err := curriculum.NewConceptID(rawID)
	if err != nil {
		t.Fatal(err)
	}
	return curriculum.Concept{
		ID: id, Title: rawID, Definition: "A testable knowledge unit.", Version: "concept-v1",
		Atomicity: atomicity, Difficulty: curriculum.DifficultyFoundational, Status: curriculum.ConceptCurrent,
	}
}
