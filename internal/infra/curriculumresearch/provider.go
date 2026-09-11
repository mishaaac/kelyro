// Package curriculumresearch bridges the durable I-03 read services to the
// I-04 evidence-ingestion port. It exposes no discovery or network operation.
package curriculumresearch

import (
	"context"
	"fmt"

	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/research"
)

type BundleGetter interface {
	Get(context.Context, research.ID) (research.SourceBundle, error)
}

type ClaimGetter interface {
	Get(context.Context, research.ClaimID) (research.Claim, error)
}

type ConflictGetter interface {
	Get(context.Context, research.ID) (research.Conflict, error)
}

type Provider struct {
	bundles   BundleGetter
	claims    ClaimGetter
	conflicts ConflictGetter
}

func NewProvider(bundles BundleGetter, claims ClaimGetter, conflicts ConflictGetter) (*Provider, error) {
	if bundles == nil || claims == nil || conflicts == nil {
		return nil, fmt.Errorf("curriculum research provider requires bundle, claim, and conflict readers")
	}
	return &Provider{bundles: bundles, claims: claims, conflicts: conflicts}, nil
}

func (provider *Provider) GetBundle(ctx context.Context, id research.ID) (research.SourceBundle, error) {
	return provider.bundles.Get(ctx, id)
}

func (provider *Provider) GetClaim(ctx context.Context, id research.ClaimID) (research.Claim, error) {
	return provider.claims.Get(ctx, id)
}

func (provider *Provider) GetConflict(ctx context.Context, id research.ID) (research.Conflict, error) {
	return provider.conflicts.Get(ctx, id)
}

var _ curriculumapp.ResearchBundleProvider = (*Provider)(nil)
