package probe

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

// 009 AC-4: tcp:// checks that the port accepts connections.
func TestTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	resp, err := New("t").Do(context.Background(), Request{URL: "tcp://" + ln.Addr().String()})
	if err != nil || !strings.HasPrefix(resp.Status, "connected to 127.0.0.1:") || resp.Timings.Connect != resp.Latency || resp.StatusCode != 0 {
		t.Fatalf("%v %+v", err, resp)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	if _, err := New("t").Do(context.Background(), Request{URL: "tcp://" + addr}); err == nil {
		t.Fatal("closed port must fail")
	}
}

type fakeResolver map[string][]string

func (f fakeResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	if a, ok := f[host]; ok {
		return a, nil
	}
	return nil, errors.New("no such host")
}

// 009 AC-5: dns:// resolves the name; the addresses are the body.
func TestDNS(t *testing.T) {
	old := Resolver
	Resolver = fakeResolver{"example.com": {"93.184.216.34", "2606:2800::1"}}
	t.Cleanup(func() { Resolver = old })
	resp, err := New("t").Do(context.Background(), Request{URL: "dns://example.com"})
	if err != nil || resp.Status != "resolved 2606:2800::1, 93.184.216.34" || string(resp.Body) != "2606:2800::1\n93.184.216.34" || resp.Timings.DNS != resp.Latency {
		t.Fatalf("%v %+v", err, resp)
	}
	if _, err := New("t").Do(context.Background(), Request{URL: "dns://nope.invalid"}); err == nil {
		t.Fatal("unknown name must fail")
	}
}

func TestResolvedSummary(t *testing.T) {
	if got := resolved([]string{"a", "b", "c", "d", "e"}); got != "resolved a, b, c and 2 more" {
		t.Fatal(got)
	}
}
