package updategithub

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/update"
)

func TestProviderUsesRequiredHeadersAndSelectsChannel(t *testing.T) {
	t.Parallel()
	provider := New("v1.0.0")
	provider.client = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Query().Get("per_page") != "100" {
			t.Errorf("request = %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("Accept") != "application/vnd.github+json" || request.Header.Get("X-GitHub-Api-Version") != apiVersion || request.Header.Get("User-Agent") != "kelyro/v1.0.0" {
			t.Errorf("headers = %+v", request.Header)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: ioNopCloser(`[
			{"tag_name":"v1.2.0","html_url":"https://github.com/mishaaac/kelyro/releases/tag/v1.2.0","draft":false,"prerelease":false,"published_at":"2026-08-01T00:00:00Z"},
			{"tag_name":"v1.3.0-beta.1","html_url":"https://github.com/mishaaac/kelyro/releases/tag/v1.3.0-beta.1","draft":false,"prerelease":true,"published_at":"2026-08-02T00:00:00Z"},
			{"tag_name":"v9.0.0","draft":true,"prerelease":false},
			{"tag_name":"not-semver","draft":false,"prerelease":false}
		]`),
		}, nil
	})
	for channel, want := range map[update.Channel]string{update.Stable: "1.2.0", update.Prerelease: "1.3.0-beta.1"} {
		release, found, err := provider.Latest(context.Background(), channel)
		if err != nil || !found || release.Version != want || !strings.Contains(release.URL, want) {
			t.Fatalf("Latest(%s) = %+v, %v, %v; want %s", channel, release, found, err, want)
		}
	}
}

func TestProviderReturnsNoReleaseWithoutCallingRealNetwork(t *testing.T) {
	t.Parallel()
	provider := New("dev")
	provider.client = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioNopCloser(`[{"tag_name":"malformed","draft":false}]`),
			Header:     make(http.Header),
		}, nil
	})
	if _, found, err := provider.Latest(context.Background(), update.Stable); err != nil || found {
		t.Fatalf("Latest() found=%v error=%v", found, err)
	}
}

func TestProviderMapsTransportAndStatusFailures(t *testing.T) {
	t.Parallel()
	transportErr := errors.New("offline")
	provider := New("dev")
	provider.client = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, transportErr })
	if _, _, err := provider.Latest(context.Background(), update.Stable); !errors.Is(err, update.ErrProviderUnavailable) {
		t.Fatalf("Latest(transport) error = %v", err)
	}

	provider.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusForbidden, Body: ioNopCloser(""), Header: make(http.Header)}, nil
	})
	if _, _, err := provider.Latest(context.Background(), update.Stable); !errors.Is(err, update.ErrProviderUnavailable) {
		t.Fatalf("Latest(status) error = %v", err)
	}
}

func TestSelectLatestOmitsUntrustedReleaseURL(t *testing.T) {
	t.Parallel()
	release, found, err := selectLatest([]releaseDocument{{
		TagName: "v1.0.0", HTMLURL: "https://example.invalid/private/path",
	}}, update.Stable)
	if err != nil || !found || release.Version != "1.0.0" || release.URL != "" {
		t.Fatalf("selectLatest() = %+v, %v, %v", release, found, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

func ioNopCloser(value string) *stringReadCloser {
	return &stringReadCloser{Reader: strings.NewReader(value)}
}

type stringReadCloser struct{ *strings.Reader }

func (*stringReadCloser) Close() error { return nil }
