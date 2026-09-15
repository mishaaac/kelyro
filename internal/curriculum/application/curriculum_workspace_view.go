package application

import (
	"context"
	"fmt"
	"strings"
)

type CurriculumUpdateStatus string

const (
	CurriculumUpdateNone        CurriculumUpdateStatus = "none_available"
	CurriculumUpdateAvailable   CurriculumUpdateStatus = "available"
	CurriculumUpdateUnavailable CurriculumUpdateStatus = "unavailable"
)

type CurriculumWorkspaceView struct {
	Inspection       CurriculumInspection
	UpdateStatus     CurriculumUpdateStatus
	UpdateReason     string
	MigrationPreview *PackUpgradeResult
}

type CurriculumWorkspaceViewService interface {
	View(context.Context, string) (CurriculumWorkspaceView, error)
}

type CurriculumWorkspaceViewV1 struct {
	packs     PackInstallService
	inspector CurriculumInspectionService
	upgrades  PackUpgradeService
}

func NewCurriculumWorkspaceViewV1(packs PackInstallService, inspector CurriculumInspectionService, upgrades PackUpgradeService) *CurriculumWorkspaceViewV1 {
	return &CurriculumWorkspaceViewV1{packs: packs, inspector: inspector, upgrades: upgrades}
}

func (service *CurriculumWorkspaceViewV1) View(ctx context.Context, workspaceRoot string) (CurriculumWorkspaceView, error) {
	const operation = "read curriculum workspace view"
	if service == nil || service.packs == nil || service.inspector == nil {
		return CurriculumWorkspaceView{}, Classify(ErrorUnavailable, operation, fmt.Errorf("curriculum workspace view is not configured"))
	}
	if strings.TrimSpace(workspaceRoot) == "" {
		return CurriculumWorkspaceView{}, Invalid(operation, fmt.Errorf("workspace root is empty"))
	}
	active, err := service.packs.Active(ctx, workspaceRoot)
	if err != nil {
		return CurriculumWorkspaceView{}, RepositoryError(operation, err)
	}
	inspection, err := service.inspector.Inspect(ctx, active.Pack)
	if err != nil {
		return CurriculumWorkspaceView{}, ExternalError(operation, err)
	}
	view := CurriculumWorkspaceView{Inspection: inspection, UpdateStatus: CurriculumUpdateNone, UpdateReason: "no newer installed pack version is available"}
	if service.upgrades == nil {
		view.UpdateStatus = CurriculumUpdateUnavailable
		view.UpdateReason = "pack upgrade preview service is unavailable"
		return view, nil
	}
	preview, err := service.upgrades.Upgrade(ctx, PackUpgradeRequest{
		WorkspaceRoot: workspaceRoot, PackID: active.Pack.Manifest.ID, DryRun: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "no newer installed version is available") {
			return view, nil
		}
		view.UpdateStatus = CurriculumUpdateUnavailable
		view.UpdateReason = err.Error()
		return view, nil
	}
	view.UpdateStatus = CurriculumUpdateAvailable
	view.UpdateReason = fmt.Sprintf("%s is installed and ready for preview", preview.Candidate.Manifest.Version.String())
	view.MigrationPreview = &preview
	return view, nil
}

var _ CurriculumWorkspaceViewService = (*CurriculumWorkspaceViewV1)(nil)
