package researchsearch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/privacy"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestBraveSearchIsBlockedBeforeProviderRequestWhenNetworkIsDisabled(t *testing.T) {
	t.Parallel()

	requests := 0
	provider, err := NewBrave(roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("HTTP must not be reached")
	}), "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewDiscoveryService(provider, nil, application.NetworkResearchAccess{
		Gate: privacy.NewNetworkGate(privacy.Policy{AllowNetwork: false}, nil),
	})

	_, err = service.Search(context.Background(), application.ResearchModeOnline, validQuery(t), application.SearchOptions{Limit: 1})
	if !errors.Is(err, application.ErrNetworkDisabled) ||
		!errors.Is(err, application.ErrNetworkResearchBlocked) ||
		!errors.Is(err, privacy.ErrNetworkBlocked) {
		t.Fatalf("Search() error = %v, want network_disabled privacy denial", err)
	}
	if requests != 0 {
		t.Fatalf("provider HTTP requests = %d, want zero", requests)
	}
}

func TestBraveSearchMapsAndPaginatesBoundedResults(t *testing.T) {
	t.Parallel()
	client := &fixtureClient{responses: []*http.Response{
		fixtureResponse(t, "brave_page_1.json", http.Header{
			"X-Ratelimit-Limit":     {"50"},
			"X-Ratelimit-Policy":    {"50;w=1"},
			"X-Ratelimit-Remaining": {"49"},
			"X-Ratelimit-Reset":     {"1"},
		}),
		fixtureResponse(t, "brave_page_2.json", nil),
	}}
	observer := &recordingObserver{}
	provider, err := NewBrave(client, "fixture-token", WithPageObserver(observer))
	if err != nil {
		t.Fatal(err)
	}

	results, err := provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Search() returned %d results, want 3", len(results))
	}
	if results[0].Title != "The Go Programming Language Specification" ||
		results[0].Locator.String() != "https://go.dev/ref/spec" ||
		results[0].Snippet != "Interface types define type sets." ||
		results[0].Provider != ProviderID || results[0].Rank != 0 || results[0].PublishedHint == nil {
		t.Fatalf("first result not normalized and mapped: %+v", results[0])
	}
	if results[2].Rank != 3 || results[2].PublishedHint != nil {
		t.Fatalf("second page rank/hint = %d/%v, want 3/nil", results[2].Rank, results[2].PublishedHint)
	}

	requests := client.Requests()
	if len(requests) != 2 {
		t.Fatalf("HTTP calls = %d, want 2", len(requests))
	}
	assertRequest(t, requests[0], "Go interfaces", "3", "0")
	assertRequest(t, requests[1], "Go interfaces", "3", "1")
	if requests[0].Header.Get(credentialHeader) != "fixture-token" {
		t.Fatal("credential header was not set")
	}
	if requests[0].Header.Get("Accept") != "application/json" {
		t.Fatal("JSON accept header was not set")
	}

	events := observer.Events()
	if len(events) != 2 || events[0].RequestedCount != 3 || events[0].Offset != 0 ||
		events[0].RateLimit.Remaining != "49" || events[1].RequestedCount != 3 {
		t.Fatalf("page observations = %+v", events)
	}
}

func TestBraveSearchHonorsResultAndPaginationBounds(t *testing.T) {
	t.Parallel()
	client := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		count := request.URL.Query().Get("count")
		offset := request.URL.Query().Get("offset")
		if count != "20" {
			return nil, fmt.Errorf("page count = %s, want 20", count)
		}
		return jsonResponse(http.StatusOK, fmt.Sprintf(`{
			"query":{"more_results_available":true},
			"web":{"results":[{"title":"result %s","url":"https://example.test/%s"}]}
		}`, offset, offset), nil), nil
	})
	provider, err := NewBrave(client, "fixture-token")
	if err != nil {
		t.Fatal(err)
	}

	results, err := provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 100})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) != 10 {
		t.Fatalf("results = %d, want one per each of 10 bounded pages", len(results))
	}
	if results[9].Rank != 180 {
		t.Fatalf("last rank = %d, want 180", results[9].Rank)
	}
}

