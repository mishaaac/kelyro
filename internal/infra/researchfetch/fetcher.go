package researchfetch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/researchhttp"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

const FetchVersion = "source-fetch-v1"

type HTTPClient interface {
	Do(context.Context, researchhttp.Request) (researchhttp.Response, error)
}

type Option func(*Fetcher)

func WithClock(now func() time.Time) Option {
	return func(fetcher *Fetcher) {
		if now != nil {
			fetcher.now = now
		}
	}
}

type Fetcher struct {
	client HTTPClient
	now    func() time.Time
}

func New(client HTTPClient, options ...Option) *Fetcher {
	fetcher := &Fetcher{client: client, now: time.Now}
	for _, option := range options {
		option(fetcher)
	}
	return fetcher
}

func (fetcher *Fetcher) Fetch(ctx context.Context, request application.FetchRequest) (application.FetchedSource, error) {
	if fetcher == nil || fetcher.client == nil {
		return application.FetchedSource{}, errors.New("source fetch HTTP client is unavailable")
	}
	if err := request.Validate(); err != nil {
		return application.FetchedSource{}, fmt.Errorf("validate source fetch request: %w", err)
	}
	header := make(http.Header)
	if request.ETag != "" {
		header.Set("If-None-Match", request.ETag)
	}
	if request.LastModified != "" {
		header.Set("If-Modified-Since", request.LastModified)
	}
	response, err := fetcher.client.Do(ctx, researchhttp.Request{
		URL: request.Locator.String(), Header: header, MaxResponseBytes: request.MaximumBytes,
	})
	if err != nil {
		return application.FetchedSource{}, err
	}
	locatorValue := response.FinalURL
	if locatorValue == "" {
		locatorValue = request.Locator.String()
	}
	locator, err := research.NewSourceLocator(locatorValue)
	if err != nil {
		return application.FetchedSource{}, fmt.Errorf("source fetch final locator: %w", err)
	}
	fetchedAt, err := research.NewTimestamp(fetcher.now())
	if err != nil {
		return application.FetchedSource{}, fmt.Errorf("source fetch timestamp: %w", err)
	}
	metadata := research.FetchMetadata{
		StatusCode: response.StatusCode, ContentType: response.ContentType,
		ETag: response.ETag, LastModified: response.LastModified,
		ContentLength: int64(len(response.Body)), FetchVersion: FetchVersion,
	}
	if response.StatusCode == http.StatusNotModified {
		metadata.ETag = firstNonEmpty(metadata.ETag, request.ETag)
		metadata.LastModified = firstNonEmpty(metadata.LastModified, request.LastModified)
	} else if response.StatusCode != http.StatusNoContent {
		metadata.ContentHash = research.CanonicalContentHashV1(response.Body)
	}
	result := application.FetchedSource{
		SourceID: request.SourceID, Locator: locator, FetchedAt: fetchedAt,
		Metadata: metadata, Body: append([]byte(nil), response.Body...), Origin: application.FetchOriginLive,
	}
	if err := result.Validate(); err != nil {
		return application.FetchedSource{}, fmt.Errorf("validate source fetch response: %w", err)
	}
	return result, nil
}

func firstNonEmpty(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

var _ application.SourceFetcher = (*Fetcher)(nil)
