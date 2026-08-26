package application

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type deprecationIntelligenceService struct {
	repository DeprecationRepository
	claims     ClaimRepository
	evidence   EvidenceRepository
	clock      Clock
}

func NewDeprecationIntelligenceService(
	repository DeprecationRepository,
	claims ClaimRepository,
	evidence EvidenceRepository,
	clock Clock,
) DeprecationIntelligenceService {
	return &deprecationIntelligenceService{
		repository: repository,
		claims:     claims,
		evidence:   evidence,
		clock:      clock,
	}
}

func (service *deprecationIntelligenceService) Assess(
	ctx context.Context,
	request DeprecationAssessmentRequest,
) (DeprecationAssessmentResult, error) {
	const operation = "assess deprecation intelligence"
	if err := request.Validate(); err != nil {
		return DeprecationAssessmentResult{}, invalid(operation, err)
	}
	for name, dependency := range map[string]any{
		"deprecation repository": service.repository,
		"claim repository":       service.claims,
		"evidence repository":    service.evidence,
		"clock":                  service.clock,
	} {
		if err := requireDependency(operation, name, dependency); err != nil {
			return DeprecationAssessmentResult{}, err
		}
	}

	verifiedAt := service.clock.Now()
	if err := verifiedAt.Validate(); err != nil {
		return DeprecationAssessmentResult{}, invalid(operation, fmt.Errorf("clock: %w", err))
	}

	sourceIDs := make([]research.SourceID, 0, len(request.Signals))
	evidenceIDs := make([]research.ID, 0, len(request.Signals))
	for index, signal := range request.Signals {
		claim, err := service.claims.Get(ctx, signal.ClaimID)
		if err != nil {
			return DeprecationAssessmentResult{}, repositoryError(operation, err)
		}
		if claim.Type != research.ClaimDeprecation {
			return DeprecationAssessmentResult{}, invalid(operation,
				fmt.Errorf("signal %d claim %q is not a deprecation claim", index, claim.ID.String()))
		}
		if !containsSourceID(claim.SourceIDs, signal.SourceID) {
			return DeprecationAssessmentResult{}, invalid(operation,
				fmt.Errorf("signal %d source is not declared by claim", index))
		}
		if !containsResearchID(claim.EvidenceIDs, signal.EvidenceID) {
			return DeprecationAssessmentResult{}, invalid(operation,
				fmt.Errorf("signal %d evidence is not declared by claim", index))
		}
		item, err := service.evidence.Get(ctx, signal.EvidenceID)
		if err != nil {
			return DeprecationAssessmentResult{}, repositoryError(operation, err)
		}
		if item.SourceID != signal.SourceID {
			return DeprecationAssessmentResult{}, invalid(operation,
				fmt.Errorf("signal %d evidence source does not match", index))
		}
		if claim.CreatedAt.After(verifiedAt) || item.ExtractedAt.After(verifiedAt) {
			return DeprecationAssessmentResult{}, invalid(operation,
				fmt.Errorf("signal %d was recorded after the assessment time", index))
		}
		if signal.Kind == DeprecationSignalStrongInference &&
			claim.Confidence.Value() < MinimumDeprecationStrongInferenceConfidence {
			return DeprecationAssessmentResult{}, invalid(operation,
				fmt.Errorf("signal %d confidence %.3f is below strong inference minimum %.3f",
					index, claim.Confidence.Value(), MinimumDeprecationStrongInferenceConfidence))
		}
		sourceIDs = appendUniqueSourceID(sourceIDs, signal.SourceID)
		evidenceIDs = appendUniqueResearchID(evidenceIDs, signal.EvidenceID)
	}

	determination := research.DeprecationExplicitEvidence
	if request.Signals[0].Kind == DeprecationSignalStrongInference {
		determination = research.DeprecationMultiSourceStrongInference
		if len(sourceIDs) < 2 {
			return DeprecationAssessmentResult{}, invalid(operation,
				fmt.Errorf("strong inference requires evidence from at least 2 distinct sources"))
		}
	}
	sort.Slice(sourceIDs, func(i, j int) bool { return sourceIDs[i].String() < sourceIDs[j].String() })
	sort.Slice(evidenceIDs, func(i, j int) bool { return evidenceIDs[i].String() < evidenceIDs[j].String() })
	conclusion := request.Signals[0]

	record := research.DeprecationRecord{
		Subject:          request.Subject,
		Status:           conclusion.Status,
		Determination:    determination,
		IntroducedIn:     cloneSourceVersion(conclusion.IntroducedIn),
		DeprecatedIn:     cloneSourceVersion(conclusion.DeprecatedIn),
		RemovedIn:        cloneSourceVersion(conclusion.RemovedIn),
		Replacement:      conclusion.Replacement,
		SourceIDs:        sourceIDs,
		EvidenceIDs:      evidenceIDs,
		VerifiedAt:       verifiedAt,
		AlgorithmVersion: research.DeprecationIntelligenceAlgorithmV1,
	}
	record.ID = stableResearchID("deprecation", deprecationIdentityParts(record)...)
	if err := record.Validate(); err != nil {
		return DeprecationAssessmentResult{}, invalid(operation, err)
	}
	if err := service.repository.Append(ctx, record); err != nil {
		return DeprecationAssessmentResult{}, repositoryError(operation, err)
	}
	return DeprecationAssessmentResult{Record: record}, nil
}

func (service *deprecationIntelligenceService) Get(ctx context.Context, id research.ID) (research.DeprecationRecord, error) {
	const operation = "get deprecation intelligence"
	if err := id.Validate(); err != nil {
		return research.DeprecationRecord{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "deprecation repository", service.repository); err != nil {
		return research.DeprecationRecord{}, err
	}
	record, err := service.repository.Get(ctx, id)
	return record, repositoryError(operation, err)
}

func (service *deprecationIntelligenceService) History(ctx context.Context, subject string) ([]research.DeprecationRecord, error) {
	const operation = "list deprecation history"
	if err := requireText("deprecation subject", subject); err != nil {
		return nil, invalid(operation, err)
	}
	if err := requireDependency(operation, "deprecation repository", service.repository); err != nil {
		return nil, err
	}
	records, err := service.repository.ListBySubject(ctx, subject)
	return records, repositoryError(operation, err)
}

func deprecationIdentityParts(record research.DeprecationRecord) []string {
	parts := []string{
		record.Subject,
		string(record.Status),
		string(record.Determination),
		optionalSourceVersionString(record.IntroducedIn),
		optionalSourceVersionString(record.DeprecatedIn),
		optionalSourceVersionString(record.RemovedIn),
		record.Replacement,
		record.VerifiedAt.Time().Format(time.RFC3339Nano),
	}
	for _, id := range record.SourceIDs {
		parts = append(parts, id.String())
	}
	for _, id := range record.EvidenceIDs {
		parts = append(parts, id.String())
	}
	return parts
}

func optionalSourceVersionString(version *research.SourceVersion) string {
	if version == nil {
		return ""
	}
	return version.String()
}

func cloneSourceVersion(version *research.SourceVersion) *research.SourceVersion {
	if version == nil {
		return nil
	}
	clone := *version
	return &clone
}

func containsSourceID(values []research.SourceID, value research.SourceID) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func containsResearchID(values []research.ID, value research.ID) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func appendUniqueResearchID(values []research.ID, value research.ID) []research.ID {
	if containsResearchID(values, value) {
		return values
	}
	return append(values, value)
}

var _ DeprecationIntelligenceService = (*deprecationIntelligenceService)(nil)
