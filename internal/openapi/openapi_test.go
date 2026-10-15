package openapi

import (
	"os"
	"strings"
	"testing"

	"github.com/fabiodrneles/go-checker/internal/config"
)

func load(t *testing.T, name string) *Document {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	d, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// 006 AC-1: only GET operations without path or required parameters become checks.
// 006 AC-2: the expected status is the lowest documented 2xx.
func TestGenerate(t *testing.T) {
	res, err := Generate(load(t, "petstore.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, c := range res.Checks {
		got[c.Name+" "+c.URL] = c.Expect.Status
	}
	want := map[string]int{
		"health https://api.petstore.example/v1/health":      200,
		"listPets https://api.petstore.example/v1/pets":      200,
		"GET /ping https://api.petstore.example/v1/ping":     204,
		"GET /status https://api.petstore.example/v1/status": 0,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if s, ok := got[k]; !ok || s != v {
			t.Errorf("%s: status %d (present %v), want %d", k, s, ok, v)
		}
	}
	skipped := map[string]string{}
	for _, s := range res.Skipped {
		skipped[s.Operation] = s.Reason
	}
	if skipped["GET /pets/{id}"] != "needs a path parameter" || skipped["GET /search"] != `needs the query parameter "q"` {
		t.Errorf("skipped %v", skipped)
	}
}

// 006 AC-3
func TestGenerateBaseURL(t *testing.T) {
	res, err := Generate(load(t, "petstore.yaml"), "http://localhost:8080/")
	if err != nil || res.Checks[0].URL != "http://localhost:8080/health" {
		t.Fatalf("%v %+v", err, res)
	}
	if _, err := Generate(load(t, "minimal.json"), ""); err == nil || !strings.Contains(err.Error(), "--base-url") {
		t.Fatalf("no servers: %v", err)
	}
	if _, err := Generate(load(t, "minimal.json"), "localhost:8080"); err == nil {
		t.Fatal("relative base accepted")
	}
}

// 006 AC-4: the generated file is a valid check file.
func TestYAMLIsValidConfig(t *testing.T) {
	res, err := Generate(load(t, "petstore.yaml"), "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := res.YAML()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	if len(cfg.Checks) != 4 || !strings.Contains(string(data), "#   GET /pets/{id}: needs a path parameter") || !strings.Contains(string(data), "for Petstore (https://api.petstore.example/v1)") {
		t.Fatalf("%s", data)
	}
	if !cfg.Checks[1].Expect.Status.Has(200) {
		t.Errorf("expect.status lost: %+v", cfg.Checks[1].Expect)
	}
}

func TestParseErrors(t *testing.T) {
	for in, want := range map[string]string{
		"swagger: '2.0'\n": "only OpenAPI 3",
		"{":                "not a YAML or JSON document",
	} {
		if _, err := Parse([]byte(in)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%q) = %v", in, err)
		}
	}
	d, _ := Parse([]byte("openapi: 3.0.0\npaths:\n  /a/{id}:\n    get: {}\n  /b:\n    get: [1]\n"))
	if _, err := Generate(d, "https://x.example"); err == nil {
		t.Fatal("want an error when nothing can be checked")
	}
}
