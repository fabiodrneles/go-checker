// Package probe performs one HTTP request and measures it.
package probe

import (
	"context"
	"io"
	"net/http"
	"strings"
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
}

// Response is what came back, with the time it took.
type Response struct {
	StatusCode int
	Status     string
	Header     http.Header
	Body       []byte
	Latency    time.Duration
}

// Prober sends requests. The zero value is not usable: use New.
type Prober struct {
	client    *http.Client
	userAgent string
}

// New returns a Prober that identifies itself with the given User-Agent.
func New(userAgent string) *Prober {
	return &Prober{client: &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}, userAgent: userAgent}
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

	start := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody))
	latency := time.Since(start)
	if err != nil {
		return nil, err
	}
	return &Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header,
		Body:       data,
		Latency:    latency,
	}, nil
}
