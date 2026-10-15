// Package notify sends state changes to webhooks and Slack.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Event is a confirmed state change of one check.
type Event struct {
	Name    string    `json:"name"`
	URL     string    `json:"url"`
	From    string    `json:"from"`
	To      string    `json:"to"`
	Reasons []string  `json:"reasons"`
	At      time.Time `json:"at"`
}

// Notifier delivers events.
type Notifier interface {
	Name() string
	Notify(ctx context.Context, e Event) error
}

var client = &http.Client{Timeout: 10 * time.Second}

func post(ctx context.Context, url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "go-checker")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// Webhook posts the event as JSON.
type Webhook struct{ URL string }

// Name identifies the notifier in logs.
func (w Webhook) Name() string { return "webhook" }

// Notify sends the event.
func (w Webhook) Notify(ctx context.Context, e Event) error { return post(ctx, w.URL, e) }

// Slack posts a message to an incoming webhook.
type Slack struct{ URL string }

// Name identifies the notifier in logs.
func (s Slack) Name() string { return "slack" }

// Notify sends the event as a Slack message.
func (s Slack) Notify(ctx context.Context, e Event) error {
	return post(ctx, s.URL, map[string]string{"text": SlackText(e)})
}

// SlackText formats an event for Slack.
func SlackText(e Event) string {
	icon := map[string]string{"UP": ":large_green_circle:", "WARN": ":large_yellow_circle:", "DOWN": ":red_circle:"}[e.To]
	text := fmt.Sprintf("%s *%s* is %s (was %s) · %s", icon, e.Name, e.To, e.From, e.URL)
	if len(e.Reasons) > 0 {
		text += "\n> " + strings.Join(e.Reasons, "\n> ")
	}
	return text
}
