package researchsearch

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	TransportSecurityVersion = "brave-search-transport-v1"
	maximumUserAgentBytes    = 128
	maximumHeaderBytes       = 256 * 1024
	maximumTransportTimeout  = 2 * time.Minute
)

// TransportConfig owns the network limits that apply before the adapter's
// separate one-MiB decoded-body bound.
type TransportConfig struct {
	UserAgent                 string
	RequestTimeout            time.Duration
	DialTimeout               time.Duration
	TLSHandshakeTimeout       time.Duration
	ResponseHeaderTimeout     time.Duration
	IdleConnTimeout           time.Duration
	MaxResponseHeaderBytes    int64
	MaxIdleConnections        int
	MaxIdleConnectionsPerHost int
}

func DefaultTransportConfig() TransportConfig {
	return TransportConfig{
		UserAgent:                 "Kelyro/dev",
		RequestTimeout:            15 * time.Second,
		DialTimeout:               5 * time.Second,
		TLSHandshakeTimeout:       5 * time.Second,
		ResponseHeaderTimeout:     10 * time.Second,
		IdleConnTimeout:           60 * time.Second,
		MaxResponseHeaderBytes:    64 * 1024,
		MaxIdleConnections:        8,
		MaxIdleConnectionsPerHost: 4,
	}
}

// SecureHTTPClient is restricted to the selected search endpoint. It is not a
// general fetch client and cannot be used for discovered result URLs.
type SecureHTTPClient struct {
	client    *http.Client
	transport *http.Transport
	endpoint  *url.URL
	userAgent string
}

// NewBraveHTTPClient constructs the production transport with a fixed HTTPS
// endpoint, no proxy inheritance, TLS 1.2+, no redirects, and bounded timeouts.
func NewBraveHTTPClient(config TransportConfig) (*SecureHTTPClient, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(EndpointURL)
	if err != nil {
		return nil, errors.New("brave search endpoint is invalid")
	}
	dialer := &net.Dialer{Timeout: config.DialTimeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           config.MaxIdleConnections,
		MaxIdleConnsPerHost:    config.MaxIdleConnectionsPerHost,
		MaxConnsPerHost:        config.MaxIdleConnectionsPerHost,
		IdleConnTimeout:        config.IdleConnTimeout,
		TLSHandshakeTimeout:    config.TLSHandshakeTimeout,
		ResponseHeaderTimeout:  config.ResponseHeaderTimeout,
		ExpectContinueTimeout:  time.Second,
		MaxResponseHeaderBytes: config.MaxResponseHeaderBytes,
		DisableCompression:     true,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return newSecureHTTPClient(config, endpoint, transport), nil
}

func newSecureHTTPClient(config TransportConfig, endpoint *url.URL, transport http.RoundTripper) *SecureHTTPClient {
	client := &SecureHTTPClient{endpoint: endpoint, userAgent: config.UserAgent}
	if typed, ok := transport.(*http.Transport); ok {
		client.transport = typed
	}
	client.client = &http.Client{
		Transport: transport,
		Timeout:   config.RequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return client
}

func (client *SecureHTTPClient) Do(request *http.Request) (*http.Response, error) {
	if client == nil || client.client == nil || client.endpoint == nil {
		return nil, &Error{Kind: ErrorInvalidRequest}
	}
	if err := validateSearchRequest(request, client.endpoint); err != nil {
		return nil, &Error{Kind: ErrorInvalidRequest}
	}
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Header.Set("User-Agent", client.userAgent)
	clone.Header.Set("Accept-Encoding", "identity")
	response, err := client.client.Do(clone)
	if err != nil {
		if ctxErr := request.Context().Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &Error{Kind: ErrorUnavailable}
		}
		return nil, &Error{Kind: ErrorTransport}
	}
	return response, nil
}

func (client *SecureHTTPClient) CloseIdleConnections() {
	if client != nil && client.transport != nil {
		client.transport.CloseIdleConnections()
	}
}

func (config TransportConfig) validate() error {
	if config.UserAgent == "" || len(config.UserAgent) > maximumUserAgentBytes ||
		!strings.HasPrefix(config.UserAgent, "Kelyro/") || strings.ContainsAny(config.UserAgent, "\r\n\x00") {
		return errors.New("search user agent must be a bounded Kelyro identifier")
	}
	for _, setting := range []struct {
		name  string
		value time.Duration
	}{
		{"request timeout", config.RequestTimeout},
		{"dial timeout", config.DialTimeout},
		{"TLS handshake timeout", config.TLSHandshakeTimeout},
		{"response header timeout", config.ResponseHeaderTimeout},
		{"idle connection timeout", config.IdleConnTimeout},
	} {
		if setting.value <= 0 || setting.value > maximumTransportTimeout {
			return fmt.Errorf("%s must be positive and at most two minutes", setting.name)
		}
	}
	if config.MaxResponseHeaderBytes <= 0 || config.MaxResponseHeaderBytes > maximumHeaderBytes {
		return fmt.Errorf("maximum response header bytes must be between 1 and %d", maximumHeaderBytes)
	}
	if config.MaxIdleConnections < 1 || config.MaxIdleConnections > 64 ||
		config.MaxIdleConnectionsPerHost < 1 || config.MaxIdleConnectionsPerHost > config.MaxIdleConnections {
		return errors.New("search idle connection limits are invalid")
	}
	return nil
}

func validateSearchRequest(request *http.Request, endpoint *url.URL) error {
	if request == nil || request.URL == nil || request.Context() == nil {
		return errors.New("search request is incomplete")
	}
	if request.Method != http.MethodGet || request.Body != nil || request.URL.User != nil || request.URL.Fragment != "" ||
		request.URL.Scheme != endpoint.Scheme || !strings.EqualFold(request.URL.Host, endpoint.Host) ||
		request.URL.Path != endpoint.Path || request.URL.RawPath != endpoint.RawPath {
		return errors.New("search request target is not the fixed endpoint")
	}
	if len(request.Header) != 2 {
		return errors.New("search request contains an unexpected header")
	}
	for name := range request.Header {
		if !strings.EqualFold(name, "Accept") && !strings.EqualFold(name, credentialHeader) {
			return errors.New("search request contains an unexpected header")
		}
	}
	if len(request.Header.Values("Accept")) != 1 || len(request.Header.Values(credentialHeader)) != 1 ||
		request.Header.Get("Accept") != "application/json" || validateToken(request.Header.Get(credentialHeader)) != nil {
		return errors.New("search request is missing required headers")
	}
	parameters := request.URL.Query()
	query := parameters.Get("q")
	if len(parameters) != 3 || len(parameters["q"]) != 1 || len(parameters["count"]) != 1 ||
		len(parameters["offset"]) != 1 || strings.TrimSpace(query) == "" || query != strings.TrimSpace(query) ||
		!utf8.ValidString(query) || utf8.RuneCountInString(query) > maximumQueryRunes || len(strings.Fields(query)) > maximumQueryWords ||
		strings.IndexFunc(query, unicode.IsControl) >= 0 {
		return errors.New("search request parameters are invalid")
	}
	count, countErr := strconv.Atoi(parameters.Get("count"))
	offset, offsetErr := strconv.Atoi(parameters.Get("offset"))
	if countErr != nil || count < 1 || count > maximumResultsPerPage ||
		offsetErr != nil || offset < 0 || offset > maximumPageOffset {
		return errors.New("search request bounds are invalid")
	}
	for name := range parameters {
		if name != "q" && name != "count" && name != "offset" {
			return errors.New("search request contains an unexpected parameter")
		}
	}
	return nil
}

var _ HTTPClient = (*SecureHTTPClient)(nil)
