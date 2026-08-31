package researchsearch

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestNewBraveHTTPClientAppliesProductionSecurityDefaults(t *testing.T) {
	t.Parallel()
	config := DefaultTransportConfig()
	client, err := NewBraveHTTPClient(config)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()

	if client.endpoint.String() != EndpointURL || client.client.Timeout != config.RequestTimeout {
		t.Fatalf("endpoint/timeout = %q/%s", client.endpoint, client.client.Timeout)
	}
	transport := client.transport
	if transport == nil || transport.Proxy != nil || !transport.ForceAttemptHTTP2 || !transport.DisableCompression {
		t.Fatalf("transport invariants not applied: %+v", transport)
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("minimum TLS version = %v, want TLS 1.2", transport.TLSClientConfig)
	}
	if transport.TLSHandshakeTimeout != config.TLSHandshakeTimeout ||
		transport.ResponseHeaderTimeout != config.ResponseHeaderTimeout ||
		transport.MaxResponseHeaderBytes != config.MaxResponseHeaderBytes {
		t.Fatal("bounded transport timeouts/headers were not applied")
	}
}

func TestSecureHTTPClientPinsEndpointMethodParametersAndHeaders(t *testing.T) {
	t.Parallel()
	endpoint, _ := url.Parse(EndpointURL)
	var calls atomic.Int32
	base := roundTripperFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return jsonResponse(http.StatusOK, `{"web":{"results":[]}}`, nil), nil
	})
	client := newSecureHTTPClient(DefaultTransportConfig(), endpoint, base)

	valid := secureFixtureRequest(t, endpoint.String())
	invalid := []*http.Request{
		valid.Clone(context.Background()),
		secureFixtureRequest(t, "https://example.test/res/v1/web/search"),
		secureFixtureRequest(t, endpoint.String()+"/other"),
		secureFixtureRequest(t, endpoint.String()+"?q=Go&count=21&offset=0"),
	}
	invalid[0].Method = http.MethodPost
	for index, request := range invalid {
		if _, err := client.Do(request); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("invalid request %d error = %v", index, err)
		}
	}
	withHeader := secureFixtureRequest(t, endpoint.String())
	withHeader.Header.Set("Authorization", "Bearer should-not-leave")
	if _, err := client.Do(withHeader); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("unexpected header error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("rejected requests reached transport %d times", calls.Load())
	}
}

func TestSecureHTTPClientOwnsUserAgentAndCompression(t *testing.T) {
	t.Parallel()
	endpoint, _ := url.Parse(EndpointURL)
	base := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("User-Agent") != "Kelyro/test" || request.Header.Get("Accept-Encoding") != "identity" {
			t.Fatalf("owned headers = %v", request.Header)
		}
		return jsonResponse(http.StatusOK, `{"web":{"results":[]}}`, nil), nil
	})
	config := DefaultTransportConfig()
	config.UserAgent = "Kelyro/test"
	client := newSecureHTTPClient(config, endpoint, base)
	request := secureFixtureRequest(t, endpoint.String())
	if _, err := client.Do(request); err != nil {
		t.Fatal(err)
	}
	if request.Header.Get("User-Agent") != "" || request.Header.Get("Accept-Encoding") != "" {
		t.Fatal("secure client mutated caller-owned request")
	}
}

func TestBraveSearchTransportBlocksRedirectAndCredentialForwarding(t *testing.T) {
	t.Parallel()
	var endpointCalls atomic.Int32
	var redirectedCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/res/v1/web/search":
			endpointCalls.Add(1)
			http.Redirect(writer, request, "/credential-sink", http.StatusFound)
		case "/credential-sink":
			redirectedCalls.Add(1)
			if request.Header.Get(credentialHeader) != "" {
				t.Error("credential reached redirected endpoint")
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	endpoint, err := url.Parse(server.URL + "/res/v1/web/search")
	if err != nil {
		t.Fatal(err)
	}
	client := newSecureHTTPClient(DefaultTransportConfig(), endpoint, server.Client().Transport)
	provider, err := NewBrave(client, "fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	provider.endpoint = endpoint.String()
	_, err = provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 1})
	if !errors.Is(err, ErrResponse) {
		t.Fatalf("redirect response error = %v", err)
	}
	if endpointCalls.Load() != 1 || redirectedCalls.Load() != 0 {
		t.Fatalf("endpoint/redirect calls = %d/%d", endpointCalls.Load(), redirectedCalls.Load())
	}
}

