package update

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/privacy"
)

func TestParseVersionFollowsSemVerPrecedence(t *testing.T) {
	t.Parallel()
	ordered := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0",
	}
	for index := 0; index < len(ordered)-1; index++ {
		left, err := ParseVersion(ordered[index])
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", ordered[index], err)
		}
		right, err := ParseVersion(ordered[index+1])
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", ordered[index+1], err)
		}
		if left.Compare(right) >= 0 {
			t.Errorf("%s does not precede %s", left, right)
		}
	}

	withBuild, err := ParseVersion("v1.2.3-rc.1+linux.amd64")
	if err != nil || withBuild.String() != "1.2.3-rc.1+linux.amd64" || !withBuild.IsPrerelease() {
		t.Fatalf("ParseVersion(with build) = %q, %v", withBuild, err)
	}
	withoutBuild, _ := ParseVersion("1.2.3-rc.1+darwin.arm64")
	if withBuild.Compare(withoutBuild) != 0 {
		t.Error("build metadata affected precedence")
	}
}

func TestParseVersionRejectsMalformedVersions(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "dev", "unknown", "arbitrary", "1", "1.2", "01.2.3", "1.02.3", "1.2.03", "1.2.3-01", "1.2.3-", "1.2.3+", "v1.2.3 extra"} {
		if _, err := ParseVersion(value); !errors.Is(err, ErrMalformedVersion) {
			t.Errorf("ParseVersion(%q) error = %v, want ErrMalformedVersion", value, err)
		}
	}
}

func TestParseVersionAcceptsReleaseVersions(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"v0.1.0-alpha.1", "0.1.0-alpha.1", "0.1.0", "1.2.3"} {
		if _, err := ParseVersion(value); err != nil {
			t.Errorf("ParseVersion(%q) error = %v", value, err)
		}
	}
}

func TestCheckerFindsNewerPatchMinorAndMajorWithoutDowngrade(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		current string
		latest  string
		status  Status
	}{
		{name: "no update", current: "1.2.3", latest: "1.2.3", status: UpToDate},
		{name: "new patch", current: "1.2.3", latest: "1.2.4", status: UpdateAvailable},
		{name: "new minor", current: "1.2.3", latest: "1.3.0", status: UpdateAvailable},
		{name: "new major", current: "1.2.3", latest: "2.0.0", status: UpdateAvailable},
		{name: "no downgrade", current: "2.0.0", latest: "1.9.9", status: UpToDate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			provider := &fakeProvider{release: Release{Version: test.latest, URL: "https://example.invalid/release"}, found: true}
			result, err := New(test.current, provider, nil).Check(context.Background(), Stable, allowGate{})
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if result.Status != test.status || result.LatestVersion != test.latest || result.Source != SourceNetwork {
				t.Fatalf("Check() = %+v, want status %s latest %s", result, test.status, test.latest)
			}
			if test.name == "no downgrade" && result.Detail == "" {
				t.Fatal("downgrade protection lacks an explanation")
			}
		})
	}
}

func TestCheckerSupportsPrereleaseChannelAndRejectsPrereleaseOnStable(t *testing.T) {
	t.Parallel()
	provider := &fakeProvider{release: Release{Version: "1.3.0-beta.2"}, found: true}
	result, err := New("1.2.3", provider, nil).Check(context.Background(), Prerelease, allowGate{})
	if err != nil || result.Status != UpdateAvailable || result.LatestVersion != "1.3.0-beta.2" {
		t.Fatalf("Check(prerelease) = %+v, %v", result, err)
	}

	_, err = New("1.2.3", provider, nil).Check(context.Background(), Stable, allowGate{})
	if err == nil {
		t.Fatal("Check(stable with prerelease metadata) error = nil")
	}
}

func TestCheckerUsesFreshCacheBeforePrivacyGateOrProvider(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	cache := &fakeCache{found: true, check: CachedCheck{
		Channel: Stable, CheckedAt: now.Add(-time.Hour), Found: true,
		Release: Release{Version: "1.1.0"},
	}}
	provider := &fakeProvider{err: errors.New("must not run")}
	gate := &recordingGate{err: errors.New("must not run")}
	result, err := New("1.0.0", provider, cache).withClock(func() time.Time { return now }).Check(context.Background(), Stable, gate)
	if err != nil || result.Status != UpdateAvailable || result.Source != SourceCache {
		t.Fatalf("Check(cache) = %+v, %v", result, err)
	}
	if gate.calls != 0 || provider.calls != 0 {
		t.Fatalf("gate calls = %d, provider calls = %d; want zero", gate.calls, provider.calls)
	}
}

