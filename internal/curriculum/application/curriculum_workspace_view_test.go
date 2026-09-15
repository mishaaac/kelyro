package application

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCurriculumWorkspaceViewV1ComposesInspectionAndDryRunUpgrade(t *testing.T) {
	t.Parallel()
	packID, _ := curriculum.NewID("pack.backend")
	current, _ := curriculum.NewPackVersion("1.0.0")
	candidate, _ := curriculum.NewPackVersion("1.1.0")
	pack := curriculum.LearningPack{Manifest: curriculum.PackManifest{ID: packID, Version: current}}
	packs := &workspaceViewPacks{active: InstalledPack{Pack: pack}}
	inspector := &workspaceViewInspector{result: CurriculumInspection{Pack: pack, AlgorithmVersion: CurriculumInspectionVersionV1}}
	upgrades := &workspaceViewUpgrades{result: PackUpgradeResult{Current: pack, Candidate: curriculum.LearningPack{Manifest: curriculum.PackManifest{ID: packID, Version: candidate}}}}

	view, err := NewCurriculumWorkspaceViewV1(packs, inspector, upgrades).View(context.Background(), "/workspace")
	if err != nil {
		t.Fatalf("View() error = %v", err)
	}
	if view.UpdateStatus != CurriculumUpdateAvailable || view.MigrationPreview == nil || !upgrades.request.DryRun || upgrades.request.WorkspaceRoot != "/workspace" {
		t.Fatalf("view = %+v, request = %+v", view, upgrades.request)
	}
}

func TestCurriculumWorkspaceViewV1KeepsBaseViewWhenNoUpdateExists(t *testing.T) {
	t.Parallel()
	packID, _ := curriculum.NewID("pack.backend")
	version, _ := curriculum.NewPackVersion("1.0.0")
	pack := curriculum.LearningPack{Manifest: curriculum.PackManifest{ID: packID, Version: version}}
	view, err := NewCurriculumWorkspaceViewV1(
		&workspaceViewPacks{active: InstalledPack{Pack: pack}},
		&workspaceViewInspector{result: CurriculumInspection{Pack: pack, AlgorithmVersion: CurriculumInspectionVersionV1}},
		&workspaceViewUpgrades{err: errors.New("no newer installed version is available for pack.backend")},
	).View(context.Background(), "/workspace")
	if err != nil || view.UpdateStatus != CurriculumUpdateNone || view.MigrationPreview != nil {
		t.Fatalf("View() = %+v, %v", view, err)
	}
}

type workspaceViewPacks struct{ active InstalledPack }

func (*workspaceViewPacks) Install(context.Context, PackInstallRequest) (PackInstallResult, error) {
	return PackInstallResult{}, nil
}
func (*workspaceViewPacks) Activate(context.Context, PackActivateRequest) (PackActivation, error) {
	return PackActivation{}, nil
}
func (*workspaceViewPacks) List(context.Context) ([]InstalledPack, error) { return nil, nil }
func (*workspaceViewPacks) Find(context.Context, curriculum.ID) ([]InstalledPack, error) {
	return nil, nil
}
func (fake *workspaceViewPacks) Active(context.Context, string) (InstalledPack, error) {
	return fake.active, nil
}

type workspaceViewInspector struct{ result CurriculumInspection }

func (fake *workspaceViewInspector) Inspect(context.Context, curriculum.LearningPack) (CurriculumInspection, error) {
	return fake.result, nil
}

type workspaceViewUpgrades struct {
	result  PackUpgradeResult
	err     error
	request PackUpgradeRequest
}

func (fake *workspaceViewUpgrades) Upgrade(_ context.Context, request PackUpgradeRequest) (PackUpgradeResult, error) {
	fake.request = request
	return fake.result, fake.err
}
