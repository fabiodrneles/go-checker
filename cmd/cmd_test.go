package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fabiodrneles/go-checker/internal/testcert"
)

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Execute(args, Env{Stdout: &out, Stderr: &errb, Version: "v1.2.3", Commit: "abc1234", Getenv: func(string) string { return "" }})
	return code, out.String(), errb.String()
}

func server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/down":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			_, _ = w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// 002 AC-2: every check up exits 0.
func TestCheckAllUp(t *testing.T) {
	srv := server(t)
	f := writeFile(t, t.TempDir(), "c.yaml", "checks:\n  - name: home\n    url: "+srv.URL+"\n")
	code, out, _ := run(t, "check", "-f", f)
	if code != ExitOK || !strings.Contains(out, "UP    home") || !strings.Contains(out, "1 up, 0 warn, 0 down") {
		t.Fatalf("code %d\n%s", code, out)
	}
}

// 002 AC-3: one check down exits 1.
// 002 AC-10: URLs given as arguments become checks.
func TestCheckDownExitsOne(t *testing.T) {
	srv := server(t)
	code, out, errOut := run(t, "check", srv.URL, srv.URL+"/down")
	if code != ExitFail || !strings.Contains(out, "1 up, 0 warn, 1 down") || errOut != "" {
		t.Fatalf("code %d\nout %s\nerr %s", code, out, errOut)
	}
}

// 002 AC-4: usage and configuration errors exit 2 with a message on stderr.
func TestCheckUsageErrors(t *testing.T) {
	dir := t.TempDir()
	bad := writeFile(t, dir, "bad.yaml", "checks:\n  - url: nope\n")
	cases := [][]string{
		{"check", "-f", filepath.Join(dir, "missing.yaml")},
		{"check", "-f", bad},
		{"check", "-f", bad, "https://example.com"},
		{"check", "not-a-url"},
		{"check", "https://example.com", "--concurrency", "0"},
		{"check", "https://example.com", "--timeout", "0s"},
		{"check", "https://example.com", "--retries", "11"},
		{"check", "--no-such-flag"},
		{"no-such-command"},
	}
	for _, args := range cases {
		code, _, errOut := run(t, args...)
		if code != ExitUsage || !strings.HasPrefix(errOut, "error: ") {
			t.Errorf("%v: code %d, stderr %q", args, code, errOut)
		}
	}
}

func TestCheckFindsDefaultFile(t *testing.T) {
	srv := server(t)
	dir := t.TempDir()
	t.Chdir(dir)
	if code, _, errOut := run(t, "check"); code != ExitUsage || !strings.Contains(errOut, "no check file found") {
		t.Fatalf("empty dir: code %d, %s", code, errOut)
	}
	writeFile(t, dir, "config.yaml", "sites:\n  - url: "+srv.URL+"/down\n")
	if code, _, _ := run(t, "check"); code != ExitFail {
		t.Fatalf("config.yaml: code %d", code)
	}
	writeFile(t, dir, "go-checker.yaml", "checks:\n  - url: "+srv.URL+"\n")
	if code, _, _ := run(t, "check"); code != ExitOK {
		t.Fatalf("go-checker.yaml should win: code %d", code)
	}
}

func TestCheckFlagsOverrideFile(t *testing.T) {
	srv := server(t)
	f := writeFile(t, t.TempDir(), "c.yaml", "checks:\n  - url: "+srv.URL+"/down\n    retries: 0\n")
	code, out, _ := run(t, "check", "-f", f, "--retries", "1", "--timeout", "2s")
	if code != ExitFail || !strings.Contains(out, "(2 attempts)") {
		t.Fatalf("code %d\n%s", code, out)
	}
}

func TestVerdictFailOnWarn(t *testing.T) {
	if verdict(nil, true) != nil {
		t.Fatal("no results should pass")
	}
}

// 001 AC-1: the build injects the version and the commit.
func TestVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		code, out, _ := run(t, args...)
		if code != ExitOK || out != "go-checker v1.2.3 (abc1234)\n" {
			t.Errorf("%v: code %d, out %q", args, code, out)
		}
	}
}

func TestHelpIsEnglish(t *testing.T) {
	code, out, _ := run(t, "--help")
	if code != ExitOK || !strings.Contains(out, "verifies that websites and APIs are up") || !strings.Contains(out, "check") {
		t.Fatalf("code %d\n%s", code, out)
	}
}

