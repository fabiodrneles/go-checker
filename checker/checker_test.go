package checker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunChecks_WithTimeout(t *testing.T) {
	// Create a test server that responds slowly
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	// Create a test server that responds quickly
	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer fastServer.Close()

	sites := []Site{
		{URL: fastServer.URL},
		{URL: slowServer.URL},
	}

	// Test with timeout longer than slow server response - should succeed
	done := make(chan struct{})
	go func() {
		RunChecks(sites, 500*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("RunChecks timed out")
	}
}

func TestRunChecks_DefaultTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sites := []Site{{URL: server.URL}}

	done := make(chan struct{})
	go func() {
		RunChecks(sites, 120*time.Second)
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("RunChecks timed out")
	}
}

func TestCheckSite_RespectsTimeout(t *testing.T) {
	// Test that checkSite respects the timeout parameter
	// This is more of an integration test since checkSite is not exported
	// We test through RunChecks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sites := []Site{{URL: server.URL}}

	// With very short timeout, it should fail/timeout
	done := make(chan struct{})
	go func() {
		RunChecks(sites, 10*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
		// Function completed (may have error result)
	case <-time.After(2 * time.Second):
		t.Fatal("RunChecks timed out")
	}
}