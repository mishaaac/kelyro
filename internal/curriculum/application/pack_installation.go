package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

const PackInstallationPolicyVersionV1 = "pack-installation-policy-v1"

var canonicalPackHash = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func canonicalSHA256(value string) bool { return canonicalPackHash.MatchString(value) }

type SystemClock struct{}

func (SystemClock) Now() curriculum.Timestamp {
	timestamp, _ := curriculum.NewTimestamp(time.Now())
	return timestamp
}

// PackInstallerV1 validates before every install, requires the complete local
// dependency graph, and stores only immutable validated artifacts. Activation
// changes one workspace reference; it never copies a pack into the workspace.
type PackInstallerV1 struct {
	validator  PackValidationService
	resolver   PackDependencyResolverService
	repository PackInstallationRepository
	clock      Clock
}

func NewPackInstallerV1(validator PackValidationService, resolver PackDependencyResolverService, repository PackInstallationRepository, clock Clock) *PackInstallerV1 {
	return &PackInstallerV1{validator: validator, resolver: resolver, repository: repository, clock: clock}
}

func (service *PackInstallerV1) Install(ctx context.Context, request PackInstallRequest) (PackInstallResult, error) {
	const operation = "install Learning Pack"
	if err := service.requireDependencies(operation); err != nil {
		return PackInstallResult{}, err
	}
	if strings.TrimSpace(request.Source.Path) == "" {
		return PackInstallResult{}, Invalid(operation, fmt.Errorf("pack source path is empty"))
	}
	validation, err := service.validator.Validate(ctx, request.Source)
	if err != nil {
		return PackInstallResult{}, ExternalError(operation, err)
	}
	if err := validatedArtifactError(validation); err != nil {
		return PackInstallResult{}, Invalid(operation, err)
	}
	pack := *validation.Pack
	if existing, getErr := service.repository.Get(ctx, pack.Manifest.ID, pack.Manifest.Version); getErr == nil {
		if existing.ContentHash != validation.ContentHash {
			return PackInstallResult{}, Classify(ErrorConflict, operation, fmt.Errorf("%s@%s is already installed with different content", pack.Manifest.ID, pack.Manifest.Version.String()))
		}
		return PackInstallResult{Pack: existing.Pack, ContentHash: existing.ContentHash, Installed: false}, nil
	} else if !errors.Is(getErr, ErrNotFound) {
		return PackInstallResult{}, RepositoryError(operation, getErr)
	}

	installed, err := service.repository.List(ctx)
	if err != nil {
		return PackInstallResult{}, RepositoryError(operation, err)
	}
	if err := service.requireResolvedDependencies(ctx, operation, pack.Manifest, installed); err != nil {
		return PackInstallResult{}, err
	}
	artifact := PackInstallationArtifact{
		InstalledPack:   InstalledPack{Pack: pack, ContentHash: validation.ContentHash, InstalledAt: service.clock.Now()},
		PortableArchive: append([]byte(nil), validation.PortableArchive...),
	}
	if err := artifact.Validate(); err != nil {
		return PackInstallResult{}, Invalid(operation, err)
	}
	if err := service.repository.Add(ctx, artifact); err != nil {
		return PackInstallResult{}, RepositoryError(operation, err)
	}
	return PackInstallResult{Pack: pack, ContentHash: validation.ContentHash, Installed: true}, nil
}

func (service *PackInstallerV1) Activate(ctx context.Context, request PackActivateRequest) (PackActivation, error) {
	const operation = "activate Learning Pack"
	if err := service.requireDependencies(operation); err != nil {
		return PackActivation{}, err
	}
	if strings.TrimSpace(request.WorkspaceRoot) == "" {
		return PackActivation{}, Invalid(operation, fmt.Errorf("workspace root is empty"))
	}
	if err := request.PackID.Validate(); err != nil {
		return PackActivation{}, Invalid(operation, err)
	}
	if err := request.Version.Validate(); err != nil {
		return PackActivation{}, Invalid(operation, err)
	}
	installed, err := service.repository.Get(ctx, request.PackID, request.Version)
	if err != nil {
		return PackActivation{}, RepositoryError(operation, err)
	}
	available, err := service.repository.List(ctx)
	if err != nil {
		return PackActivation{}, RepositoryError(operation, err)
	}
	if err := service.requireResolvedDependencies(ctx, operation, installed.Pack.Manifest, available); err != nil {
		return PackActivation{}, err
	}
	activation := PackActivation{PackID: request.PackID, Version: request.Version, ActivatedAt: service.clock.Now()}
	if err := activation.Validate(); err != nil {
		return PackActivation{}, Invalid(operation, err)
	}
	if err := service.repository.Activate(ctx, request.WorkspaceRoot, activation); err != nil {
		return PackActivation{}, RepositoryError(operation, err)
	}
	return activation, nil
}

