package curriculumresearch

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
)

var errSentinel = errors.New("sentinel")

type bundleGetter struct{}

func (bundleGetter) Get(context.Context, research.ID) (research.SourceBundle, error) {
	return research.SourceBundle{}, errSentinel
}

type claimGetter struct{}

func (claimGetter) Get(context.Context, research.ClaimID) (research.Claim, error) {
	return research.Claim{}, errSentinel
}

type conflictGetter struct{}

func (conflictGetter) Get(context.Context, research.ID) (research.Conflict, error) {
	return research.Conflict{}, errSentinel
}

func TestProviderDelegatesOnlyDurableReads(t *testing.T) {
	t.Parallel()
	provider, err := NewProvider(bundleGetter{}, claimGetter{}, conflictGetter{})
	if err != nil {
		t.Fatal(err)
	}
	id, _ := research.NewID("record.one")
	claimID, _ := research.NewClaimID("claim.one")
	if _, err := provider.GetBundle(context.Background(), id); !errors.Is(err, errSentinel) {
		t.Fatalf("GetBundle() error=%v", err)
	}
	if _, err := provider.GetClaim(context.Background(), claimID); !errors.Is(err, errSentinel) {
		t.Fatalf("GetClaim() error=%v", err)
	}
	if _, err := provider.GetConflict(context.Background(), id); !errors.Is(err, errSentinel) {
		t.Fatalf("GetConflict() error=%v", err)
	}
}

func TestProviderRequiresAllReaders(t *testing.T) {
	t.Parallel()
	if _, err := NewProvider(bundleGetter{}, claimGetter{}, nil); err == nil {
		t.Fatal("NewProvider() accepted a missing reader")
	}
}
