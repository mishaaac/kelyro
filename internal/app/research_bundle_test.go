package app

import (
	"testing"

	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestServiceAssemblesLiveBundleStageFromWorkspaceService(t *testing.T) {
	t.Parallel()
	store := &fakeSourceRegistryStore{
		bundles: application.NewSourceBundleService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil),
		close:   func() {},
	}
	if stage, err := NewService(nil, nil).researchBundleForRun(store); err != nil || stage == nil {
		t.Fatalf("bundle stage = (%T, %v)", stage, err)
	}
}
