package application

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/learning"
)

type masteryCalculationService struct {
	evidence EvidenceRepository
}

func NewMasteryCalculationService(evidence EvidenceRepository) MasteryCalculationService {
	return &masteryCalculationService{evidence: evidence}
}

func (service *masteryCalculationService) Calculate(ctx context.Context, studentID, conceptID learning.ID) (learning.MasteryCalculation, error) {
	const operation = "calculate concept mastery"
	if err := validatePair("student", studentID, "concept", conceptID); err != nil {
		return learning.MasteryCalculation{}, invalid(operation, err)
	}
	if err := requireRepository(operation, service.evidence); err != nil {
		return learning.MasteryCalculation{}, err
	}
	items, err := service.evidence.ListByConcept(ctx, studentID, conceptID)
	if err != nil {
		return learning.MasteryCalculation{}, repositoryError(operation, err)
	}
	calculation, err := learning.CalculateMasteryV1(studentID, conceptID, items)
	if err != nil {
		return learning.MasteryCalculation{}, invalid(operation, err)
	}
	return calculation, nil
}

func (service *masteryCalculationService) Explain(ctx context.Context, studentID, conceptID learning.ID) (MasteryExplanation, error) {
	calculation, err := service.Calculate(ctx, studentID, conceptID)
	if err != nil {
		return MasteryExplanation{}, err
	}
	if !calculation.Known {
		return MasteryExplanation{
			Calculation: calculation,
			Summary:     fmt.Sprintf("Concept %s mastery is unknown because no evidence has been recorded (policy %s).", conceptID, calculation.PolicyVersion),
		}, nil
	}
	return MasteryExplanation{
		Calculation: calculation,
		Summary: fmt.Sprintf("Concept %s mastery is %.0f%% from %d evidence item(s), with total weight %.3f (policy %s).",
			conceptID, calculation.Score.Value()*100, calculation.EvidenceCount, calculation.TotalWeight, calculation.PolicyVersion),
	}, nil
}

var _ MasteryCalculationService = (*masteryCalculationService)(nil)
