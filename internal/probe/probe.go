// Package probe performs one HTTP request and measures it.
package probe

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"
)

// MaxBody is the most bytes of a response body kept for assertions.
const MaxBody = 1 << 20

// Request describes what to send.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
	Timeout time.Duration
	// Insecure skips certificate verification (self-signed internal
	// services); the certificate expiry is still reported.
	Insecure bool
}

// Timings split the latency into its phases (spec 003). DNS, Connect and TLS
// are zero when the phase did not happen (an IP address, plain HTTP).
type Timings struct {
	DNS     time.Duration
	Connect time.Duration
	TLS     time.Duration
	// TTFB is the time from the start until the first response byte.
	TTFB  time.Duration
	Total time.Duration
}

// TLSInfo describes the certificate the server presented.
type TLSInfo struct {
	NotAfter time.Time
	Subject  string
	Issuer   string
	Version  string
}

// DaysLeft is the number of whole days until the certificate expires;
// negative once it has expired.
func (t TLSInfo) DaysLeft(now time.Time) int {
	d := t.NotAfter.Sub(now)
	days := int(d / (24 * time.Hour))
	if d < 0 && d%(24*time.Hour) != 0 {
		days--
	}
	return days
}

// Response is what came back, with the time it took.
type Response struct {
	StatusCode int
	Status     string
	Header     http.Header
	Body       []byte
	Latency    time.Duration
	Timings    Timings
	TLS        *TLSInfo
}

// Prober sends requests. The zero value is not usable: use New.
type Prober struct {
	secure, insecure *http.Client
	userAgent        string
}

// New returns a Prober that identifies itself with the given User-Agent.
// Every request opens a new connection, so that each measurement includes
// DNS, TCP and TLS, as a new visitor would see them.
func New(userAgent string) *Prober {
	return NewWithTLS(userAgent, nil)
}

// NewWithTLS is New with a custom TLS configuration (e.g. extra root CAs).
func NewWithTLS(userAgent string, cfg *tls.Config) *Prober {
	client := func(insecure bool) *http.Client {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.DisableKeepAlives = true
		if cfg != nil {
			t.TLSClientConfig = cfg.Clone()
		}
		if insecure {
			if t.TLSClientConfig == nil {
				t.TLSClientConfig = &tls.Config{}
			}
			t.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // opt-in per check
		}
		return &http.Client{Transport: t}
	}
	return &Prober{secure: client(false), insecure: client(true), userAgent: userAgent}
}

// Do sends the request. Latency covers the whole exchange: from dialing until
// the body (up to MaxBody) has been read.
func (p *Prober) Do(ctx context.Context, r Request) (*Response, error) {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, r.URL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.userAgent)
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	if h := req.Header.Get("Host"); h != "" {
		req.Host = h
	}

	// The dialer may race IPv4 and IPv6 (happy eyeballs), so the hooks can
	// run on several goroutines: guard the shared state.
	var (
		mu                            sync.Mutex
		tm                            Timings
		dnsStart, connStart, tlsStart time.Time
	)
	at := func(f func()) { mu.Lock(); f(); mu.Unlock() }
	start := time.Now()
	trace := &httptrace.ClientTrace{
		DNSStart:     func(httptrace.DNSStartInfo) { at(func() { dnsStart = time.Now() }) },
		DNSDone:      func(httptrace.DNSDoneInfo) { at(func() { tm.DNS = since(dnsStart) }) },
		ConnectStart: func(string, string) { at(func() { connStart = time.Now() }) },
		ConnectDone: func(_, _ string, err error) {
			if err == nil {
				at(func() { tm.Connect = since(connStart) })
			}
		},
		TLSHandshakeStart:    func() { at(func() { tlsStart = time.Now() }) },
		TLSHandshakeDone:     func(tls.ConnectionState, error) { at(func() { tm.TLS = since(tlsStart) }) },
		GotFirstResponseByte: func() { at(func() { tm.TTFB = time.Since(start) }) },
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	client := p.secure
	if r.Insecure {
		client = p.insecure
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody))
	mu.Lock()
	defer mu.Unlock()
	tm.Total = time.Since(start)
	if err != nil {
		return nil, err
	}
	out := &Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       data,
		Latency:    tm.Total,
		Timings:    tm,
	}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		c := resp.TLS.PeerCertificates[0]
		out.TLS = &TLSInfo{
			NotAfter: c.NotAfter,
			Subject:  c.Subject.CommonName,
			Issuer:   c.Issuer.CommonName,
			Version:  tls.VersionName(resp.TLS.Version),
		}
	}
	return out, nil
}

func since(t time.Time) time.Duration {
	if t.IsZero() {
		return 0
	}
	return time.Since(t)
}
