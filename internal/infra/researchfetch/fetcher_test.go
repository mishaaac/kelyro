package researchfetch

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/researchhttp"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestFetcherBuildsCanonicalMetadataAndChangedBodyHash(t *testing.T) {
	firstResponseBody := []byte("first")
	client := &recordingClient{responses: []researchhttp.Response{
		{StatusCode: http.StatusOK, ContentType: "text/plain", ETag: `"one"`, FinalURL: "https://docs.example/source", Body: firstResponseBody},
		{StatusCode: http.StatusOK, ContentType: "text/plain", ETag: `"two"`, FinalURL: "https://docs.example/source", Body: []byte("second")},
	}}
	fetcher := New(client, WithClock(func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.FixedZone("PET", -5*60*60)) }))
	request := fetchRequest(t)

	first, err := fetcher.Fetch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fetcher.Fetch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Metadata.ContentHash != "sha256:a7937b64b8caa58f03721bb6bacf5c78cb235febe0e70b1b84cd99541461a08e" {
		t.Fatalf("first content hash = %q", first.Metadata.ContentHash)
	}
	if first.Metadata.ContentHash == second.Metadata.ContentHash {
		t.Fatal("changed response bodies produced the same content hash")
	}
	if first.Metadata.ContentLength != 5 || first.Metadata.FetchVersion != FetchVersion {
		t.Fatalf("metadata = %+v", first.Metadata)
	}
	if got := first.FetchedAt.Time(); !got.Equal(time.Date(2026, 8, 24, 17, 0, 0, 0, time.UTC)) || got.Location() != time.UTC {
		t.Fatalf("fetched at = %v", got)
	}
	first.Body[0] = 'X'
	if string(firstResponseBody) != "first" {
		t.Fatal("fetched result aliases the HTTP response body")
	}
}

func TestFetcherSendsConditionalValidatorsAndRepresents304(t *testing.T) {
	client := &recordingClient{responses: []researchhttp.Response{{
		StatusCode: http.StatusNotModified, FinalURL: "https://docs.example/source",
	}}}
	fetcher := New(client, WithClock(func() time.Time { return time.Date(2026, 8, 24, 18, 0, 0, 0, time.UTC) }))
	request := fetchRequest(t)
	request.ETag = `"stable"`
	request.LastModified = "Mon, 24 Aug 2026 12:00:00 GMT"

	result, err := fetcher.Fetch(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got := client.requests[0].Header.Get("If-None-Match"); got != request.ETag {
		t.Fatalf("If-None-Match = %q", got)
	}
	if got := client.requests[0].Header.Get("If-Modified-Since"); got != request.LastModified {
		t.Fatalf("If-Modified-Since = %q", got)
	}
	if result.Metadata.StatusCode != http.StatusNotModified || result.Metadata.ETag != request.ETag ||
		result.Metadata.LastModified != request.LastModified || result.Metadata.ContentHash != "" || len(result.Body) != 0 {
		t.Fatalf("304 result = %+v", result)
	}
}

func TestFetcherDelegatesContentAndSizeEnforcementToHardenedClient(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "invalid content type", err: researchhttp.ErrContentType},
		{name: "size limit", err: researchhttp.ErrResponseTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &recordingClient{err: test.err}
			_, err := New(client).Fetch(context.Background(), fetchRequest(t))
			if !errors.Is(err, test.err) {
				t.Fatalf("Fetch() error = %v, want %v", err, test.err)
			}
			if client.requests[0].MaxResponseBytes != fetchRequest(t).MaximumBytes {
				t.Fatalf("maximum bytes = %d", client.requests[0].MaxResponseBytes)
			}
		})
	}
}

type recordingClient struct {
	requests  []researchhttp.Request
	responses []researchhttp.Response
	err       error
}

func (client *recordingClient) Do(_ context.Context, request researchhttp.Request) (researchhttp.Response, error) {
	request.Header = request.Header.Clone()
	client.requests = append(client.requests, request)
	if client.err != nil {
		return researchhttp.Response{}, client.err
	}
	response := client.responses[0]
	client.responses = client.responses[1:]
	return response, nil
}

func fetchRequest(t *testing.T) application.FetchRequest {
	t.Helper()
	sourceID, err := research.NewSourceID("source.fetch-test")
	if err != nil {
		t.Fatal(err)
	}
	locator, err := research.NewSourceLocator("https://docs.example/source")
	if err != nil {
		t.Fatal(err)
	}
	return application.FetchRequest{SourceID: sourceID, Locator: locator, MaximumBytes: 1024}
}
