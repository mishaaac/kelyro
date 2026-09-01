package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/diversity"
)

const LiveMultiSourceVerificationV1 = "live-multi-source-verification-v1"

type LiveMultiSourceVerificationRequest struct {
	Claims []research.Claim
}

type LiveMultiSourceVerificationResult struct {
	Verifications    []research.VerificationResult
	Diversity        []LiveClaimDiversityAssessment
	AlgorithmVersion string
}

type LiveClaimDiversityAssessment struct {
	ClaimID    research.ClaimID
	Assessment diversity.Assessment
}

func (item LiveClaimDiversityAssessment) Validate() error {
	if err := item.ClaimID.Validate(); err != nil {
		return err
	}
	return item.Assessment.Validate()
}

func (result LiveMultiSourceVerificationResult) Validate() error {
	if len(result.Verifications) == 0 || len(result.Verifications) > MaximumClaimCandidatesPerRun {
		return fmt.Errorf("live verifications must contain between 1 and %d entries", MaximumClaimCandidatesPerRun)
	}
	seen := make(map[research.ClaimID]struct{}, len(result.Verifications))
	for index, verification := range result.Verifications {
		if err := verification.Validate(); err != nil {
			return fmt.Errorf("live verification %d: %w", index, err)
		}
		if verification.AlgorithmVersion != research.MultiSourceVerificationAlgorithmV1 {
			return fmt.Errorf("live verification %d does not use %q", index, research.MultiSourceVerificationAlgorithmV1)
		}
		if _, duplicate := seen[verification.ClaimID]; duplicate {
			return fmt.Errorf("live verification repeats Claim %q", verification.ClaimID)
		}
		seen[verification.ClaimID] = struct{}{}
	}
	if len(result.Diversity) != len(result.Verifications) {
		return fmt.Errorf("live diversity assessments must cover every verification")
	}
	for index, item := range result.Diversity {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("live diversity assessment %d: %w", index, err)
		}
		if _, exists := seen[item.ClaimID]; !exists {
			return fmt.Errorf("live diversity assessment references unknown Claim %q", item.ClaimID)
		}
		if index > 0 && item.ClaimID.String() <= result.Diversity[index-1].ClaimID.String() {
			return fmt.Errorf("live diversity assessments must be unique and ordered")
		}
	}
	if result.AlgorithmVersion != LiveMultiSourceVerificationV1 {
		return fmt.Errorf("live verification algorithm must be %q", LiveMultiSourceVerificationV1)
	}
	return nil
}

type liveMultiSourceVerificationService struct {
	verification VerificationService
	diversity    SourceDiversityService
}

func NewLiveMultiSourceVerificationService(verification VerificationService, diversityService SourceDiversityService) (LiveMultiSourceVerificationService, error) {
	const operation = "configure live multi-source verification"
	if err := requireDependency(operation, "verification service", verification); err != nil {
		return nil, err
	}
	if err := requireDependency(operation, "source diversity service", diversityService); err != nil {
		return nil, err
	}
	return &liveMultiSourceVerificationService{verification: verification, diversity: diversityService}, nil
}

