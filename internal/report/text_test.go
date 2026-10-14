package report

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/probe"
	"github.com/fabiodrneles/go-checker/internal/runner"
)

func sample() []runner.Result {
	return []runner.Result{
		{Check: config.Check{Name: "api"}, State: runner.Up, Attempts: 1, Response: &probe.Response{StatusCode: 200, Status: "200 OK", Latency: 87 * time.Millisecond}},
		{Check: config.Check{Name: "a-very-long-check-name-that-goes-beyond-forty-runes"}, State: runner.Warn, Attempts: 1, Reasons: []string{"slow"}, Response: &probe.Response{StatusCode: 200, Status: "200 OK", Latency: 1240 * time.Millisecond}},
		{Check: config.Check{Name: "db"}, State: runner.Down, Attempts: 3, Err: errors.New("refused"), Reasons: []string{"refused"}},
		{Check: config.Check{Name: "web"}, State: runner.Down, Attempts: 1, Reasons: []string{"status 503"}, Response: &probe.Response{StatusCode: 503, Status: "503 Service Unavailable", Latency: 500 * time.Microsecond}},
	}
}

func TestText(t *testing.T) {
	var b bytes.Buffer
	if err := Text(&b, sample(), 1500*time.Millisecond, TextOptions{}); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"✓ UP    api", "200 OK  87ms",
		"! WARN  a-very-long-check-name-that-goes-beyond…", "1.24s", "→ slow",
		"✗ DOWN  db", "no response  (3 attempts)", "→ refused",
		"503 Service Unavailable  500µs",
		"4 checks: 1 up, 1 warn, 2 down in 1.50s",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "→ status 503") || strings.Contains(out, "\033[") {
		t.Errorf("redundant reason or color in plain output:\n%s", out)
	}
}

func TestTextTrace(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rs := []runner.Result{{Check: config.Check{Name: "api"}, State: runner.Up, Attempts: 1, Response: &probe.Response{
		StatusCode: 200, Status: "200 OK", Latency: 90 * time.Millisecond,
		Timings: probe.Timings{DNS: 3 * time.Millisecond, Connect: 10 * time.Millisecond, TLS: 25 * time.Millisecond, TTFB: 80 * time.Millisecond, Total: 90 * time.Millisecond},
		TLS:     &probe.TLSInfo{NotAfter: now.Add(40 * 24 * time.Hour), Version: "TLS 1.3", Subject: "api.example.com"},
	}}}
	var b bytes.Buffer
	if err := Text(&b, rs, time.Second, TextOptions{Trace: true, Now: now}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"dns 3ms · connect 10ms · tls 25ms · ttfb 80ms · total 90ms",
		"cert TLS 1.3 · expires 2026-02-10 (40 days) · subject api.example.com · issuer -",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, b.String())
		}
	}
	if FormatDuration(0) != "0" {
		t.Error("zero duration")
	}
}

func TestTextColor(t *testing.T) {
	var b bytes.Buffer
	if err := Text(&b, sample(), time.Second, TextOptions{Color: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), red+"✗"+reset) || !strings.Contains(b.String(), green+"✓"+reset) {
		t.Fatalf("no colors:\n%q", b.String())
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestTextWriteError(t *testing.T) {
	if err := Text(failWriter{}, sample(), time.Second, TextOptions{}); err == nil {
		t.Fatal("want write error")
	}
}
