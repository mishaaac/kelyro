package learning

import (
	"fmt"
	"math"
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

func (calculation MasteryCalculation) Validate() error {
	if err := calculation.StudentID.Validate(); err != nil {
		return fmt.Errorf("mastery calculation student: %w", err)
	}
	if err := calculation.ConceptID.Validate(); err != nil {
		return fmt.Errorf("mastery calculation concept: %w", err)
	}
	if calculation.PolicyVersion != MasteryAlgorithmVersion {
		return fmt.Errorf("unsupported mastery calculation policy %q", calculation.PolicyVersion)
	}
	if err := calculation.Score.Validate(); err != nil {
		return fmt.Errorf("mastery calculation: %w", err)
	}
	if math.IsNaN(calculation.TotalWeight) || math.IsInf(calculation.TotalWeight, 0) || calculation.TotalWeight < 0 {
		return fmt.Errorf("mastery calculation total weight is invalid")
	}
	if !calculation.Known {
		if calculation.Score.Value() != 0 || calculation.EvidenceCount != 0 || calculation.TotalWeight != 0 || len(calculation.Contributions) != 0 {
			return fmt.Errorf("unknown mastery calculation cannot contain evidence")
		}
		return nil
	}
	if calculation.EvidenceCount == 0 || calculation.EvidenceCount != len(calculation.Contributions) || calculation.TotalWeight <= 0 {
		return fmt.Errorf("known mastery calculation has inconsistent evidence totals")
	}
	seen := make(map[ID]struct{}, len(calculation.Contributions))
	effectiveWeightSum := 0.0
	normalizedWeightSum := 0.0
	weightedScoreSum := 0.0
	for index, contribution := range calculation.Contributions {
		if err := contribution.EvidenceID.Validate(); err != nil {
			return fmt.Errorf("mastery contribution: %w", err)
		}
		if !contribution.Type.Valid() {
			return fmt.Errorf("mastery contribution evidence type %q is invalid", contribution.Type)
		}
		if err := contribution.Score.Validate(); err != nil {
			return fmt.Errorf("mastery contribution: %w", err)
		}
		if err := contribution.OccurredAt.Validate(); err != nil {
			return fmt.Errorf("mastery contribution occurred at: %w", err)
		}
		if err := requireText("mastery contribution source ref", contribution.SourceRef); err != nil {
			return err
		}
		for _, field := range []struct {
			name  string
			value float64
		}{
			{name: "type weight", value: contribution.TypeWeight},
			{name: "confidence", value: contribution.Confidence},
			{name: "independence", value: contribution.Independence},
			{name: "difficulty", value: contribution.Difficulty},
			{name: "effective weight", value: contribution.EffectiveWeight},
			{name: "normalized weight", value: contribution.NormalizedWeight},
			{name: "weighted score", value: contribution.WeightedScore},
		} {
			if math.IsNaN(field.value) || math.IsInf(field.value, 0) || field.value < 0 {
				return fmt.Errorf("mastery contribution %s is invalid", field.name)
			}
		}
		if contribution.TypeWeight != masteryEvidenceTypeWeight(contribution.Type) || contribution.Confidence <= 0 || contribution.Confidence > 1 ||
			contribution.Independence > 1 || contribution.Difficulty > 1 || contribution.EffectiveWeight <= 0 || contribution.NormalizedWeight > 1 {
			return fmt.Errorf("mastery contribution weights are inconsistent")
		}
		wantEffective := contribution.TypeWeight * contribution.Confidence * (.25 + .75*contribution.Independence) * (.75 + .5*contribution.Difficulty)
		if math.Abs(contribution.EffectiveWeight-wantEffective) > 1e-12 ||
			math.Abs(contribution.WeightedScore-contribution.Score.Value()*contribution.EffectiveWeight) > 1e-12 {
			return fmt.Errorf("mastery contribution formula is inconsistent")
		}
		if _, exists := seen[contribution.EvidenceID]; exists {
			return fmt.Errorf("mastery calculation contains duplicate contribution %q", contribution.EvidenceID)
		}
		seen[contribution.EvidenceID] = struct{}{}
		if index > 0 {
			previous := calculation.Contributions[index-1]
			if contribution.OccurredAt.Before(previous.OccurredAt) ||
				(contribution.OccurredAt == previous.OccurredAt && contribution.EvidenceID.String() <= previous.EvidenceID.String()) {
				return fmt.Errorf("mastery contributions are not canonically ordered")
			}
		}
		effectiveWeightSum += contribution.EffectiveWeight
		normalizedWeightSum += contribution.NormalizedWeight
		weightedScoreSum += contribution.WeightedScore
	}
	if math.Abs(effectiveWeightSum-calculation.TotalWeight) > 1e-12 || math.Abs(normalizedWeightSum-1) > 1e-12 ||
		math.Abs(weightedScoreSum/calculation.TotalWeight-calculation.Score.Value()) > 1e-12 {
		return fmt.Errorf("mastery calculation aggregates are inconsistent")
	}
	return nil
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
		return calculation, calculation.Validate()
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
	return calculation, calculation.Validate()
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
