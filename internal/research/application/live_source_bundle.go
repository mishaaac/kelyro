package application

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/mishaaac/kelyro/internal/research"
)

const (
	LiveSourceBundleV1         = "live-source-bundle-v1"
	LiveBundleClaimSelectionV1 = "live-bundle-claim-selection-v1"
)

type LiveSourceBundleRequest struct {
	RunID  research.ID
	Claims []research.Claim
}

type LiveSourceBundleResult struct {
	Bundle           research.SourceBundle
	AlgorithmVersion string
}

func (result LiveSourceBundleResult) Validate() error {
	if err := result.Bundle.Validate(); err != nil {
		return err
	}
	if result.Bundle.AlgorithmVersion != research.SourceBundleAlgorithmV1 {
		return fmt.Errorf("live bundle does not use %q", research.SourceBundleAlgorithmV1)
	}
	if result.AlgorithmVersion != LiveSourceBundleV1 {
		return fmt.Errorf("live bundle algorithm must be %q", LiveSourceBundleV1)
	}
	return nil
}

type liveSourceBundleService struct {
	bundles SourceBundleService
}

func NewLiveSourceBundleService(bundles SourceBundleService) (LiveSourceBundleService, error) {
	const operation = "configure live source bundle"
	if err := requireDependency(operation, "source bundle service", bundles); err != nil {
		return nil, err
	}
	return &liveSourceBundleService{bundles: bundles}, nil
}

func (service *liveSourceBundleService) Assemble(ctx context.Context, request LiveSourceBundleRequest) (LiveSourceBundleResult, error) {
	const operation = "assemble live research source bundle"
	if ctx == nil {
		return LiveSourceBundleResult{}, invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return LiveSourceBundleResult{}, Classify(ErrorUnavailable, operation, err)
	}
	if err := request.RunID.Validate(); err != nil {
		return LiveSourceBundleResult{}, invalid(operation, err)
	}
	if len(request.Claims) == 0 || len(request.Claims) > research.MaximumSourceBundleItems {
		return LiveSourceBundleResult{}, invalid(operation,
			fmt.Errorf("live bundle Claims must contain between 1 and %d entries", research.MaximumSourceBundleItems))
	}
	claimIDs := make([]research.ClaimID, len(request.Claims))
	seen := make(map[research.ClaimID]struct{}, len(request.Claims))
	for index, claim := range request.Claims {
		if err := claim.Validate(); err != nil {
			return LiveSourceBundleResult{}, invalid(operation, fmt.Errorf("live bundle Claim %d: %w", index, err))
		}
		if _, duplicate := seen[claim.ID]; duplicate {
			return LiveSourceBundleResult{}, invalid(operation, fmt.Errorf("live bundle repeats Claim %q", claim.ID))
		}
		seen[claim.ID] = struct{}{}
		claimIDs[index] = claim.ID
	}
	sort.Slice(claimIDs, func(i, j int) bool { return claimIDs[i].String() < claimIDs[j].String() })
	bundle, err := service.bundles.Assemble(ctx, AssembleSourceBundleRequest{RunID: request.RunID, ClaimIDs: claimIDs})
	if err != nil {
		return LiveSourceBundleResult{}, boundaryError(ErrorPersistenceFailure, operation, err)
	}
	if bundle.RunID != request.RunID || !sameLiveBundleClaimIDs(bundle.ClaimIDs, claimIDs) {
		return LiveSourceBundleResult{}, invalid(operation, errors.New("assembled bundle does not match the live run and Claims"))
	}
	result := LiveSourceBundleResult{Bundle: cloneSourceBundleArtifact(bundle), AlgorithmVersion: LiveSourceBundleV1}
	if err := result.Validate(); err != nil {
		return LiveSourceBundleResult{}, invalid(operation, err)
	}
	return result, nil
}

func (service *liveSourceBundleService) Execute(ctx context.Context, input LiveResearchStageInput) (LiveResearchArtifacts, error) {
	claims, selectionErr := selectLiveBundleClaims(input.Artifacts.Claims, input.Artifacts.Verifications)
	if selectionErr != nil {
		return cloneLiveResearchArtifacts(input.Artifacts), invalid("select verified live bundle Claims", selectionErr)
	}
	result, err := service.Assemble(ctx, LiveSourceBundleRequest{RunID: input.Run.ID, Claims: claims})
	artifacts := cloneLiveResearchArtifacts(input.Artifacts)
	if result.Bundle.ID.Validate() == nil {
		bundle := cloneSourceBundleArtifact(result.Bundle)
		artifacts.Bundle = &bundle
	}
	return artifacts, err
}

func selectLiveBundleClaims(claims []research.Claim, verifications []research.VerificationResult) ([]research.Claim, error) {
	byClaim := make(map[research.ClaimID]research.VerificationResult, len(verifications))
	for _, verification := range verifications {
		if err := verification.Validate(); err != nil {
			return nil, err
		}
		if _, duplicate := byClaim[verification.ClaimID]; duplicate {
			return nil, fmt.Errorf("duplicate verification for Claim %q", verification.ClaimID)
		}
		byClaim[verification.ClaimID] = verification
	}
	selected := make([]research.Claim, 0, len(claims))
	for _, claim := range claims {
		verification, exists := byClaim[claim.ID]
		if !exists {
			return nil, fmt.Errorf("Claim %q has no verification", claim.ID)
		}
		switch verification.Status {
		case research.VerificationVerified, research.VerificationVerifiedCaveat, research.VerificationConflicted:
			selected = append(selected, claim)
		case research.VerificationInsufficient, research.VerificationRejected:
		}
	}
	return selected, nil
}

func sameLiveBundleClaimIDs(left, right []research.ClaimID) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func cloneSourceBundleArtifact(bundle research.SourceBundle) research.SourceBundle {
	clone := bundle
	clone.TargetVersion = cloneSourceVersion(bundle.TargetVersion)
	clone.ClaimIDs = append([]research.ClaimID(nil), bundle.ClaimIDs...)
	clone.Sources = make([]research.SourceBundleSource, len(bundle.Sources))
	for index, source := range bundle.Sources {
		clone.Sources[index] = source
		clone.Sources[index].VersionScope = cloneSourceVersion(source.VersionScope)
	}
	clone.ConflictIDs = append([]research.ID(nil), bundle.ConflictIDs...)
	if bundle.Freshness.LastVerifiedAt != nil {
		lastVerified := *bundle.Freshness.LastVerifiedAt
		clone.Freshness.LastVerifiedAt = &lastVerified
	}
	clone.Freshness.MissingClaimIDs = append([]research.ClaimID(nil), bundle.Freshness.MissingClaimIDs...)
	clone.Freshness.SourceAlgorithms = append([]string(nil), bundle.Freshness.SourceAlgorithms...)
	clone.Issues = append([]research.SourceBundleIssue(nil), bundle.Issues...)
	return clone
}

var (
	_ LiveSourceBundleService  = (*liveSourceBundleService)(nil)
	_ LiveResearchStageService = (*liveSourceBundleService)(nil)
)
