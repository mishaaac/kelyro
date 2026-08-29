package live_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/researchfetch"
	"github.com/mishaaac/kelyro/internal/infra/researchhttp"
	"github.com/mishaaac/kelyro/internal/infra/researchnormalize"
	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
	"github.com/mishaaac/kelyro/internal/research/trust"
)

const liveResearchEnvironment = "KELYRO_LIVE_RESEARCH_TESTS"

func TestLiveResearchSourceAdapters(t *testing.T) {
	if os.Getenv(liveResearchEnvironment) != "1" {
		t.Skipf("set %s=1 to run controlled public-web checks", liveResearchEnvironment)
	}

	config := researchhttp.DefaultConfig()
	config.UserAgent = "Kelyro/live-research-test"
	config.RequestTimeout = 8 * time.Second
	config.DialTimeout = 4 * time.Second
	config.TLSHandshakeTimeout = 4 * time.Second
	config.ResponseHeaderTimeout = 5 * time.Second
	config.MaxResponseBytes = 2 << 20
	config.MaxAttempts = 1
	config.MaxRedirects = 3
	config.MinimumIntervalPerHost = 10 * time.Millisecond
	client, err := researchhttp.New(config, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.CloseIdleConnections)

	clock := liveClock{}
	countingFetcher := &countingSourceFetcher{delegate: researchfetch.New(client)}
	allow := application.NetworkResearchAccess{
		Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: true}, nil),
	}
	deny := application.NetworkResearchAccess{
		Gate: privacy.NewNetworkGate(privacy.Policy{}, nil),
	}

	cases := []struct {
		name      string
		locator   string
		publisher string
		kind      research.SourceKind
		topic     research.ResearchTopic
		useCase   trust.UseCase
		wantTier  research.AuthorityTier
	}{
		{
			name: "How to Write Go Code", locator: "https://go.dev/doc/code",
			publisher: "The Go Authors", kind: research.SourceOfficialDocumentation,
			topic:   mustTopic(t, "Organizing Go code", "software", "Go"),
			useCase: trust.UseCaseGeneral, wantTier: research.AuthorityTierB,
		},
		{
			name: "RFC 2606 reserved DNS names", locator: "https://www.rfc-editor.org/rfc/rfc2606.txt",
			publisher: "RFC Editor", kind: research.SourceStandard,
			topic:   mustTopic(t, "Reserved DNS names", "networking", "DNS"),
			useCase: trust.UseCaseGeneral, wantTier: research.AuthorityTierA,
		},
	}

	for index, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			now := clock.Now()
			locator := mustLocator(t, test.locator)
			source := research.Source{
				ID:   mustSourceID(t, fmt.Sprintf("source.live.%d", index+1)),
				Kind: test.kind, Locator: locator, TemporalScope: research.SourceTemporalCurrent,
				Metadata: research.SourceMetadata{
					Title: test.name, Publisher: test.publisher, Language: "en",
				},
				CreatedAt: now,
			}
			decision, err := (trust.PolicyV1{}).Evaluate(trust.Input{
				Source: source, Topic: test.topic, Purpose: research.PurposeConceptDefinition,
				UseCase: test.useCase, Freshness: research.FreshnessFresh,
				Relevance: trust.RelevanceExact, Directness: trust.DirectnessPrimary,
				Stability: trust.StabilityStable, Corroboration: trust.CorroborationSingleSource,
				EvaluatedAt: now,
			})
			if err != nil {
				t.Fatal(err)
			}
			if decision.State != research.TrustAccepted || decision.Tier != test.wantTier || decision.Policy != trust.PolicyVersionV1 {
				t.Fatalf("classification = (%s, %s, %s), want (accepted, %s, %s)",
					decision.State, decision.Tier, decision.Policy, test.wantTier, trust.PolicyVersionV1)
			}

			store := memory.New()
			repositories := store.Repositories()
			if err := repositories.Sources.Create(ctx, source); err != nil {
				t.Fatal(err)
			}
			sequence := index + 1
			captureService := application.NewSnapshotCaptureService(
				repositories.Sources,
				repositories.Snapshots,
				application.NewFetchService(countingFetcher, nil, allow),
				application.WithSnapshotIDGenerator(func() (research.ID, error) {
					return research.NewID(fmt.Sprintf("snapshot.live.%d", sequence))
				}),
			)
			capture, err := captureService.Capture(ctx, application.ResearchModeOnline, application.SnapshotCaptureRequest{
				SourceID: source.ID, MaximumBytes: config.MaxResponseBytes,
				BodyPolicy: application.SnapshotNormalizedExcerpt,
			})
			if err != nil {
				t.Fatalf("live source is not reachable through adapters: %v", err)
			}
			if capture.NormalizationInput == nil {
				t.Fatal("successful live snapshot has no transient normalization input")
			}
			snapshot := capture.Snapshot
			if snapshot.Fetch.StatusCode < http.StatusOK || snapshot.Fetch.StatusCode >= http.StatusMultipleChoices ||
				snapshot.Fetch.ContentType == "" || snapshot.Fetch.ContentLength <= 0 {
				t.Fatalf("live snapshot metadata = %+v", snapshot.Fetch)
			}
			if err := research.ValidateCanonicalContentHashV1(snapshot.Fetch.ContentHash); err != nil ||
				snapshot.Fetch.ContentHash != research.CanonicalContentHashV1(capture.NormalizationInput.Body) {
				t.Fatalf("live snapshot hash = %q: %v", snapshot.Fetch.ContentHash, err)
			}
			stored, err := repositories.Snapshots.LatestBySource(ctx, source.ID)
			if err != nil || stored.ID != snapshot.ID || stored.Fetch.ContentHash != snapshot.Fetch.ContentHash {
				t.Fatalf("persisted snapshot = (%+v, %v)", stored, err)
			}

			normalized, err := researchnormalize.New().Normalize(ctx, *capture.NormalizationInput)
			if err != nil {
				t.Fatalf("normalize live source: %v", err)
			}
			htmlWithoutStructure := strings.Contains(normalized.ContentType, "html") &&
				normalized.Title == "" && len(normalized.Headings) == 0
			if normalized.ContentType == "" || htmlWithoutStructure ||
				(len(normalized.TextSegments) == 0 && len(normalized.CodeBlocks) == 0) {
				t.Fatalf("normalized metadata/content is incomplete: title=%q headings=%d segments=%d code=%d type=%q",
					normalized.Title, len(normalized.Headings), len(normalized.TextSegments), len(normalized.CodeBlocks), normalized.ContentType)
			}

			callsBeforePrivacyCheck := countingFetcher.calls
			blocked := application.NewFetchService(countingFetcher, nil, deny)
			_, err = blocked.Fetch(ctx, application.ResearchModeOnline, application.FetchRequest{
				SourceID: source.ID, Locator: source.Locator, MaximumBytes: config.MaxResponseBytes,
			})
			if !errors.Is(err, application.ErrNetworkResearchBlocked) || countingFetcher.calls != callsBeforePrivacyCheck {
				t.Fatalf("privacy gate = %v, live calls %d -> %d", err, callsBeforePrivacyCheck, countingFetcher.calls)
			}
		})
	}
}

type countingSourceFetcher struct {
	delegate application.SourceFetcher
	calls    int
}

func (fetcher *countingSourceFetcher) Fetch(ctx context.Context, request application.FetchRequest) (application.FetchedSource, error) {
	fetcher.calls++
	return fetcher.delegate.Fetch(ctx, request)
}

type liveClock struct{}

func (liveClock) Now() research.Timestamp {
	timestamp, err := research.NewTimestamp(time.Now())
	if err != nil {
		panic(err)
	}
	return timestamp
}

func mustTopic(t *testing.T, subject, domain, technology string) research.ResearchTopic {
	t.Helper()
	topic, err := research.NewResearchTopic(subject, domain, technology)
	if err != nil {
		t.Fatal(err)
	}
	return topic
}

func mustLocator(t *testing.T, value string) research.SourceLocator {
	t.Helper()
	locator, err := research.NewSourceLocator(value)
	if err != nil {
		t.Fatal(err)
	}
	return locator
}

func mustSourceID(t *testing.T, value string) research.SourceID {
	t.Helper()
	id, err := research.NewSourceID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
