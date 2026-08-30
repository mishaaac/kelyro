package researchhttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maximumURLLength = 8 * 1024

const SecurityPolicyVersion = "research-input-security-v1"

type resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type contextDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

type networkDependencies struct {
	resolver      resolver
	dialer        contextDialer
	addressPolicy func(net.IP) error
	sleep         func(context.Context, time.Duration) error
	now           func() time.Time
}

var blockedMetadataHosts = map[string]struct{}{
	"metadata":                 {},
	"metadata.google.internal": {},
	"metadata.azure.internal":  {},
}

var blockedMetadataAddresses = map[string]struct{}{
	"100.100.100.200": {},
	"168.63.129.16":   {},
	"fd00:ec2::254":   {},
}

func validateTarget(ctx context.Context, parsed *url.URL, lookup resolver, policy func(net.IP) error) error {
	if parsed == nil || len(parsed.String()) > maximumURLLength ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Opaque != "" ||
		strings.Contains(parsed.Host, `\`) || strings.IndexFunc(parsed.String(), isUnsafeURLRune) >= 0 {
		return classified(ErrorInvalidRequest, errors.New("target must be an absolute HTTP(S) URL without user information"))
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" || strings.Contains(host, "%") || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return classified(ErrorBlockedAddress, errors.New("target host is blocked"))
	}
	if metadataHostBlocked(host) {
		return classified(ErrorBlockedAddress, errors.New("metadata host is blocked"))
	}
	if _, err := targetPort(parsed); err != nil {
		return classified(ErrorInvalidRequest, err)
	}
	addresses, err := resolveAddresses(ctx, host, lookup)
	if err != nil {
		return classified(ErrorTransport, err)
	}
	for _, address := range addresses {
		if err := policy(address); err != nil {
			return classified(ErrorBlockedAddress, err)
		}
	}
	return nil
}

func isUnsafeURLRune(value rune) bool {
	return value == 0 || value < 0x20 || value == 0x7f
}

func metadataHostBlocked(host string) bool {
	if _, blocked := blockedMetadataHosts[host]; blocked {
		return true
	}
	for blocked := range blockedMetadataHosts {
		if strings.HasSuffix(host, "."+blocked) {
			return true
		}
	}
	return false
}

func resolveAddresses(ctx context.Context, host string, lookup resolver) ([]net.IP, error) {
	if parsed := net.ParseIP(host); parsed != nil {
		return []net.IP{parsed}, nil
	}
	if lookup == nil {
		return nil, errors.New("DNS resolver is unavailable")
	}
	resolved, err := lookup.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	addresses := make([]net.IP, 0, len(resolved))
	for _, item := range resolved {
		if item.IP != nil {
			addresses = append(addresses, item.IP)
		}
	}
	if len(addresses) == 0 {
		return nil, errors.New("target host resolved without addresses")
	}
	return addresses, nil
}

func publicAddressPolicy(address net.IP) error {
	if address == nil || address.IsUnspecified() || address.IsLoopback() || address.IsPrivate() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || !address.IsGlobalUnicast() {
		return errors.New("non-public target address is blocked")
	}
	if _, blocked := blockedMetadataAddresses[address.String()]; blocked {
		return errors.New("metadata service address is blocked")
	}
	return nil
}

func targetPort(parsed *url.URL) (string, error) {
	port := parsed.Port()
	if port == "" {
		if parsed.Scheme == "https" {
			return "443", nil
		}
		return "80", nil
	}
	numeric, err := strconv.Atoi(port)
	if err != nil || numeric < 1 || numeric > 65535 {
		return "", fmt.Errorf("invalid target port")
	}
	return port, nil
}

func secureDialContext(ctx context.Context, network, address string, dependencies networkDependencies) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, classified(ErrorInvalidRequest, errors.New("invalid dial address"))
	}
	addresses, err := resolveAddresses(ctx, strings.Trim(host, "[]"), dependencies.resolver)
	if err != nil {
		return nil, classified(ErrorTransport, err)
	}
	var dialErr error
	for _, candidate := range addresses {
		if err := dependencies.addressPolicy(candidate); err != nil {
			return nil, classified(ErrorBlockedAddress, err)
		}
		connection, err := dependencies.dialer.DialContext(ctx, network, net.JoinHostPort(candidate.String(), port))
		if err == nil {
			return connection, nil
		}
		dialErr = err
	}
	return nil, classified(ErrorTransport, dialErr)
}
