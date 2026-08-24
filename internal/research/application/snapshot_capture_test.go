package application_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestSnapshotCapturePersistsImmutableChangedAnd304History(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "snapshot-capture")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	firstBody := []byte("first body")
	secondBody := []byte("changed body")
	fetch := &recordingCaptureFetchService{results: []application.FetchedSource{
		fetchedFixture(t, source, 10, http.StatusOK, `"one"`, firstBody),
		fetchedFixture(t, source, 11, http.StatusOK, `"one"`, secondBody),
		fetchedFixture(t, source, 12, http.StatusNotModified, `"one"`, nil),
	}}
	sequence := 0
	service := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, fetch,
		application.WithSnapshotIDGenerator(func() (research.ID, error) {
			sequence++
			return research.NewID(fmt.Sprintf("snapshot.capture.%d", sequence))
		}))

	first, err := service.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: 4096, BodyPolicy: application.SnapshotMetadataOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: 4096, BodyPolicy: application.SnapshotNormalizedExcerpt,
	})
	if err != nil {
		t.Fatal(err)
	}
	third, err := service.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: 4096, BodyPolicy: application.SnapshotBoundedCachedBody,
	})
	if err != nil {
		t.Fatal(err)
	}

	if first.NormalizationInput != nil || len(first.CacheCandidate) != 0 {
		t.Fatalf("metadata-only capture retained body: %+v", first)
	}
	if second.NormalizationInput == nil || string(second.NormalizationInput.Body) != string(secondBody) || len(second.CacheCandidate) != 0 {
		t.Fatalf("normalized-excerpt capture body disposition = %+v", second)
	}
	if second.Snapshot.Fetch.ContentHash == first.Snapshot.Fetch.ContentHash {
		t.Fatal("changed body did not create a distinct content hash")
	}
	if fetch.requests[1].ETag != first.Snapshot.Fetch.ETag {
		t.Fatalf("second conditional ETag = %q", fetch.requests[1].ETag)
	}
	if third.RevalidatedSnapshotID == nil || *third.RevalidatedSnapshotID != second.Snapshot.ID {
		t.Fatalf("304 revalidation reference = %+v", third.RevalidatedSnapshotID)
	}
	if third.Snapshot.Fetch.StatusCode != http.StatusNotModified ||
		third.Snapshot.Fetch.ContentHash != second.Snapshot.Fetch.ContentHash ||
		third.Snapshot.Fetch.ContentLength != second.Snapshot.Fetch.ContentLength ||
		third.Snapshot.Fetch.ContentType != second.Snapshot.Fetch.ContentType {
		t.Fatalf("304 snapshot metadata = %+v", third.Snapshot.Fetch)
	}
	if len(third.CacheCandidate) != 0 {
		t.Fatalf("304 capture invented a cached body: %q", third.CacheCandidate)
	}
	history, err := repositories.Snapshots.ListBySource(ctx, source.ID)
	if err != nil || len(history) != 3 {
		t.Fatalf("snapshot history = (%+v, %v)", history, err)
	}
	if history[0].Fetch.ContentHash != first.Snapshot.Fetch.ContentHash {
		t.Fatal("later captures overwrote historical snapshot metadata")
	}
}

func TestSnapshotCaptureProducesBoundedCacheCandidateWithoutPersistingBody(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "cache-candidate")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	body := []byte("bounded fixture")
	fetch := &recordingCaptureFetchService{results: []application.FetchedSource{
		fetchedFixture(t, source, 10, http.StatusOK, `"cache"`, body),
	}}
	service := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, fetch,
		application.WithSnapshotIDGenerator(func() (research.ID, error) { return research.NewID("snapshot.cache-candidate") }))

	result, err := service.Capture(ctx, application.ResearchModeAuto, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: int64(len(body)), BodyPolicy: application.SnapshotBoundedCachedBody,
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.CacheCandidate) != string(body) || result.NormalizationInput != nil {
		t.Fatalf("cache candidate disposition = %+v", result)
	}
	result.CacheCandidate[0] = 'X'
	if string(body) != "bounded fixture" {
		t.Fatal("capture result aliases fetched body")
	}
	stored, err := repositories.Snapshots.Get(ctx, result.Snapshot.ID)
	if err != nil || stored.Fetch.ContentLength != int64(len(body)) {
		t.Fatalf("stored snapshot = (%+v, %v)", stored, err)
	}
}

func TestSnapshotCaptureRejectsInvalidRetentionAndOrphan304(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	source := testSource(t, "invalid-capture")
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	service := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots,
		&recordingCaptureFetchService{results: []application.FetchedSource{
			fetchedFixture(t, source, 10, http.StatusNotModified, `"orphan"`, nil),
		}}, application.WithSnapshotIDGenerator(func() (research.ID, error) { return research.NewID("snapshot.orphan") }))

	_, err := service.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: 4096, BodyPolicy: application.SnapshotBoundedCachedBody,
	})
	if !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("orphan 304 error = %v, want invalid state", err)
	}
	_, err = service.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: application.MaximumCachedSourceBodyBytes + 1,
		BodyPolicy: application.SnapshotBoundedCachedBody,
	})
	if !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("oversize cached body policy error = %v, want invalid state", err)
	}

	cached := fetchedFixture(t, source, 11, http.StatusOK, `"cached"`, []byte("cached"))
	cached.Origin = application.FetchOriginCache
	cachedService := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots,
		&recordingCaptureFetchService{results: []application.FetchedSource{cached}})
	_, err = cachedService.Capture(ctx, application.ResearchModeOffline, application.SnapshotCaptureRequest{
		SourceID: source.ID, MaximumBytes: 4096, BodyPolicy: application.SnapshotMetadataOnly,
	})
	if !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("cached capture error = %v, want invalid state", err)
	}
}

type recordingCaptureFetchService struct {
	requests []application.FetchRequest
	modes    []application.ResearchMode
	results  []application.FetchedSource
	err      error
}

func (service *recordingCaptureFetchService) Fetch(_ context.Context, mode application.ResearchMode, request application.FetchRequest) (application.FetchedSource, error) {
	service.modes = append(service.modes, mode)
	service.requests = append(service.requests, request)
	if service.err != nil {
		return application.FetchedSource{}, service.err
	}
	result := service.results[0]
	service.results = service.results[1:]
	result.Body = append([]byte(nil), result.Body...)
	return result, nil
}

func fetchedFixture(t *testing.T, source research.Source, hour, status int, etag string, body []byte) application.FetchedSource {
	t.Helper()
	metadata := research.FetchMetadata{
		StatusCode: status, ETag: etag, ContentLength: int64(len(body)), FetchVersion: "source-fetch-v1",
	}
	if status != http.StatusNotModified && status != http.StatusNoContent {
		metadata.ContentType = "text/plain"
		metadata.ContentHash = research.CanonicalContentHashV1(body)
	}
	return application.FetchedSource{
		SourceID: source.ID, Locator: source.Locator, FetchedAt: testTimestamp(t, hour),
		Metadata: metadata, Body: append([]byte(nil), body...), Origin: application.FetchOriginLive,
	}
}
