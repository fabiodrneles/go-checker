// Package watch runs the checks on an interval, keeps the metrics and sends
// alerts on confirmed state changes.
package watch

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/metrics"
	"github.com/fabiodrneles/go-checker/internal/notify"
	"github.com/fabiodrneles/go-checker/internal/runner"
)

// Watcher holds everything a monitoring loop needs.
type Watcher struct {
	Checks    []config.Check
	Doer      runner.Doer
	Options   runner.Options
	Interval  time.Duration
	Confirm   int // rounds a new state must last before it counts (flap protection)
	Metrics   *metrics.Registry
	Notifiers []notify.Notifier
	Out       io.Writer // one line per round, one per change
	Errs      io.Writer // notifier failures
	Now       func() time.Time

	tracks map[string]*track
}

// track is the flap filter of one check.
type track struct {
	stable  runner.State
	pending runner.State
	count   int
}

// Run loops until ctx is done. The first round sets each check's baseline
// without alerting.
func (w *Watcher) Run(ctx context.Context) error {
	if w.Now == nil {
		w.Now = time.Now
	}
	if w.Confirm < 1 {
		w.Confirm = 1
	}
	t := time.NewTicker(w.Interval)
	defer t.Stop()
	for {
		w.Round(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// Round runs the checks once and returns the confirmed changes it found.
func (w *Watcher) Round(ctx context.Context) []notify.Event {
	if w.tracks == nil {
		w.tracks = map[string]*track{}
	}
	if w.Now == nil {
		w.Now = time.Now
	}
	start := w.Now()
	rs := runner.Run(ctx, w.Doer, w.Checks, w.Options)
	if ctx.Err() != nil {
		return nil // interrupted mid-round: the results are not real failures
	}
	now := w.Now()
	var events []notify.Event
	for _, r := range rs {
		w.observe(r, now)
		if e, ok := w.step(r, now); ok {
			events = append(events, e)
		}
	}
	if w.Metrics != nil {
		w.Metrics.RoundDone(now, now.Sub(start))
	}
	s := runner.Summarize(rs)
	w.printf(w.Out, "%s  %d up, %d warn, %d down\n", now.Format("15:04:05"), s.Up, s.Warn, s.Down)
	for _, e := range events {
		line := fmt.Sprintf("          %s: %s → %s", e.Name, e.From, e.To)
		if len(e.Reasons) > 0 {
			line += " · " + strings.Join(e.Reasons, "; ")
		}
		w.printf(w.Out, "%s\n", line)
		for _, n := range w.Notifiers {
			if err := n.Notify(ctx, e); err != nil {
				w.printf(w.Errs, "warning: %s alert for %s failed: %v\n", n.Name(), e.Name, err)
			}
		}
	}
	return events
}

func (w *Watcher) observe(r runner.Result, now time.Time) {
	if w.Metrics == nil {
		return
	}
	o := metrics.Observation{Check: r.Check.Name, State: string(r.State)}
	if r.Response != nil {
		o.Latency, o.HasLatency = r.Response.Latency, true
		if r.Response.TLS != nil {
			d := r.Response.TLS.DaysLeft(now)
			o.TLSDays = &d
		}
	}
	w.Metrics.Observe(o)
}

// step runs the flap filter: a state different from the stable one must be
// seen Confirm rounds in a row before it becomes stable and is reported.
func (w *Watcher) step(r runner.Result, now time.Time) (notify.Event, bool) {
	tr, ok := w.tracks[r.Check.Name]
	if !ok {
		w.tracks[r.Check.Name] = &track{stable: r.State}
		return notify.Event{}, false
	}
	if r.State == tr.stable {
		tr.pending, tr.count = "", 0
		return notify.Event{}, false
	}
	if r.State == tr.pending {
		tr.count++
	} else {
		tr.pending, tr.count = r.State, 1
	}
	if tr.count < w.Confirm {
		return notify.Event{}, false
	}
	e := notify.Event{Name: r.Check.Name, URL: r.Check.URL, From: string(tr.stable), To: string(r.State), Reasons: r.Reasons, At: now.UTC()}
	if e.Reasons == nil {
		e.Reasons = []string{}
	}
	tr.stable, tr.pending, tr.count = r.State, "", 0
	return e, true
}

func (w *Watcher) printf(out io.Writer, format string, a ...any) {
	if out != nil {
		_, _ = fmt.Fprintf(out, format, a...)
	}
}
