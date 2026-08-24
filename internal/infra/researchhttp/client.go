package researchhttp

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	retryDrainBytes           = 4 * 1024
	maximumRequestHeaderBytes = 32 * 1024
)

// RateLimiter is invoked before every network attempt. Host is normalized and
// never contains credentials, a path, query parameters, or a fragment.
type RateLimiter interface {
	Wait(context.Context, string) error
}

type Outcome string

const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeRetrying  Outcome = "retrying"
	OutcomeFailed    Outcome = "failed"
)

// Event is deliberately safe for structured logs: it cannot carry URLs,
// headers, response bodies, credentials, or transport error strings.
type Event struct {
	Attempt    int
	StatusCode int
	Outcome    Outcome
	RetryDelay time.Duration
}

type Observer interface {
	Observe(context.Context, Event)
}

// Request is a bounded GET request. MaxResponseBytes may lower, but never
// raise, the configured response limit. Sensitive request headers are rejected
// and the User-Agent and compression behavior remain owned by Client.
type Request struct {
	URL              string
	Header           http.Header
	MaxResponseBytes int64
}

// Response exposes only headers needed by future source snapshot adapters.
// Authentication, cookies, and other response headers are never retained.
type Response struct {
	StatusCode   int
	ContentType  string
	ETag         string
	LastModified string
	FinalURL     string
	Body         []byte
}

type Client struct {
	config    Config
	http      *http.Client
	transport *http.Transport
	limiter   RateLimiter
	observer  Observer
	network   networkDependencies
}

// New constructs a reusable hardened client. It performs no privacy check;
// Research application services own authorization before this adapter runs.
func New(config Config, limiter RateLimiter, observer Observer) (*Client, error) {
	return newClient(config, limiter, observer, networkDependencies{
		resolver:      net.DefaultResolver,
		dialer:        &net.Dialer{Timeout: config.DialTimeout, KeepAlive: 30 * time.Second},
		addressPolicy: publicAddressPolicy,
		sleep:         sleepContext,
		now:           time.Now,
	})
}

func newClient(config Config, limiter RateLimiter, observer Observer, network networkDependencies) (*Client, error) {
	if err := config.validate(); err != nil {
		return nil, classified(ErrorInvalidRequest, err)
	}
	if network.resolver == nil || network.dialer == nil || network.addressPolicy == nil || network.sleep == nil || network.now == nil {
		return nil, classified(ErrorInvalidRequest, errors.New("network dependencies are incomplete"))
	}
	config.AllowedContentTypes = append([]string(nil), config.AllowedContentTypes...)
	client := &Client{config: config, limiter: limiter, observer: observer, network: network}
	client.transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, networkName, address string) (net.Conn, error) {
			return secureDialContext(ctx, networkName, address, client.network)
		},
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           config.MaxIdleConnections,
		MaxIdleConnsPerHost:    config.MaxIdleConnectionsPerHost,
		IdleConnTimeout:        config.IdleConnTimeout,
		TLSHandshakeTimeout:    config.TLSHandshakeTimeout,
		ResponseHeaderTimeout:  config.ResponseHeaderTimeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: config.MaxResponseHeaderBytes,
		DisableCompression:     false,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client.http = &http.Client{
		Transport: client.transport,
		Timeout:   config.RequestTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) > config.MaxRedirects {
				return classified(ErrorRedirectLimit, errors.New("redirect limit exceeded"))
			}
			if err := validateTarget(request.Context(), request.URL, client.network.resolver, client.network.addressPolicy); err != nil {
				return err
			}
			stripSensitiveHeaders(request.Header)
			request.Header.Set("User-Agent", config.UserAgent)
			return nil
		},
	}
	return client, nil
}

// CloseIdleConnections releases pooled idle connections. The client remains
// usable and will establish new connections on demand.
func (client *Client) CloseIdleConnections() {
	if client != nil && client.transport != nil {
		client.transport.CloseIdleConnections()
	}
}

