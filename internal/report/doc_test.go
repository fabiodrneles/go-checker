package report

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/probe"
	"github.com/fabiodrneles/go-checker/internal/runner"
)

var now = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

func docSample() Doc {
	rs := []runner.Result{
		{Check: config.Check{Name: "api", URL: "https://api.example.com", Method: "GET"}, State: runner.Up, Attempts: 1, Response: &probe.Response{
			StatusCode: 200, Latency: 87500 * time.Microsecond,
			Timings: probe.Timings{DNS: time.Millisecond, Connect: 2 * time.Millisecond, TLS: 10 * time.Millisecond, TTFB: 80 * time.Millisecond, Total: 87500 * time.Microsecond},
			TLS:     &probe.TLSInfo{NotAfter: now.Add(60 * 24 * time.Hour), Subject: "api.example.com", Issuer: "R11", Version: "TLS 1.3"},
		}},
		{Check: config.Check{Name: "slow | page", URL: "https://example.com/slow", Method: "GET"}, State: runner.Warn, Attempts: 1, Reasons: []string{"latency 2s above max_latency 1s"}, Response: &probe.Response{StatusCode: 200, Latency: 2 * time.Second}},
		{Check: config.Check{Name: "db", URL: "http://db.internal:8080/health", Method: "HEAD"}, State: runner.Down, Attempts: 3, Reasons: []string{"connection refused"}},
	}
	return Build(rs, 2100*time.Millisecond, "go-checker v1.0.0", now)
}

// 005 AC-1: the JSON report has the documented, stable fields.
func TestJSONSchema(t *testing.T) {
	var b bytes.Buffer
	if err := JSON(&b, docSample()); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, b.String())
	}
	for _, k := range []string{"tool", "version", "generated_at", "summary", "checks"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing %q", k)
		}
	}
	sum := got["summary"].(map[string]any)
	if sum["total"] != 3.0 || sum["up"] != 1.0 || sum["warn"] != 1.0 || sum["down"] != 1.0 || sum["duration_ms"] != 2100.0 {
		t.Errorf("summary %v", sum)
	}
	api := got["checks"].([]any)[0].(map[string]any)
	for k, want := range map[string]any{"name": "api", "url": "https://api.example.com", "method": "GET", "state": "UP", "status": 200.0, "latency_ms": 87.5, "attempts": 1.0} {
		if api[k] != want {
			t.Errorf("checks[0].%s = %v, want %v", k, api[k], want)
		}
	}
	if r, ok := api["reasons"].([]any); !ok || len(r) != 0 {
		t.Errorf("reasons must be an empty list, got %v", api["reasons"])
	}
	tm := api["timings"].(map[string]any)
	if tm["dns_ms"] != 1.0 || tm["tls_ms"] != 10.0 || tm["ttfb_ms"] != 80.0 || tm["total_ms"] != 87.5 {
		t.Errorf("timings %v", tm)
	}
	tl := api["tls"].(map[string]any)
	if tl["days_left"] != 60.0 || tl["expires_at"] != "2026-06-30T12:00:00Z" || tl["issuer"] != "R11" {
		t.Errorf("tls %v", tl)
	}
	db := got["checks"].([]any)[2].(map[string]any)
	if _, ok := db["status"]; ok || db["timings"] != nil || db["tls"] != nil {
		t.Errorf("a check without a response has no status, timings or tls: %v", db)
	}
}

// 005 AC-2: DOWN checks are JUnit failures; WARN only with failOnWarn.
func TestJUnit(t *testing.T) {
	for _, tc := range []struct {
		failOnWarn bool
		failures   string
	}{{false, `failures="1"`}, {true, `failures="2"`}} {
		var b bytes.Buffer
		if err := JUnit(&b, docSample(), tc.failOnWarn); err != nil {
			t.Fatal(err)
		}
		out := b.String()
		var v struct{}
		if err := xml.Unmarshal(b.Bytes(), &v); err != nil {
			t.Fatalf("invalid XML: %v", err)
		}
		for _, want := range []string{xml.Header[:20], `<testsuites name="go-checker" tests="3" ` + tc.failures, `<testcase name="db" classname="go-checker.HEAD" time="0.000">`, `<failure message="connection refused" type="DOWN">`, `time="0.087"`} {
			if !strings.Contains(out, want) {
				t.Errorf("failOnWarn=%v: output lacks %q:\n%s", tc.failOnWarn, want, out)
			}
		}
		if !tc.failOnWarn && !strings.Contains(out, "<system-out>WARN https://example.com/slow") {
			t.Errorf("warn not in system-out:\n%s", out)
		}
	}
}

// 005 AC-3: one table row per check, with cells escaped.
func TestMarkdown(t *testing.T) {
	var b bytes.Buffer
	if err := Markdown(&b, docSample()); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"### go-checker: 1 up, 1 warn, 1 down",
		"| 🟢 UP | [api](https://api.example.com) | 200 | 87ms | cert: 60 days left |",
		`| 🟡 WARN | [slow \| page](https://example.com/slow) | 200 | 2.00s | latency 2s above max_latency 1s |`,
		"| 🔴 DOWN | [db](http://db.internal:8080/health) | - | - | connection refused |",
		"<sub>3 checks in 2.10s · go-checker v1.0.0</sub>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "\n| ") != 4 {
		t.Errorf("want a header and 3 rows:\n%s", out)
	}
}

func TestRenderWriteErrors(t *testing.T) {
	d := docSample()
	if JUnit(failWriter{}, d, false) == nil || Markdown(failWriter{}, d) == nil || JSON(failWriter{}, d) == nil {
		t.Fatal("write errors must surface")
	}
	if Plural(1, "check") != "1 check" || Plural(0, "check") != "0 checks" {
		t.Fatal("plural")
	}
}
