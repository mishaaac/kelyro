package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"github.com/mishaaac/kelyro/internal/research"
)

type SnapshotCaptureOption func(*snapshotCaptureService)

func WithSnapshotIDGenerator(generate func() (research.ID, error)) SnapshotCaptureOption {
	return func(service *snapshotCaptureService) {
		if generate != nil {
			service.generateID = generate
		}
	}
}

type snapshotCaptureService struct {
	sources    SourceRepository
	snapshots  SnapshotRepository
	fetch      FetchService
	generateID func() (research.ID, error)
}

func NewSnapshotCaptureService(sources SourceRepository, snapshots SnapshotRepository, fetch FetchService, options ...SnapshotCaptureOption) SnapshotCaptureService {
	service := &snapshotCaptureService{
		sources: sources, snapshots: snapshots, fetch: fetch, generateID: randomSnapshotID,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *snapshotCaptureService) Capture(ctx context.Context, mode ResearchMode, request SnapshotCaptureRequest) (SnapshotCapture, error) {
	const operation = "capture source snapshot"
	if err := request.Validate(); err != nil {
		return SnapshotCapture{}, invalid(operation, err)
	}
	if err := mode.Validate(); err != nil {
		return SnapshotCapture{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "source repository", service.sources); err != nil {
		return SnapshotCapture{}, err
	}
	if err := requireDependency(operation, "snapshot repository", service.snapshots); err != nil {
		return SnapshotCapture{}, err
	}
	if err := requireDependency(operation, "privacy-gated fetch service", service.fetch); err != nil {
		return SnapshotCapture{}, err
	}
	source, err := service.sources.Get(ctx, request.SourceID)
	if err != nil {
		return SnapshotCapture{}, repositoryError(operation, err)
	}
	previous, hasPrevious, err := service.latest(ctx, request.SourceID)
	if err != nil {
		return SnapshotCapture{}, err
	}
	fetchRequest := FetchRequest{
		SourceID: request.SourceID, Locator: source.Locator, MaximumBytes: request.MaximumBytes,
	}
	if hasPrevious {
		fetchRequest.ETag = previous.Fetch.ETag
		fetchRequest.LastModified = previous.Fetch.LastModified
	}
	fetched, err := service.fetch.Fetch(ctx, mode, fetchRequest)
	if err != nil {
		return SnapshotCapture{}, err
	}
	if err := fetched.Validate(); err != nil {
		return SnapshotCapture{}, invalid(operation, fmt.Errorf("fetched source: %w", err))
	}
	if fetched.Origin != FetchOriginLive {
		return SnapshotCapture{}, invalid(operation, fmt.Errorf("cached source retrieval is not a new fetch observation"))
	}
	if fetched.SourceID != request.SourceID {
		return SnapshotCapture{}, invalid(operation, fmt.Errorf("fetched source identity does not match request"))
	}
	metadata, revalidated, err := snapshotMetadata(fetched, previous, hasPrevious)
	if err != nil {
		return SnapshotCapture{}, invalid(operation, err)
	}
	id, err := service.generateID()
	if err != nil {
		return SnapshotCapture{}, Classify(ErrorUnavailable, operation, fmt.Errorf("generate snapshot id: %w", err))
	}
	snapshot := research.SourceSnapshot{
		ID: id, SourceID: fetched.SourceID, Locator: fetched.Locator,
		FetchedAt: fetched.FetchedAt, Fetch: metadata,
	}
	if err := snapshot.Validate(); err != nil {
		return SnapshotCapture{}, invalid(operation, err)
	}
	if err := service.snapshots.Append(ctx, snapshot); err != nil {
		return SnapshotCapture{}, repositoryError(operation, err)
	}
	result := SnapshotCapture{Snapshot: snapshot}
	if revalidated {
		previousID := previous.ID
		result.RevalidatedSnapshotID = &previousID
	}
	switch request.BodyPolicy {
	case SnapshotNormalizedExcerpt:
		if len(fetched.Body) > 0 {
			normalizationInput := fetched
			normalizationInput.Body = append([]byte(nil), fetched.Body...)
			result.NormalizationInput = &normalizationInput
		}
	case SnapshotBoundedCachedBody:
		if fetched.NoStore {
			result.CacheSuppressed = true
		} else {
			result.CacheCandidate = append([]byte(nil), fetched.Body...)
		}
	}
	return result, nil
}

func (service *snapshotCaptureService) latest(ctx context.Context, sourceID research.SourceID) (research.SourceSnapshot, bool, error) {
	const operation = "capture source snapshot"
	previous, err := service.snapshots.LatestBySource(ctx, sourceID)
	if err == nil {
		return previous, true, nil
	}
	if errors.Is(err, ErrNotFound) {
		return research.SourceSnapshot{}, false, nil
	}
	return research.SourceSnapshot{}, false, repositoryError(operation, err)
}

func snapshotMetadata(fetched FetchedSource, previous research.SourceSnapshot, hasPrevious bool) (research.FetchMetadata, bool, error) {
	metadata := fetched.Metadata
	if metadata.StatusCode == http.StatusNotModified {
		if !hasPrevious {
			return research.FetchMetadata{}, false, fmt.Errorf("304 response has no previous snapshot")
		}
		if len(fetched.Body) != 0 {
			return research.FetchMetadata{}, false, fmt.Errorf("304 response unexpectedly contains a body")
		}
		if previous.Fetch.ETag == "" && previous.Fetch.LastModified == "" {
			return research.FetchMetadata{}, false, fmt.Errorf("304 response has no previous conditional validator")
		}
		if previous.Fetch.ContentHash == "" {
			return research.FetchMetadata{}, false, fmt.Errorf("304 response references a snapshot without content hash")
		}
		if err := research.ValidateCanonicalContentHashV1(previous.Fetch.ContentHash); err != nil {
			return research.FetchMetadata{}, false, fmt.Errorf("304 previous snapshot: %w", err)
		}
		if fetched.FetchedAt.Before(previous.FetchedAt) {
			return research.FetchMetadata{}, false, fmt.Errorf("304 response predates the previous snapshot")
		}
		metadata.ContentType = previous.Fetch.ContentType
		metadata.ContentHash = previous.Fetch.ContentHash
		metadata.ContentLength = previous.Fetch.ContentLength
		metadata.ETag = firstText(metadata.ETag, previous.Fetch.ETag)
		metadata.LastModified = firstText(metadata.LastModified, previous.Fetch.LastModified)
		return metadata, true, nil
	}
	if metadata.StatusCode == http.StatusNoContent && len(fetched.Body) != 0 {
		return research.FetchMetadata{}, false, fmt.Errorf("204 response unexpectedly contains a body")
	}
	if metadata.StatusCode != http.StatusNoContent {
		if int64(len(fetched.Body)) != metadata.ContentLength {
			return research.FetchMetadata{}, false, fmt.Errorf("fetched content length does not match body")
		}
		wantHash := research.CanonicalContentHashV1(fetched.Body)
		if metadata.ContentHash != wantHash {
			return research.FetchMetadata{}, false, fmt.Errorf("fetched content hash is not canonical")
		}
	}
	return metadata, false, nil
}

func randomSnapshotID() (research.ID, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return research.ID{}, err
	}
	return research.NewID("snapshot." + hex.EncodeToString(value))
}

func firstText(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

var _ SnapshotCaptureService = (*snapshotCaptureService)(nil)
