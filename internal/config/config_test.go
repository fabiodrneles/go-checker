package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseResolvesDefaults(t *testing.T) {
	cfg, err := Parse([]byte(`
defaults:
  timeout: 3s
  retries: 2
  headers: {X-Team: sre}
checks:
  - name: api
    url: https://api.example.com/health
    method: head
    headers: {Accept: application/json}
  - url: https://example.com
    timeout: 500ms
    retries: 0
`))
	if err != nil {
		t.Fatal(err)
	}
	a, b := cfg.Checks[0], cfg.Checks[1]
	if a.Method != "HEAD" || a.Timeout.Duration != 3*time.Second || *a.Retries != 2 {
		t.Errorf("defaults not applied: %+v", a)
	}
	if *a.TLSMinDays != DefaultTLSMinDays {
		t.Errorf("tls_min_days default = %d", *a.TLSMinDays)
	}
	if a.Headers["X-Team"] != "sre" || a.Headers["Accept"] != "application/json" {
		t.Errorf("headers not merged: %v", a.Headers)
	}
	if b.Name != "https://example.com" || b.Method != "GET" || b.Timeout.Duration != 500*time.Millisecond || *b.Retries != 0 {
		t.Errorf("check values lost: %+v", b)
	}
}

// 002 AC-7: the v0 "sites:" format still works.
func TestParseLegacySites(t *testing.T) {
	cfg, err := Parse([]byte("sites:\n  - url: https://a.example\n  - url: https://b.example\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Checks) != 2 || cfg.Checks[0].Timeout.Duration != DefaultTimeout {
		t.Fatalf("got %+v", cfg.Checks)
	}
}

// 002 AC-8: a typo is an error that names the line, never a silent default.
func TestParseRejectsUnknownKeys(t *testing.T) {
	_, err := Parse([]byte("checks:\n  - url: https://a.example\n    timout: 3s\n"))
	if err == nil || err.Error() != `line 3: unknown key "timout" (did you mean "timeout"?)` {
		t.Fatalf("err = %v", err)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"":                                      "no checks defined",
		"checks: []":                            "no checks defined",
		"checks:\n  - name: x\n":                "url is required",
		"checks:\n  - url: example.com\n":       "must be an absolute",
		"checks:\n  - url: ftp://example.com\n": "must be an absolute",
		"checks:\n  - url: https://a\n    method: TRACE\n":    "unsupported method",
		"checks:\n  - url: https://a\n    timeout: 5\n":       "invalid duration",
		"checks:\n  - url: https://a\n    timeout: [1]\n":     "duration must be a string",
		"checks:\n  - url: https://a\n    timeout: -1s\n":     "timeout must be positive",
		"checks:\n  - url: https://a\n    retries: 11\n":      "retries must be between 0 and 10",
		"checks:\n  - url: https://a\n  - url: https://a\n":   "duplicate name",
		"checks:\n  - url: https://a\n    tls_min_days: -1\n": "tls_min_days must not be negative",
		"checks: {": "did not find",
	}
	for in, want := range cases {
		_, err := Parse([]byte(in))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%q) = %v, want error containing %q", in, err, want)
		}
	}
}

func TestParseReportsEveryError(t *testing.T) {
	_, err := Parse([]byte("checks:\n  - name: one\n  - name: two\n"))
	if err == nil || !strings.Contains(err.Error(), "check 1 (one)") || !strings.Contains(err.Error(), "check 2 (two)") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "ok.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(good, []byte("checks:\n  - url: https://a.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("checks:\n  - url: nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(good); err != nil {
		t.Errorf("Load(good) = %v", err)
	}
	if _, err := Load(bad); err == nil || !strings.Contains(err.Error(), "bad.yaml") {
		t.Errorf("Load(bad) = %v, want the file name in the error", err)
	}
	if _, err := Load(filepath.Join(dir, "missing.yaml")); err == nil {
		t.Error("Load(missing) = nil")
	}
}

func TestFromURLs(t *testing.T) {
	cfg, err := FromURLs([]string{"https://a.example", "http://b.example:8080/x"})
	if err != nil || len(cfg.Checks) != 2 {
		t.Fatalf("FromURLs = %v, %v", cfg, err)
	}
	if _, err := FromURLs([]string{"not a url"}); err == nil {
		t.Error("FromURLs(bad) = nil error")
	}
}

func TestDurationMarshal(t *testing.T) {
	v, err := Duration{1500 * time.Millisecond}.MarshalYAML()
	if err != nil || v != "1.5s" {
		t.Fatalf("MarshalYAML = %v, %v", v, err)
	}
}

// 004 AC-7: broken assertions are configuration errors.
func TestExpectErrors(t *testing.T) {
	base := "checks:\n  - url: https://a.example\n    expect:\n"
	cases := map[string]string{
		"      body_regex: \"(\"\n":                             "expect.body_regex",
		"      json: [{path: status}]\n":                        "must start with $",
		"      json: [{path: $.a, equals: 1, exists: false}]\n": "contradict",
		"      status: 999\n":                                   "not an HTTP status",
		"      status: ok\n":                                    "status must be a number",
		"      status: [ok]\n":                                  "status must be a number",
		"      max_latency: -1s\n":                              "max_latency must be positive",
		"      json: [{path: $.a, equal: 1}]\n":                 "equal",
	}
	for in, want := range cases {
		_, err := Parse([]byte(base + in))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", in, err, want)
		}
	}
}

func TestFriendlyErrors(t *testing.T) {
	cases := map[string]string{
		"checks:\n  - url: https://a\n    expect: {stauts: 200}\n": `line 3: unknown key "stauts" (did you mean "status"?)`,
		"chekcs:\n  - url: https://a\n":                            `line 1: unknown key "chekcs" (did you mean "checks"?)`,
		"defaults: {retry: 1}\nchecks:\n  - url: https://a\n":      `line 1: unknown key "retry" (did you mean "retries"?)`,
		"checks:\n  - url: https://a\n    zzzzzzzz: 1\n":           `line 3: unknown key "zzzzzzzz"`,
		"checks:\n  - url: https://a\n    retries: many\n":         "cannot unmarshal",
	}
	for in, want := range cases {
		_, err := Parse([]byte(in))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v, want %q", in, err, want)
		}
	}
	if keysOf("Nope") != nil {
		t.Error("unknown type has keys")
	}
}
