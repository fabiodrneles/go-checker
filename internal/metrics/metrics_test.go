package metrics

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

// 007 AC-1: nearest-rank quantiles over the window.
func TestQuantiles(t *testing.T) {
	r := New(100)
	for i := 100; i >= 1; i-- { // out of order on purpose
		r.Observe(Observation{Check: "api", State: "UP", Latency: time.Duration(i) * time.Millisecond, HasLatency: true})
	}
	for q, want := range map[float64]float64{0.5: 0.050, 0.95: 0.095, 0.99: 0.099, 1: 0.1, 0: 0.001} {
		got, ok := r.Quantile("api", q)
		if !ok || got != want {
			t.Errorf("q%v = %v, want %v", q, got, want)
		}
	}
	if _, ok := r.Quantile("nope", 0.5); ok {
		t.Error("unknown check has a quantile")
	}
}

func TestWindowSlides(t *testing.T) {
	r := New(3)
	for _, ms := range []int{1000, 1000, 1000, 10, 20, 30} {
		r.Observe(Observation{Check: "a", State: "UP", Latency: time.Duration(ms) * time.Millisecond, HasLatency: true})
	}
	if got, _ := r.Quantile("a", 1); got != 0.03 {
		t.Fatalf("max of the last 3 = %v, want 0.03", got)
	}
	if New(0).window != 100 {
		t.Fatal("default window")
	}
}

// 007 AC-6: the exposition follows the Prometheus text format.
func TestWrite(t *testing.T) {
	r := New(10)
	days := 30
	r.Observe(Observation{Check: `web "main"`, State: "UP", Latency: 120 * time.Millisecond, HasLatency: true, TLSDays: &days})
	r.Observe(Observation{Check: "db", State: "DOWN"})
	r.Observe(Observation{Check: "slow", State: "WARN", Latency: 2 * time.Second, HasLatency: true})
	r.RoundDone(time.Unix(1700000000, 500e6), time.Second)
	var b bytes.Buffer
	if err := r.Write(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"# TYPE go_checker_up gauge",
		`go_checker_up{check="web \"main\""} 1`,
		`go_checker_up{check="db"} 0`,
		`go_checker_up{check="slow"} 1`,
		`go_checker_warn{check="slow"} 1`,
		"# TYPE go_checker_latency_seconds summary",
		`go_checker_latency_seconds{check="web \"main\"",quantile="0.95"} 0.12`,
		`go_checker_latency_seconds_count{check="db"} 0`,
		`go_checker_latency_seconds_sum{check="slow"} 2`,
		`go_checker_checks_total{check="db",state="DOWN"} 1`,
		`go_checker_tls_days_left{check="web \"main\""} 30`,
		"go_checker_last_round_timestamp_seconds 1700000000.5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	line := regexp.MustCompile(`^(# (HELP|TYPE) \w+ .+|[a-z_]+(\{([a-z]+="([^"\\]|\\.)*",?)+\})? [0-9.e+-]+)$`)
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if !line.MatchString(l) {
			t.Errorf("bad exposition line %q", l)
		}
	}
	if strings.Contains(out, `go_checker_tls_days_left{check="db"}`) {
		t.Error("db has no certificate")
	}
}
