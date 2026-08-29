package researchhttp

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestClientFetchesBoundedContentWithKelyroUserAgentAndSafeMetadata(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != "Kelyro/test" {
			t.Errorf("User-Agent = %q", request.Header.Get("User-Agent"))
		}
		if request.Header.Get("If-None-Match") != `"previous"` {
			t.Errorf("If-None-Match = %q", request.Header.Get("If-None-Match"))
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.Header().Set("ETag", `"current"`)
		writer.Header().Set("Last-Modified", "Mon, 24 Aug 2026 12:00:00 GMT")
		writer.Header().Set("Set-Cookie", "session=must-not-be-retained")
		_, _ = writer.Write([]byte("fixture"))
	}))
	defer server.Close()

	config := testConfig()
	client := newTestClient(t, config, nil, nil, nil)
	response, err := client.Do(context.Background(), Request{
		URL:    server.URL + "/source?token=must-not-be-logged",
		Header: http.Header{"If-None-Match": []string{`"previous"`}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || response.ContentType != "text/plain" ||
		response.ETag != `"current"` || response.LastModified == "" || string(response.Body) != "fixture" {
		t.Fatalf("response = %+v", response)
	}
	if !strings.HasSuffix(response.FinalURL, "/source") || strings.Contains(response.FinalURL, "token") || strings.Contains(response.FinalURL, "must-not-be-logged") {
		t.Fatalf("final URL = %q", response.FinalURL)
	}
}

func TestClientCarriesExplicitNoStoreWithoutExposingOtherCacheHeaders(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Add("Cache-Control", "public, max-age=3600")
		writer.Header().Add("Cache-Control", `No-Store="reason"`)
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("transient"))
	}))
	defer server.Close()

	response, err := newTestClient(t, testConfig(), nil, nil, nil).Do(context.Background(), Request{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if !response.NoStore || string(response.Body) != "transient" {
		t.Fatalf("response = %+v", response)
	}
}

func TestClientEnforcesTimeoutAndCallerCancellation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()

	t.Run("client timeout", func(t *testing.T) {
		config := testConfig()
		config.RequestTimeout = 30 * time.Millisecond
		config.MaxAttempts = 1
		client := newTestClient(t, config, nil, nil, nil)
		_, err := client.Do(context.Background(), Request{URL: server.URL})
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("timeout error = %v", err)
		}
	})

	t.Run("caller cancellation", func(t *testing.T) {
		config := testConfig()
		client := newTestClient(t, config, nil, nil, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := client.Do(ctx, Request{URL: server.URL})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v", err)
		}
	})
}

func TestClientEnforcesRedirectLimitAndRevalidatesRedirectTargets(t *testing.T) {
	t.Parallel()
	t.Run("limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, "/again", http.StatusFound)
		}))
		defer server.Close()
		config := testConfig()
		config.MaxRedirects = 1
		client := newTestClient(t, config, nil, nil, nil)
		_, err := client.Do(context.Background(), Request{URL: server.URL})
		if !errors.Is(err, ErrRedirectLimit) {
			t.Fatalf("redirect error = %v", err)
		}
	})

	t.Run("blocked target", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, "http://169.254.169.254/latest/meta-data", http.StatusFound)
		}))
		defer server.Close()
		client := newTestClient(t, testConfig(), nil, nil, loopbackTestPolicy)
		_, err := client.Do(context.Background(), Request{URL: server.URL})
		if !errors.Is(err, ErrBlockedAddress) {
			t.Fatalf("redirect SSRF error = %v", err)
		}
	})
}

