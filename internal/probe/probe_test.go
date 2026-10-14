package probe

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fabiodrneles/go-checker/internal/testcert"
)

// 002 AC-1: latency is the real duration of the exchange.
func TestDoMeasuresLatency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(60 * time.Millisecond)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()
	resp, err := New("test").Do(context.Background(), Request{Method: "GET", URL: srv.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Latency < 60*time.Millisecond || resp.Latency > 5*time.Second {
		t.Fatalf("latency = %v, want between 60ms and 5s", resp.Latency)
	}
	if resp.StatusCode != 200 || string(resp.Body) != "ok" {
		t.Fatalf("resp = %d %q", resp.StatusCode, resp.Body)
	}
}

func TestDoSendsMethodHeadersAndBody(t *testing.T) {
	var got *http.Request
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()
	resp, err := New("go-checker/test").Do(context.Background(), Request{
		Method:  "POST",
		URL:     srv.URL + "/items",
		Headers: map[string]string{"Authorization": "Bearer x", "Host": "api.internal"},
		Body:    `{"a":1}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated || got.Method != "POST" || body != `{"a":1}` {
		t.Fatalf("status %d, method %s, body %q", resp.StatusCode, got.Method, body)
	}
	if got.Header.Get("Authorization") != "Bearer x" || got.UserAgent() != "go-checker/test" || got.Host != "api.internal" {
		t.Fatalf("headers %v, host %s", got.Header, got.Host)
	}
}

// 002 AC-5: a slow endpoint fails at the timeout instead of hanging.
func TestDoTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()
	start := time.Now()
	_, err := New("test").Do(context.Background(), Request{Method: "GET", URL: srv.URL, Timeout: 50 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("took %v", time.Since(start))
	}
}

func TestDoLimitsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", MaxBody+10))
	}))
	defer srv.Close()
	resp, err := New("test").Do(context.Background(), Request{Method: "GET", URL: srv.URL})
	if err != nil || len(resp.Body) != MaxBody {
		t.Fatalf("len = %d, err = %v", len(resp.Body), err)
	}
}

func TestDoBadRequest(t *testing.T) {
	if _, err := New("test").Do(context.Background(), Request{Method: "BAD METHOD", URL: "http://x"}); err == nil {
		t.Fatal("want error for an invalid method")
	}
}

// 003 AC-1: an HTTPS exchange reports every phase, and the total is the largest.
func TestDoTraceAndCertificate(t *testing.T) {
	notAfter := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	srv, pool := testcert.Server(t, notAfter, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(w, "ok")
	}))
	p := NewWithTLS("test", &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})
	resp, err := p.Do(context.Background(), Request{Method: "GET", URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	tm := resp.Timings
	// A loopback connect can finish below the clock resolution (Windows):
	// only the TLS handshake is long enough to be always measurable.
	if tm.Connect < 0 || tm.TLS <= 0 || tm.TTFB < 20*time.Millisecond || tm.Total < tm.TTFB || tm.Total < tm.TLS || tm.Total != resp.Latency {
		t.Fatalf("timings %+v", tm)
	}
	if resp.TLS == nil || !resp.TLS.NotAfter.Equal(notAfter) || resp.TLS.Subject != "localhost" || resp.TLS.Issuer != "go-checker test CA" || resp.TLS.Version == "" {
		t.Fatalf("tls %+v", resp.TLS)
	}
	// A second request opens a new connection: TLS is measured again.
	again, err := p.Do(context.Background(), Request{Method: "GET", URL: srv.URL})
	if err != nil || again.Timings.TLS <= 0 {
		t.Fatalf("second request: %v, %+v", err, again)
	}
}

// 003 AC-3: an expired certificate fails verification...
func TestDoExpiredCertificate(t *testing.T) {
	srv, pool := testcert.Server(t, time.Now().Add(-time.Hour), http.NotFoundHandler())
	p := NewWithTLS("test", &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12})
	if _, err := p.Do(context.Background(), Request{Method: "GET", URL: srv.URL}); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("err = %v", err)
	}
	// ...and with insecure the request goes through, still reporting the date.
	resp, err := New("test").Do(context.Background(), Request{Method: "GET", URL: srv.URL, Insecure: true})
	if err != nil || resp.TLS == nil || resp.TLS.DaysLeft(time.Now()) >= 0 {
		t.Fatalf("insecure: %v, %+v", err, resp)
	}
}

func TestDaysLeft(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
	for after, want := range map[time.Duration]int{
		5*24*time.Hour + time.Hour: 5,
		23 * time.Hour:             0,
		0:                          0,
		-time.Hour:                 -1,
		-48 * time.Hour:            -2,
	} {
		if got := (TLSInfo{NotAfter: now.Add(after)}).DaysLeft(now); got != want {
			t.Errorf("DaysLeft(%v) = %d, want %d", after, got, want)
		}
	}
}

func TestDoPlainHTTPHasNoTLS(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	resp, err := New("test").Do(context.Background(), Request{Method: "GET", URL: srv.URL})
	if err != nil || resp.TLS != nil || resp.Timings.TLS != 0 || resp.Timings.DNS != 0 {
		t.Fatalf("%v %+v", err, resp)
	}
}
