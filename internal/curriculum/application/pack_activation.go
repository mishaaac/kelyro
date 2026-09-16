package application

import (
	"context"
	"errors"
	"fmt"
)

const PackActivationCoordinatorVersionV1 = "pack-activation-coordinator-v1"

// PackActivationCoordinatorV1 makes initial pack activation a Student Core
// hand-off, so a successful CLI activation is immediately visible through the
// I-02 roadmap. Version transitions remain owned by PackUpgradeExecutorV1.
type PackActivationCoordinatorV1 struct {
	packs    PackInstallService
	students StudentCurriculumActivationService
}

func NewPackActivationCoordinatorV1(packs PackInstallService, students StudentCurriculumActivationService) *PackActivationCoordinatorV1 {
	return &PackActivationCoordinatorV1{packs: packs, students: students}
}

func (service *PackActivationCoordinatorV1) Activate(ctx context.Context, request PackActivateRequest) (PackActivation, error) {
	const operation = "activate Learning Pack for learner"
	if service == nil || service.packs == nil || service.students == nil {
		return PackActivation{}, Classify(ErrorUnavailable, operation, fmt.Errorf("pack or Student Core activation service is not configured"))
	}
	active, activeErr := service.packs.Active(ctx, request.WorkspaceRoot)
	if activeErr == nil && (active.Pack.Manifest.ID != request.PackID || active.Pack.Manifest.Version != request.Version) {
		return PackActivation{}, Invalid(operation, fmt.Errorf("workspace already uses %s@%s; use the pack upgrade workflow", active.Pack.Manifest.ID, active.Pack.Manifest.Version.String()))
	}
	if activeErr != nil && !errors.Is(activeErr, ErrNotFound) {
		return PackActivation{}, activeErr
	}
	installed, err := service.packs.Find(ctx, request.PackID)
	if err != nil {
		return PackActivation{}, err
	}
	var selected *InstalledPack
	for index := range installed {
		if installed[index].Pack.Manifest.Version == request.Version {
			selected = &installed[index]
			break
		}
	}
	if selected == nil {
		return PackActivation{}, Classify(ErrorNotFound, operation, fmt.Errorf("pack %s@%s is not installed", request.PackID, request.Version.String()))
	}
	studentRequest := StudentCurriculumActivationRequest{WorkspaceRoot: request.WorkspaceRoot, Curriculum: selected.Pack.Curriculum}
	if err := service.students.Preflight(ctx, studentRequest); err != nil {
		return PackActivation{}, Invalid(operation, err)
	}
	if _, err := service.students.ActivateCurriculum(ctx, studentRequest); err != nil {
		return PackActivation{}, ExternalError(operation, err)
	}
	activation, err := service.packs.Activate(ctx, request)
	if err != nil {
		return PackActivation{}, err
	}
	return activation, nil
}

var _ PackActivationService = (*PackActivationCoordinatorV1)(nil)
