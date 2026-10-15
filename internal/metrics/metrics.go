// Package metrics keeps per-check measurements and writes them in the
// Prometheus text exposition format (no client library, D4).
package metrics

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Quantiles exported for the latency summary.
var Quantiles = []float64{0.5, 0.95, 0.99}

// Registry holds the metrics of every check. It is safe for concurrent use.
type Registry struct {
	mu     sync.Mutex
	window int
	checks map[string]*series
	order  []string
	last   time.Time
}

type series struct {
	up       float64
	warn     float64
	lat      []float64 // ring buffer of the last latencies, in seconds
	next     int
	count    uint64
	sum      float64
	states   map[string]uint64
	tlsDays  float64
	hasTLS   bool
	duration float64
}

// New returns a registry that keeps the last window latencies per check.
func New(window int) *Registry {
	if window < 1 {
		window = 100
	}
	return &Registry{window: window, checks: map[string]*series{}}
}

// Observation is one check result, as the registry sees it.
type Observation struct {
	Check   string
	State   string // UP, WARN or DOWN
	Latency time.Duration
	// HasLatency is false when no response came back.
	HasLatency bool
	TLSDays    *int
}

// Observe records one result.
func (r *Registry) Observe(o Observation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.checks[o.Check]
	if !ok {
		s = &series{states: map[string]uint64{"UP": 0, "WARN": 0, "DOWN": 0}}
		r.checks[o.Check] = s
		r.order = append(r.order, o.Check)
	}
	s.up, s.warn = 1, 0
	switch o.State {
	case "DOWN":
		s.up = 0
	case "WARN":
		s.warn = 1
	}
	s.states[o.State]++
	if o.HasLatency {
		v := o.Latency.Seconds()
		if len(s.lat) < r.window {
			s.lat = append(s.lat, v)
		} else {
			s.lat[s.next] = v
			s.next = (s.next + 1) % r.window
		}
		s.count++
		s.sum += v
	}
	if o.TLSDays != nil {
		s.tlsDays, s.hasTLS = float64(*o.TLSDays), true
	}
}

// RoundDone marks the end of a round.
func (r *Registry) RoundDone(at time.Time, took time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last = at
	for _, s := range r.checks {
		s.duration = took.Seconds()
	}
}

// Quantile returns the q-quantile of the window (nearest rank), and false
// when there is no measurement yet.
func (r *Registry) Quantile(check string, q float64) (float64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.checks[check]
	if !ok || len(s.lat) == 0 {
		return 0, false
	}
	return quantile(s.lat, q), true
}

func quantile(values []float64, q float64) float64 {
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	i := int(math.Ceil(q*float64(len(v)))) - 1
	return v[max(0, min(i, len(v)-1))]
}

// Write renders every metric in the Prometheus text format.
func (r *Registry) Write(w io.Writer) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var b strings.Builder
	family := func(name, typ, help string, rows func(add func(labels, value string))) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, typ)
		rows(func(labels, value string) { fmt.Fprintf(&b, "%s%s %s\n", name, labels, value) })
	}
	each := func(fn func(name string, s *series)) {
		for _, n := range r.order {
			fn(n, r.checks[n])
		}
	}
	family("go_checker_up", "gauge", "1 when the check passed (UP or WARN) in the last round, 0 when DOWN.", func(add func(string, string)) {
		each(func(n string, s *series) { add(label("check", n), num(s.up)) })
	})
	family("go_checker_warn", "gauge", "1 when the check passed with a warning in the last round.", func(add func(string, string)) {
		each(func(n string, s *series) { add(label("check", n), num(s.warn)) })
	})
	family("go_checker_latency_seconds", "summary", "Latency of the last responses (sliding window).", func(add func(string, string)) {
		each(func(n string, s *series) {
			if len(s.lat) > 0 {
				for _, q := range Quantiles {
					add(label("check", n, "quantile", num(q)), num(quantile(s.lat, q)))
				}
			}
		})
		each(func(n string, s *series) {
			fmt.Fprintf(&b, "go_checker_latency_seconds_sum%s %s\n", label("check", n), num(s.sum))
			fmt.Fprintf(&b, "go_checker_latency_seconds_count%s %d\n", label("check", n), s.count)
		})
	})
	family("go_checker_checks_total", "counter", "Checks run, by resulting state.", func(add func(string, string)) {
		each(func(n string, s *series) {
			for _, st := range []string{"UP", "WARN", "DOWN"} {
				add(label("check", n, "state", st), fmt.Sprint(s.states[st]))
			}
		})
	})
	family("go_checker_tls_days_left", "gauge", "Days until the server certificate expires.", func(add func(string, string)) {
		each(func(n string, s *series) {
			if s.hasTLS {
				add(label("check", n), num(s.tlsDays))
			}
		})
	})
	family("go_checker_last_round_timestamp_seconds", "gauge", "Unix time of the last completed round.", func(add func(string, string)) {
		if !r.last.IsZero() {
			add("", num(float64(r.last.UnixMilli())/1000))
		}
	})
	_, err := io.WriteString(w, b.String())
	return err
}

// label renders {k="v",...} with Prometheus escaping.
func label(kv ...string) string {
	esc := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	parts := make([]string, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, kv[i], esc.Replace(kv[i+1])))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func num(f float64) string {
	return strings.TrimSuffix(strings.TrimRight(fmt.Sprintf("%.6f", f), "0"), ".")
}