// Do performs one bounded GET, retrying only transient failures and explicitly
// retryable status codes.
func (client *Client) Do(ctx context.Context, input Request) (Response, error) {
	if client == nil || client.http == nil {
		return Response{}, classified(ErrorInvalidRequest, errors.New("research HTTP client is unavailable"))
	}
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	parsed, header, responseLimit, err := client.validateRequest(ctx, input)
	if err != nil {
		return Response{}, err
	}
	for attempt := 1; attempt <= client.config.MaxAttempts; attempt++ {
		if err := validateTarget(ctx, parsed, client.network.resolver, client.network.addressPolicy); err != nil {
			client.observe(ctx, Event{Attempt: attempt, Outcome: OutcomeFailed})
			return Response{}, err
		}
		if client.limiter != nil {
			if err := client.limiter.Wait(ctx, strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))); err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return Response{}, ctxErr
				}
				client.observe(ctx, Event{Attempt: attempt, Outcome: OutcomeFailed})
				return Response{}, classified(ErrorRateLimitHook, err)
			}
		}

		response, requestErr := client.attempt(ctx, parsed, header, responseLimit)
		if requestErr == nil {
			client.observe(ctx, Event{Attempt: attempt, StatusCode: response.StatusCode, Outcome: OutcomeSucceeded})
			return response, nil
		}
		status := errorStatus(requestErr)
		if attempt == client.config.MaxAttempts || !retryable(requestErr) {
			client.observe(ctx, Event{Attempt: attempt, StatusCode: status, Outcome: OutcomeFailed})
			return Response{}, requestErr
		}
		delay := client.retryDelay(attempt, requestErr)
		client.observe(ctx, Event{Attempt: attempt, StatusCode: status, Outcome: OutcomeRetrying, RetryDelay: delay})
		if err := client.network.sleep(ctx, delay); err != nil {
			return Response{}, err
		}
	}
	return Response{}, classified(ErrorTransport, errors.New("retry loop ended unexpectedly"))
}

func (client *Client) validateRequest(ctx context.Context, input Request) (*url.URL, http.Header, int64, error) {
	parsed, err := url.Parse(input.URL)
	if err != nil {
		return nil, nil, 0, classified(ErrorInvalidRequest, errors.New("target URL is invalid"))
	}
	if err := validateTarget(ctx, parsed, client.network.resolver, client.network.addressPolicy); err != nil {
		return nil, nil, 0, err
	}
	responseLimit := input.MaxResponseBytes
	if responseLimit == 0 {
		responseLimit = client.config.MaxResponseBytes
	}
	if responseLimit < 0 || responseLimit > client.config.MaxResponseBytes {
		return nil, nil, 0, classified(ErrorInvalidRequest, errors.New("request response limit is outside the configured bound"))
	}
	header := input.Header.Clone()
	if header == nil {
		header = make(http.Header)
	}
	headerBytes := 0
	for name, values := range header {
		canonical := http.CanonicalHeaderKey(name)
		if isSensitiveHeader(canonical) || strings.EqualFold(canonical, "Accept-Encoding") || strings.EqualFold(canonical, "User-Agent") {
			return nil, nil, 0, classified(ErrorInvalidRequest, errors.New("request contains a client-owned or sensitive header"))
		}
		headerBytes += len(canonical)
		for _, value := range values {
			if strings.ContainsAny(value, "\r\n\x00") {
				return nil, nil, 0, classified(ErrorInvalidRequest, errors.New("request header contains control characters"))
			}
			headerBytes += len(value)
		}
	}
	if headerBytes > maximumRequestHeaderBytes {
		return nil, nil, 0, classified(ErrorInvalidRequest, errors.New("request headers exceed the bounded limit"))
	}
	header.Set("User-Agent", client.config.UserAgent)
	return parsed, header, responseLimit, nil
}

func (client *Client) attempt(ctx context.Context, parsed *url.URL, header http.Header, responseLimit int64) (Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return Response{}, classified(ErrorInvalidRequest, errors.New("create request"))
	}
	request.Header = header.Clone()
	response, err := client.http.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Response{}, ctxErr
		}
		return Response{}, classifyTransportError(err)
	}
	if response == nil || response.Body == nil {
		return Response{}, classified(ErrorTransport, errors.New("transport returned an incomplete response"))
	}
	if retryableStatus(response.StatusCode) {
		retryAfter := parseRetryAfter(response.Header.Get("Retry-After"), client.network.now())
		drainAndClose(response.Body)
		err := statusError(response.StatusCode)
		if retryAfter > 0 {
			return Response{}, &retryError{cause: err, retryAfter: retryAfter}
		}
		return Response{}, err
	}
	if !successfulStatus(response.StatusCode) {
		drainAndClose(response.Body)
		return Response{}, statusError(response.StatusCode)
	}
	return client.readResponse(response, responseLimit)
}

