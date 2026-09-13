package curriculum

import "fmt"

const GuidanceClassifierVersionV1 = "guidance-classifier-v1"

type GuidanceType string

const (
	GuidanceRecommendedCurrent GuidanceType = "recommended_current"
	GuidanceAcceptableCurrent  GuidanceType = "acceptable_current"
	GuidanceLegacyMaintenance  GuidanceType = "legacy_maintenance"
	GuidanceHistoricalContext  GuidanceType = "historical_context"
	GuidanceAvoid              GuidanceType = "avoid"
	GuidanceExperimental       GuidanceType = "experimental"
)

func (guidance GuidanceType) Validate() error {
	switch guidance {
	case GuidanceRecommendedCurrent, GuidanceAcceptableCurrent,
		GuidanceLegacyMaintenance, GuidanceHistoricalContext,
		GuidanceAvoid, GuidanceExperimental:
		return nil
	default:
		return fmt.Errorf("invalid guidance type %q", guidance)
	}
}

type GuidanceClassification struct {
	TargetKind   TemporalTargetKind
	TargetID     ID
	Status       ConceptStatus
	Guidance     GuidanceType
	EvidenceRefs []EvidenceRef
	Reason       string
}

func (classification GuidanceClassification) Validate() error {
	if err := classification.TargetKind.Validate(); err != nil {
		return err
	}
	if err := classification.TargetID.Validate(); err != nil {
		return fmt.Errorf("guidance target: %w", err)
	}
	if err := classification.Status.Validate(); err != nil {
		return err
	}
	if err := classification.Guidance.Validate(); err != nil {
		return err
	}
	if err := validateEvidenceRefs("guidance evidence", classification.EvidenceRefs); err != nil {
		return err
	}
	if len(classification.EvidenceRefs) == 0 {
		return fmt.Errorf("guidance classification %q has no evidence", classification.TargetID)
	}
	if err := requireText("guidance reason", classification.Reason); err != nil {
		return err
	}
	switch classification.Guidance {
	case GuidanceRecommendedCurrent, GuidanceAcceptableCurrent:
		if classification.Status != ConceptCurrent {
			return fmt.Errorf("current guidance %q has non-current status", classification.TargetID)
		}
	case GuidanceLegacyMaintenance:
		if classification.Status != ConceptLegacy {
			return fmt.Errorf("legacy guidance %q has incompatible status", classification.TargetID)
		}
	case GuidanceHistoricalContext:
		if classification.Status != ConceptHistorical {
			return fmt.Errorf("historical guidance %q has incompatible status", classification.TargetID)
		}
	case GuidanceAvoid:
		if classification.Status != ConceptDeprecated {
			return fmt.Errorf("avoid guidance %q has incompatible status", classification.TargetID)
		}
	case GuidanceExperimental:
		if classification.Status != ConceptExperimental && classification.Status != ConceptPreview {
			return fmt.Errorf("experimental guidance %q has incompatible status", classification.TargetID)
		}
	}
	return nil
}

type GuidanceClassificationResult struct {
	Classifications         []GuidanceClassification
	CurrentGuidanceFindings []CurrentGuidanceFinding
	AlgorithmVersion        string
}

func (result GuidanceClassificationResult) Validate() error {
	if len(result.Classifications) == 0 {
		return fmt.Errorf("guidance classification result is empty")
	}
	targets := make(map[string]struct{}, len(result.Classifications))
	for _, classification := range result.Classifications {
		if err := classification.Validate(); err != nil {
			return err
		}
		key := string(classification.TargetKind) + "\x00" + classification.TargetID.String()
		if _, exists := targets[key]; exists {
			return fmt.Errorf("duplicate guidance classification for %s %q", classification.TargetKind, classification.TargetID)
		}
		targets[key] = struct{}{}
	}
	seenFindings := make(map[ID]struct{}, len(result.CurrentGuidanceFindings))
	for _, finding := range result.CurrentGuidanceFindings {
		if err := finding.Validate(); err != nil {
			return err
		}
		if _, exists := seenFindings[finding.TargetID]; exists {
			return fmt.Errorf("duplicate current-guidance finding for %q", finding.TargetID)
		}
		seenFindings[finding.TargetID] = struct{}{}
	}
	if result.AlgorithmVersion != GuidanceClassifierVersionV1 {
		return fmt.Errorf("unsupported guidance classifier version %q", result.AlgorithmVersion)
	}
	return nil
}