// 003 AC-4: --trace shows every phase and the certificate.
func TestCheckTrace(t *testing.T) {
	srv, _ := testcert.Server(t, time.Now().Add(90*24*time.Hour), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	f := writeFile(t, t.TempDir(), "c.yaml", "checks:\n  - name: tls\n    url: "+srv.URL+"\n    insecure: true\n")
	code, out, _ := run(t, "check", "-f", f, "--trace")
	if code != ExitOK {
		t.Fatalf("code %d\n%s", code, out)
	}
	for _, want := range []string{"dns ", "connect ", "tls ", "ttfb ", "total ", "cert TLS 1.3", "(89 days)", "issuer go-checker test CA"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestCheckCertificateWarn(t *testing.T) {
	srv, _ := testcert.Server(t, time.Now().Add(3*24*time.Hour), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	f := writeFile(t, t.TempDir(), "c.yaml", "defaults:\n  tls_min_days: 7\nchecks:\n  - url: "+srv.URL+"\n    insecure: true\n")
	if code, out, _ := run(t, "check", "-f", f); code != ExitOK || !strings.Contains(out, "certificate expires in 2 days (minimum 7)") {
		t.Fatalf("code %d\n%s", code, out)
	}
	if code, _, _ := run(t, "check", "-f", f, "--fail-on-warn"); code != ExitFail {
		t.Fatalf("--fail-on-warn: code %d", code)
	}
}

func TestCheckAssertions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"degraded","db":{"ok":false}}`))
	}))
	t.Cleanup(srv.Close)
	f := writeFile(t, t.TempDir(), "c.yaml", `checks:
  - name: api
    url: `+srv.URL+`
    expect:
      status: 200
      headers: {Content-Type: json}
      json:
        - {path: $.status, equals: ok}
        - {path: $.db.ok, equals: true}
`)
	code, out, _ := run(t, "check", "-f", f)
	if code != ExitFail || !strings.Contains(out, `→ $.status is "degraded", want "ok"`) || !strings.Contains(out, "→ $.db.ok is false, want true") {
		t.Fatalf("code %d\n%s", code, out)
	}
}

// 005 AC-1, 005 AC-5, 005 AC-4: JSON to a file, text on the terminal, table in the job summary.
func TestCheckOutputsAndSummary(t *testing.T) {
	srv := server(t)
	dir := t.TempDir()
	report := filepath.Join(dir, "report.json")
	junit := filepath.Join(dir, "junit.xml")
	summary := writeFile(t, dir, "summary.md", "# earlier step\n")
	var out, errb bytes.Buffer
	code := Execute([]string{"check", srv.URL, srv.URL + "/down", "--format", "json", "--output", report, "--junit", junit},
		Env{Stdout: &out, Stderr: &errb, Version: "v1", Getenv: func(k string) string {
			if k == "GITHUB_STEP_SUMMARY" {
				return summary
			}
			return ""
		}})
	if code != ExitFail || !strings.Contains(out.String(), "1 up, 0 warn, 1 down") {
		t.Fatalf("code %d\n%s%s", code, out.String(), errb.String())
	}
	data, err := os.ReadFile(report)
	if err != nil || !strings.Contains(string(data), `"down": 1`) {
		t.Fatalf("report: %v\n%s", err, data)
	}
	if x, err := os.ReadFile(junit); err != nil || !strings.Contains(string(x), `failures="1"`) {
		t.Fatalf("junit: %v\n%s", err, x)
	}
	s, _ := os.ReadFile(summary)
	if !strings.HasPrefix(string(s), "# earlier step\n### go-checker: 1 up, 0 warn, 1 down") {
		t.Fatalf("summary not appended:\n%s", s)
	}
}

func TestCheckFormatsOnStdout(t *testing.T) {
	srv := server(t)
	for format, want := range map[string]string{"json": `"tool": "go-checker"`, "junit": "<testsuites", "markdown": "| 🟢 UP |", "text": "1 check: 1 up"} {
		code, out, _ := run(t, "check", srv.URL, "--format", format)
		if code != ExitOK || !strings.Contains(out, want) {
			t.Errorf("%s: code %d\n%s", format, code, out)
		}
	}
}

// 005 AC-6
func TestCheckBadFormatAndPaths(t *testing.T) {
	srv := server(t)
	missing := filepath.Join(t.TempDir(), "no", "such", "dir", "r.json")
	for _, args := range [][]string{
		{"check", srv.URL, "--format", "yaml"},
		{"check", srv.URL, "--output", missing},
		{"check", srv.URL, "--junit", missing},
	} {
		if code, _, errOut := run(t, args...); code != ExitUsage || !strings.HasPrefix(errOut, "error: ") {
			t.Errorf("%v: code %d, %s", args, code, errOut)
		}
	}
	// A summary that cannot be written is a warning, never a failure.
	var out, errb bytes.Buffer
	code := Execute([]string{"check", srv.URL}, Env{Stdout: &out, Stderr: &errb, Getenv: func(k string) string {
		if k == "GITHUB_STEP_SUMMARY" {
			return missing
		}
		return ""
	}})
	if code != ExitOK || !strings.Contains(errb.String(), "warning: job summary") {
		t.Fatalf("code %d, stderr %s", code, errb.String())
	}
}

// 006 AC-4: init writes a file that check reads; 006 AC-5: it never overwrites without --force.
func TestInit(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	spec := writeFile(t, dir, "api.yaml", "openapi: 3.0.0\ninfo: {title: Demo}\npaths:\n  /health:\n    get:\n      responses: {'204': {description: ok}}\n  /items/{id}:\n    get: {}\n")
	code, out, errOut := run(t, "init", "--from-openapi", spec, "--base-url", "http://127.0.0.1:9")
	if code != ExitOK || !strings.Contains(out, "wrote go-checker.yaml with 1 check (1 operations skipped") {
		t.Fatalf("code %d\n%s%s", code, out, errOut)
	}
	data, _ := os.ReadFile("go-checker.yaml")
	if !strings.Contains(string(data), "url: http://127.0.0.1:9/health") || !strings.Contains(string(data), "status: 204") {
		t.Fatalf("%s", data)
	}
	if code, _, errOut := run(t, "init"); code != ExitUsage || !strings.Contains(errOut, "already exists") {
		t.Fatalf("overwrite: code %d %s", code, errOut)
	}
	after, _ := os.ReadFile("go-checker.yaml")
	if string(after) != string(data) {
		t.Fatal("file changed without --force")
	}
	if code, out, _ := run(t, "init", "--force"); code != ExitOK || !strings.Contains(out, "with 1 check") {
		t.Fatalf("--force: %d %s", code, out)
	}
	if code, out, _ := run(t, "init", "-f", "-"); code != ExitOK || !strings.HasPrefix(out, "# go-checker checks.") {
		t.Fatalf("stdout: %d %s", code, out)
	}
	for _, args := range [][]string{
		{"init", "--from-openapi", "missing.yaml"},
		{"init", "--from-openapi", writeFile(t, dir, "bad.yaml", "swagger: '2.0'\n")},
		{"init", "--from-openapi", writeFile(t, dir, "none.yaml", "openapi: 3.0.0\npaths: {}\n"), "--base-url", "http://x.example"},
		{"init", "--base-url", "http://x.example"},
		{"init", "-f", filepath.Join(dir, "no", "dir", "x.yaml")},
	} {
		if code, _, errOut := run(t, args...); code != ExitUsage {
			t.Errorf("%v: code %d %s", args, code, errOut)
		}
	}
}

func TestWatchServesMetricsAndStops(t *testing.T) {
	srv := server(t)
	ctx, cancel := context.WithCancel(context.Background())
	var out, errb safeBuffer
	done := make(chan int)
	go func() {
		done <- Execute([]string{"watch", srv.URL, "--interval", "1s", "--metrics-addr", "127.0.0.1:0"},
			Env{Stdout: &out, Stderr: &errb, Version: "v1", Getenv: func(string) string { return "" }, Context: ctx})
	}()
	var addr string
	deadline := time.Now().Add(5 * time.Second)
	for addr == "" && time.Now().Before(deadline) {
		if m := regexp.MustCompile(`metrics on (http://\S+)/metrics`).FindStringSubmatch(out.String()); m != nil && strings.Contains(out.String(), "1 up") {
			addr = m[1]
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" {
		cancel()
		t.Fatalf("no metrics address:\n%s", out.String())
	}
	resp, err := http.Get(addr + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "go_checker_up{check=\""+srv.URL+"\"} 1") || resp.Header.Get("Content-Type") != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("metrics:\n%s", body)
	}
	if r, err := http.Get(addr + "/healthz"); err != nil || r.StatusCode != 200 {
		t.Fatalf("healthz: %v", err)
	}
	cancel()
	if code := <-done; code != ExitOK || !strings.Contains(out.String(), "stopped") {
		t.Fatalf("code %d\n%s", code, out.String())
	}
}

func TestWatchUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"watch", "https://example.com", "--interval", "10ms"},
		{"watch", "https://example.com", "--confirm", "0"},
		{"watch", "https://example.com", "--window", "0"},
		{"watch", "https://example.com", "--metrics-addr", "256.0.0.1:99999"},
		{"watch", "https://example.com", "--concurrency", "0"},
		{"watch", "-f", "missing.yaml"},
	} {
		if code, _, errOut := run(t, args...); code != ExitUsage {
			t.Errorf("%v: code %d %s", args, code, errOut)
		}
	}
}

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestCheckTCPFromCLI(t *testing.T) {
	srv := server(t)
	host := strings.TrimPrefix(srv.URL, "http://")
	code, out, _ := run(t, "check", "tcp://"+host, srv.URL)
	if code != ExitOK || !strings.Contains(out, "connected to 127.0.0.1") || !strings.Contains(out, "2 checks: 2 up") {
		t.Fatalf("code %d\n%s", code, out)
	}
}

func TestWatchBadTelegram(t *testing.T) {
	if code, _, errOut := run(t, "watch", "https://example.com", "--telegram", "nocolon"); code != ExitUsage || !strings.Contains(errOut, "BOT_TOKEN:CHAT_ID") {
		t.Fatalf("code %d %s", code, errOut)
	}
}