func (client *Client) readResponse(response *http.Response, responseLimit int64) (Response, error) {
	defer response.Body.Close()
	if encoding := strings.TrimSpace(response.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return Response{}, classified(ErrorUnsupportedEncoding, errors.New("unsupported content encoding"))
	}
	contentType := ""
	if response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusNotModified {
		mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if err != nil || !client.allowedContentType(strings.ToLower(mediaType)) {
			return Response{}, classified(ErrorContentType, errors.New("response content type is not allowed"))
		}
		contentType = strings.ToLower(mediaType)
	}
	if response.ContentLength > responseLimit {
		return Response{}, classified(ErrorResponseTooLarge, errors.New("declared response size exceeds limit"))
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil {
		return Response{}, classifyTransportError(err)
	}
	if int64(len(body)) > responseLimit {
		return Response{}, classified(ErrorResponseTooLarge, errors.New("response body exceeds limit"))
	}
	return Response{
		StatusCode: response.StatusCode, ContentType: contentType,
		ETag: boundedHeader(response.Header.Get("ETag")), LastModified: boundedHeader(response.Header.Get("Last-Modified")),
		FinalURL: responseURL(response), Body: body,
	}, nil
}

func (client *Client) allowedContentType(mediaType string) bool {
	major, minor, found := strings.Cut(mediaType, "/")
	if !found {
		return false
	}
	for _, pattern := range client.config.AllowedContentTypes {
		patternMajor, patternMinor, _ := strings.Cut(pattern, "/")
		switch {
		case patternMajor != major:
			continue
		case patternMinor == "*" || patternMinor == minor:
			return true
		case strings.HasPrefix(patternMinor, "*+") && strings.HasSuffix(minor, strings.TrimPrefix(patternMinor, "*")):
			return true
		}
	}
	return false
}

func (client *Client) retryDelay(attempt int, err error) time.Duration {
	delay := client.config.InitialBackoff
	for index := 1; index < attempt && delay < client.config.MaxBackoff; index++ {
		delay *= 2
	}
	var retry *retryError
	if errors.As(err, &retry) && retry.retryAfter > delay {
		delay = retry.retryAfter
	}
	if delay > client.config.MaxBackoff {
		return client.config.MaxBackoff
	}
	return delay
}

func (client *Client) observe(ctx context.Context, event Event) {
	if client.observer != nil {
		client.observer.Observe(ctx, event)
	}
}

type retryError struct {
	cause      error
	retryAfter time.Duration
}

func (err *retryError) Error() string { return err.cause.Error() }
func (err *retryError) Unwrap() error { return err.cause }

func retryable(err error) bool {
	if errors.Is(err, ErrTimeout) {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrBlockedAddress) ||
		errors.Is(err, ErrRedirectLimit) || errors.Is(err, ErrResponseTooLarge) ||
		errors.Is(err, ErrContentType) || errors.Is(err, ErrUnsupportedEncoding) {
		return false
	}
	var status *Error
	if errors.As(err, &status) && status.Kind == ErrorHTTPStatus {
		return retryableStatus(status.StatusCode)
	}
	var networkError net.Error
	if errors.As(err, &networkError) && (networkError.Timeout() || networkError.Temporary()) {
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.EPIPE)
}

func retryableStatus(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests ||
		status == http.StatusInternalServerError || status == http.StatusBadGateway ||
		status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func successfulStatus(status int) bool {
	return status >= 200 && status < 300 || status == http.StatusNotModified
}

func classifyTransportError(err error) error {
	cause := safeCause(err)
	if kind, ok := KindOf(cause); ok {
		return classified(kind, cause)
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return classified(ErrorTimeout, cause)
	}
	return classified(ErrorTransport, cause)
}

func safeCause(err error) error {
	var urlError *url.Error
	if errors.As(err, &urlError) && urlError.Err != nil {
		return urlError.Err
	}
	return err
}

func errorStatus(err error) int {
	var typed *Error
	if errors.As(err, &typed) {
		return typed.StatusCode
	}
	return 0
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds <= 0 {
			return 0
		}
		if seconds > int64((24*time.Hour)/time.Second) {
			return 24 * time.Hour
		}
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil || !when.After(now) {
		return 0
	}
	delay := when.Sub(now)
	if delay > 24*time.Hour {
		return 24 * time.Hour
	}
	return delay
}

func responseURL(response *http.Response) string {
	if response.Request == nil || response.Request.URL == nil {
		return ""
	}
	return response.Request.URL.String()
}

func boundedHeader(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 1024 || strings.ContainsAny(value, "\r\n\x00") {
		return ""
	}
	return value
}

func isSensitiveHeader(name string) bool {
	canonical := http.CanonicalHeaderKey(name)
	switch canonical {
	case "Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "Www-Authenticate", "Proxy-Authenticate":
		return true
	}
	normalized := strings.ToLower(strings.ReplaceAll(canonical, "-", "_"))
	return strings.Contains(normalized, "api_key") || strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "token") || strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") || strings.Contains(normalized, "credential")
}

func stripSensitiveHeaders(header http.Header) {
	for name := range header {
		if isSensitiveHeader(name) {
			header.Del(name)
		}
	}
}

func drainAndClose(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, retryDrainBytes))
	_ = body.Close()
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
