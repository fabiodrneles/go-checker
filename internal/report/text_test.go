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
	if err := Text(&b, sample(), 1500*time.Millisecond, false); err != nil {
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

func TestTextColor(t *testing.T) {
	var b bytes.Buffer
	if err := Text(&b, sample(), time.Second, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), red+"✗"+reset) || !strings.Contains(b.String(), green+"✓"+reset) {
		t.Fatalf("no colors:\n%q", b.String())
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestTextWriteError(t *testing.T) {
	if err := Text(failWriter{}, sample(), time.Second, false); err == nil {
		t.Fatal("want write error")
	}
}
