package learning

import (
	"fmt"
	"sort"
)

const MasteryAlgorithmVersion = "mastery-v1"

// MasteryContribution is the complete, auditable effect of one evidence item
// after mastery-v1 has applied its type and metadata weights.
type MasteryContribution struct {
	EvidenceID       ID
	Type             EvidenceType
	Score            MasteryScore
	TypeWeight       float64
	Confidence       float64
	Independence     float64
	Difficulty       float64
	EffectiveWeight  float64
	NormalizedWeight float64
	WeightedScore    float64
	OccurredAt       Timestamp
	SourceRef        string
}

// MasteryCalculation distinguishes absence of knowledge from observed
// failure with Known. A known calculation may validly have score zero.
type MasteryCalculation struct {
	StudentID     ID
	ConceptID     ID
	Known         bool
	Score         MasteryScore
	EvidenceCount int
	TotalWeight   float64
	PolicyVersion string
	Contributions []MasteryContribution
}

func (calculation MasteryCalculation) MeetsThreshold(threshold MasteryThreshold) (bool, error) {
	if err := threshold.Validate(); err != nil {
		return false, err
	}
	return calculation.Known && calculation.Score.Value() >= threshold.Value(), nil
}

// CalculateMasteryV1 computes a deterministic weighted mean. It deliberately
// performs no time decay; retention owns the effect of age in a later policy.
func CalculateMasteryV1(studentID, conceptID ID, items []Evidence) (MasteryCalculation, error) {
	if err := studentID.Validate(); err != nil {
		return MasteryCalculation{}, fmt.Errorf("mastery student: %w", err)
	}
	if err := conceptID.Validate(); err != nil {
		return MasteryCalculation{}, fmt.Errorf("mastery concept: %w", err)
	}
	calculation := MasteryCalculation{StudentID: studentID, ConceptID: conceptID, PolicyVersion: MasteryAlgorithmVersion}
	if len(items) == 0 {
		return calculation, nil
	}

	canonical := append([]Evidence(nil), items...)
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].ObservedAt == canonical[j].ObservedAt {
			return canonical[i].ID.String() < canonical[j].ID.String()
		}
		return canonical[i].ObservedAt.Before(canonical[j].ObservedAt)
	})
	seen := make(map[ID]struct{}, len(canonical))
	weightedSum := 0.0
	for _, evidence := range canonical {
		if err := evidence.Validate(); err != nil {
			return MasteryCalculation{}, fmt.Errorf("mastery evidence %q: %w", evidence.ID, err)
		}
		if evidence.StudentID != studentID || evidence.ConceptID != conceptID {
			return MasteryCalculation{}, fmt.Errorf("mastery evidence %q belongs to another student or concept", evidence.ID)
		}
		if _, exists := seen[evidence.ID]; exists {
			return MasteryCalculation{}, fmt.Errorf("mastery contains duplicate evidence %q", evidence.ID)
		}
		seen[evidence.ID] = struct{}{}

		typeWeight := masteryEvidenceTypeWeight(evidence.Type)
		independenceFactor := .25 + .75*evidence.Independence
		difficultyFactor := .75 + .5*evidence.Difficulty
		effectiveWeight := typeWeight * evidence.Confidence * independenceFactor * difficultyFactor
		weightedScore := evidence.Score.Value() * effectiveWeight
		calculation.TotalWeight += effectiveWeight
		weightedSum += weightedScore
		calculation.Contributions = append(calculation.Contributions, MasteryContribution{
			EvidenceID: evidence.ID, Type: evidence.Type, Score: evidence.Score,
			TypeWeight: typeWeight, Confidence: evidence.Confidence,
			Independence: evidence.Independence, Difficulty: evidence.Difficulty,
			EffectiveWeight: effectiveWeight, WeightedScore: weightedScore,
			OccurredAt: evidence.ObservedAt, SourceRef: evidence.Source,
		})
	}

	score, err := NewMasteryScore(weightedSum / calculation.TotalWeight)
	if err != nil {
		return MasteryCalculation{}, fmt.Errorf("mastery result: %w", err)
	}
	calculation.Known = true
	calculation.Score = score
	calculation.EvidenceCount = len(canonical)
	for index := range calculation.Contributions {
		calculation.Contributions[index].NormalizedWeight = calculation.Contributions[index].EffectiveWeight / calculation.TotalWeight
	}
	return calculation, nil
}

func masteryEvidenceTypeWeight(evidenceType EvidenceType) float64 {
	switch evidenceType {
	case EvidenceAssessment, EvidenceProject:
		return 1
	case EvidenceDiagnosticObjective, EvidenceKnowledgeCheck, EvidenceReviewRecall:
		return .9
	case EvidencePracticeSuccess, EvidencePracticeFailure:
		return .75
	case EvidenceManualImport:
		return .5
	case EvidenceDiagnosticSelfReport:
		return .4
	default:
		return 0 // Evidence.Validate rejects this path before the value is used.
	}
}
