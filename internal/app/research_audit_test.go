package app

import (
	"testing"

	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestServiceAssemblesTerminalResearchAuditFromWorkspaceServices(t *testing.T) {
	t.Parallel()
	repositories := memory.New().Repositories()
	store := &fakeSourceRegistryStore{
		research: application.NewResearchService(repositories.Runs),
		costs:    application.NewResearchCostService(repositories.Costs),
		close:    func() {},
	}
	if audit, err := NewService(nil, nil).researchTerminalAuditForRun(store); err != nil || audit == nil {
		t.Fatalf("terminal audit = (%T, %v)", audit, err)
	}
}
