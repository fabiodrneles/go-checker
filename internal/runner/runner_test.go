package runner

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/probe"
)

func check(t *testing.T, url string, retries int) config.Check {
	t.Helper()
	cfg, err := config.File{Checks: []config.Check{{URL: url, Retries: &retries}}}.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Checks[0]
}

func noSleep(context.Context, time.Duration) {}

// 002 AC-6: transient failures are retried; the verdict comes from the last attempt.
func TestRunRetriesTransientFailures(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()
	rs := Run(context.Background(), probe.New("t"), []config.Check{check(t, srv.URL, 2)}, Options{Sleep: noSleep})
	if rs[0].State != Up || rs[0].Attempts != 3 {
		t.Fatalf("state %s after %d attempts", rs[0].State, rs[0].Attempts)
	}
}

// 002 AC-6: a 4xx is the server's final answer and is not retried.
func TestRunDoesNotRetryClientErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	rs := Run(context.Background(), probe.New("t"), []config.Check{check(t, srv.URL, 3)}, Options{Sleep: noSleep})
	if rs[0].State != Down || calls.Load() != 1 || rs[0].Reasons[0] != "status 404" {
		t.Fatalf("state %s, calls %d, reasons %v", rs[0].State, calls.Load(), rs[0].Reasons)
	}
}

func TestRunNetworkErrorIsDownAfterAllAttempts(t *testing.T) {
	var waits []time.Duration
	sleep := func(_ context.Context, d time.Duration) { waits = append(waits, d) }
	rs := Run(context.Background(), probe.New("t"), []config.Check{check(t, "http://127.0.0.1:1", 2)}, Options{Sleep: sleep})
	if rs[0].State != Down || rs[0].Attempts != 3 || rs[0].Err == nil || len(waits) != 2 {
		t.Fatalf("got %+v, waits %v", rs[0], waits)
	}
}

// 002 AC-9: checks run concurrently, bounded by Concurrency, results in input order.
func TestRunConcurrency(t *testing.T) {
	var inFlight, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		inFlight.Add(-1)
	}))
	defer srv.Close()
	var checks []config.Check
	for i := 0; i < 8; i++ {
		checks = append(checks, check(t, srv.URL+"/"+string(rune('a'+i)), 0))
	}
	rs := Run(context.Background(), probe.New("t"), checks, Options{Concurrency: 3})
	if peak.Load() > 3 || peak.Load() < 2 {
		t.Fatalf("peak concurrency %d, want 2..3", peak.Load())
	}
	for i, r := range rs {
		if r.Check.URL != checks[i].URL || r.State != Up {
			t.Fatalf("result %d = %s %s", i, r.Check.URL, r.State)
		}
	}
}

// 002 AC-11: backoff doubles, is capped, and carries jitter.
func TestBackoff(t *testing.T) {
	base, max := 100*time.Millisecond, time.Second
	for n, want := range map[int]time.Duration{1: 100 * time.Millisecond, 2: 200 * time.Millisecond, 3: 400 * time.Millisecond, 10: time.Second} {
		for i := 0; i < 50; i++ {
			got := Backoff(n, base, max)
			if got < want/2 || got > want {
				t.Fatalf("Backoff(%d) = %v, want in [%v, %v]", n, got, want/2, want)
			}
		}
	}
	seen := map[time.Duration]bool{}
	for i := 0; i < 20; i++ {
		seen[Backoff(3, base, max)] = true
	}
	if len(seen) < 2 {
		t.Fatal("no jitter")
	}
}

func TestSleepStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	sleep(ctx, time.Minute)
	sleep(context.Background(), time.Millisecond)
	if time.Since(start) > time.Second {
		t.Fatal("sleep ignored the context")
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize([]Result{{State: Up}, {State: Warn}, {State: Down}, {State: Down}})
	if s != (Summary{Up: 1, Warn: 1, Down: 2}) {
		t.Fatalf("got %+v", s)
	}
}

func TestEvaluateError(t *testing.T) {
	r := Result{Err: errors.New("boom")}
	Evaluate(&r)
	if r.State != Down || r.Reasons[0] != "boom" {
		t.Fatalf("got %+v", r)
	}
}
