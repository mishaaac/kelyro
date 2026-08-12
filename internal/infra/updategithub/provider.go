// Package updategithub implements release metadata lookup through GitHub's
// public REST API. It never downloads or installs release artifacts.
package updategithub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/update"
)

const (
	defaultEndpoint = "https://api.github.com/repos/mishaaac/kelyro/releases"
	apiVersion      = "2026-03-10"
	maxResponseSize = 2 * 1024 * 1024
	requestTimeout  = 10 * time.Second
)

type httpClient interface {
	Do(request *http.Request) (*http.Response, error)
}

type Provider struct {
	client    httpClient
	endpoint  string
	userAgent string
}

// New creates the optional production GitHub adapter for Kelyro's public
// releases. No credential is read or sent.
func New(appVersion string) *Provider {
	return &Provider{
		client:    &http.Client{Timeout: requestTimeout},
		endpoint:  defaultEndpoint,
		userAgent: safeUserAgent(appVersion),
	}
}

func (provider *Provider) Latest(ctx context.Context, channel update.Channel) (update.Release, bool, error) {
	if err := ctx.Err(); err != nil {
		return update.Release{}, false, err
	}
	if !channel.Valid() {
		return update.Release{}, false, fmt.Errorf("invalid update channel %q", channel)
	}
	if provider == nil || provider.client == nil || provider.endpoint == "" {
		return update.Release{}, false, errors.New("GitHub release provider is unavailable")
	}
	endpoint, err := url.Parse(provider.endpoint)
	if err != nil {
		return update.Release{}, false, fmt.Errorf("invalid GitHub releases endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("per_page", "100")
	endpoint.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return update.Release{}, false, fmt.Errorf("create GitHub release request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", apiVersion)
	request.Header.Set("User-Agent", provider.userAgent)

	response, err := provider.client.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return update.Release{}, false, ctxErr
		}
		return update.Release{}, false, fmt.Errorf("%w: query GitHub releases: %v", update.ErrProviderUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return update.Release{}, false, fmt.Errorf("%w: GitHub releases returned HTTP %d", update.ErrProviderUnavailable, response.StatusCode)
	}
	encoded, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return update.Release{}, false, fmt.Errorf("%w: read GitHub releases: %v", update.ErrProviderUnavailable, err)
	}
	if len(encoded) > maxResponseSize {
		return update.Release{}, false, fmt.Errorf("GitHub release metadata exceeds %d bytes", maxResponseSize)
	}
	var releases []releaseDocument
	if err := json.Unmarshal(encoded, &releases); err != nil {
		return update.Release{}, false, fmt.Errorf("decode GitHub release metadata: %w", err)
	}
	return selectLatest(releases, channel)
}

type releaseDocument struct {
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
}

func selectLatest(releases []releaseDocument, channel update.Channel) (update.Release, bool, error) {
	var selected releaseDocument
	var selectedVersion update.Version
	found := false
	for _, candidate := range releases {
		if candidate.Draft {
			continue
		}
		version, err := update.ParseVersion(candidate.TagName)
		if err != nil {
			continue
		}
		if channel == update.Stable && (candidate.Prerelease || version.IsPrerelease()) {
			continue
		}
		if !found || version.Compare(selectedVersion) > 0 {
			selected, selectedVersion, found = candidate, version, true
		}
	}
	if !found {
		return update.Release{}, false, nil
	}
	return update.Release{
		Version: selectedVersion.String(), URL: safeReleaseURL(selected.HTMLURL), PublishedAt: selected.PublishedAt,
	}, true, nil
}

func safeReleaseURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "github.com" || parsed.User != nil {
		return ""
	}
	return parsed.String()
}

func safeUserAgent(version string) string {
	version = strings.Map(func(character rune) rune {
		if character >= 33 && character <= 126 && character != '/' && character != '\\' {
			return character
		}
		return -1
	}, version)
	if version == "" {
		version = "dev"
	}
	return "kelyro/" + version
}
