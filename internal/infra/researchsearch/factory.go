package researchsearch

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/research/application"
)

var (
	ErrProviderDisabled    = errors.New("research search provider is disabled")
	ErrProviderUnavailable = errors.New("research search provider is unavailable")
)

// Factory owns the fixed production search transport. It performs no network
// request while being constructed or while assembling a provider for a run.
type Factory struct {
	client HTTPClient
}

func NewFactory(config TransportConfig) (*Factory, error) {
	client, err := NewBraveHTTPClient(config)
	if err != nil {
		return nil, fmt.Errorf("create research search transport: %w", err)
	}
	return &Factory{client: client}, nil
}

func newFactory(client HTTPClient) *Factory { return &Factory{client: client} }

// Build assembles the selected provider and the only production discovery
// boundary. Disabled or unknown providers never read Secrets and no branch
// substitutes a fixture/static provider.
func (factory *Factory) Build(ctx context.Context, request application.LiveSearchBuildRequest) (application.LiveSearchBuildResult, error) {
	if err := ctx.Err(); err != nil {
		return application.LiveSearchBuildResult{}, err
	}
	if err := request.Validate(); err != nil {
		return application.LiveSearchBuildResult{}, fmt.Errorf("assemble live research search: %w", err)
	}
	switch request.Settings.Provider {
	case "":
		return application.LiveSearchBuildResult{}, ErrProviderDisabled
	case ProviderID:
		// Continue with the only selected production adapter.
	default:
		return application.LiveSearchBuildResult{}, ErrProviderUnavailable
	}
	if factory == nil || factory.client == nil {
		return application.LiveSearchBuildResult{}, ErrProviderUnavailable
	}

	provider, _, err := NewBraveFromSecrets(factory.client, request.Secrets)
	if err != nil {
		return application.LiveSearchBuildResult{}, err
	}
	discovery, err := application.NewCostControlledDiscoveryService(
		provider, nil, request.Access, request.Costs, request.Clock,
		application.LiveSearchCostPolicy{
			RunID: request.RunID, MaxResultsPerQuery: request.Settings.MaxResultsPerQuery,
			AlgorithmVersion: application.LiveSearchCostPolicyV1,
		},
	)
	if err != nil {
		return application.LiveSearchBuildResult{}, fmt.Errorf("assemble live research discovery: %w", err)
	}
	return application.LiveSearchBuildResult{Provider: provider, Discovery: discovery}, nil
}

var _ application.LiveSearchProviderFactory = (*Factory)(nil)
