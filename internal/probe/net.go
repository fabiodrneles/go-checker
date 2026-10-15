package probe

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Resolver looks up host names; tests replace it.
var Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
} = net.DefaultResolver

// tcp checks that host:port accepts a connection (spec 009).
func (p *Prober) tcp(ctx context.Context, u *url.URL) (*Response, error) {
	var d net.Dialer
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", u.Host)
	if err != nil {
		return nil, err
	}
	took := time.Since(start)
	_ = conn.Close()
	return &Response{Status: "connected to " + conn.RemoteAddr().String(), Latency: took, Timings: Timings{Connect: took, Total: took}}, nil
}

// dns checks that a name resolves; the sorted addresses become the body,
// so that body_contains and body_regex can assert on them.
func (p *Prober) dns(ctx context.Context, u *url.URL) (*Response, error) {
	start := time.Now()
	addrs, err := Resolver.LookupHost(ctx, u.Hostname())
	took := time.Since(start)
	if err != nil {
		return nil, err
	}
	sort.Strings(addrs)
	return &Response{
		Status:  fmt.Sprintf("resolved %d address(es)", len(addrs)),
		Body:    []byte(strings.Join(addrs, "\n")),
		Latency: took,
		Timings: Timings{DNS: took, Total: took},
	}, nil
}
