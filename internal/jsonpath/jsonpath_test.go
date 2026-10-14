package jsonpath

import (
	"encoding/json"
	"testing"
)

func TestGet(t *testing.T) {
	var doc any
	if err := json.Unmarshal([]byte(`{"status":"ok","data":{"items":[{"name":"a"},{"name":"b"}],"total":2},"odd key":true,"n":null}`), &doc); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		want any
		ok   bool
	}{
		{"$", doc, true},
		{"$.status", "ok", true},
		{"$.data.total", 2.0, true},
		{"$.data.items[1].name", "b", true},
		{`$["odd key"]`, true, true},
		{`$['data']['total']`, 2.0, true},
		{"$.n", nil, true},
		{"$.missing", nil, false},
		{"$.data.items[5]", nil, false},
		{"$.status[0]", nil, false},
		{"$.status.x", nil, false},
	}
	for _, c := range cases {
		p, err := Parse(c.path)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.path, err)
		}
		got, ok := p.Get(doc)
		if ok != c.ok || (c.path != "$" && got != c.want) {
			t.Errorf("Get(%q) = %v, %v; want %v, %v", c.path, got, ok, c.want, c.ok)
		}
		if p.String() != c.path {
			t.Errorf("String() = %q", p.String())
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, s := range []string{"status", "$..a", "$.a[", "$.a[x]", "$.a[-1]", "$x", "$."} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) = nil error", s)
		}
	}
}
