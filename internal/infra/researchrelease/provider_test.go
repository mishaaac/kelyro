package researchrelease_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/researchrelease"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestJSONProviderSupportsRepositoryAndRegistryShapes(t *testing.T) {
	provider, err := researchrelease.NewJSONProvider("official-registry")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"items":[{"tag_name":"v2.1.0","published_at":"2026-08-25T10:00:00Z","body":"Stable API."},{"version":"2.2.0-beta.1","prerelease":true,"notes":["Beta API."]},{"version":"9.9.9","draft":true}]}`
	releases, err := provider.Discover(context.Background(), fetched(t, "application/json", body))
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 || releases[0].Version.String() != "2.1.0" || releases[0].Channel != research.ReleaseStable || releases[1].Channel != research.ReleasePreview {
		t.Fatalf("JSON releases = %+v", releases)
	}
	if len(releases[0].Changes) != 1 || releases[0].Changes[0].Excerpt != "Stable API." {
		t.Fatalf("JSON notes = %+v", releases[0].Changes)
	}
}

func TestAtomProviderExtractsStableAndRCReleaseNotes(t *testing.T) {
	provider, err := researchrelease.NewAtomProvider("official-atom")
	if err != nil {
		t.Fatal(err)
	}
	body := `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><entry><title>Runtime v3.0.0</title><published>2026-08-24T10:00:00Z</published><category term="stable"/><content>Stable API.</content></entry><entry><title>Runtime v3.1.0-rc.1</title><updated>2026-08-25T10:00:00Z</updated><category term="rc"/><summary>Candidate API.</summary></entry></feed>`
	releases, err := provider.Discover(context.Background(), fetched(t, "application/atom+xml", body))
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 2 || releases[0].Channel != research.ReleaseStable || releases[1].Channel != research.ReleaseRC {
		t.Fatalf("Atom releases = %+v", releases)
	}
	if len(releases[1].Changes) != 1 || releases[1].Changes[0].Statement != "Candidate API." {
		t.Fatalf("Atom notes = %+v", releases[1].Changes)
	}
}

func TestProvidersRejectMalformedFeeds(t *testing.T) {
	jsonProvider, _ := researchrelease.NewJSONProvider("json")
	if _, err := jsonProvider.Discover(context.Background(), fetched(t, "application/json", `{"releases":`)); err == nil {
		t.Fatal("JSON provider accepted malformed feed")
	}
	atomProvider, _ := researchrelease.NewAtomProvider("atom")
	if _, err := atomProvider.Discover(context.Background(), fetched(t, "application/atom+xml", `<feed><entry></feed>`)); err == nil {
		t.Fatal("Atom provider accepted malformed feed")
	}
}

func fetched(t *testing.T, contentType, body string) application.FetchedSource {
	t.Helper()
	sourceID, err := research.NewSourceID("source.release-feed")
	if err != nil {
		t.Fatal(err)
	}
	locator, err := research.NewSourceLocator("https://releases.example.test/feed")
	if err != nil {
		t.Fatal(err)
	}
	at, err := research.NewTimestamp(time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	bytes := []byte(body)
	return application.FetchedSource{
		SourceID: sourceID, Locator: locator, FetchedAt: at, Origin: application.FetchOriginLive,
		Metadata: research.FetchMetadata{StatusCode: http.StatusOK, ContentType: contentType, ContentLength: int64(len(bytes)), ContentHash: research.CanonicalContentHashV1(bytes), FetchVersion: "source-fetch-v1"},
		Body:     bytes,
	}
}
