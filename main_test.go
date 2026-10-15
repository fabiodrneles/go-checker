package main

import (
	"runtime/debug"
	"testing"
)

// 001 AC-2: go install reports the module version instead of "dev".
func TestResolveVersion(t *testing.T) {
	info := func(v string, ok bool) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) { return &debug.BuildInfo{Main: debug.Module{Version: v}}, ok }
	}
	cases := []struct {
		embedded, module string
		ok               bool
		want             string
	}{
		{"v1.2.3", "v9.9.9", true, "v1.2.3"},
		{"dev", "v1.0.1", true, "v1.0.1"},
		{"dev", "v1.1.0-rc.1", true, "v1.1.0-rc.1"},
		{"dev", "v0.0.0-20261015003000-abcdef123456", true, "dev"},
		{"dev", "(devel)", true, "dev"},
		{"dev", "v1.0.1+dirty", true, "dev"},
		{"dev", "", false, "dev"},
	}
	for _, c := range cases {
		if got := resolveVersion(c.embedded, info(c.module, c.ok)); got != c.want {
			t.Errorf("resolveVersion(%q, %q) = %q, want %q", c.embedded, c.module, got, c.want)
		}
	}
}
