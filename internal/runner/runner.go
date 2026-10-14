// Package runner executes checks concurrently, with retries, and decides
// whether each one is up.
package runner

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/probe"
)

// State is the verdict for one check.
type State string

// States, from best to worst.
const (
	Up   State = "UP"
	Warn State = "WARN"
	Down State = "DOWN"
)

// Result is the outcome of one check after all attempts.
type Result struct {
	Check    config.Check
	State    State
	Response *probe.Response
	Err      error
	Reasons  []string
	Attempts int
	Duration time.Duration
}

// Doer sends one request; *probe.Prober implements it.
type Doer interface {
	Do(ctx context.Context, r probe.Request) (*probe.Response, error)
}

// Options tune a run.
type Options struct {
	// Concurrency caps how many checks run at once (default 10).
	Concurrency int
	// BaseBackoff is the wait before the first retry; it doubles each time,
	// with jitter, up to MaxBackoff.
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
	// Sleep waits between attempts; tests replace it.
	Sleep func(ctx context.Context, d time.Duration)
}

func (o *Options) defaults() {
	if o.Concurrency <= 0 {
		o.Concurrency = 10
	}
	if o.BaseBackoff <= 0 {
		o.BaseBackoff = 250 * time.Millisecond
	}
	if o.MaxBackoff <= 0 {
		o.MaxBackoff = 5 * time.Second
	}
	if o.Sleep == nil {
		o.Sleep = sleep
	}
}

// Run executes every check and returns the results in the order of the input.
func Run(ctx context.Context, d Doer, checks []config.Check, opts Options) []Result {
	opts.defaults()
	results := make([]Result, len(checks))
	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup
	for i := range checks {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = runOne(ctx, d, checks[i], opts)
		}(i)
	}
	wg.Wait()
	return results
}

func runOne(ctx context.Context, d Doer, c config.Check, opts Options) Result {
	start := time.Now()
	attempts := 1 + *c.Retries
	var res Result
	for n := 1; n <= attempts; n++ {
		if n > 1 {
			opts.Sleep(ctx, Backoff(n-1, opts.BaseBackoff, opts.MaxBackoff))
		}
		res = attempt(ctx, d, c)
		res.Attempts = n
		if !retryable(res) || ctx.Err() != nil {
			break
		}
	}
	res.Duration = time.Since(start)
	return res
}

func attempt(ctx context.Context, d Doer, c config.Check) Result {
	resp, err := d.Do(ctx, probe.Request{
		Method:   c.Method,
		URL:      c.URL,
		Headers:  c.Headers,
		Body:     c.Body,
		Timeout:  c.Timeout.Duration,
		Insecure: c.Insecure,
	})
	r := Result{Check: c, Response: resp, Err: err}
	Evaluate(&r, time.Now())
	return r
}

// Evaluate sets State and Reasons from the response, as of now.
func Evaluate(r *Result, now time.Time) {
	r.State, r.Reasons = Up, nil
	if r.Err != nil {
		r.fail(Down, r.Err.Error())
		return
	}
	if r.Response.StatusCode >= 400 {
		r.fail(Down, fmt.Sprintf("status %d", r.Response.StatusCode))
	}
	if t := r.Response.TLS; t != nil {
		switch days := t.DaysLeft(now); {
		case days < 0 || !now.Before(t.NotAfter):
			r.fail(Down, fmt.Sprintf("certificate expired on %s", t.NotAfter.Format(time.DateOnly)))
		case r.Check.TLSMinDays != nil && days < *r.Check.TLSMinDays:
			r.fail(Warn, fmt.Sprintf("certificate expires in %d days (minimum %d)", days, *r.Check.TLSMinDays))
		}
	}
}

// fail records a reason and keeps the worst state seen.
func (r *Result) fail(s State, reason string) {
	if rank[s] > rank[r.State] {
		r.State = s
	}
	r.Reasons = append(r.Reasons, reason)
}

var rank = map[State]int{Up: 0, Warn: 1, Down: 2}

// retryable reports whether another attempt could change the verdict: network
// errors and 5xx answers are transient, a 4xx is not.
func retryable(r Result) bool {
	if r.State != Down {
		return false
	}
	return r.Response == nil || r.Response.StatusCode >= 500
}

// Backoff returns the wait before retry n (1-based): base·2^(n-1), capped at
// max, with up to 50% random jitter so that clients do not retry in lockstep.
func Backoff(n int, base, max time.Duration) time.Duration {
	d := base
	for i := 1; i < n && d < max; i++ {
		d *= 2
	}
	if d > max {
		d = max
	}
	return d/2 + rand.N(d/2+1)
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// Summary counts results by state.
type Summary struct{ Up, Warn, Down int }

// Summarize counts the results.
func Summarize(rs []Result) Summary {
	var s Summary
	for _, r := range rs {
		switch r.State {
		case Up:
			s.Up++
		case Warn:
			s.Warn++
		default:
			s.Down++
		}
	}
	return s
}
