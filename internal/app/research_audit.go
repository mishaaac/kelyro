package app

import (
	"fmt"

	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func (service *Service) researchTerminalAuditForRun(store researchapp.SourceRegistryStore) (researchapp.LiveResearchTerminalAuditService, error) {
	if store == nil || store.Research() == nil || store.Costs() == nil {
		return nil, fmt.Errorf("research terminal audit services are unavailable")
	}
	audit, err := researchapp.NewLiveResearchTerminalAuditService(store.Research(), store.Costs())
	if err != nil {
		return nil, fmt.Errorf("assemble live research terminal audit: %w", err)
	}
	return audit, nil
}
