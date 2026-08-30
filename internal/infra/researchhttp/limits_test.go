package researchhttp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestClientBoundsGlobalAndPerHostAttempts(t *testing.T) {
	t.Parallel()
	config := testConfig()
	config.MaxConcurrentRequests = 3
	config.MaxConcurrentPerHost = 2
	client := newTestClient(t, config, nil, nil, nil)

	var active atomic.Int64
	var maximum atomic.Int64
	activeByHost := make(map[string]int)
	maximumByHost := make(map[string]int)
	var mu sync.Mutex
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	client.http.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		current := active.Add(1)
		updateMaximum(&maximum, current)
		mu.Lock()
		host := request.URL.Hostname()
		activeByHost[host]++
		if activeByHost[host] > maximumByHost[host] {
			maximumByHost[host] = activeByHost[host]
		}
		mu.Unlock()
		if current == 3 {
			once.Do(func() { close(started) })
		}
		<-release
		mu.Lock()
		activeByHost[host]--
		mu.Unlock()
		active.Add(-1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/plain"}},
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    request,
		}, nil
	})

	urls := []string{
		"http://127.0.0.1/a", "http://127.0.0.1/b", "http://127.0.0.1/c",
		"http://127.0.0.2/d", "http://127.0.0.2/e", "http://127.0.0.2/f",
	}
	errorsByIndex := make([]error, len(urls))
	var wait sync.WaitGroup
	wait.Add(len(urls))
	for index, target := range urls {
		go func() {
			defer wait.Done()
			_, errorsByIndex[index] = client.Do(context.Background(), Request{URL: target})
		}()
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("three globally bounded attempts did not start")
	}
	close(release)
	wait.Wait()
	for index, err := range errorsByIndex {
		if err != nil {
			t.Fatalf("request %d: %v", index, err)
		}
	}
	if maximum.Load() != 3 {
		t.Fatalf("maximum global attempts = %d, want 3", maximum.Load())
	}
	mu.Lock()
	defer mu.Unlock()
	for host, got := range maximumByHost {
		if got > 2 {
			t.Fatalf("maximum attempts for %s = %d, want at most 2", host, got)
		}
	}
}

func TestRequestConcurrencyCancellationReleasesHostEntries(t *testing.T) {
	t.Parallel()
	limits := newRequestConcurrency(1, 1)
	release, err := limits.acquire(context.Background(), "example.test")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := limits.acquire(ctx, "example.test"); err != context.Canceled {
		t.Fatalf("canceled acquire error = %v", err)
	}
	release()
	release, err = limits.acquire(context.Background(), "example.test")
	if err != nil {
		t.Fatalf("slot was not reusable: %v", err)
	}
	release()
	limits.mu.Lock()
	tracked := len(limits.hosts)
	limits.mu.Unlock()
	if tracked != 0 {
		t.Fatalf("tracked host entries after releases = %d", tracked)
	}
}

func TestHostIntervalLimiterSchedulesPerHostAndCancels(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	var delays []time.Duration
	limiter := newHostIntervalLimiter(100*time.Millisecond, func() time.Time { return now }, func(ctx context.Context, delay time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		delays = append(delays, delay)
		now = now.Add(delay)
		return nil
	})
	for _, host := range []string{"a.test", "a.test", "b.test", "a.test"} {
		if err := limiter.Wait(context.Background(), host); err != nil {
			t.Fatal(err)
		}
	}
	if len(delays) != 2 || delays[0] != 100*time.Millisecond || delays[1] != 100*time.Millisecond {
		t.Fatalf("scheduled delays = %v", delays)
	}

	blocked := newHostIntervalLimiter(time.Second, func() time.Time { return now }, func(ctx context.Context, _ time.Duration) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err := blocked.Wait(context.Background(), "cancel.test"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := blocked.Wait(ctx, "cancel.test"); err != context.Canceled {
		t.Fatalf("canceled rate wait error = %v", err)
	}
}

func TestHostIntervalLimiterBoundsTrackedHosts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	var delays []time.Duration
	limiter := newHostIntervalLimiter(time.Second, func() time.Time { return now }, func(_ context.Context, delay time.Duration) error {
		delays = append(delays, delay)
		now = now.Add(delay)
		return nil
	})
	for index := range maximumTrackedRateHosts + 1 {
		if err := limiter.Wait(context.Background(), fmt.Sprintf("host-%04d.test", index)); err != nil {
			t.Fatal(err)
		}
	}
	if len(delays) != 1 || delays[0] != time.Second {
		t.Fatalf("capacity wait delays = %v", delays)
	}
	limiter.mu.Lock()
	tracked := len(limiter.next)
	limiter.mu.Unlock()
	if tracked > maximumTrackedRateHosts {
		t.Fatalf("tracked rate hosts = %d, maximum = %d", tracked, maximumTrackedRateHosts)
	}
}

func TestClientRejectsRaisedActiveAndRateLimits(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.MaxConcurrentRequests = maximumConcurrentRequests + 1
	if _, err := New(config, nil, nil); err == nil {
		t.Fatal("client accepted global concurrency above hard ceiling")
	}
	config = DefaultConfig()
	config.MinimumIntervalPerHost = maximumRequestInterval + time.Nanosecond
	if _, err := New(config, nil, nil); err == nil {
		t.Fatal("client accepted per-host interval above hard ceiling")
	}
}

func updateMaximum(maximum *atomic.Int64, value int64) {
	for {
		current := maximum.Load()
		if value <= current || maximum.CompareAndSwap(current, value) {
			return
		}
	}
}
