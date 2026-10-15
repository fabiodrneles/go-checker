package watch

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/metrics"
	"github.com/fabiodrneles/go-checker/internal/notify"
	"github.com/fabiodrneles/go-checker/internal/probe"
)

// script answers each round with the next status (0 = network error).
type script struct {
	mu    sync.Mutex
	codes []int
}

func (s *script) Do(context.Context, probe.Request) (*probe.Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.codes[0]
	if len(s.codes) > 1 {
		s.codes = s.codes[1:]
	}
	if c == 0 {
		return nil, errors.New("connection refused")
	}
	return &probe.Response{StatusCode: c, Status: "x", Latency: time.Millisecond}, nil
}

type recorder struct {
	events []notify.Event
	fail   bool
}

func (r *recorder) Name() string { return "rec" }
func (r *recorder) Notify(_ context.Context, e notify.Event) error {
	r.events = append(r.events, e)
	if r.fail {
		return errors.New("boom")
	}
	return nil
}

func watcher(t *testing.T, confirm int, codes ...int) (*Watcher, *recorder, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cfg, err := config.FromURLs([]string{"https://api.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	var out, errs bytes.Buffer
	return &Watcher{Checks: cfg.Checks, Doer: &script{codes: codes}, Confirm: confirm, Interval: time.Millisecond,
		Metrics: metrics.New(10), Notifiers: []notify.Notifier{rec}, Out: &out, Errs: &errs}, rec, &out, &errs
}

// 007 AC-2: one bad round is noise; two in a row is an incident.
func TestFlapProtection(t *testing.T) {
	w, rec, out, _ := watcher(t, 2, 200, 503, 200, 503, 503, 503, 200, 200)
	for i := 0; i < 8; i++ {
		w.Round(context.Background())
	}
	if len(rec.events) != 2 {
		t.Fatalf("events %+v\n%s", rec.events, out)
	}
	down, up := rec.events[0], rec.events[1]
	if down.From != "UP" || down.To != "DOWN" || down.Reasons[0] != "status 503" || up.From != "DOWN" || up.To != "UP" || len(up.Reasons) != 0 {
		t.Fatalf("events %+v", rec.events)
	}
	if !strings.Contains(out.String(), "api.example.com: UP → DOWN [status 503]") || !strings.Contains(out.String(), "0 up, 0 warn, 1 down") {
		t.Fatalf("output\n%s", out)
	}
	if v, _ := w.Metrics.Quantile("https://api.example.com", 0.5); v != 0.001 {
		t.Fatalf("metrics not fed: %v", v)
	}
}

func TestConfirmOneAlertsAtOnce(t *testing.T) {
	w, rec, _, _ := watcher(t, 0, 200, 0, 0)
	w.Round(context.Background())
	w.Round(context.Background())
	if len(rec.events) != 1 || rec.events[0].Reasons[0] != "connection refused" {
		t.Fatalf("%+v", rec.events)
	}
}

func TestNotifierFailureIsLoggedNotFatal(t *testing.T) {
	w, rec, _, errs := watcher(t, 1, 200, 503)
	rec.fail = true
	w.Round(context.Background())
	if ev := w.Round(context.Background()); len(ev) != 1 || !strings.Contains(errs.String(), "warning: rec alert for https://api.example.com failed: boom") {
		t.Fatalf("events %v, errs %q", ev, errs)
	}
}

// 007 AC-5: a cancelled context ends Run cleanly.
func TestRunStopsOnCancel(t *testing.T) {
	w, _, out, _ := watcher(t, 1, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := w.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "1 up") < 2 {
		t.Fatalf("expected several rounds:\n%s", out)
	}
	cancelled, stop := context.WithCancel(context.Background())
	stop()
	if ev := (&Watcher{Checks: w.Checks, Doer: w.Doer}).Round(cancelled); ev != nil {
		t.Fatal("a cancelled round reports nothing")
	}
}