func TestCheckerTreatsDevelopmentBuildsAsUnavailableWithoutDependencies(t *testing.T) {
	t.Parallel()
	for _, current := range []string{"dev", "unknown"} {
		t.Run(current, func(t *testing.T) {
			t.Parallel()
			cache := &fakeCache{err: errors.New("must not run")}
			provider := &fakeProvider{err: errors.New("must not run")}
			gate := &recordingGate{err: errors.New("must not run")}
			result, err := New(current, provider, cache).Check(context.Background(), Stable, gate)
			if err != nil {
				t.Fatalf("Check(%q) error = %v", current, err)
			}
			if result.Status != Unavailable || result.Source != SourceNone || result.Channel != Stable ||
				result.CurrentVersion != current || result.Detail != "development build" {
				t.Fatalf("Check(%q) = %+v", current, result)
			}
			if cache.loads != 0 || cache.saves != 0 || gate.calls != 0 || provider.calls != 0 {
				t.Fatalf("dependency calls for %q: cache loads=%d saves=%d gate=%d provider=%d", current, cache.loads, cache.saves, gate.calls, provider.calls)
			}
		})
	}
}

func TestCheckerRefreshesExpiredCacheAndCachesEmptyResult(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	cache := &fakeCache{found: true, check: CachedCheck{
		Channel: Stable, CheckedAt: now.Add(-25 * time.Hour), Found: true,
		Release: Release{Version: "9.0.0"},
	}}
	provider := &fakeProvider{}
	result, err := New("1.0.0", provider, cache).withClock(func() time.Time { return now }).Check(context.Background(), Stable, allowGate{})
	if err != nil || result.Status != UpToDate || result.Detail != "no published releases found" {
		t.Fatalf("Check(expired cache) = %+v, %v", result, err)
	}
	if provider.calls != 1 || cache.saves != 1 || cache.saved.Found {
		t.Fatalf("provider calls = %d, saved = %+v", provider.calls, cache.saved)
	}
}

func TestCheckerTreatsOfflineAsNonFatalAndNeverCallsProvider(t *testing.T) {
	t.Parallel()
	provider := &fakeProvider{release: Release{Version: "2.0.0"}, found: true}
	result, err := New("1.0.0", provider, nil).Check(context.Background(), Stable, &recordingGate{err: privacy.ErrNetworkBlocked})
	if err != nil {
		t.Fatalf("Check(offline) error = %v", err)
	}
	if result.Status != Unavailable || result.Source != SourceNone || result.Detail == "" {
		t.Fatalf("Check(offline) = %+v", result)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want zero", provider.calls)
	}
}

func TestCheckerTreatsProviderUnavailabilityAsNonFatal(t *testing.T) {
	t.Parallel()
	provider := &fakeProvider{err: fmtError(ErrProviderUnavailable)}
	result, err := New("1.0.0", provider, nil).Check(context.Background(), Stable, allowGate{})
	if err != nil || result.Status != Unavailable {
		t.Fatalf("Check(unavailable) = %+v, %v", result, err)
	}
}

func TestCheckerRejectsMalformedCurrentAndProviderVersions(t *testing.T) {
	t.Parallel()
	_, err := New("not-a-version", &fakeProvider{}, nil).Check(context.Background(), Stable, allowGate{})
	if !errors.Is(err, ErrMalformedVersion) {
		t.Fatalf("Check(malformed current) error = %v", err)
	}

	_, err = New("1.0.0", &fakeProvider{release: Release{Version: "latest"}, found: true}, nil).
		Check(context.Background(), Stable, allowGate{})
	if !errors.Is(err, ErrMalformedVersion) {
		t.Fatalf("Check(malformed provider version) error = %v, want ErrMalformedVersion", err)
	}
}

type fakeProvider struct {
	release Release
	found   bool
	err     error
	calls   int
}

func (provider *fakeProvider) Latest(context.Context, Channel) (Release, bool, error) {
	provider.calls++
	return provider.release, provider.found, provider.err
}

type fakeCache struct {
	check CachedCheck
	found bool
	err   error
	saved CachedCheck
	loads int
	saves int
}

func (cache *fakeCache) Load(context.Context, Channel) (CachedCheck, bool, error) {
	cache.loads++
	return cache.check, cache.found, cache.err
}

func (cache *fakeCache) Save(_ context.Context, check CachedCheck) error {
	cache.saved = check
	cache.saves++
	return nil
}

type allowGate struct{}

func (allowGate) Authorize(context.Context, privacy.Request) error { return nil }

type recordingGate struct {
	err   error
	calls int
}

func (gate *recordingGate) Authorize(context.Context, privacy.Request) error {
	gate.calls++
	return gate.err
}

func fmtError(target error) error { return errors.Join(errors.New("network failed"), target) }