func (service *liveMultiSourceVerificationService) VerifyClaims(ctx context.Context, request LiveMultiSourceVerificationRequest) (LiveMultiSourceVerificationResult, error) {
	const operation = "verify live research claims"
	if ctx == nil {
		return LiveMultiSourceVerificationResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveMultiSourceVerificationResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if len(request.Claims) == 0 || len(request.Claims) > MaximumClaimCandidatesPerRun {
		return LiveMultiSourceVerificationResult{}, invalid(operation,
			fmt.Errorf("live Claims must contain between 1 and %d entries", MaximumClaimCandidatesPerRun))
	}
	claims := append([]research.Claim(nil), request.Claims...)
	seen := make(map[research.ClaimID]struct{}, len(claims))
	for index, claim := range claims {
		if err := claim.Validate(); err != nil {
			return LiveMultiSourceVerificationResult{}, invalid(operation, fmt.Errorf("live Claim %d: %w", index, err))
		}
		if _, duplicate := seen[claim.ID]; duplicate {
			return LiveMultiSourceVerificationResult{}, invalid(operation, fmt.Errorf("live verification repeats Claim %q", claim.ID))
		}
		seen[claim.ID] = struct{}{}
	}
	sort.Slice(claims, func(i, j int) bool { return claims[i].ID.String() < claims[j].ID.String() })
	result := LiveMultiSourceVerificationResult{AlgorithmVersion: LiveMultiSourceVerificationV1}
	for _, claim := range claims {
		if err := ctx.Err(); err != nil {
			return cloneLiveMultiSourceVerificationResult(result), Classify(ErrorUnavailable, operation, err)
		}
		verification, err := service.verification.Verify(ctx, claim.ID)
		if err != nil {
			return cloneLiveMultiSourceVerificationResult(result), boundaryError(ErrorPersistenceFailure, operation, err)
		}
		if verification.ClaimID != claim.ID {
			return cloneLiveMultiSourceVerificationResult(result), invalid(operation,
				fmt.Errorf("verification returned Claim %q for %q", verification.ClaimID, claim.ID))
		}
		result.Verifications = append(result.Verifications, verification)
		annotations := make([]SourceDiversityAnnotation, len(claim.SourceIDs))
		for index, sourceID := range claim.SourceIDs {
			annotations[index] = SourceDiversityAnnotation{
				SourceID: sourceID, Perspective: diversity.PerspectiveUnknown, TechnicalRole: diversity.TechnicalRoleUnknown,
			}
		}
		assessment, assessErr := service.diversity.Assess(ctx, AssessSourceDiversityRequest{ClaimID: claim.ID, Annotations: annotations})
		if assessErr != nil {
			return cloneLiveMultiSourceVerificationResult(result), boundaryError(ErrorPersistenceFailure, operation, assessErr)
		}
		result.Diversity = append(result.Diversity, LiveClaimDiversityAssessment{ClaimID: claim.ID, Assessment: assessment})
	}
	if err := result.Validate(); err != nil {
		return LiveMultiSourceVerificationResult{}, invalid(operation, err)
	}
	return cloneLiveMultiSourceVerificationResult(result), nil
}

func (service *liveMultiSourceVerificationService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	result, err := service.VerifyClaims(ctx, LiveMultiSourceVerificationRequest{Claims: input.Artifacts.Claims})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	artifacts.Verifications = make([]research.VerificationResult, len(result.Verifications))
	for index, verification := range result.Verifications {
		artifacts.Verifications[index] = cloneVerificationResult(verification)
	}
	artifacts.DiversityAssessments = make([]LiveClaimDiversityAssessment, len(result.Diversity))
	for index, assessment := range result.Diversity {
		artifacts.DiversityAssessments[index] = cloneLiveClaimDiversityAssessment(assessment)
	}
	return artifacts, err
}

func cloneLiveMultiSourceVerificationResult(result LiveMultiSourceVerificationResult) LiveMultiSourceVerificationResult {
	clone := result
	clone.Verifications = make([]research.VerificationResult, len(result.Verifications))
	for index, verification := range result.Verifications {
		clone.Verifications[index] = cloneVerificationResult(verification)
	}
	clone.Diversity = make([]LiveClaimDiversityAssessment, len(result.Diversity))
	for index, assessment := range result.Diversity {
		clone.Diversity[index] = cloneLiveClaimDiversityAssessment(assessment)
	}
	return clone
}

func cloneLiveClaimDiversityAssessment(item LiveClaimDiversityAssessment) LiveClaimDiversityAssessment {
	clone := item
	clone.Assessment.Warnings = make([]diversity.Warning, len(item.Assessment.Warnings))
	for index, warning := range item.Assessment.Warnings {
		clone.Assessment.Warnings[index] = warning
		clone.Assessment.Warnings[index].SourceIDs = append([]research.SourceID(nil), warning.SourceIDs...)
	}
	clone.Assessment.DeferredDimensions = append([]diversity.DeferredDimension(nil), item.Assessment.DeferredDimensions...)
	return clone
}

func cloneVerificationResult(result research.VerificationResult) research.VerificationResult {
	clone := result
	clone.SourceIDs = append([]research.SourceID(nil), result.SourceIDs...)
	clone.ReasonCodes = append([]research.ClaimVerificationReason(nil), result.ReasonCodes...)
	return clone
}

var (
	_ LiveMultiSourceVerificationService = (*liveMultiSourceVerificationService)(nil)
	_ LiveResearchStageService           = (*liveMultiSourceVerificationService)(nil)
)
