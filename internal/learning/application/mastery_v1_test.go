package application_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestMasteryCalculationServiceLoadsEvidenceAndExplainsKnownResult(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	studentID := testID(t, "student.mastery")
	conceptID := testID(t, "concept.variables")
	first, err := learning.NewEvidence(testID(t, "evidence.first"), studentID, conceptID, learning.EvidenceAssessment,
		"fixture/assessment", testScore(t, .8), testTimestamp(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := learning.NewEvidence(testID(t, "evidence.second"), studentID, conceptID, learning.EvidencePracticeFailure,
		"fixture/practice", testScore(t, .2), testTimestamp(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []learning.Evidence{second, first} {
		if err := store.Repositories().Evidence.Append(ctx, item); err != nil {
			t.Fatal(err)
		}
	}

	service := application.NewMasteryCalculationService(store.Repositories().Evidence)
	explanation, err := service.Explain(ctx, studentID, conceptID)
	if err != nil {
		t.Fatal(err)
	}
	if !explanation.Calculation.Known || explanation.Calculation.EvidenceCount != 2 ||
		!strings.Contains(explanation.Summary, "2 evidence item(s)") || !strings.Contains(explanation.Summary, learning.MasteryAlgorithmVersion) {
		t.Fatalf("Explain() = %+v", explanation)
	}
}

func TestMasteryCalculationServiceExplainsUnknownWithoutInventingZero(t *testing.T) {
	t.Parallel()
	studentID := testID(t, "student.mastery")
	conceptID := testID(t, "concept.unknown")
	service := application.NewMasteryCalculationService(memory.New().Repositories().Evidence)

	explanation, err := service.Explain(context.Background(), studentID, conceptID)
	if err != nil || explanation.Calculation.Known || !strings.Contains(explanation.Summary, "unknown") {
		t.Fatalf("Explain(no evidence) = (%+v, %v)", explanation, err)
	}
}
