package application_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/mishaaac/kelyro/internal/infra/researchnormalize"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestLiveSourceNormalizationReusesAllExistingFormatNormalizers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{name: "html", contentType: "text/html", body: `<html><body><h1>Guide</h1><p>HTML body.</p></body></html>`},
		{name: "markdown", contentType: "text/markdown", body: "# Guide\n\nMarkdown body."},
		{name: "json", contentType: "application/json", body: `{"title":"Guide","summary":"JSON body."}`},
		{name: "text", contentType: "text/plain", body: "Plain text body."},
	}
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	capture := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, nil)
	inputs := make([]application.FetchedSource, 0, len(tests))
	snapshots := make([]research.SourceSnapshot, 0, len(tests))
	for index, test := range tests {
		source := testSource(t, "live-normalization-"+test.name)
		if err := repositories.Sources.Create(ctx, source); err != nil {
			t.Fatal(err)
		}
		fetched := fetchedFixture(t, source, 20+index, http.StatusOK, `"normalization"`, []byte(test.body))
		fetched.Metadata.ContentType = test.contentType
		captured, err := capture.CaptureFetched(ctx, fetched, application.SnapshotCaptureRequest{
			SourceID: source.ID, MaximumBytes: 4096, BodyPolicy: application.SnapshotNormalizedExcerpt,
		})
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, *captured.NormalizationInput)
		snapshots = append(snapshots, captured.Snapshot)
	}
	service, err := application.NewLiveSourceNormalizationService(researchnormalize.New())
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.NormalizeSources(ctx, application.LiveSourceNormalizationRequest{Inputs: inputs, Snapshots: snapshots})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Sources) != len(tests) || len(result.Failures) != 0 || result.AlgorithmVersion != application.LiveSourceNormalizationV1 {
		t.Fatalf("normalization result = %+v", result)
	}
	for index, normalized := range result.Sources {
		if normalized.SourceID != inputs[index].SourceID || normalized.Locator != inputs[index].Locator ||
			normalized.NormalizationVersion != researchnormalize.Version || len(normalized.TextSegments) == 0 {
			t.Fatalf("normalized source %d = %+v", index, normalized)
		}
	}
}

func TestLiveSourceNormalizationAllowsDocumentFailureButRequiresOneSuccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	capture := application.NewSnapshotCaptureService(repositories.Sources, repositories.Snapshots, nil)
	goodSource := testSource(t, "live-normalization-good")
	badSource := testSource(t, "live-normalization-pdf")
	var inputs []application.FetchedSource
	var snapshots []research.SourceSnapshot
	for index, fixture := range []struct {
		source      research.Source
		contentType string
		body        []byte
	}{{goodSource, "text/plain", []byte("usable")}, {badSource, "application/pdf", []byte("%PDF-1.7")}} {
		if err := repositories.Sources.Create(ctx, fixture.source); err != nil {
			t.Fatal(err)
		}
		fetched := fetchedFixture(t, fixture.source, 30+index, http.StatusOK, `"normalization"`, fixture.body)
		fetched.Metadata.ContentType = fixture.contentType
		captured, err := capture.CaptureFetched(ctx, fetched, application.SnapshotCaptureRequest{
			SourceID: fixture.source.ID, MaximumBytes: 4096, BodyPolicy: application.SnapshotNormalizedExcerpt,
		})
		if err != nil {
			t.Fatal(err)
		}
		inputs = append(inputs, *captured.NormalizationInput)
		snapshots = append(snapshots, captured.Snapshot)
	}
	service, _ := application.NewLiveSourceNormalizationService(researchnormalize.New())
	result, err := service.NormalizeSources(ctx, application.LiveSourceNormalizationRequest{Inputs: inputs, Snapshots: snapshots})
	if err != nil || len(result.Sources) != 1 || result.Sources[0].SourceID != goodSource.ID ||
		len(result.Failures) != 1 || result.Failures[0].SourceID != badSource.ID || result.Failures[0].Kind != application.ErrorExternalFailure {
		t.Fatalf("partial normalization = (%+v,%v)", result, err)
	}

	allFailed, err := service.NormalizeSources(ctx, application.LiveSourceNormalizationRequest{
		Inputs: inputs[1:], Snapshots: snapshots[1:],
	})
	if !errors.Is(err, application.ErrExternalFailure) || len(allFailed.Sources) != 0 || len(allFailed.Failures) != 1 {
		t.Fatalf("all-failed normalization = (%+v,%v)", allFailed, err)
	}
}

func TestLiveSourceNormalizationRejectsInputWithoutMatchingDurableSnapshot(t *testing.T) {
	t.Parallel()
	source := testSource(t, "live-normalization-orphan")
	input := fetchedFixture(t, source, 40, http.StatusOK, `"normalization"`, []byte("orphan"))
	normalizer := &recordingSourceNormalizer{}
	service, _ := application.NewLiveSourceNormalizationService(normalizer)
	_, err := service.NormalizeSources(context.Background(), application.LiveSourceNormalizationRequest{Inputs: []application.FetchedSource{input}})
	if !errors.Is(err, application.ErrInvalidState) || normalizer.calls != 0 {
		t.Fatalf("orphan normalization = (%v), calls=%d", err, normalizer.calls)
	}
}

type recordingSourceNormalizer struct {
	calls int
}

func (normalizer *recordingSourceNormalizer) Normalize(context.Context, application.FetchedSource) (application.NormalizedSource, error) {
	normalizer.calls++
	return application.NormalizedSource{}, errors.New("unexpected normalize call")
}

var _ application.SourceNormalizer = (*recordingSourceNormalizer)(nil)
