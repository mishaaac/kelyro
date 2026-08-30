//go:build e2e

package researchhttp

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"
)

// NewLoopbackFixtureClient constructs the production HTTP client around an
// explicitly bounded fixture resolver. It exists only in e2e builds so local
// tests can exercise redirects, retries, validators, and response bounds
// without weakening the public-address policy used by production binaries.
func NewLoopbackFixtureClient(config Config, hosts []string, limiter RateLimiter, observer Observer) (*Client, error) {
	resolver := loopbackFixtureResolver{hosts: make(map[string]struct{}, len(hosts))}
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host == "" || !strings.HasSuffix(host, ".fixture.test") || net.ParseIP(host) != nil {
			return nil, classified(ErrorInvalidRequest, errors.New("fixture host must be a named subdomain of fixture.test"))
		}
		resolver.hosts[host] = struct{}{}
	}
	if len(resolver.hosts) == 0 {
		return nil, classified(ErrorInvalidRequest, errors.New("fixture hosts are empty"))
	}
	return newClient(config, limiter, observer, networkDependencies{
		resolver: resolver,
		dialer:   &net.Dialer{Timeout: config.DialTimeout, KeepAlive: 30 * time.Second},
		addressPolicy: func(address net.IP) error {
			if address == nil || !address.IsLoopback() {
				return errors.New("fixture target is not loopback")
			}
			return nil
		},
		sleep: sleepContext,
		now:   time.Now,
	})
}

type loopbackFixtureResolver struct {
	hosts map[string]struct{}
}

func (resolver loopbackFixtureResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if _, allowed := resolver.hosts[host]; !allowed {
		return nil, errors.New("fixture host is not allowlisted")
	}
	return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
}
