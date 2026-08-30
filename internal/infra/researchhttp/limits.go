package researchhttp

import (
	"context"
	"sync"
	"time"
)

const maximumTrackedRateHosts = maximumConcurrentRequests * 4

// requestConcurrency bounds attempts globally and per normalized target host.
// A host entry exists only while callers hold or wait for one of its slots.
type requestConcurrency struct {
	global       chan struct{}
	perHostLimit int
	mu           sync.Mutex
	hosts        map[string]*hostSlots
}

type hostSlots struct {
	active chan struct{}
	refs   int
}

func newRequestConcurrency(global, perHost int) *requestConcurrency {
	return &requestConcurrency{
		global:       make(chan struct{}, global),
		perHostLimit: perHost,
		hosts:        make(map[string]*hostSlots),
	}
}

func (limits *requestConcurrency) acquire(ctx context.Context, hostname string) (func(), error) {
	limits.mu.Lock()
	slots := limits.hosts[hostname]
	if slots == nil {
		slots = &hostSlots{active: make(chan struct{}, limits.perHostLimit)}
		limits.hosts[hostname] = slots
	}
	slots.refs++
	limits.mu.Unlock()

	select {
	case slots.active <- struct{}{}:
	case <-ctx.Done():
		limits.dropReference(hostname, slots)
		return nil, ctx.Err()
	}
	select {
	case limits.global <- struct{}{}:
	case <-ctx.Done():
		<-slots.active
		limits.dropReference(hostname, slots)
		return nil, ctx.Err()
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			<-limits.global
			<-slots.active
			limits.dropReference(hostname, slots)
		})
	}, nil
}

func (limits *requestConcurrency) dropReference(hostname string, slots *hostSlots) {
	limits.mu.Lock()
	defer limits.mu.Unlock()
	slots.refs--
	if slots.refs == 0 && limits.hosts[hostname] == slots {
		delete(limits.hosts, hostname)
	}
}

// hostIntervalLimiter schedules attempt starts for a host at least interval
// apart. It uses the client's injectable clock/sleep boundary so tests stay
// deterministic and cancellation remains immediate.
type hostIntervalLimiter struct {
	interval time.Duration
	now      func() time.Time
	sleep    func(context.Context, time.Duration) error
	mu       sync.Mutex
	next     map[string]time.Time
}

func newHostIntervalLimiter(interval time.Duration, now func() time.Time, sleep func(context.Context, time.Duration) error) *hostIntervalLimiter {
	return &hostIntervalLimiter{interval: interval, now: now, sleep: sleep, next: make(map[string]time.Time)}
}

func (limiter *hostIntervalLimiter) Wait(ctx context.Context, host string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		limiter.mu.Lock()
		now := limiter.now()
		for trackedHost, next := range limiter.next {
			if trackedHost != host && !next.After(now) {
				delete(limiter.next, trackedHost)
			}
		}
		_, tracked := limiter.next[host]
		if tracked || len(limiter.next) < maximumTrackedRateHosts {
			start := now
			if next := limiter.next[host]; next.After(start) {
				start = next
			}
			limiter.next[host] = start.Add(limiter.interval)
			limiter.mu.Unlock()
			if delay := start.Sub(now); delay > 0 {
				return limiter.sleep(ctx, delay)
			}
			return ctx.Err()
		}
		earliest := time.Time{}
		for _, next := range limiter.next {
			if earliest.IsZero() || next.Before(earliest) {
				earliest = next
			}
		}
		limiter.mu.Unlock()
		if err := limiter.sleep(ctx, earliest.Sub(now)); err != nil {
			return err
		}
	}
}
