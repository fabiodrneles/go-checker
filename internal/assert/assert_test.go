package assert

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fabiodrneles/go-checker/internal/config"
	"github.com/fabiodrneles/go-checker/internal/probe"
)

func expect(t *testing.T, yaml string) config.Expect {
	t.Helper()
	cfg, err := config.Parse([]byte("checks:\n  - url: https://a.example\n    expect:\n" + yaml))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Checks[0].Expect
}

func resp(status int, body string, header http.Header) *probe.Response {
	if header == nil {
		header = http.Header{}
	}
	return &probe.Response{StatusCode: status, Body: []byte(body), Header: header, Latency: 50 * time.Millisecond}
}

func reasons(fs []Failure) string {
	var s []string
	for _, f := range fs {
		s = append(s, f.Reason)
	}
	return strings.Join(s, "; ")
}

// 004 AC-1
func TestStatusList(t *testing.T) {
	e := expect(t, "      status: [200, 204]\n")
	fs := Evaluate(e, resp(201, "", nil))
	if len(fs) != 1 || fs[0].Severity != Down || fs[0].Reason != "status 201, want 200 or 204" {
		t.Fatalf("got %v", fs)
	}
	if fs := Evaluate(e, resp(204, "", nil)); len(fs) != 0 {
		t.Fatalf("204: %v", fs)
	}
	// A single code works too, and an expected 503 is not a failure.
	if fs := Evaluate(expect(t, "      status: 503\n"), resp(503, "", nil)); len(fs) != 0 {
		t.Fatalf("503: %v", fs)
	}
}

func TestDefaultStatus(t *testing.T) {
	if fs := Evaluate(config.Expect{}, resp(302, "", nil)); len(fs) != 0 {
		t.Fatalf("302: %v", fs)
	}
	if fs := Evaluate(config.Expect{}, resp(404, "", nil)); reasons(fs) != "status 404" {
		t.Fatalf("404: %v", fs)
	}
}

// 004 AC-2, 004 AC-3
func TestBody(t *testing.T) {
	e := expect(t, "      body_contains: ok\n      body_regex: \"v[0-9]+\"\n")
	if fs := Evaluate(e, resp(200, `{"status":"ok","version":"v12"}`, nil)); len(fs) != 0 {
		t.Fatalf("match: %v", fs)
	}
	fs := Evaluate(e, resp(200, "fail", nil))
	if reasons(fs) != `body does not contain "ok"; body does not match /v[0-9]+/` {
		t.Fatalf("got %q", reasons(fs))
	}
}

// 004 AC-4
func TestJSON(t *testing.T) {
	body := `{"status":"ok","code":200,"data":{"items":[{"name":"a"},{"name":"b","tags":["x"]}]},"meta":{"v":1},"gone":null}`
	e := expect(t, `      json:
        - path: $.data.items[1].name
          equals: b
        - path: $.code
          equals: 200
        - path: $.data.items[1].tags
          equals: [x]
        - path: $.meta
          equals: {v: 1}
        - path: $.gone
          equals: null
        - path: $.status
        - path: $.secret
          exists: false
`)
	if fs := Evaluate(e, resp(200, body, nil)); len(fs) != 0 {
		t.Fatalf("match: %v", reasons(fs))
	}
	bad := `{"status":"ok","code":500,"data":{"items":[{"name":"a"},{"name":"c","tags":["x","y"]}]},"meta":{"v":2,"w":1},"secret":1}`
	want := `$.data.items[1].name is "c", want "b"; $.code is 500, want 200; $.data.items[1].tags is ["x","y"], want ["x"]; $.meta is {"v":2,"w":1}, want {"v":1}; $.gone not found; $.secret exists, want it absent`
	if got := reasons(Evaluate(e, resp(200, bad, nil))); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
	if got := reasons(Evaluate(e, resp(200, "<html>", nil))); !strings.HasPrefix(got, "body is not JSON") {
		t.Fatalf("not JSON: %s", got)
	}
}

// 004 AC-5
func TestHeaders(t *testing.T) {
	e := expect(t, "      headers: {Content-Type: json}\n")
	h := http.Header{}
	h.Set("Content-Type", "text/html")
	if got := reasons(Evaluate(e, resp(200, "", h))); got != `header Content-Type is "text/html", want it to contain "json"` {
		t.Fatalf("got %s", got)
	}
	h.Set("Content-Type", "application/json; charset=utf-8")
	if fs := Evaluate(e, resp(200, "", h)); len(fs) != 0 {
		t.Fatalf("match: %v", fs)
	}
}

// 004 AC-6: too slow is a warning, not an outage.
func TestMaxLatency(t *testing.T) {
	fs := Evaluate(expect(t, "      max_latency: 10ms\n"), resp(200, "", nil))
	if len(fs) != 1 || fs[0].Severity != Warn || fs[0].Reason != "latency 50ms above max_latency 10ms" {
		t.Fatalf("got %v", fs)
	}
}

func TestEqual(t *testing.T) {
	cases := []struct {
		j, y any
		want bool
	}{
		{2.0, 2, true}, {2.0, int64(2), true}, {2.0, uint64(2), true}, {2.5, 2, false},
		{"2", 2, false}, {2.0, "2", false}, {true, true, true}, {nil, nil, true},
		{[]any{1.0}, []any{1, 2}, false}, {"x", []any{"x"}, false},
		{map[string]any{"a": 1.0}, map[string]any{"b": 1}, false}, {"x", map[string]any{}, false},
	}
	for _, c := range cases {
		if got := Equal(c.j, c.y); got != c.want {
			t.Errorf("Equal(%v, %v) = %v", c.j, c.y, got)
		}
	}
	if show(func() {}) == "" {
		t.Error("show of an unmarshalable value")
	}
}
