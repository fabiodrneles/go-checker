package config

import (
	"strings"
	"testing"
)

func fakeEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	old := LookupEnv
	LookupEnv = func(k string) (string, bool) { v, ok := vars[k]; return v, ok }
	t.Cleanup(func() { LookupEnv = old })
}

// 009 AC-1: ${VAR} and ${VAR:-default}; "$" elsewhere is untouched.
func TestExpandEnv(t *testing.T) {
	fakeEnv(t, map[string]string{"HOST": "api.example.com", "TOKEN": "s3cret", "EMPTY": ""})
	cfg, err := Parse([]byte(`checks:
  # ${NOT_SET} in a comment is fine
  - name: api
    url: https://${HOST}/health
    headers: {X-Env: "${ENV:-prod}", X-Empty: "${EMPTY:-fallback}", X-Literal: "$${HOST}"}
    auth: {bearer: "${TOKEN}"}
    expect:
      body_regex: "^ok$"
      json: [{path: $.status, equals: ok}]
`))
	if err != nil {
		t.Fatal(err)
	}
	c := cfg.Checks[0]
	if c.URL != "https://api.example.com/health" || c.Headers["X-Env"] != "prod" || c.Headers["X-Empty"] != "fallback" || c.Headers["X-Literal"] != "${HOST}" {
		t.Fatalf("%+v", c)
	}
	if c.Headers["Authorization"] != "Bearer s3cret" || c.Expect.BodyRegex != "^ok$" || c.Expect.JSON[0].Path != "$.status" {
		t.Fatalf("%+v", c)
	}
}

// 009 AC-2: an unset variable is an error with the line.
func TestExpandEnvMissing(t *testing.T) {
	fakeEnv(t, nil)
	_, err := Parse([]byte("checks:\n  - url: https://a.example\n    auth: {bearer: \"${API_TOKEN}\"}\n"))
	if err == nil || !strings.Contains(err.Error(), "line 3: environment variable API_TOKEN is not set") {
		t.Fatalf("err = %v", err)
	}
}

// 009 AC-3: basic auth becomes an Authorization header.
func TestAuth(t *testing.T) {
	cfg, err := Parse([]byte("checks:\n  - url: https://a.example\n    auth: {basic: {username: ana, password: pw}}\n"))
	if err != nil || cfg.Checks[0].Headers["Authorization"] != "Basic YW5hOnB3" {
		t.Fatalf("%v %+v", err, cfg)
	}
	for in, want := range map[string]string{
		"auth: {bearer: x, basic: {username: a}}": "not both",
		"auth: {}": "set basic or bearer",
	} {
		if _, err := Parse([]byte("checks:\n  - url: https://a.example\n    " + in + "\n")); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", in, err)
		}
	}
}

func TestTCPAndDNSValidation(t *testing.T) {
	ok := []string{"tcp://db.internal:5432", "dns://example.com", "dns://example.com\n    expect: {body_contains: 93.184, max_latency: 50ms}"}
	for _, u := range ok {
		cfg, err := Parse([]byte("checks:\n  - url: " + u + "\n"))
		if err != nil {
			t.Errorf("%s: %v", u, err)
			continue
		}
		if k := cfg.Checks[0].Kind(); k != strings.SplitN(u, ":", 2)[0] {
			t.Errorf("%s: kind %s", u, k)
		}
	}
	for in, want := range map[string]string{
		"tcp://db.internal":                                  "needs a port",
		"tcp://db:5432\n    expect: {status: 200}":           "support only",
		"tcp://db:5432\n    expect: {body_contains: x}":      "tcp checks have no body",
		"dns://example.com\n    auth: {bearer: x}":           "support only",
		"dns://example.com\n    expect: {json: [{path: $}]}": "support only",
	} {
		if _, err := Parse([]byte("checks:\n  - url: " + in + "\n")); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v", in, err)
		}
	}
	if (Check{URL: "https://x"}).Kind() != "http" {
		t.Error("kind http")
	}
}