func (service *PackInstallerV1) List(ctx context.Context) ([]InstalledPack, error) {
	const operation = "list installed Learning Packs"
	if service == nil || service.repository == nil {
		return nil, Classify(ErrorUnavailable, operation, fmt.Errorf("pack installation repository is not configured"))
	}
	values, err := service.repository.List(ctx)
	if err != nil {
		return nil, RepositoryError(operation, err)
	}
	return sortedInstalledPacks(values), nil
}

func (service *PackInstallerV1) Find(ctx context.Context, id curriculum.ID) ([]InstalledPack, error) {
	const operation = "show installed Learning Pack"
	if err := id.Validate(); err != nil {
		return nil, Invalid(operation, err)
	}
	values, err := service.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]InstalledPack, 0)
	for _, value := range values {
		if value.Pack.Manifest.ID == id {
			result = append(result, value)
		}
	}
	if len(result) == 0 {
		return nil, Classify(ErrorNotFound, operation, fmt.Errorf("pack %q is not installed", id))
	}
	return result, nil
}

func (service *PackInstallerV1) Active(ctx context.Context, workspaceRoot string) (InstalledPack, error) {
	const operation = "get active Learning Pack"
	if service == nil || service.repository == nil {
		return InstalledPack{}, Classify(ErrorUnavailable, operation, fmt.Errorf("pack installation repository is not configured"))
	}
	value, err := service.repository.Active(ctx, workspaceRoot)
	if err != nil {
		return InstalledPack{}, RepositoryError(operation, err)
	}
	return value, nil
}

func (service *PackInstallerV1) requireDependencies(operation string) error {
	if service == nil {
		return Classify(ErrorUnavailable, operation, fmt.Errorf("pack installer is not configured"))
	}
	for name, dependency := range map[string]any{
		"pack validator":               service.validator,
		"pack dependency resolver":     service.resolver,
		"pack installation repository": service.repository,
		"clock":                        service.clock,
	} {
		if dependency == nil {
			return Classify(ErrorUnavailable, operation, fmt.Errorf("%s is not configured", name))
		}
	}
	return nil
}

func (service *PackInstallerV1) requireResolvedDependencies(ctx context.Context, operation string, root curriculum.PackManifest, installed []InstalledPack) error {
	available := make([]curriculum.PackManifest, 0, len(installed))
	for _, value := range installed {
		if err := value.Validate(); err != nil {
			return RepositoryError(operation, err)
		}
		available = append(available, value.Pack.Manifest)
	}
	resolution, err := service.resolver.Resolve(ctx, PackDependencyResolutionRequest{Root: root, Available: available})
	if err != nil {
		return ExternalError(operation, err)
	}
	if resolution.Status != curriculum.PackDependenciesResolved {
		parts := make([]string, 0, len(resolution.Issues))
		for _, issue := range resolution.Issues {
			parts = append(parts, fmt.Sprintf("%s %s required by %s (%s)", issue.Kind, issue.PackID, issue.RequiredBy, issue.Reason))
		}
		return Invalid(operation, fmt.Errorf("pack dependencies are unresolved: %s", strings.Join(parts, "; ")))
	}
	return nil
}

func validatedArtifactError(result PackValidationResult) error {
	if len(result.Errors) > 0 {
		parts := make([]string, len(result.Errors))
		for index, issue := range result.Errors {
			parts[index] = fmt.Sprintf("[%s] %s: %s", issue.Code, issue.Path, issue.Message)
		}
		return fmt.Errorf("pack validation failed: %s", strings.Join(parts, "; "))
	}
	if result.Pack == nil {
		return fmt.Errorf("pack validator returned no pack")
	}
	if err := result.Pack.Validate(); err != nil {
		return err
	}
	if !canonicalSHA256(result.ContentHash) {
		return fmt.Errorf("pack validator returned an invalid content hash")
	}
	if len(result.PortableArchive) == 0 {
		return fmt.Errorf("pack validator returned no portable archive")
	}
	return nil
}

func sortedInstalledPacks(values []InstalledPack) []InstalledPack {
	result := append([]InstalledPack(nil), values...)
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i].Pack.Manifest, result[j].Pack.Manifest
		if left.ID != right.ID {
			return left.ID.String() < right.ID.String()
		}
		compared := compareSemver(parseSemver(left.Version.String()), parseSemver(right.Version.String()))
		if compared != 0 {
			return compared > 0
		}
		return left.Version.String() < right.Version.String()
	})
	return result
}

var _ PackInstallService = (*PackInstallerV1)(nil)
