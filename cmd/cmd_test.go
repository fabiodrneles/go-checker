package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
