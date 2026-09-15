package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

// PackUpgradePlannerV1 discovers only immutable, already-installed versions.
// It performs validation through the pack repository read path and produces a
// dry-run plan without writing activation or learner state.
type PackUpgradePlannerV1 struct {
	packs      PackInstallService
	classifier CurriculumChangeClassificationService
	planner    CurriculumMigrationPlanningService
	versioning PackVersioningService
}

func NewPackUpgradePlannerV1(packs PackInstallService, classifier CurriculumChangeClassificationService,
	planner CurriculumMigrationPlanningService, versioning PackVersioningService) *PackUpgradePlannerV1 {
	return &PackUpgradePlannerV1{packs: packs, classifier: classifier, planner: planner, versioning: versioning}
}

func (service *PackUpgradePlannerV1) Upgrade(ctx context.Context, request PackUpgradeRequest) (PackUpgradeResult, error) {
	const operation = "plan Learning Pack upgrade"
	if service == nil || service.packs == nil || service.classifier == nil || service.planner == nil || service.versioning == nil {
		return PackUpgradeResult{}, Classify(ErrorUnavailable, operation, fmt.Errorf("pack upgrade planner is not configured"))
	}
	if strings.TrimSpace(request.WorkspaceRoot) == "" {
		return PackUpgradeResult{}, Invalid(operation, fmt.Errorf("workspace root is empty"))
	}
	if err := request.PackID.Validate(); err != nil {
		return PackUpgradeResult{}, Invalid(operation, err)
	}
	if !request.DryRun {
		return PackUpgradeResult{}, Invalid(operation, fmt.Errorf("applying pack upgrades is not configured; use dry-run"))
	}
	current, err := service.packs.Active(ctx, request.WorkspaceRoot)
	if err != nil {
		return PackUpgradeResult{}, RepositoryError(operation, err)
	}
	if current.Pack.Manifest.ID != request.PackID {
		return PackUpgradeResult{}, Invalid(operation, fmt.Errorf("active pack is %s, not requested pack %s", current.Pack.Manifest.ID, request.PackID))
	}
	installed, err := service.packs.Find(ctx, request.PackID)
	if err != nil {
		return PackUpgradeResult{}, RepositoryError(operation, err)
	}
	candidate, err := selectUpgradeCandidate(current, installed, request.TargetVersion)
	if err != nil {
		return PackUpgradeResult{}, Invalid(operation, err)
	}
	classification, err := service.classifier.Classify(ctx, CurriculumChangeClassificationRequest{
		Old: current.Pack.Curriculum, New: candidate.Pack.Curriculum,
		OldEnvironment: current.Pack.Environment, NewEnvironment: candidate.Pack.Environment,
		IdentityMappings: request.IdentityMappings,
	})
	if err != nil {
		return PackUpgradeResult{}, ExternalError(operation, err)
	}
	decision, err := service.versioning.Classify(ctx, PackVersioningRequest{
		CurrentVersion: current.Pack.Manifest.Version, CandidateVersion: candidate.Pack.Manifest.Version,
		Changes: classification.Changes,
	})
	if err != nil {
		return PackUpgradeResult{}, ExternalError(operation, err)
	}
	if !decision.Allowed {
		return PackUpgradeResult{}, Invalid(operation, fmt.Errorf("candidate version is incompatible with classified changes: %s", strings.Join(decision.Reasons, "; ")))
	}
	plan, err := service.planner.Plan(ctx, CurriculumMigrationPlanningRequest{
		Old: current.Pack.Curriculum, New: candidate.Pack.Curriculum,
		Classification: classification, IdentityMappings: request.IdentityMappings,
	})
	if err != nil {
		return PackUpgradeResult{}, ExternalError(operation, err)
	}
	return PackUpgradeResult{
		Current: current.Pack, Candidate: candidate.Pack, Changes: classification.Changes,
		MigrationPlan: plan, VersioningDecision: decision,
	}, nil
}

func selectUpgradeCandidate(current InstalledPack, installed []InstalledPack, target curriculum.PackVersion) (InstalledPack, error) {
	if target.String() != "" {
		if err := target.Validate(); err != nil {
			return InstalledPack{}, err
		}
		for _, candidate := range installed {
			if candidate.Pack.Manifest.Version == target {
				if compareSemver(parseSemver(candidate.Pack.Manifest.Version.String()), parseSemver(current.Pack.Manifest.Version.String())) <= 0 {
					return InstalledPack{}, fmt.Errorf("target version must be newer than active version")
				}
				return candidate, nil
			}
		}
		return InstalledPack{}, fmt.Errorf("target pack version %s is not installed", target.String())
	}
	var selected *InstalledPack
	for index := range installed {
		candidate := installed[index]
		if candidate.Pack.Manifest.ID != current.Pack.Manifest.ID || compareSemver(parseSemver(candidate.Pack.Manifest.Version.String()), parseSemver(current.Pack.Manifest.Version.String())) <= 0 {
			continue
		}
		if selected == nil || compareSemver(parseSemver(candidate.Pack.Manifest.Version.String()), parseSemver(selected.Pack.Manifest.Version.String())) > 0 {
			copy := candidate
			selected = &copy
		}
	}
	if selected == nil {
		return InstalledPack{}, fmt.Errorf("no newer installed version is available for %s", current.Pack.Manifest.ID)
	}
	return *selected, nil
}

var _ PackUpgradeService = (*PackUpgradePlannerV1)(nil)
