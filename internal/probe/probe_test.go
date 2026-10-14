package probe

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
