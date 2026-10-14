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

// Text writes a human-readable table. Color adds ANSI colors.
func Text(w io.Writer, rs []runner.Result, elapsed time.Duration, color bool) error {
	paint := func(code, s string) string {
		if !color {
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
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}