func TestClientStripsConditionalHeadersAcrossOrigins(t *testing.T) {
	t.Parallel()
	var received http.Header
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received = request.Header.Clone()
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("redirected"))
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer origin.Close()

	client := newTestClient(t, testConfig(), nil, nil, loopbackTestPolicy)
	_, err := client.Do(context.Background(), Request{URL: origin.URL, Header: http.Header{
		"If-None-Match": []string{`"private-validator"`}, "Range": []string{"bytes=0-10"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if received.Get("If-None-Match") != "" || received.Get("Range") != "" {
		t.Fatalf("cross-origin redirect leaked conditional headers: %+v", received)
	}
}

func TestClientRejectsOversizeAndUnexpectedContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		encoding    string
		body        string
		want        error
	}{
		{name: "oversize", contentType: "text/plain", body: strings.Repeat("x", 33), want: ErrResponseTooLarge},
		{name: "content type", contentType: "application/octet-stream", body: "fixture", want: ErrContentType},
		{name: "encoding", contentType: "text/plain", encoding: "br", body: "fixture", want: ErrUnsupportedEncoding},
		{name: "binary declared text", contentType: "text/plain", body: "image\x00payload", want: ErrContentType},
		{name: "invalid declared JSON", contentType: "application/json", body: `{"open":`, want: ErrContentType},
		{name: "invalid declared XML", contentType: "application/xml", body: `<open>`, want: ErrContentType},
		{name: "fake PDF", contentType: "application/pdf", body: "not a pdf", want: ErrContentType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.Header().Set("Content-Type", test.contentType)
				if test.encoding != "" {
					writer.Header().Set("Content-Encoding", test.encoding)
				}
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			config := testConfig()
			config.MaxResponseBytes = 32
			client := newTestClient(t, config, nil, nil, nil)
			_, err := client.Do(context.Background(), Request{URL: server.URL})
			if !errors.Is(err, test.want) {
				t.Fatalf("Do() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestClientAcceptsValidatedStructuredRepresentations(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		contentType string
		body        string
	}{
		{contentType: "application/json", body: `{"ok":true}`},
		{contentType: "application/xml", body: `<root><ok>true</ok></root>`},
		{contentType: "application/pdf", body: "%PDF-1.7\nfixture"},
	} {
		t.Run(test.contentType, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", test.contentType)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()
			response, err := newTestClient(t, testConfig(), nil, nil, nil).Do(context.Background(), Request{URL: server.URL})
			if err != nil || string(response.Body) != test.body {
				t.Fatalf("Do() = (%+v, %v)", response, err)
			}
		})
	}
}

func TestClientEnforcesPerRequestResponseLimit(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("five!"))
	}))
	defer server.Close()

	config := testConfig()
	config.MaxResponseBytes = 64
	client := newTestClient(t, config, nil, nil, nil)
	_, err := client.Do(context.Background(), Request{URL: server.URL, MaxResponseBytes: 4})
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("Do() error = %v, want ErrResponseTooLarge", err)
	}
	_, err = client.Do(context.Background(), Request{URL: server.URL, MaxResponseBytes: 65})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("Do() oversized request limit error = %v, want ErrInvalidRequest", err)
	}
}

func TestClientRetriesOnlyTransientStatusesWithBoundedBackoff(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		status      int
		retryAfter  string
		wantCalls   int32
		wantSuccess bool
	}{
		{name: "404", status: http.StatusNotFound, wantCalls: 1},
		{name: "429", status: http.StatusTooManyRequests, retryAfter: "120", wantCalls: 3, wantSuccess: true},
		{name: "500", status: http.StatusInternalServerError, wantCalls: 3, wantSuccess: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				call := calls.Add(1)
				if test.wantSuccess && call == test.wantCalls {
					writer.Header().Set("Content-Type", "text/plain")
					_, _ = writer.Write([]byte("recovered"))
					return
				}
				if test.retryAfter != "" {
					writer.Header().Set("Retry-After", test.retryAfter)
				}
				writer.WriteHeader(test.status)
			}))
			defer server.Close()
			delays := make([]time.Duration, 0)
			config := testConfig()
			config.MaxAttempts = 3
			config.InitialBackoff = 5 * time.Millisecond
			config.MaxBackoff = 20 * time.Millisecond
			client := newTestClient(t, config, nil, nil, func(_ context.Context, delay time.Duration) error {
				delays = append(delays, delay)
				return nil
			})
			response, err := client.Do(context.Background(), Request{URL: server.URL})
			if test.wantSuccess {
				if err != nil || string(response.Body) != "recovered" {
					t.Fatalf("Do() = (%+v, %v)", response, err)
				}
			} else if !errors.Is(err, ErrHTTPStatus) {
				t.Fatalf("Do() error = %v, want HTTP status", err)
			}
			if got := calls.Load(); got != test.wantCalls {
				t.Fatalf("calls = %d, want %d", got, test.wantCalls)
			}
			for _, delay := range delays {
				if delay > config.MaxBackoff {
					t.Fatalf("retry delay = %v, max = %v", delay, config.MaxBackoff)
				}
			}
		})
	}
}

func TestClientSupportsAutomaticGzipWithinDecompressedLimit(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept-Encoding") != "gzip" {
			t.Errorf("Accept-Encoding = %q", request.Header.Get("Accept-Encoding"))
		}
		writer.Header().Set("Content-Type", "text/plain")
		writer.Header().Set("Content-Encoding", "gzip")
		compressed := gzip.NewWriter(writer)
		_, _ = compressed.Write([]byte("compressed fixture"))
		_ = compressed.Close()
	}))
	defer server.Close()
	client := newTestClient(t, testConfig(), nil, nil, nil)
	response, err := client.Do(context.Background(), Request{URL: server.URL})
	if err != nil || string(response.Body) != "compressed fixture" {
		t.Fatalf("Do() = (%+v, %v)", response, err)
	}
}