func TestSecureHTTPClientBoundsTimeoutAndPreservesCancellation(t *testing.T) {
	t.Parallel()
	endpoint, _ := url.Parse(EndpointURL)
	blocking := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	config := DefaultTransportConfig()
	config.RequestTimeout = 20 * time.Millisecond
	client := newSecureHTTPClient(config, endpoint, blocking)
	started := time.Now()
	if _, err := client.Do(secureFixtureRequest(t, endpoint.String())); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("request timeout error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("request timeout took %s", elapsed)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := secureFixtureRequest(t, endpoint.String()).WithContext(ctx)
	if _, err := client.Do(request); !errors.Is(err, context.Canceled) {
		t.Fatalf("explicit cancellation error = %v", err)
	}
}

func TestSecureHTTPClientRedactsTransportErrors(t *testing.T) {
	t.Parallel()
	endpoint, _ := url.Parse(EndpointURL)
	client := newSecureHTTPClient(DefaultTransportConfig(), endpoint, roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("fixture-token Go interfaces")
	}))
	_, err := client.Do(secureFixtureRequest(t, endpoint.String()))
	if !errors.Is(err, ErrTransport) {
		t.Fatalf("transport error = %v", err)
	}
	if strings.Contains(err.Error(), "fixture-token") || strings.Contains(err.Error(), "Go interfaces") {
		t.Fatalf("transport error leaked request data: %v", err)
	}
}

func TestBraveSearchRejectsContentTypeAndDeclaredBodyOverflow(t *testing.T) {
	t.Parallel()
	wrongType := jsonResponse(http.StatusOK, `{"web":{"results":[]}}`, nil)
	wrongType.Header.Set("Content-Type", "text/html")
	tooLarge := jsonResponse(http.StatusOK, `{}`, nil)
	tooLarge.ContentLength = maximumResponseBytes + 1
	for name, response := range map[string]*http.Response{"content type": wrongType, "declared length": tooLarge} {
		t.Run(name, func(t *testing.T) {
			provider, err := NewBrave(staticClient(response), "fixture-token")
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 1})
			if !errors.Is(err, ErrResponse) {
				t.Fatalf("response error = %v", err)
			}
		})
	}
}

func TestTransportConfigRejectsUnsafeValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*TransportConfig)
	}{
		{"user agent", func(config *TransportConfig) { config.UserAgent = "curl/unsafe" }},
		{"request timeout", func(config *TransportConfig) { config.RequestTimeout = 0 }},
		{"header bytes", func(config *TransportConfig) { config.MaxResponseHeaderBytes = maximumHeaderBytes + 1 }},
		{"connections", func(config *TransportConfig) { config.MaxIdleConnectionsPerHost = config.MaxIdleConnections + 1 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultTransportConfig()
			test.mutate(&config)
			if _, err := NewBraveHTTPClient(config); err == nil {
				t.Fatal("unsafe transport config accepted")
			}
		})
	}
}

func secureFixtureRequest(t *testing.T, endpoint string) *http.Request {
	t.Helper()
	parsed, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	parameters := parsed.Query()
	if parameters.Get("q") == "" {
		parameters.Set("q", "Go interfaces")
	}
	if parameters.Get("count") == "" {
		parameters.Set("count", "8")
	}
	if parameters.Get("offset") == "" {
		parameters.Set("offset", "0")
	}
	parsed.RawQuery = parameters.Encode()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, parsed.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set(credentialHeader, "fixture-token")
	return request
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
