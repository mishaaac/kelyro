package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func TestServiceAssemblesProductionResearchFetchBehindResolvedPrivacy(t *testing.T) {
	t.Parallel()
	configs := &recordingConfigStore{project: config.Settings{config.KeyAllowNetwork: config.BoolValue(true)}}
	fetcher := &recordingProductionSourceFetcher{at: time.Date(2026, 8, 31, 15, 0, 0, 0, time.UTC)}
	service := NewService(nil, nil).WithConfig(configs).WithResearchFetcher(fetcher)
	stage, err := service.researchFetchForRun(context.Background(), Command{}, "/workspace")
	if err != nil {
		t.Fatal(err)
	}
	source := appResearchFetchSource(t, "source.app-fetch")
	artifacts, err := stage.Execute(context.Background(), researchapp.LiveResearchStageInput{
		Mode: researchapp.ResearchModeOnline, Artifacts: researchapp.LiveResearchArtifacts{Sources: []research.Source{source}},
	})
	if err != nil || len(artifacts.FetchedSources) != 1 || artifacts.FetchedSources[0].SourceID != source.ID || fetcher.calls != 1 {
		t.Fatalf("production fetch stage = (%+v,%v), calls=%d", artifacts, err, fetcher.calls)
	}
}

func TestServiceResearchFetchPrivacyDenialNeverReachesAdapter(t *testing.T) {
	t.Parallel()
	configs := &recordingConfigStore{project: config.Settings{config.KeyAllowNetwork: config.BoolValue(false)}}
	fetcher := &recordingProductionSourceFetcher{at: time.Date(2026, 8, 31, 15, 0, 0, 0, time.UTC)}
	service := NewService(nil, nil).WithConfig(configs).WithResearchFetcher(fetcher)
	stage, err := service.researchFetchForRun(context.Background(), Command{}, "/workspace")
	if err != nil {
		t.Fatal(err)
	}
	source := appResearchFetchSource(t, "source.app-fetch-blocked")
	artifacts, err := stage.Execute(context.Background(), researchapp.LiveResearchStageInput{
		Mode: researchapp.ResearchModeOnline, Artifacts: researchapp.LiveResearchArtifacts{Sources: []research.Source{source}},
	})
	if !errors.Is(err, researchapp.ErrNetworkResearchBlocked) || !errors.Is(err, privacy.ErrNetworkBlocked) ||
		len(artifacts.FetchFailures) != 1 || fetcher.calls != 0 {
		t.Fatalf("blocked production fetch = (%+v,%v), calls=%d", artifacts, err, fetcher.calls)
	}
}

type recordingProductionSourceFetcher struct {
	at    time.Time
	calls int
}

func (fetcher *recordingProductionSourceFetcher) Fetch(_ context.Context, request researchapp.FetchRequest) (researchapp.FetchedSource, error) {
	fetcher.calls++
	body := []byte("fixture")
	at, _ := research.NewTimestamp(fetcher.at)
	return researchapp.FetchedSource{
		SourceID: request.SourceID, Locator: request.Locator, FetchedAt: at, Body: body, Origin: researchapp.FetchOriginLive,
		Metadata: research.FetchMetadata{StatusCode: 200, ContentType: "text/plain", ContentHash: research.CanonicalContentHashV1(body),
			ContentLength: int64(len(body)), FetchVersion: "fixture-fetch-v1"},
	}, nil
}

func appResearchFetchSource(t *testing.T, idValue string) research.Source {
	t.Helper()
	id, err := research.NewSourceID(idValue)
	if err != nil {
		t.Fatal(err)
	}
	locator, err := research.NewSourceLocator("https://docs.example.test/" + idValue)
	if err != nil {
		t.Fatal(err)
	}
	at, err := research.NewTimestamp(time.Date(2026, 8, 31, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return research.Source{ID: id, Kind: research.SourceOther, Locator: locator, TemporalScope: research.SourceTemporalCurrent,
		Metadata: research.SourceMetadata{Title: "Fixture"}, CreatedAt: at}
}

var _ researchapp.SourceFetcher = (*recordingProductionSourceFetcher)(nil)