func TestClientBoundsGzipAfterDecompression(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		writer.Header().Set("Content-Encoding", "gzip")
		compressed := gzip.NewWriter(writer)
		_, _ = compressed.Write([]byte(strings.Repeat("x", 4096)))
		_ = compressed.Close()
	}))
	defer server.Close()
	config := testConfig()
	config.MaxResponseBytes = 128
	_, err := newTestClient(t, config, nil, nil, nil).Do(context.Background(), Request{URL: server.URL})
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("gzip bomb error = %v, want response_too_large", err)
	}
}

func TestClientRetriesTransientTransportFailure(t *testing.T) {
	t.Parallel()
	config := testConfig()
	config.MaxAttempts = 2
	client := newTestClient(t, config, nil, nil, nil)
	var calls int
	client.http.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, syscall.ECONNRESET
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("recovered")),
			Request:    request,
		}, nil
	})
	response, err := client.Do(context.Background(), Request{URL: "http://127.0.0.1/source"})
	if err != nil || calls != 2 || string(response.Body) != "recovered" {
		t.Fatalf("Do() = (%+v, %v), calls = %d", response, err, calls)
	}
}

func TestClientRateLimitAndObserverHooksExposeOnlyBoundedMetadata(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()
	limiter := &recordingLimiter{}
	observer := &recordingObserver{}
	client := newTestClient(t, testConfig(), limiter, observer, nil)
	_, err := client.Do(context.Background(), Request{URL: server.URL + "/private?token=secret"})
	if err != nil {
		t.Fatal(err)
	}
	if len(limiter.hosts) != 1 || limiter.hosts[0] != "127.0.0.1" {
		t.Fatalf("limiter hosts = %+v", limiter.hosts)
	}
	if len(observer.events) != 1 || observer.events[0].Outcome != OutcomeSucceeded || observer.events[0].Attempt != 1 {
		t.Fatalf("observer events = %+v", observer.events)
	}
}

func TestClientRejectsSensitiveHeadersAndRedactsErrors(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, testConfig(), nil, nil, nil)
	_, err := client.Do(context.Background(), Request{
		URL:    "http://127.0.0.1/private?token=super-secret",
		Header: http.Header{"Authorization": []string{"Bearer super-secret"}},
	})
	if !errors.Is(err, ErrInvalidRequest) || strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("sensitive header error = %v", err)
	}
	_, err = client.Do(context.Background(), Request{
		URL:    "http://127.0.0.1/private",
		Header: http.Header{"X-API-Key": []string{"super-secret"}},
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("API key header error = %v", err)
	}

	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("transport echoed super-secret")
	})
	_, err = client.Do(context.Background(), Request{URL: "http://127.0.0.1/private?token=super-secret"})
	if !errors.Is(err, ErrTransport) || strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("transport error = %v", err)
	}
}

func testConfig() Config {
	config := DefaultConfig()
	config.UserAgent = "Kelyro/test"
	config.RequestTimeout = time.Second
	config.MaxAttempts = 1
	return config
}

func newTestClient(t *testing.T, config Config, limiter RateLimiter, observer Observer, custom any) *Client {
	t.Helper()
	policy := func(net.IP) error { return nil }
	sleep := func(context.Context, time.Duration) error { return nil }
	switch value := custom.(type) {
	case func(net.IP) error:
		policy = value
	case func(context.Context, time.Duration) error:
		sleep = value
	case nil:
	default:
		t.Fatalf("unsupported test dependency %T", custom)
	}
	client, err := newClient(config, limiter, observer, networkDependencies{
		resolver: net.DefaultResolver, dialer: &net.Dialer{Timeout: config.DialTimeout},
		addressPolicy: policy, sleep: sleep, now: func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.CloseIdleConnections)
	return client
}

func loopbackTestPolicy(address net.IP) error {
	if address.IsLoopback() {
		return nil
	}
	return publicAddressPolicy(address)
}

type recordingLimiter struct{ hosts []string }

func (limiter *recordingLimiter) Wait(_ context.Context, host string) error {
	limiter.hosts = append(limiter.hosts, host)
	return nil
}

type recordingObserver struct{ events []Event }

func (observer *recordingObserver) Observe(_ context.Context, event Event) {
	observer.events = append(observer.events, event)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
