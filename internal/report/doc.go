package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fabiodrneles/go-checker/internal/runner"
)

// Doc is the machine-readable report. Its JSON form is a stable contract
// (spec 005 FR-2): fields are only ever added.
type Doc struct {
	Tool        string    `json:"tool"`
	Version     string    `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	Summary     DocSum    `json:"summary"`
	Checks      []DocItem `json:"checks"`
}

// DocSum counts the checks.
type DocSum struct {
	Total      int   `json:"total"`
	Up         int   `json:"up"`
	Warn       int   `json:"warn"`
	Down       int   `json:"down"`
	DurationMS int64 `json:"duration_ms"`
}

// DocItem is one check.
type DocItem struct {
	Name      string      `json:"name"`
	URL       string      `json:"url"`
	Method    string      `json:"method"`
	State     string      `json:"state"`
	Status    int         `json:"status,omitempty"`
	LatencyMS float64     `json:"latency_ms,omitempty"`
	Attempts  int         `json:"attempts"`
	Reasons   []string    `json:"reasons"`
	Timings   *DocTimings `json:"timings,omitempty"`
	TLS       *DocTLS     `json:"tls,omitempty"`
}

// DocTimings are the phases in milliseconds.
type DocTimings struct {
	DNS     float64 `json:"dns_ms"`
	Connect float64 `json:"connect_ms"`
	TLS     float64 `json:"tls_ms"`
	TTFB    float64 `json:"ttfb_ms"`
	Total   float64 `json:"total_ms"`
}

// DocTLS describes the certificate.
type DocTLS struct {
	ExpiresAt time.Time `json:"expires_at"`
	DaysLeft  int       `json:"days_left"`
	Subject   string    `json:"subject,omitempty"`
	Issuer    string    `json:"issuer,omitempty"`
	Version   string    `json:"version,omitempty"`
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// Build turns results into a Doc.
func Build(rs []runner.Result, elapsed time.Duration, version string, now time.Time) Doc {
	s := runner.Summarize(rs)
	d := Doc{
		Tool: "go-checker", Version: version, GeneratedAt: now.UTC().Truncate(time.Second),
		Summary: DocSum{Total: len(rs), Up: s.Up, Warn: s.Warn, Down: s.Down, DurationMS: elapsed.Milliseconds()},
		Checks:  make([]DocItem, 0, len(rs)),
	}
	for _, r := range rs {
		it := DocItem{Name: r.Check.Name, URL: r.Check.URL, Method: r.Check.Method, State: string(r.State), Attempts: r.Attempts, Reasons: r.Reasons}
		if it.Reasons == nil {
			it.Reasons = []string{}
		}
		if p := r.Response; p != nil {
			it.Status, it.LatencyMS = p.StatusCode, ms(p.Latency)
			t := p.Timings
			it.Timings = &DocTimings{DNS: ms(t.DNS), Connect: ms(t.Connect), TLS: ms(t.TLS), TTFB: ms(t.TTFB), Total: ms(t.Total)}
			if c := p.TLS; c != nil {
				it.TLS = &DocTLS{ExpiresAt: c.NotAfter.UTC(), DaysLeft: c.DaysLeft(now), Subject: c.Subject, Issuer: c.Issuer, Version: c.Version}
			}
		}
		d.Checks = append(d.Checks, it)
	}
	return d
}

// JSON writes the Doc, indented.
func JSON(w io.Writer, d Doc) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}

type junitSuites struct {
	XMLName  xml.Name     `xml:"testsuites"`
	Name     string       `xml:"name,attr"`
	Tests    int          `xml:"tests,attr"`
	Failures int          `xml:"failures,attr"`
	Time     string       `xml:"time,attr"`
	Suites   []junitSuite `xml:"testsuite"`
}

type junitSuite struct {
	Name      string      `xml:"name,attr"`
	Tests     int         `xml:"tests,attr"`
	Failures  int         `xml:"failures,attr"`
	Errors    int         `xml:"errors,attr"`
	Skipped   int         `xml:"skipped,attr"`
	Time      string      `xml:"time,attr"`
	Timestamp string      `xml:"timestamp,attr"`
	Cases     []junitCase `xml:"testcase"`
}

type junitCase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

// JUnit writes the JUnit XML that GitHub, GitLab and Jenkins read: one
// testcase per check, a failure per DOWN check (and per WARN with failOnWarn).
func JUnit(w io.Writer, d Doc, failOnWarn bool) error {
	secs := func(msv float64) string { return fmt.Sprintf("%.3f", msv/1000) }
	suite := junitSuite{Name: "go-checker", Tests: len(d.Checks), Time: secs(float64(d.Summary.DurationMS)), Timestamp: d.GeneratedAt.Format(time.RFC3339)}
	for _, c := range d.Checks {
		tc := junitCase{Name: c.Name, Classname: "go-checker." + c.Method, Time: secs(c.LatencyMS)}
		detail := c.URL + "\n" + strings.Join(c.Reasons, "\n")
		switch {
		case c.State == string(runner.Down) || (failOnWarn && c.State == string(runner.Warn)):
			tc.Failure = &junitFailure{Message: strings.Join(c.Reasons, "; "), Type: c.State, Text: detail}
			suite.Failures++
		case c.State == string(runner.Warn):
			tc.SystemOut = "WARN " + detail
		}
		suite.Cases = append(suite.Cases, tc)
	}
	doc := junitSuites{Name: "go-checker", Tests: suite.Tests, Failures: suite.Failures, Time: suite.Time, Suites: []junitSuite{suite}}
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// Markdown writes a table for pull requests and job summaries.
func Markdown(w io.Writer, d Doc) error {
	var b strings.Builder
	icon := map[string]string{"UP": "🟢", "WARN": "🟡", "DOWN": "🔴"}
	s := d.Summary
	fmt.Fprintf(&b, "### go-checker: %d up, %d warn, %d down\n\n", s.Up, s.Warn, s.Down)
	b.WriteString("| | Check | Status | Latency | Notes |\n|---|---|---|---|---|\n")
	for _, c := range d.Checks {
		status, latency := "-", "-"
		if c.Status != 0 {
			status = fmt.Sprint(c.Status)
		}
		if c.Timings != nil {
			latency = FormatDuration(time.Duration(c.LatencyMS * float64(time.Millisecond)))
		}
		notes := strings.Join(c.Reasons, "; ")
		if c.TLS != nil && notes == "" {
			notes = fmt.Sprintf("cert: %d days left", c.TLS.DaysLeft)
		}
		fmt.Fprintf(&b, "| %s %s | [%s](%s) | %s | %s | %s |\n", icon[c.State], c.State, cell(c.Name), c.URL, status, latency, cell(notes))
	}
	fmt.Fprintf(&b, "\n<sub>%s in %s · %s</sub>\n", Plural(s.Total, "check"), FormatDuration(time.Duration(s.DurationMS)*time.Millisecond), d.Version)
	_, err := io.WriteString(w, b.String())
	return err
}

// cell escapes a value for a Markdown table cell.
func cell(s string) string {
	r := strings.NewReplacer("|", `\|`, "\n", " ", "[", `\[`, "]", `\]`)
	return r.Replace(s)
}
