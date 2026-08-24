package researchhttp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"testing"
)

func TestDefaultClientUsesSecureTransportDefaults(t *testing.T) {
	t.Parallel()
	client, err := New(DefaultConfig(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	if client.transport.Proxy == nil || client.transport.DisableCompression || !client.transport.ForceAttemptHTTP2 {
		t.Fatalf("transport proxy/compression/HTTP2 = %+v", client.transport)
	}
	if client.transport.TLSClientConfig == nil || client.transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLS config = %+v", client.transport.TLSClientConfig)
	}
	if client.http.Timeout != DefaultConfig().RequestTimeout {
		t.Fatalf("client timeout = %v", client.http.Timeout)
	}
}

func TestDefaultAddressPolicyBlocksPrivateAndMetadataTargets(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"0.0.0.0", "127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.0.1",
		"169.254.169.254", "100.100.100.200", "168.63.129.16", "::1", "fe80::1", "fd00:ec2::254",
	} {
		if err := publicAddressPolicy(net.ParseIP(value)); err == nil {
			t.Errorf("publicAddressPolicy(%s) allowed a blocked address", value)
		}
	}
}

func TestDefaultClientBlocksSSRFBeforeDial(t *testing.T) {
	t.Parallel()
	client, err := New(DefaultConfig(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	for _, target := range []string{
		"http://localhost/source",
		"http://127.0.0.1/source",
		"http://169.254.169.254/latest/meta-data",
		"http://metadata.google.internal/computeMetadata/v1/",
		"http://user:password@example.com/source",
	} {
		_, err := client.Do(context.Background(), Request{URL: target})
		if !errors.Is(err, ErrBlockedAddress) && !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("Do(%q) error = %v", target, err)
		}
	}
}

func TestConfigRejectsUnsafeValuesDeterministically(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "user agent", mutate: func(config *Config) { config.UserAgent = "Other/1" }},
		{name: "timeout", mutate: func(config *Config) { config.RequestTimeout = 0 }},
		{name: "response", mutate: func(config *Config) { config.MaxResponseBytes = maximumConfiguredResponse + 1 }},
		{name: "redirect", mutate: func(config *Config) { config.MaxRedirects = maximumRedirects + 1 }},
		{name: "attempts", mutate: func(config *Config) { config.MaxAttempts = maximumAttempts + 1 }},
		{name: "backoff", mutate: func(config *Config) { config.MaxBackoff = config.InitialBackoff / 2 }},
		{name: "content type", mutate: func(config *Config) { config.AllowedContentTypes = []string{"APPLICATION/JSON"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := DefaultConfig()
			test.mutate(&config)
			if _, err := New(config, nil, nil); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("New() error = %v", err)
			}
		})
	}
}

func TestRetryableStatusPolicyDoesNotRetryArbitraryClientErrors(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		if !retryableStatus(status) {
			t.Errorf("status %d should retry", status)
		}
	}
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity} {
		if retryableStatus(status) {
			t.Errorf("status %d should not retry", status)
		}
	}
}
