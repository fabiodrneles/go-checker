package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func capture(t *testing.T, status int) (*httptest.Server, chan []byte) {
	t.Helper()
	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("%s %s", r.Method, r.Header.Get("Content-Type"))
		}
		b, _ := io.ReadAll(r.Body)
		got <- b
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

var ev = Event{Name: "api", URL: "https://api.example.com", From: "UP", To: "DOWN", Reasons: []string{"status 503"}, At: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}

// 007 AC-3
func TestWebhook(t *testing.T) {
	srv, got := capture(t, 204)
	w := Webhook{URL: srv.URL}
	if err := w.Notify(context.Background(), ev); err != nil || w.Name() != "webhook" {
		t.Fatal(err)
	}
	var e map[string]any
	if err := json.Unmarshal(<-got, &e); err != nil {
		t.Fatal(err)
	}
	if e["name"] != "api" || e["from"] != "UP" || e["to"] != "DOWN" || e["reasons"].([]any)[0] != "status 503" || e["at"] != "2026-01-02T03:04:05Z" {
		t.Fatalf("payload %v", e)
	}
}

// 007 AC-4
func TestSlack(t *testing.T) {
	srv, got := capture(t, 200)
	s := Slack{URL: srv.URL}
	if err := s.Notify(context.Background(), ev); err != nil || s.Name() != "slack" {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(<-got, &m); err != nil {
		t.Fatal(err)
	}
	if m["text"] != ":red_circle: *api* is DOWN (was UP) · https://api.example.com\n> status 503" {
		t.Fatalf("text %q", m["text"])
	}
	up := SlackText(Event{Name: "api", To: "UP", From: "DOWN"})
	if !strings.HasPrefix(up, ":large_green_circle:") || strings.Contains(up, ">") {
		t.Fatalf("%q", up)
	}
}

func TestNotifyErrors(t *testing.T) {
	srv, _ := capture(t, 500)
	if err := (Webhook{URL: srv.URL}).Notify(context.Background(), ev); err == nil || err.Error() != "status 500" {
		t.Fatalf("err = %v", err)
	}
	if err := (Webhook{URL: "http://127.0.0.1:1"}).Notify(context.Background(), ev); err == nil {
		t.Fatal("refused connection must fail")
	}
	if err := (Webhook{URL: "::bad"}).Notify(context.Background(), ev); err == nil {
		t.Fatal("bad URL must fail")
	}
	if err := post(context.Background(), srv.URL, func() {}); err == nil {
		t.Fatal("unmarshalable payload must fail")
	}
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// 010 AC-1
func TestDiscord(t *testing.T) {
	srv, got := capture(t, 204)
	d := Discord{URL: srv.URL}
	if err := d.Notify(context.Background(), ev); err != nil || d.Name() != "discord" {
		t.Fatal(err)
	}
	if m := decode(t, <-got); m["content"] != "🔴 **api** is DOWN (was UP) · https://api.example.com\n→ status 503" {
		t.Fatalf("%q", m["content"])
	}
}

// 010 AC-2
func TestTeams(t *testing.T) {
	srv, got := capture(t, 200)
	tm := Teams{URL: srv.URL}
	if err := tm.Notify(context.Background(), ev); err != nil || tm.Name() != "teams" {
		t.Fatal(err)
	}
	m := decode(t, <-got)
	if m["@type"] != "MessageCard" || m["title"] != "api is DOWN (was UP)" || m["themeColor"] != "E01E5A" || m["text"] != "https://api.example.com<br>status 503" {
		t.Fatalf("%v", m)
	}
}

// 010 AC-3
func TestTelegram(t *testing.T) {
	var path string
	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		got <- b
	}))
	defer srv.Close()
	old := TelegramAPI
	TelegramAPI = srv.URL
	t.Cleanup(func() { TelegramAPI = old })
	tg, err := ParseTelegram("123456:ABC-def:-1001234")
	if err != nil || tg.Token != "123456:ABC-def" || tg.ChatID != "-1001234" || tg.Name() != "telegram" {
		t.Fatalf("%v %+v", err, tg)
	}
	if err := tg.Notify(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	m := decode(t, <-got)
	if path != "/bot123456:ABC-def/sendMessage" || m["chat_id"] != "-1001234" || !strings.HasPrefix(m["text"].(string), "🔴 api is DOWN") {
		t.Fatalf("%s %v", path, m)
	}
}

// 010 AC-4
func TestParseTelegramErrors(t *testing.T) {
	for _, s := range []string{"", "token", "123:abc", "123:abc:", ":x"} {
		if _, err := ParseTelegram(s); err == nil {
			t.Errorf("ParseTelegram(%q) = nil", s)
		}
	}
}
