package app

import (
	"testing"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestServiceAssemblesFinalizationBoundariesFromWorkspaceStore(t *testing.T) {
	t.Parallel()
	repositories := memory.New().Repositories()
	store := &fakeSourceRegistryStore{
		bundles:      researchapp.NewSourceBundleService(repositories.Bundles, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		finalization: researchapp.NewResearchFinalizationService(repositories.Finalization),
		close:        func() {},
	}
	stage, finalization, err := NewService(nil, nil).researchFinalizationForRun(store)
	if err != nil || stage == nil || finalization == nil {
		t.Fatalf("finalization boundaries = (%T, %T, %v)", stage, finalization, err)
	}
}
