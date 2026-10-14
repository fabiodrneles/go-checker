// Package report renders results for people and for machines.
package report

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fabiodrneles/go-checker/internal/runner"
)

const (
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	dim    = "\033[2m"
	reset  = "\033[0m"
)

// TextOptions tune the human-readable output.
type TextOptions struct {
	Color bool // ANSI colors
	Trace bool // per-phase timings and certificate details (spec 003 FR-4)
	// Now dates the certificate expiry; zero means time.Now().
	Now time.Time
}

// Text writes a human-readable table.
func Text(w io.Writer, rs []runner.Result, elapsed time.Duration, opts TextOptions) error {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	paint := func(code, s string) string {
		if !opts.Color {
			return s
		}
		return code + s + reset
	}
	width := 0
	for _, r := range rs {
		width = max(width, utf8.RuneCountInString(r.Check.Name))
	}
	width = min(width, 40)

	var b strings.Builder
	for _, r := range rs {
		mark, code := "✓", green
		switch r.State {
		case runner.Warn:
			mark, code = "!", yellow
		case runner.Down:
			mark, code = "✗", red
		}
		name := truncate(r.Check.Name, width)
		fmt.Fprintf(&b, "%s %s  %-*s  %s", paint(code, mark), paint(code, fmt.Sprintf("%-4s", r.State)), width, name, detail(r))
		if r.Response != nil {
			fmt.Fprintf(&b, "  %s", paint(dim, FormatDuration(r.Response.Latency)))
		}
		if r.Attempts > 1 {
			fmt.Fprintf(&b, "  %s", paint(dim, fmt.Sprintf("(%d attempts)", r.Attempts)))
		}
		b.WriteByte('\n')
		if opts.Trace && r.Response != nil {
			for _, line := range traceLines(r, opts.Now) {
				fmt.Fprintf(&b, "        %s\n", paint(dim, line))
			}
		}
		if r.State != runner.Up {
			for _, reason := range reasons(r) {
				fmt.Fprintf(&b, "        %s %s\n", paint(code, "→"), reason)
			}
		}
	}
	s := runner.Summarize(rs)
	fmt.Fprintf(&b, "\n%d checks: %s, %s, %s in %s\n", len(rs),
		paint(green, fmt.Sprintf("%d up", s.Up)),
		paint(yellow, fmt.Sprintf("%d warn", s.Warn)),
		paint(red, fmt.Sprintf("%d down", s.Down)),
		FormatDuration(elapsed))
	_, err := io.WriteString(w, b.String())
	return err
}

func traceLines(r runner.Result, now time.Time) []string {
	t := r.Response.Timings
	lines := []string{fmt.Sprintf("dns %s · connect %s · tls %s · ttfb %s · total %s",
		FormatDuration(t.DNS), FormatDuration(t.Connect), FormatDuration(t.TLS), FormatDuration(t.TTFB), FormatDuration(t.Total))}
	if c := r.Response.TLS; c != nil {
		lines = append(lines, fmt.Sprintf("cert %s · expires %s (%d days) · subject %s · issuer %s",
			c.Version, c.NotAfter.Format(time.DateOnly), c.DaysLeft(now), orDash(c.Subject), orDash(c.Issuer)))
	}
	return lines
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func detail(r runner.Result) string {
	if r.Response == nil {
		return "no response"
	}
	return r.Response.Status
}

// reasons lists what went wrong, skipping a status line already shown.
func reasons(r runner.Result) []string {
	var out []string
	for _, s := range r.Reasons {
		if r.Response != nil && s == fmt.Sprintf("status %d", r.Response.StatusCode) {
			continue
		}
		out = append(out, s)
	}
	return out
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	rs := []rune(s)
	return string(rs[:n-1]) + "…"
}

// FormatDuration rounds for display: 87ms, 1.24s.
func FormatDuration(d time.Duration) string {
	switch {
	case d == 0:
		return "0"
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}