func TestBraveSearchAuthorizesEveryProviderCallBeforePagination(t *testing.T) {
	t.Parallel()

	client := &fixtureClient{responses: []*http.Response{
		fixtureResponse(t, "brave_page_1.json", nil),
		fixtureResponse(t, "brave_page_2.json", nil),
	}}
	provider, err := NewBrave(client, "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	authorizations := 0
	results, err := provider.SearchWithCostControl(
		context.Background(), validQuery(t), application.SearchOptions{Limit: 3},
		func(context.Context) error {
			authorizations++
			return nil
		},
	)
	if err != nil || len(results) != 3 || authorizations != 2 || len(client.Requests()) != 2 {
		t.Fatalf("SearchWithCostControl() = (%d results, %v), authorizations/requests = %d/%d", len(results), err, authorizations, len(client.Requests()))
	}

	client = &fixtureClient{responses: []*http.Response{
		fixtureResponse(t, "brave_page_1.json", nil),
		fixtureResponse(t, "brave_page_2.json", nil),
	}}
	provider, err = NewBrave(client, "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	want := application.Classify(application.ErrorBudgetExceeded, "authorize fixture call", errors.New("fixture budget"))
	authorizations = 0
	_, err = provider.SearchWithCostControl(
		context.Background(), validQuery(t), application.SearchOptions{Limit: 3},
		func(context.Context) error {
			authorizations++
			if authorizations == 2 {
				return want
			}
			return nil
		},
	)
	if !errors.Is(err, application.ErrBudgetExceeded) || authorizations != 2 || len(client.Requests()) != 1 {
		t.Fatalf("blocked pagination = (%v), authorizations/requests = %d/%d", err, authorizations, len(client.Requests()))
	}
}

func TestBraveSearchMapsProviderStatusWithoutLeakingPayload(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrAuthentication},
		{http.StatusForbidden, ErrAuthentication},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusBadRequest, ErrInvalidRequest},
		{http.StatusServiceUnavailable, ErrUnavailable},
		{http.StatusTeapot, ErrResponse},
	}
	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			t.Parallel()
			client := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(test.status, `{"secret":"fixture-token","query":"Go interfaces"}`, http.Header{
					"X-Ratelimit-Remaining": {"0"},
				}), nil
			})
			provider, err := NewBrave(client, "fixture-token")
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 1})
			if !errors.Is(err, test.want) {
				t.Fatalf("Search() error = %v, want %v", err, test.want)
			}
			if strings.Contains(err.Error(), "fixture-token") || strings.Contains(err.Error(), "Go interfaces") {
				t.Fatalf("provider error leaked sensitive data: %v", err)
			}
			var providerErr *Error
			if !errors.As(err, &providerErr) || providerErr.RateLimit.Remaining != "0" {
				t.Fatalf("rate metadata missing from error: %+v", providerErr)
			}
		})
	}
}

func TestBraveSearchRejectsMalformedOrOversizeResponses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{`},
		{name: "trailing JSON", body: `{"web":{"results":[]}} {}`},
		{name: "invalid result", body: `{"web":{"results":[{"title":"","url":"https://example.test"}]}}`},
		{name: "too many results", body: `{"web":{"results":[{"title":"one","url":"https://example.test/1"},{"title":"two","url":"https://example.test/2"}]}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			provider, err := NewBrave(staticClient(jsonResponse(http.StatusOK, test.body, nil)), "fixture-token")
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 1})
			if !errors.Is(err, ErrResponse) {
				t.Fatalf("Search() error = %v, want response error", err)
			}
		})
	}

	oversize := strings.Repeat("x", maximumResponseBytes+1)
	provider, err := NewBrave(staticClient(jsonResponse(http.StatusOK, oversize, nil)), "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 1})
	if !errors.Is(err, ErrResponse) {
		t.Fatalf("oversize response error = %v, want response error", err)
	}
}

func TestBraveSearchValidatesInputsAndCancellation(t *testing.T) {
	t.Parallel()
	if _, err := NewBrave(nil, "fixture-token"); err == nil {
		t.Fatal("NewBrave accepted nil client")
	}
	if _, err := NewBrave(staticClient(nil), ""); err == nil {
		t.Fatal("NewBrave accepted empty credential")
	}

	calls := 0
	client := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return jsonResponse(http.StatusOK, `{"web":{"results":[]}}`, nil), nil
	})
	provider, err := NewBrave(client, "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	query := validQuery(t)
	query.Text = strings.Repeat("a", maximumQueryRunes+1)
	if _, err := provider.Search(context.Background(), query, application.SearchOptions{Limit: 1}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("oversize query error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("invalid query performed %d calls", calls)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := provider.Search(ctx, validQuery(t), application.SearchOptions{Limit: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled search error = %v", err)
	}
}

func TestBraveSearchProviderHTTPResponseMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		body       string
		want       error
		wantResult bool
	}{
		{
			name:       "success",
			status:     http.StatusOK,
			body:       `{"web":{"results":[{"title":" Go  specification ","url":"https://go.dev/ref/spec#Interface_types","description":" Interface  types. "}]}}`,
			wantResult: true,
		},
		{name: "empty", status: http.StatusOK, body: `{"web":{"results":[]}}`},
		{name: "malformed", status: http.StatusOK, body: `{`, want: ErrResponse},
		{name: "401", status: http.StatusUnauthorized, body: `{"error":"unauthorized"}`, want: ErrAuthentication},
		{name: "403", status: http.StatusForbidden, body: `{"error":"forbidden"}`, want: ErrAuthentication},
		{name: "429", status: http.StatusTooManyRequests, body: `{"error":"rate limited"}`, want: ErrRateLimited},
		{name: "500", status: http.StatusInternalServerError, body: `{"error":"unavailable"}`, want: ErrUnavailable},
		{name: "oversized response", status: http.StatusOK, body: strings.Repeat("x", maximumResponseBytes+1), want: ErrResponse},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			provider := newHTTPFixtureBrave(t, DefaultTransportConfig(), http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || request.URL.Path != "/res/v1/web/search" ||
					request.URL.Query().Get("q") != "Go interfaces" || request.URL.Query().Get("count") != "1" ||
					request.URL.Query().Get("offset") != "0" {
					t.Errorf("unexpected provider request: %s %s", request.Method, request.URL)
				}
				if request.Header.Get(credentialHeader) != "fixture-token" ||
					request.Header.Get("Accept") != "application/json" ||
					request.Header.Get("User-Agent") != "Kelyro/dev" ||
					request.Header.Get("Accept-Encoding") != "identity" {
					t.Errorf("unexpected provider request headers: %v", request.Header)
				}
				writer.Header().Set("Content-Type", "application/json")
				if test.status == http.StatusTooManyRequests {
					writer.Header().Set("X-RateLimit-Remaining", "0")
				}
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, test.body)
			}))

			results, err := provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 1})
			if test.want != nil {
				if !errors.Is(err, test.want) {
					t.Fatalf("Search() error = %v, want %v", err, test.want)
				}
				if test.status == http.StatusTooManyRequests {
					var providerErr *Error
					if !errors.As(err, &providerErr) || providerErr.RateLimit.Remaining != "0" {
						t.Fatalf("rate-limit metadata = %+v", providerErr)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if !test.wantResult {
				if len(results) != 0 {
					t.Fatalf("Search() results = %+v, want empty", results)
				}
				return
			}
			if len(results) != 1 || results[0].Title != "Go specification" ||
				results[0].Locator.String() != "https://go.dev/ref/spec" ||
				results[0].Snippet != "Interface types." || results[0].Provider != ProviderID || results[0].Rank != 0 {
				t.Fatalf("Search() result = %+v", results)
			}
		})
	}
}

func TestBraveSearchProviderHTTPTimeout(t *testing.T) {
	t.Parallel()

	config := DefaultTransportConfig()
	config.RequestTimeout = 20 * time.Millisecond
	provider := newHTTPFixtureBrave(t, config, http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))

	_, err := provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 1})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Search() timeout error = %v, want unavailable", err)
	}
}

func TestBraveSearchProviderHTTPInFlightCancellation(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	provider := newHTTPFixtureBrave(t, DefaultTransportConfig(), http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(started)
		<-request.Context().Done()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := provider.Search(ctx, validQuery(t), application.SearchOptions{Limit: 1})
		done <- err
	}()

	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("provider request did not start")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Search() cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("provider search did not stop after cancellation")
	}
}

func newHTTPFixtureBrave(t *testing.T, config TransportConfig, handler http.Handler) *Brave {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	endpoint, err := url.Parse(server.URL + "/res/v1/web/search")
	if err != nil {
		t.Fatal(err)
	}
	client := newSecureHTTPClient(config, endpoint, server.Client().Transport)
	t.Cleanup(client.CloseIdleConnections)
	provider, err := NewBrave(client, "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	provider.endpoint = endpoint.String()
	return provider
}

func validQuery(t *testing.T) application.SearchQuery {
	t.Helper()
	id, err := research.NewID("request.brave-fixture")
	if err != nil {
		t.Fatal(err)
	}
	return application.SearchQuery{RequestID: id, Text: "Go interfaces"}
}

func assertRequest(t *testing.T, request *http.Request, query, count, offset string) {
	t.Helper()
	if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "api.search.brave.com" ||
		request.URL.Path != "/res/v1/web/search" {
		t.Fatalf("unexpected request target: %s %s", request.Method, request.URL)
	}
	parameters := request.URL.Query()
	if parameters.Get("q") != query || parameters.Get("count") != count || parameters.Get("offset") != offset {
		t.Fatalf("query parameters = %v", parameters)
	}
	if len(parameters) != 3 {
		t.Fatalf("unexpected provider parameters = %v", parameters)
	}
}

func fixtureResponse(t *testing.T, name string, header http.Header) *http.Response {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return jsonResponse(http.StatusOK, string(body), header)
}

func jsonResponse(status int, body string, header http.Header) *http.Response {
	if header == nil {
		header = make(http.Header)
	}
	header.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

func staticClient(response *http.Response) HTTPClient {
	return roundTripFunc(func(*http.Request) (*http.Response, error) { return response, nil })
}

type fixtureClient struct {
	mu        sync.Mutex
	responses []*http.Response
	requests  []*http.Request
}

func (client *fixtureClient) Do(request *http.Request) (*http.Response, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	clone := request.Clone(context.Background())
	clone.Header = request.Header.Clone()
	client.requests = append(client.requests, clone)
	if len(client.responses) == 0 {
		return nil, errors.New("fixture responses exhausted")
	}
	response := client.responses[0]
	client.responses = client.responses[1:]
	return response, nil
}

func (client *fixtureClient) Requests() []*http.Request {
	client.mu.Lock()
	defer client.mu.Unlock()
	return append([]*http.Request(nil), client.requests...)
}

type recordingObserver struct {
	mu     sync.Mutex
	events []PageObservation
}

func (observer *recordingObserver) ObserveSearchPage(_ context.Context, event PageObservation) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	observer.events = append(observer.events, event)
}

func (observer *recordingObserver) Events() []PageObservation {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return append([]PageObservation(nil), observer.events...)
}
