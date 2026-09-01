package app

import (
	"testing"

	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestServiceAssemblesLiveProvenanceStageFromWorkspaceService(t *testing.T) {
	t.Parallel()
	store := &fakeSourceRegistryStore{
		provenance: application.NewProvenanceService(memory.New().Repositories().Provenance),
		close:      func() {},
	}
	if stage, err := NewService(nil, nil).researchProvenanceForRun(store); err != nil || stage == nil {
		t.Fatalf("provenance stage = (%T, %v)", stage, err)
	}
}
